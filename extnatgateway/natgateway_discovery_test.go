/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extnatgateway

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

func TestNatGwDescribe(t *testing.T) {
	assert.Equal(t, TargetIDNatGateway, (&natGatewayDiscovery{}).Describe().Id)
}

func TestNatGwDescribeTarget(t *testing.T) {
	td := (&natGatewayDiscovery{}).DescribeTarget()
	assert.Equal(t, TargetIDNatGateway, td.Id)
	assert.Contains(t, td.Label.One, "NAT Gateway")
}

func TestNatGwDescribeAttributes(t *testing.T) {
	require.NotEmpty(t, (&natGatewayDiscovery{}).DescribeAttributes())
}

func TestNewNatGatewayDiscovery(t *testing.T) {
	require.NotNil(t, NewNatGatewayDiscovery())
}

func TestToNatGatewayTarget_HappyPath(t *testing.T) {
	in := map[string]any{
		"id":             "/sub/s/rg/r/Microsoft.Network/natGateways/ngw1",
		"name":           "ngw1",
		"subscriptionId": "s",
		"resourceGroup":  "r",
		"location":       "westeurope",
		"sku":            map[string]any{"name": "Standard"},
		"zones":          []any{"1"},
		"properties": map[string]any{
			"provisioningState":    "Succeeded",
			"idleTimeoutInMinutes": float64(4),
			"publicIpAddresses":    []any{map[string]any{"id": "/sub/s/rg/r/Microsoft.Network/publicIPAddresses/pip1"}},
		},
	}
	got := toNatGatewayTarget(in)
	assert.Equal(t, "ngw1", got.Label)
	assert.Equal(t, TargetIDNatGateway, got.TargetType)
	assert.Equal(t, []string{"ngw1"}, got.Attributes["azure.nat-gateway.name"])
	assert.Equal(t, []string{"Standard"}, got.Attributes["azure.nat-gateway.sku-name"])
}

func TestGetAllNatGateways_HappyPath(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponse(map[string]any{"id": "/sub/s/rg/r/Microsoft.Network/natGateways/x", "name": "x", "subscriptionId": "s", "resourceGroup": "r", "location": "westeurope", "sku": map[string]any{}, "properties": map[string]any{}}), nil)
	targets, err := getAllNatGateways(context.Background(), rg)
	require.NoError(t, err)
	require.Len(t, targets, 1)
}

func TestGetAllNatGateways_Error(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	_, err := getAllNatGateways(context.Background(), rg)
	require.Error(t, err)
}
