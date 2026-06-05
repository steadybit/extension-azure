/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extvmss

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type rgClientMock struct{ mock.Mock }

func (m *rgClientMock) Resources(ctx context.Context, q armresourcegraph.QueryRequest, o *armresourcegraph.ClientResourcesOptions) (armresourcegraph.ClientResourcesResponse, error) {
	args := m.Called(ctx, q, o)
	if r := args.Get(0); r != nil {
		return *(r.(*armresourcegraph.ClientResourcesResponse)), args.Error(1)
	}
	return armresourcegraph.ClientResourcesResponse{}, args.Error(1)
}

func rgResponse(rows ...map[string]any) *armresourcegraph.ClientResourcesResponse {
	var total = int64(len(rows))
	data := make([]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, r)
	}
	return &armresourcegraph.ClientResourcesResponse{QueryResponse: armresourcegraph.QueryResponse{TotalRecords: &total, Data: data}}
}

func TestVMSSDescribe(t *testing.T) {
	assert.Equal(t, TargetIDScaleSet, (&scaleSetDiscovery{}).Describe().Id)
}

func TestVMSSDescribeTarget(t *testing.T) {
	td := (&scaleSetDiscovery{}).DescribeTarget()
	assert.Equal(t, TargetIDScaleSet, td.Id)
	assert.Contains(t, td.Label.One, "Virtual Machine Scale Set")
}

func TestVMSSDescribeAttributes(t *testing.T) {
	attrs := (&scaleSetDiscovery{}).DescribeAttributes()
	require.NotEmpty(t, attrs)
}

func TestNewScaleSetDiscovery(t *testing.T) {
	require.NotNil(t, NewScaleSetDiscovery())
}

func TestToScaleSetTarget_HappyPath(t *testing.T) {
	in := map[string]any{
		"id":             "/sub/s/rg/r/Microsoft.Compute/virtualMachineScaleSets/vmss1",
		"name":           "vmss1",
		"subscriptionId": "s",
		"resourceGroup":  "r",
		"location":       "westeurope",
		"sku":            map[string]any{"name": "Standard_B1s", "tier": "Standard", "capacity": float64(3)},
		"zones":          []any{"1", "2", "3"},
		"properties": map[string]any{
			"provisioningState":          "Succeeded",
			"upgradePolicy":              map[string]any{"mode": "Manual"},
			"orchestrationMode":          "Uniform",
			"singlePlacementGroup":       true,
			"platformFaultDomainCount":   float64(5),
		},
	}
	got := toScaleSetTarget(in)
	assert.Equal(t, "vmss1", got.Label)
	assert.Equal(t, TargetIDScaleSet, got.TargetType)
	assert.Equal(t, []string{"vmss1"}, got.Attributes["azure.vmss.name"])
	assert.Equal(t, []string{"Standard_B1s"}, got.Attributes["azure.vmss.sku.name"])
	assert.Equal(t, []string{"3"}, got.Attributes["azure.vmss.sku.capacity"])
	assert.Equal(t, []string{"Manual"}, got.Attributes["azure.vmss.upgrade-policy.mode"])
	assert.Equal(t, []string{"Uniform"}, got.Attributes["azure.vmss.orchestration-mode"])
}

func TestGetAllScaleSets_HappyPath(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponse(map[string]any{"id": "/sub/s/rg/r/Microsoft.Compute/virtualMachineScaleSets/x", "name": "x", "subscriptionId": "s", "resourceGroup": "r", "location": "westeurope", "sku": map[string]any{}, "properties": map[string]any{}}), nil)
	targets, err := getAllScaleSets(context.Background(), rg)
	require.NoError(t, err)
	require.Len(t, targets, 1)
}

func TestGetAllScaleSets_Error(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	_, err := getAllScaleSets(context.Background(), rg)
	require.Error(t, err)
}
