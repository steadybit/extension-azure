/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extaks

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-azure/common"
)

const (
	TargetIDCluster                    = "com.steadybit.extension_azure.aks.cluster"
	TargetIDNodePool                   = "com.steadybit.extension_azure.aks.nodepool"
	NodePoolTerminateInstancesActionId = "com.steadybit.extension_azure.aks.nodepool.terminate-instances"
	targetIcon                         = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNMTIgMmwxMCA1djEwbC0xMCA1LTEwLTVWN2wxMC01eiIgZmlsbD0iY3VycmVudENvbG9yIi8+PC9zdmc+"
)

// MachinesApi captures the subset of armcontainerservice.MachinesClient used here.
type MachinesApi interface {
	NewListPager(resourceGroupName string, resourceName string, agentPoolName string, options *armcontainerservice.MachinesClientListOptions) *runtime.Pager[armcontainerservice.MachinesClientListResponse]
}

// AgentPoolsApi captures the subset of armcontainerservice.AgentPoolsClient used here.
type AgentPoolsApi interface {
	BeginDeleteMachines(ctx context.Context, resourceGroupName string, resourceName string, agentPoolName string, machines armcontainerservice.AgentPoolDeleteMachinesParameter, options *armcontainerservice.AgentPoolsClientBeginDeleteMachinesOptions) (*runtime.Poller[armcontainerservice.AgentPoolsClientDeleteMachinesResponse], error)
}

func newMachinesClient(subscriptionId string) (*armcontainerservice.MachinesClient, error) {
	cred, err := common.ConnectionAzure()
	if err != nil {
		log.Error().Err(err).Msgf("Failed to create Azure connection.")
		return nil, err
	}
	factory, err := armcontainerservice.NewClientFactory(subscriptionId, cred, nil)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to create Azure container service client factory.")
		return nil, err
	}
	return factory.NewMachinesClient(), nil
}

func newAgentPoolsClient(subscriptionId string) (*armcontainerservice.AgentPoolsClient, error) {
	cred, err := common.ConnectionAzure()
	if err != nil {
		log.Error().Err(err).Msgf("Failed to create Azure connection.")
		return nil, err
	}
	factory, err := armcontainerservice.NewClientFactory(subscriptionId, cred, nil)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to create Azure container service client factory.")
		return nil, err
	}
	return factory.NewAgentPoolsClient(), nil
}
