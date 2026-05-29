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
	targetIcon                         = "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZmlsbC1ydWxlPSJldmVub2RkIiBjbGlwLXJ1bGU9ImV2ZW5vZGQiIGQ9Ik0xMS40MTQ0IDE2LjkyMDRWMjAuNjA0N0w3LjgzMjU1IDIyLjA2OTNMNC4yMTMzMiAyMS4yODkyVjE2LjMwOTNMNy44MzI1NSAxNS42ODQ0TDExLjQxNDQgMTYuOTIwNFpNNi4xNTU5NSAxNi42MjhWMjEuMDA5M0w3LjMyODE5IDIxLjIwMDZWMTYuNDExOUw2LjE1NTk1IDE2LjYyOFpNNC43MTc2OCAxNi44NzE5VjIwLjcwMTdMNS43Mzg4OCAyMC45MDY4VjE2LjcwNTZMNC43MTc2OCAxNi44NzE5WiIgZmlsbD0iY3VycmVudENvbG9yIi8+CjxwYXRoIGZpbGwtcnVsZT0iZXZlbm9kZCIgY2xpcC1ydWxlPSJldmVub2RkIiBkPSJNMTkuMjQxNyAxNi45MjA0VjIwLjYwNDdMMTUuNjYxMyAyMi4wNjkzTDEyLjA0MjEgMjEuMjg5MlYxNi4zMDkzTDE1LjY2MTMgMTUuNjg0NEwxOS4yNDE3IDE2LjkyMDRaTTEzLjk4NDcgMTYuNjI4VjIxLjAwOTNMMTUuMTU2OSAyMS4yMDA2VjE2LjQxMTlMMTMuOTg0NyAxNi42MjhaTTEyLjU0NjQgMTYuODcxOVYyMC43MDE3TDEzLjU2NzYgMjAuOTA2OFYxNi43MDU2TDEyLjU0NjQgMTYuODcxOVoiIGZpbGw9ImN1cnJlbnRDb2xvciIvPgo8cGF0aCBmaWxsLXJ1bGU9ImV2ZW5vZGQiIGNsaXAtcnVsZT0iZXZlbm9kZCIgZD0iTTcuNzIzMDkgMTAuMTczOFYxMy44NTk2TDQuMTQyNjUgMTUuMzIyOEwwLjUyMjAzNCAxNC41NDRWOS41NjQxNEw0LjE0MjY1IDguOTM3ODRMNy43MjMwOSAxMC4xNzM4Wk0yLjQ2NDY3IDkuODgyODNWMTQuMjYyOEwzLjYzODI5IDE0LjQ1NFY5LjY2NTI5TDIuNDY0NjcgOS44ODI4M1pNMS4wMjc3OCAxMC4xMjUzVjEzLjk1NjVMMi4wNDg5OCAxNC4xNjE2VjkuOTU5MDRMMS4wMjc3OCAxMC4xMjUzWiIgZmlsbD0iY3VycmVudENvbG9yIi8+CjxwYXRoIGZpbGwtcnVsZT0iZXZlbm9kZCIgY2xpcC1ydWxlPSJldmVub2RkIiBkPSJNMTUuNTIgMTAuMTczOFYxMy44NTk2TDExLjkzOTUgMTUuMzIyOEw4LjMxODkgMTQuNTQ0VjkuNTY0MTRMMTEuOTM5NSA4LjkzNzg0TDE1LjUyIDEwLjE3MzhaTTEwLjI2MTUgOS44ODI4M1YxNC4yNjI4TDExLjQzNTIgMTQuNDU0VjkuNjY1MjlMMTAuMjYxNSA5Ljg4MjgzWk04LjgyMzI3IDEwLjEyNTNWMTMuOTU2NUw5Ljg0NTg1IDE0LjE2MTZWOS45NTkwNEw4LjgyMzI3IDEwLjEyNTNaIiBmaWxsPSJjdXJyZW50Q29sb3IiLz4KPHBhdGggZmlsbC1ydWxlPSJldmVub2RkIiBjbGlwLXJ1bGU9ImV2ZW5vZGQiIGQ9Ik0yMy4zMTY4IDEwLjE3MzhWMTMuODU5NkwxOS43MzY0IDE1LjMyMjhMMTYuMTE1OCAxNC41NDRWOS41NjQxNEwxOS43MzY0IDguOTM3ODRMMjMuMzE2OCAxMC4xNzM4Wk0xOC4wNTg0IDkuODgyODNWMTQuMjYyOEwxOS4yMzA2IDE0LjQ1NFY5LjY2NTI5TDE4LjA1ODQgOS44ODI4M1pNMTYuNjIwMSAxMC4xMjUzVjEzLjk1NjVMMTcuNjQyNyAxNC4xNjE2VjkuOTU5MDRMMTYuNjIwMSAxMC4xMjUzWiIgZmlsbD0iY3VycmVudENvbG9yIi8+CjxwYXRoIGZpbGwtcnVsZT0iZXZlbm9kZCIgY2xpcC1ydWxlPSJldmVub2RkIiBkPSJNMTEuNDE0NCAzLjMwNTMxVjYuOTkxMDVMNy44MzI1NSA4LjQ1NDI2TDQuMjEzMzIgNy42NzQxNlYyLjY5NDI1TDcuODMyNTUgMi4wNjkzNEwxMS40MTQ0IDMuMzA1MzFaTTYuMTU1OTUgMy4wMTQzM1Y3LjM5NDI2TDcuMzI4MTkgNy41ODU0OFYyLjc5Njc5TDYuMTU1OTUgMy4wMTQzM1pNNC43MTc2OCAzLjI1NjgxVjcuMDg4MDRMNS43Mzg4OCA3LjI5MTczVjMuMDkwNTRMNC43MTc2OCAzLjI1NjgxWiIgZmlsbD0iY3VycmVudENvbG9yIi8+CjxwYXRoIGZpbGwtcnVsZT0iZXZlbm9kZCIgY2xpcC1ydWxlPSJldmVub2RkIiBkPSJNMTkuMjIwOSAzLjMwNTMxVjYuOTkxMDVMMTUuNjQwNSA4LjQ1NDI2TDEyLjAxOTkgNy42NzQxNlYyLjY5NDI1TDE1LjY0MDUgMi4wNjkzNEwxOS4yMjA5IDMuMzA1MzFaTTEzLjk2MjUgMy4wMTQzM1Y3LjM5NDI2TDE1LjEzNjEgNy41ODU0OFYyLjc5Njc5TDEzLjk2MjUgMy4wMTQzM1pNMTIuNTI0MyAzLjI1NjgxVjcuMDg4MDRMMTMuNTQ2OCA3LjI5MTczVjMuMDkwNTRMMTIuNTI0MyAzLjI1NjgxWiIgZmlsbD0iY3VycmVudENvbG9yIi8+Cjwvc3ZnPgo="
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
