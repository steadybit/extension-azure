/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extcosmosdb

import (
	"context"
	"fmt"
	"sort"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cosmos/armcosmos/v3"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-azure/common"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

const CosmosDbFailoverActionId = "com.steadybit.extension_azure.cosmosdb.account.failover"

// CosmosDbFailoverState captures the original failover priorities so we can restore them on stop.
type CosmosDbFailoverState struct {
	SubscriptionId    string
	ResourceGroupName string
	AccountName       string
	// OriginalPolicies stores the (locationName, priority) pairs as observed at prepare.
	OriginalPolicies []failoverPolicy
	// PromotedRegion is the region we promoted to priority 0 at start. Captured for messaging only.
	PromotedRegion string
}

type failoverPolicy struct {
	LocationName string
	Priority     int32
}

type cosmosDatabaseAccountsApi interface {
	Get(ctx context.Context, resourceGroupName string, accountName string, options *armcosmos.DatabaseAccountsClientGetOptions) (armcosmos.DatabaseAccountsClientGetResponse, error)
	BeginFailoverPriorityChange(ctx context.Context, resourceGroupName string, accountName string, failoverParameters armcosmos.FailoverPolicies, options *armcosmos.DatabaseAccountsClientBeginFailoverPriorityChangeOptions) (*runtime.Poller[armcosmos.DatabaseAccountsClientFailoverPriorityChangeResponse], error)
}

type cosmosFailoverAttack struct {
	clientProvider func(subscriptionId string) (cosmosDatabaseAccountsApi, error)
}

var _ action_kit_sdk.Action[CosmosDbFailoverState] = (*cosmosFailoverAttack)(nil)
var _ action_kit_sdk.ActionWithStop[CosmosDbFailoverState] = (*cosmosFailoverAttack)(nil)

func NewCosmosDbFailoverAction() action_kit_sdk.ActionWithStop[CosmosDbFailoverState] {
	return &cosmosFailoverAttack{
		clientProvider: func(subscriptionId string) (cosmosDatabaseAccountsApi, error) {
			cred, err := common.ConnectionAzure()
			if err != nil {
				return nil, err
			}
			factory, err := armcosmos.NewClientFactory(subscriptionId, cred, nil)
			if err != nil {
				return nil, err
			}
			return factory.NewDatabaseAccountsClient(), nil
		},
	}
}

func (a *cosmosFailoverAttack) NewEmptyState() CosmosDbFailoverState {
	return CosmosDbFailoverState{}
}

func (a *cosmosFailoverAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:    CosmosDbFailoverActionId,
		Label: "Trigger Cosmos DB regional failover",
		Description: "Promotes the lowest-priority failover region to priority 0 (write region) for a Cosmos DB account, swapping the previous primary down. " +
			"Validates that your application correctly follows the SDK's automatic write-region tracking and that retry / backoff logic survives the promotion. " +
			"The original failover priority order is restored on stop. Multi-region account required.",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    extutil.Ptr(targetIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: TargetIDCosmosDbAccount,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by Cosmos DB account name",
					Description: extutil.Ptr("Find Cosmos DB account by name"),
					Query:       "azure.cosmosdb.account.name=\"\"",
				},
			}),
		}),
		Technology:  extutil.Ptr("Azure"),
		Category:    extutil.Ptr("Cosmos DB"),
		TimeControl: action_kit_api.TimeControlExternal,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr("How long the new failover priority order is in effect. The original priority order is restored on stop."),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: extutil.Ptr("60s"),
				Order:        extutil.Ptr(1),
				Required:     extutil.Ptr(true),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func (a *cosmosFailoverAttack) Prepare(ctx context.Context, state *CosmosDbFailoverState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.SubscriptionId = mustHave(request.Target.Attributes, "azure.subscription.id")
	state.ResourceGroupName = mustHave(request.Target.Attributes, "azure.resource-group.name")
	state.AccountName = mustHave(request.Target.Attributes, "azure.cosmosdb.account.name")
	if state.SubscriptionId == "" || state.ResourceGroupName == "" || state.AccountName == "" {
		return nil, extension_kit.ToError("Target is missing one of: azure.subscription.id, azure.resource-group.name, azure.cosmosdb.account.name", nil)
	}

	client, err := a.clientProvider(state.SubscriptionId)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize Cosmos DB client for subscription %s", state.SubscriptionId), err)
	}
	got, err := client.Get(ctx, state.ResourceGroupName, state.AccountName, nil)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to describe Cosmos DB account %s", state.AccountName), err)
	}
	if got.Properties == nil || len(got.Properties.FailoverPolicies) < 2 {
		return nil, extension_kit.ToError(fmt.Sprintf("Cosmos DB account %s has fewer than 2 regions; failover requires a multi-region account", state.AccountName), nil)
	}
	state.OriginalPolicies = make([]failoverPolicy, 0, len(got.Properties.FailoverPolicies))
	for _, p := range got.Properties.FailoverPolicies {
		if p == nil || p.LocationName == nil || p.FailoverPriority == nil {
			continue
		}
		state.OriginalPolicies = append(state.OriginalPolicies, failoverPolicy{
			LocationName: *p.LocationName,
			Priority:     *p.FailoverPriority,
		})
	}
	if len(state.OriginalPolicies) < 2 {
		return nil, extension_kit.ToError(fmt.Sprintf("Cosmos DB account %s has fewer than 2 valid failover policies", state.AccountName), nil)
	}
	// Determine which region we will promote: the current secondary with the lowest priority > 0.
	state.PromotedRegion = secondaryWithLowestPriority(state.OriginalPolicies)
	return &action_kit_api.PrepareResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Cosmos DB account %s has %d region(s); will promote %q to write region.", state.AccountName, len(state.OriginalPolicies), state.PromotedRegion),
		}}),
	}, nil
}

