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
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-azure/common"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

const CosmosDbFailoverActionId = "com.steadybit.extension_azure.cosmosdb.account.failover"

// CosmosDbFailoverState carries the computed new failover priorities from Prepare to Start. This
// attack is one-shot (no Stop / no rollback): Azure's FailoverPriorityChange is async and takes
// 5-15 min per call, so a transient experiment-style "set & restore" semantics is misleading. Users
// who want to switch back can run the experiment again against the (now-secondary) region.
type CosmosDbFailoverState struct {
	SubscriptionId    string
	ResourceGroupName string
	AccountName       string
	// PromotedRegion is the region we will promote to priority 0. Captured for log messaging.
	PromotedRegion string
	// NewPolicies is the full priority order to submit at Start (computed from the current order
	// at Prepare, with PromotedRegion swapped to priority 0).
	NewPolicies []failoverPolicy
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

func NewCosmosDbFailoverAction() action_kit_sdk.Action[CosmosDbFailoverState] {
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
		Label: "Trigger Cosmos DB Failover",
		Description: "Promotes the secondary region to write for a multi-region Cosmos DB account, simulating a regional failover. " +
			"Validates that your application correctly follows the SDK's automatic write-region tracking and that retry/backoff logic survives the promotion.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(targetIcon),
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
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters:  []action_kit_api.ActionParameter{},
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
	current := make([]failoverPolicy, 0, len(got.Properties.FailoverPolicies))
	for _, p := range got.Properties.FailoverPolicies {
		if p == nil || p.LocationName == nil || p.FailoverPriority == nil {
			continue
		}
		current = append(current, failoverPolicy{
			LocationName: *p.LocationName,
			Priority:     *p.FailoverPriority,
		})
	}
	if len(current) < 2 {
		return nil, extension_kit.ToError(fmt.Sprintf("Cosmos DB account %s has fewer than 2 valid failover policies", state.AccountName), nil)
	}
	// Promote the current secondary with the lowest priority > 0 — the same region the SDK would
	// automatically promote on a regional outage. Most realistic chaos signal.
	state.PromotedRegion = secondaryWithLowestPriority(current)
	state.NewPolicies = promotePolicies(current, state.PromotedRegion)
	return &action_kit_api.PrepareResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Cosmos DB account %s has %d region(s); will promote %q to write region. No automatic rollback.", state.AccountName, len(current), state.PromotedRegion),
		}}),
	}, nil
}

func (a *cosmosFailoverAttack) Start(ctx context.Context, state *CosmosDbFailoverState) (*action_kit_api.StartResult, error) {
	client, err := a.clientProvider(state.SubscriptionId)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize Cosmos DB client for subscription %s", state.SubscriptionId), err)
	}
	_, err = client.BeginFailoverPriorityChange(ctx, state.ResourceGroupName, state.AccountName, armcosmos.FailoverPolicies{
		FailoverPolicies: toCosmosPolicies(state.NewPolicies),
	}, nil)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to trigger failover for Cosmos DB account %s", state.AccountName), err)
	}
	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Triggered failover for Cosmos DB account %s; promoted region %q to write region. Azure-side completion takes 5-15 min.", state.AccountName, state.PromotedRegion),
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
