/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extaks

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type NodePoolTerminateInstancesState struct {
	SubscriptionId    string
	ResourceGroupName string
	ClusterName       string
	NodePoolName      string
	Percentage        int
	MachineNames      []string
}

type nodePoolTerminateInstancesAttack struct {
	machinesProvider   func(subscriptionId string) (MachinesApi, error)
	agentPoolsProvider func(subscriptionId string) (AgentPoolsApi, error)
	rng                func(n int) []int
}

var _ action_kit_sdk.Action[NodePoolTerminateInstancesState] = (*nodePoolTerminateInstancesAttack)(nil)

func NewNodePoolTerminateInstancesAction() action_kit_sdk.Action[NodePoolTerminateInstancesState] {
	return &nodePoolTerminateInstancesAttack{
		machinesProvider: func(subscriptionId string) (MachinesApi, error) {
			return newMachinesClient(subscriptionId)
		},
		agentPoolsProvider: func(subscriptionId string) (AgentPoolsApi, error) {
			return newAgentPoolsClient(subscriptionId)
		},
		rng: rand.Perm,
	}
}

func (a *nodePoolTerminateInstancesAttack) NewEmptyState() NodePoolTerminateInstancesState {
	return NodePoolTerminateInstancesState{}
}

func (a *nodePoolTerminateInstancesAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:    NodePoolTerminateInstancesActionId,
		Label: "Trigger Terminate AKS Instances",
		Description: "Deletes a percentage of nodes from an AKS managed node pool via the AKS API. " +
			"With cluster-autoscaler enabled, AKS replaces the deleted nodes within minutes; without autoscaling, the pool shrinks until manually scaled back. " +
			"Validates pod rescheduling, PDB enforcement, and cluster-autoscaler scale-up timing.",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    new(nodePoolIcon),
		TargetSelection: new(action_kit_api.TargetSelection{
			TargetType: TargetIDNodePool,
			SelectionTemplates: new([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by cluster and node pool name",
					Description: new("Find AKS node pool by cluster name and node pool name"),
					Query:       "azure.aks.cluster.name=\"\" and azure.aks.nodepool.name=\"\"",
				},
			}),
		}),
		Technology:  new("Azure"),
		Category:    new("AKS"),
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "percentage",
				Label:        "Percentage of nodes to terminate",
				Description:  new("Percentage (1-100) of the node pool's nodes to terminate. Defaults to 33%."),
				Type:         action_kit_api.ActionParameterTypeInteger,
				DefaultValue: new("33"),
				Order:        new(1),
				Required:     new(true),
				MinValue:     new(1),
				MaxValue:     new(100),
			},
		},
	}
}

func (a *nodePoolTerminateInstancesAttack) Prepare(ctx context.Context, state *NodePoolTerminateInstancesState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.SubscriptionId = mustHave(request.Target.Attributes, "azure.subscription.id")
	state.ResourceGroupName = mustHave(request.Target.Attributes, "azure.resource-group.name")
	state.ClusterName = mustHave(request.Target.Attributes, "azure.aks.cluster.name")
	state.NodePoolName = mustHave(request.Target.Attributes, "azure.aks.nodepool.name")
	if state.SubscriptionId == "" || state.ResourceGroupName == "" || state.ClusterName == "" || state.NodePoolName == "" {
		return nil, extension_kit.ToError("Target is missing one of: azure.subscription.id, azure.resource-group.name, azure.aks.cluster.name, azure.aks.nodepool.name", nil)
	}

	pct := extutil.ToInt(request.Config["percentage"])
	if pct < 1 || pct > 100 {
		return nil, extension_kit.ToError("percentage must be between 1 and 100.", nil)
	}
	state.Percentage = pct

	machinesClient, err := a.machinesProvider(state.SubscriptionId)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize AKS machines client for subscription %s", state.SubscriptionId), err)
	}

	pager := machinesClient.NewListPager(state.ResourceGroupName, state.ClusterName, state.NodePoolName, nil)
	allNames := make([]string, 0)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to list machines for AKS node pool %s/%s", state.ClusterName, state.NodePoolName), err)
		}
		for _, m := range page.Value {
			if m == nil || m.Name == nil {
				continue
			}
			allNames = append(allNames, *m.Name)
		}
	}
	sort.Strings(allNames)

	if len(allNames) == 0 {
		return nil, extension_kit.ToError(fmt.Sprintf("AKS node pool %s/%s has no machines to terminate", state.ClusterName, state.NodePoolName), nil)
	}

	sampleSize := max(int(math.Ceil(float64(len(allNames))*float64(pct)/100.0)), 1)
	if sampleSize > len(allNames) {
		sampleSize = len(allNames)
	}
	// AKS rejects deleting every node in a system pool (control-plane needs at least one survivor).
	// Catch it here with an actionable error instead of letting the deleteMachines call fail mid-experiment.
	if sampleSize >= len(allNames) {
		mode := mustHave(request.Target.Attributes, "azure.aks.nodepool.mode")
		if mode == "System" {
			return nil, extension_kit.ToError(fmt.Sprintf(
				"Cannot terminate all %d node(s) of system node pool %s/%s: AKS requires at least one surviving system node. Reduce the percentage, scale up the pool, or target a user node pool instead.",
				len(allNames), state.ClusterName, state.NodePoolName), nil)
		}
	}

	perm := a.rng(len(allNames))
	state.MachineNames = make([]string, 0, sampleSize)
	for i := 0; i < sampleSize; i++ {
		state.MachineNames = append(state.MachineNames, allNames[perm[i]])
	}
	sort.Strings(state.MachineNames)

	return &action_kit_api.PrepareResult{
		Messages: new([]action_kit_api.Message{{
			Level: extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Selected %d of %d machine(s) (%d%%) in AKS node pool %s/%s for termination: %v",
				sampleSize, len(allNames), pct, state.ClusterName, state.NodePoolName, state.MachineNames),
		}}),
	}, nil
}

func (a *nodePoolTerminateInstancesAttack) Start(ctx context.Context, state *NodePoolTerminateInstancesState) (*action_kit_api.StartResult, error) {
	if len(state.MachineNames) == 0 {
		return nil, extension_kit.ToError("No machines selected for termination.", nil)
	}
	client, err := a.agentPoolsProvider(state.SubscriptionId)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize AKS agent pools client for subscription %s", state.SubscriptionId), err)
	}
	names := make([]*string, 0, len(state.MachineNames))
	for i := range state.MachineNames {
		n := state.MachineNames[i]
		names = append(names, &n)
	}
	_, err = client.BeginDeleteMachines(ctx, state.ResourceGroupName, state.ClusterName, state.NodePoolName, armcontainerservice.AgentPoolDeleteMachinesParameter{MachineNames: names}, nil)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to delete machines in AKS node pool %s/%s", state.ClusterName, state.NodePoolName), err)
	}
	return &action_kit_api.StartResult{
		Messages: new([]action_kit_api.Message{{
			Level: extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Deletion requested for %d machine(s) in AKS node pool %s/%s: %v. AKS will replace them via the underlying VMSS.",
				len(state.MachineNames), state.ClusterName, state.NodePoolName, state.MachineNames),
		}}),
	}, nil
}

func mustHave(attrs map[string][]string, key string) string {
	v, ok := attrs[key]
	if !ok || len(v) == 0 {
		return ""
	}
	return v[0]
}
