/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extnatgateway

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-azure/common"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

const NatGatewayDisassociateActionId = "com.steadybit.extension_azure.nat-gateway.disassociate-subnets"

// NatGatewayDisassociateState holds enough information to restore each subnet's NAT Gateway association.
type NatGatewayDisassociateState struct {
	SubscriptionId    string
	ResourceGroupName string
	NatGatewayId      string
	NatGatewayName    string
	// SubnetRefs are the full ARM IDs of subnets currently associated with the NAT gateway at prepare time.
	// We re-fetch each subnet at stop time and restore only the NatGateway field, so concurrent edits to other
	// subnet fields by other operators are preserved.
	SubnetRefs []string
}

type subnetsApi interface {
	Get(ctx context.Context, resourceGroupName string, virtualNetworkName string, subnetName string, options *armnetwork.SubnetsClientGetOptions) (armnetwork.SubnetsClientGetResponse, error)
	BeginCreateOrUpdate(ctx context.Context, resourceGroupName string, virtualNetworkName string, subnetName string, subnetParameters armnetwork.Subnet, options *armnetwork.SubnetsClientBeginCreateOrUpdateOptions) (*runtime.Poller[armnetwork.SubnetsClientCreateOrUpdateResponse], error)
}

type natGatewayDisassociateAttack struct {
	subnetsClientProvider func(subscriptionId string) (subnetsApi, error)
}

var _ action_kit_sdk.Action[NatGatewayDisassociateState] = (*natGatewayDisassociateAttack)(nil)
var _ action_kit_sdk.ActionWithStop[NatGatewayDisassociateState] = (*natGatewayDisassociateAttack)(nil)

func NewNatGatewayDisassociateAction() action_kit_sdk.ActionWithStop[NatGatewayDisassociateState] {
	return &natGatewayDisassociateAttack{
		subnetsClientProvider: func(subscriptionId string) (subnetsApi, error) {
			cred, err := common.ConnectionAzure()
			if err != nil {
				return nil, err
			}
			return armnetwork.NewSubnetsClient(subscriptionId, cred, nil)
		},
	}
}

func (a *natGatewayDisassociateAttack) NewEmptyState() NatGatewayDisassociateState {
	return NatGatewayDisassociateState{}
}

func (a *natGatewayDisassociateAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:    NatGatewayDisassociateActionId,
		Label: "Disassociate NAT Gateway from its subnets",
		Description: "Disassociates the NAT Gateway from all of its currently associated subnets to simulate an outbound-internet outage for the workloads in those subnets. " +
			"Each subnet's NAT Gateway reference is restored on stop. Only the NatGateway field is modified — concurrent edits to other subnet properties are preserved.",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    extutil.Ptr(targetIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: TargetIDNatGateway,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by NAT Gateway name",
					Description: extutil.Ptr("Find NAT Gateway by name"),
					Query:       "azure.nat-gateway.name=\"\"",
				},
			}),
		}),
		Technology:  extutil.Ptr("Azure"),
		Category:    extutil.Ptr("NAT Gateway"),
		TimeControl: action_kit_api.TimeControlExternal,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr("How long subnets remain disassociated. The associations are restored on stop."),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: extutil.Ptr("60s"),
				Order:        extutil.Ptr(1),
				Required:     extutil.Ptr(true),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func (a *natGatewayDisassociateAttack) Prepare(_ context.Context, state *NatGatewayDisassociateState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.SubscriptionId = mustHave(request.Target.Attributes, "azure.subscription.id")
	state.ResourceGroupName = mustHave(request.Target.Attributes, "azure.resource-group.name")
	state.NatGatewayName = mustHave(request.Target.Attributes, "azure.nat-gateway.name")
	if state.SubscriptionId == "" || state.ResourceGroupName == "" || state.NatGatewayName == "" {
		return nil, extension_kit.ToError("Target is missing one of: azure.subscription.id, azure.resource-group.name, azure.nat-gateway.name", nil)
	}
	state.NatGatewayId = fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/natGateways/%s",
		state.SubscriptionId, state.ResourceGroupName, state.NatGatewayName)

	subnets := request.Target.Attributes["azure.nat-gateway.subnets"]
	if len(subnets) == 0 {
		return nil, extension_kit.ToError(fmt.Sprintf("NAT Gateway %s currently has no associated subnets", state.NatGatewayName), nil)
	}
	state.SubnetRefs = append(state.SubnetRefs, subnets...)
	return &action_kit_api.PrepareResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Will disassociate %d subnet(s) from NAT Gateway %s", len(subnets), state.NatGatewayName),
		}}),
	}, nil
}

