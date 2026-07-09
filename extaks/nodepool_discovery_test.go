/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extaks

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNodePoolDescribe(t *testing.T) {
	assert.Equal(t, TargetIDNodePool, (&nodePoolDiscovery{}).Describe().Id)
}

func TestNodePoolDescribeTarget(t *testing.T) {
	td := (&nodePoolDiscovery{}).DescribeTarget()
	assert.Equal(t, TargetIDNodePool, td.Id)
	assert.Contains(t, td.Label.One, "node pool")
}

func TestNodePoolDescribeAttributes(t *testing.T) {
	require.NotEmpty(t, (&nodePoolDiscovery{}).DescribeAttributes())
}

func TestNewNodePoolDiscovery(t *testing.T) {
	require.NotNil(t, NewNodePoolDiscovery())
}

func TestNodePoolTargetFromSDK_HappyPath(t *testing.T) {
	mode := armcontainerservice.AgentPoolModeSystem
	prio := armcontainerservice.ScaleSetPriorityRegular
	osType := armcontainerservice.OSTypeLinux
	osSku := armcontainerservice.OSSKUUbuntu

	p := &armcontainerservice.AgentPool{
		ID:   new("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.ContainerService/managedClusters/aks-c/agentPools/np1"),
		Name: new("np1"),
		Properties: &armcontainerservice.ManagedClusterAgentPoolProfileProperties{
			Mode:                &mode,
			ScaleSetPriority:    &prio,
			OSType:              &osType,
			OSSKU:               &osSku,
			VMSize:              new("Standard_B2s"),
			Count:               to.Ptr[int32](3),
			MinCount:            to.Ptr[int32](1),
			MaxCount:            to.Ptr[int32](5),
			EnableAutoScaling:   new(true),
			OrchestratorVersion: new("1.29.0"),
			ProvisioningState:   new("Succeeded"),
			PowerState:          &armcontainerservice.PowerState{Code: to.Ptr(armcontainerservice.CodeRunning)},
		},
	}

	c := aksClusterRef{
		name:           "aks-c",
		resourceGroup:  "rg-1",
		subscriptionId: "sub-1",
		location:       "westeurope",
	}

	got := nodePoolTargetFromSDK(p, c)
	assert.Equal(t, "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.ContainerService/managedClusters/aks-c/agentPools/np1", got.Id)
	assert.Equal(t, TargetIDNodePool, got.TargetType)
	assert.Equal(t, []string{"aks-c"}, got.Attributes["azure.aks.cluster.name"])
	assert.Equal(t, []string{"np1"}, got.Attributes["azure.aks.nodepool.name"])
	assert.Equal(t, []string{"System"}, got.Attributes["azure.aks.nodepool.mode"])
	assert.Equal(t, []string{"Standard_B2s"}, got.Attributes["azure.aks.nodepool.vm-size"])
	assert.Equal(t, []string{"1"}, got.Attributes["azure.aks.nodepool.min-count"])
	assert.Equal(t, []string{"5"}, got.Attributes["azure.aks.nodepool.max-count"])
}

func TestNodePoolTargetFromSDK_NilProperties(t *testing.T) {
	p := &armcontainerservice.AgentPool{
		ID:   new("/sub/s/rg/r/Microsoft.ContainerService/managedClusters/c/agentPools/np2"),
		Name: new("np2"),
	}
	c := aksClusterRef{name: "c", resourceGroup: "r", subscriptionId: "s", location: "westeurope"}
	got := nodePoolTargetFromSDK(p, c)
	assert.Equal(t, "c/np2", got.Label)
	assert.Equal(t, []string{"np2"}, got.Attributes["azure.aks.nodepool.name"])
}

func TestGetAllAksNodePools_HappyPath(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponse(map[string]any{"name": "aks-c", "resourceGroup": "rg-1", "subscriptionId": "sub-1", "location": "westeurope"}), nil)

	lister := func(ctx context.Context, sub, rg, cluster string) ([]*armcontainerservice.AgentPool, error) {
		assert.Equal(t, "sub-1", sub)
		assert.Equal(t, "rg-1", rg)
		assert.Equal(t, "aks-c", cluster)
		return []*armcontainerservice.AgentPool{
			{ID: new("/sub/sub-1/rg/rg-1/c/aks-c/np/np1"), Name: new("np1"), Properties: &armcontainerservice.ManagedClusterAgentPoolProfileProperties{}},
		}, nil
	}

	targets, err := getAllAksNodePools(context.Background(), rg, lister)
	require.NoError(t, err)
	require.Len(t, targets, 1)
}

func TestGetAllAksNodePools_PerClusterListerErrorIsTolerated(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponse(
			map[string]any{"name": "c1", "resourceGroup": "r1", "subscriptionId": "s1", "location": "westeurope"},
			map[string]any{"name": "c2", "resourceGroup": "r2", "subscriptionId": "s2", "location": "westeurope"},
		), nil)

	lister := func(ctx context.Context, _, _, cluster string) ([]*armcontainerservice.AgentPool, error) {
		if cluster == "c1" {
			return nil, errors.New("transient")
		}
		return []*armcontainerservice.AgentPool{
			{ID: new("/.../c2/np/ok"), Name: new("ok"), Properties: &armcontainerservice.ManagedClusterAgentPoolProfileProperties{}},
		}, nil
	}

	targets, err := getAllAksNodePools(context.Background(), rg, lister)
	require.NoError(t, err)
	require.Len(t, targets, 1, "c2 pool survives even though c1 failed")
}

func TestGetAllAksNodePools_RGError(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))

	_, err := getAllAksNodePools(context.Background(), rg, func(context.Context, string, string, string) ([]*armcontainerservice.AgentPool, error) {
		return nil, nil
	})
	require.Error(t, err)
}

func TestListAksClusterRefs_ParsesRows(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponse(
			map[string]any{"name": "c1", "resourceGroup": "r1", "subscriptionId": "s1", "location": "westeurope"},
		), nil)

	refs, err := listAksClusterRefs(context.Background(), rg)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, "c1", refs[0].name)
	assert.Equal(t, "s1", refs[0].subscriptionId)
}
