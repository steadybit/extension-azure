/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extappgateway

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

func TestAppGatewayDescribe(t *testing.T) {
	assert.Equal(t, TargetIDAppGateway, (&appGatewayDiscovery{}).Describe().Id)
}

func TestAppGatewayDescribeTarget(t *testing.T) {
	td := (&appGatewayDiscovery{}).DescribeTarget()
	assert.Equal(t, TargetIDAppGateway, td.Id)
	assert.Contains(t, td.Label.One, "Application Gateway")
}

func TestAppGatewayDescribeAttributes(t *testing.T) {
	attrs := (&appGatewayDiscovery{}).DescribeAttributes()
	require.NotEmpty(t, attrs)
}

func TestNewAppGatewayDiscovery(t *testing.T) {
	require.NotNil(t, NewAppGatewayDiscovery())
}

func TestToAppGatewayTarget_HappyPath(t *testing.T) {
	in := map[string]any{
		"id":             "/sub/s/rg/r/Microsoft.Network/applicationGateways/agw1",
		"name":           "agw1",
		"subscriptionId": "s",
		"resourceGroup":  "r",
		"location":       "westeurope",
		"properties": map[string]any{
			"provisioningState": "Succeeded",
			"operationalState":  "Running",
			"enableHttp2":       true,
			"sku":               map[string]any{"name": "Standard_v2", "tier": "Standard_v2", "capacity": float64(2)},
			"webApplicationFirewallConfiguration": map[string]any{"enabled": true, "firewallMode": "Prevention"},
			"frontendPorts":       []any{map[string]any{}, map[string]any{}},
			"httpListeners":       []any{map[string]any{}},
			"backendAddressPools": []any{map[string]any{}, map[string]any{}, map[string]any{}},
		},
	}
	got := toAppGatewayTarget(in)
	assert.Equal(t, "agw1", got.Label)
	assert.Equal(t, TargetIDAppGateway, got.TargetType)
	assert.Equal(t, []string{"agw1"}, got.Attributes["azure.application-gateway.name"])
	assert.Equal(t, []string{"Standard_v2"}, got.Attributes["azure.application-gateway.sku-name"])
	assert.Equal(t, []string{"Succeeded"}, got.Attributes["azure.application-gateway.provisioning-state"])
}

func TestGetAllAppGateways_HappyPath(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponse(map[string]any{"id": "/sub/s/rg/r/Microsoft.Network/applicationGateways/x", "name": "x", "subscriptionId": "s", "resourceGroup": "r", "location": "westeurope", "properties": map[string]any{}}), nil)
	targets, err := getAllAppGateways(context.Background(), rg)
	require.NoError(t, err)
	require.Len(t, targets, 1)
}

func TestGetAllAppGateways_Error(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	_, err := getAllAppGateways(context.Background(), rg)
	require.Error(t, err)
}