func (a *natGatewayDisassociateAttack) Start(ctx context.Context, state *NatGatewayDisassociateState) (*action_kit_api.StartResult, error) {
	client, err := a.subnetsClientProvider(state.SubscriptionId)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize subnets client for subscription %s", state.SubscriptionId), err)
	}
	disassociated := make([]string, 0, len(state.SubnetRefs))
	for _, ref := range state.SubnetRefs {
		rg, vnet, subnet, ok := parseSubnetID(ref)
		if !ok {
			log.Warn().Msgf("Skipping subnet ref with unrecognized format: %s", ref)
			continue
		}
		if err := updateSubnetNatGateway(ctx, client, rg, vnet, subnet, nil); err != nil {
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to disassociate NAT Gateway from subnet %s/%s in resource group %s", vnet, subnet, rg), err)
		}
		disassociated = append(disassociated, fmt.Sprintf("%s/%s", vnet, subnet))
	}
	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Disassociated NAT Gateway %s from %d subnet(s): %v", state.NatGatewayName, len(disassociated), disassociated),
		}}),
	}, nil
}

func (a *natGatewayDisassociateAttack) Stop(ctx context.Context, state *NatGatewayDisassociateState) (*action_kit_api.StopResult, error) {
	client, err := a.subnetsClientProvider(state.SubscriptionId)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize subnets client for subscription %s", state.SubscriptionId), err)
	}
	restored := make([]string, 0, len(state.SubnetRefs))
	for _, ref := range state.SubnetRefs {
		rg, vnet, subnet, ok := parseSubnetID(ref)
		if !ok {
			continue
		}
		ngRef := &armnetwork.SubResource{ID: extutil.Ptr(state.NatGatewayId)}
		if err := updateSubnetNatGateway(ctx, client, rg, vnet, subnet, ngRef); err != nil {
			log.Error().Err(err).Msgf("Failed to re-associate NAT Gateway %s with subnet %s/%s", state.NatGatewayName, vnet, subnet)
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to re-associate NAT Gateway %s with subnet %s/%s", state.NatGatewayName, vnet, subnet), err)
		}
		restored = append(restored, fmt.Sprintf("%s/%s", vnet, subnet))
	}
	return &action_kit_api.StopResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Re-associated NAT Gateway %s with %d subnet(s): %v", state.NatGatewayName, len(restored), restored),
		}}),
	}, nil
}

// updateSubnetNatGateway re-fetches the subnet and PUTs it back with only the NatGateway reference modified.
// This preserves any concurrent edits to other subnet properties (NSG, address prefixes, route tables, etc.).
func updateSubnetNatGateway(ctx context.Context, client subnetsApi, rg, vnet, subnet string, natGateway *armnetwork.SubResource) error {
	current, err := client.Get(ctx, rg, vnet, subnet, nil)
	if err != nil {
		return fmt.Errorf("failed to get subnet %s/%s in %s: %w", vnet, subnet, rg, err)
	}
	if current.Subnet.Properties == nil {
		current.Subnet.Properties = &armnetwork.SubnetPropertiesFormat{}
	}
	current.Subnet.Properties.NatGateway = natGateway
	_, err = client.BeginCreateOrUpdate(ctx, rg, vnet, subnet, current.Subnet, nil)
	if err != nil {
		return fmt.Errorf("failed to update subnet %s/%s in %s: %w", vnet, subnet, rg, err)
	}
	return nil
}

// parseSubnetID extracts (resourceGroup, virtualNetworkName, subnetName) from an ARM subnet resource ID like
// /subscriptions/<sub>/resourceGroups/<rg>/providers/Microsoft.Network/virtualNetworks/<vnet>/subnets/<subnet>
func parseSubnetID(id string) (rg, vnet, subnet string, ok bool) {
	parts := strings.Split(strings.TrimPrefix(id, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		switch strings.ToLower(parts[i]) {
		case "resourcegroups":
			rg = parts[i+1]
		case "virtualnetworks":
			vnet = parts[i+1]
		case "subnets":
			subnet = parts[i+1]
		}
	}
	if rg == "" || vnet == "" || subnet == "" {
		return "", "", "", false
	}
	return rg, vnet, subnet, true
}

func mustHave(attrs map[string][]string, key string) string {
	v, ok := attrs[key]
	if !ok || len(v) == 0 {
		return ""
	}
	return v[0]
}