func (a *cosmosFailoverAttack) Start(ctx context.Context, state *CosmosDbFailoverState) (*action_kit_api.StartResult, error) {
	client, err := a.clientProvider(state.SubscriptionId)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize Cosmos DB client for subscription %s", state.SubscriptionId), err)
	}
	policies := promotePolicies(state.OriginalPolicies, state.PromotedRegion)
	_, err = client.BeginFailoverPriorityChange(ctx, state.ResourceGroupName, state.AccountName, armcosmos.FailoverPolicies{
		FailoverPolicies: toCosmosPolicies(policies),
	}, nil)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to trigger failover for Cosmos DB account %s", state.AccountName), err)
	}
	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Promoted region %q to write region for Cosmos DB account %s", state.PromotedRegion, state.AccountName),
		}}),
	}, nil
}

func (a *cosmosFailoverAttack) Stop(ctx context.Context, state *CosmosDbFailoverState) (*action_kit_api.StopResult, error) {
	client, err := a.clientProvider(state.SubscriptionId)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize Cosmos DB client for subscription %s", state.SubscriptionId), err)
	}
	_, err = client.BeginFailoverPriorityChange(ctx, state.ResourceGroupName, state.AccountName, armcosmos.FailoverPolicies{
		FailoverPolicies: toCosmosPolicies(state.OriginalPolicies),
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to restore failover priorities on Cosmos DB account %s", state.AccountName)
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to restore failover priorities on Cosmos DB account %s", state.AccountName), err)
	}
	return &action_kit_api.StopResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Restored original failover priorities on Cosmos DB account %s", state.AccountName),
		}}),
	}, nil
}

// secondaryWithLowestPriority returns the locationName of the region with the lowest non-zero failover priority,
// i.e. the next region the SDK would promote on automatic failover. Promoting that region is the most realistic
// chaos test of "what happens when our primary fails over to its closest secondary."
func secondaryWithLowestPriority(policies []failoverPolicy) string {
	var best *failoverPolicy
	for i := range policies {
		p := policies[i]
		if p.Priority == 0 {
			continue
		}
		if best == nil || p.Priority < best.Priority {
			best = &p
		}
	}
	if best == nil {
		return ""
	}
	return best.LocationName
}

// promotePolicies returns a new policy slice where promotedRegion has priority 0 and the previous primary
// drops to whatever priority was vacated. Other regions keep their original priorities.
func promotePolicies(original []failoverPolicy, promotedRegion string) []failoverPolicy {
	if promotedRegion == "" {
		return original
	}
	// Find current primary and the original priority of the promoted region.
	var currentPrimary string
	var promotedOldPriority int32
	for _, p := range original {
		if p.Priority == 0 {
			currentPrimary = p.LocationName
		}
		if p.LocationName == promotedRegion {
			promotedOldPriority = p.Priority
		}
	}
	out := make([]failoverPolicy, 0, len(original))
	for _, p := range original {
		switch p.LocationName {
		case promotedRegion:
			out = append(out, failoverPolicy{LocationName: p.LocationName, Priority: 0})
		case currentPrimary:
			out = append(out, failoverPolicy{LocationName: p.LocationName, Priority: promotedOldPriority})
		default:
			out = append(out, p)
		}
	}
	// Sort by priority for stable, deterministic output (the API doesn't require it but it makes diffs readable).
	sort.SliceStable(out, func(i, j int) bool { return out[i].Priority < out[j].Priority })
	return out
}

func toCosmosPolicies(in []failoverPolicy) []*armcosmos.FailoverPolicy {
	out := make([]*armcosmos.FailoverPolicy, 0, len(in))
	for i := range in {
		p := in[i]
		out = append(out, &armcosmos.FailoverPolicy{
			LocationName:     extutil.Ptr(p.LocationName),
			FailoverPriority: extutil.Ptr(p.Priority),
		})
	}
	return out
}

func mustHave(attrs map[string][]string, key string) string {
	v, ok := attrs[key]
	if !ok || len(v) == 0 {
		return ""
	}
	return v[0]
}
