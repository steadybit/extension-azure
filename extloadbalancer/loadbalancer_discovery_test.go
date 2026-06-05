/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extloadbalancer

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

func TestLBDescribe(t *testing.T) {
	assert.Equal(t, TargetIDLoadBalancer, (&loadBalancerDiscovery{}).Describe().Id)
}

func TestLBDescribeTarget(t *testing.T) {
	td := (&loadBalancerDiscovery{}).DescribeTarget()
	assert.Equal(t, TargetIDLoadBalancer, td.Id)
	assert.Contains(t, td.Label.One, "Load Balancer")
}

func TestLBDescribeAttributes(t *testing.T) {
	attrs := (&loadBalancerDiscovery{}).DescribeAttributes()
	require.NotEmpty(t, attrs)
}

func TestNewLoadBalancerDiscovery(t *testing.T) {
	require.NotNil(t, NewLoadBalancerDiscovery())
}

func TestToLoadBalancerTarget_HappyPath(t *testing.T) {
	in := map[string]any{
		"id":             "/sub/s/rg/r/Microsoft.Network/loadBalancers/lb1",
		"name":           "lb1",
		"subscriptionId": "s",
		"resourceGroup":  "r",
		"location":       "westeurope",
		"sku":            map[string]any{"name": "Standard", "tier": "Regional"},
		"properties": map[string]any{
			"provisioningState": "Succeeded",
			"backendAddressPools": []any{
				map[string]any{"name": "pool1"},
			},
			"loadBalancingRules":     []any{map[string]any{}},
			"inboundNatRules":        []any{map[string]any{}, map[string]any{}},
			"probes":                 []any{map[string]any{}},
			"outboundRules":          []any{},
			"frontendIPConfigurations": []any{
				map[string]any{
					"name": "fe1",
					"properties": map[string]any{
						"publicIPAddress": map[string]any{"id": "/pip/1"},
					},
				},
			},
		},
	}
	got := toLoadBalancerTarget(in)
	assert.Equal(t, "lb1", got.Label)
	assert.Equal(t, TargetIDLoadBalancer, got.TargetType)
	assert.Equal(t, []string{"lb1"}, got.Attributes["azure.load-balancer.name"])
	assert.Equal(t, []string{"Standard"}, got.Attributes["azure.load-balancer.sku-name"])
	assert.Equal(t, []string{"true"}, got.Attributes["azure.load-balancer.frontend.public-exposed"])
}

func TestGetAllLoadBalancers_HappyPath(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponse(map[string]any{"id": "/sub/s/rg/r/Microsoft.Network/loadBalancers/x", "name": "x", "subscriptionId": "s", "resourceGroup": "r", "location": "westeurope", "sku": map[string]any{}, "properties": map[string]any{}}), nil)
	targets, err := getAllLoadBalancers(context.Background(), rg)
	require.NoError(t, err)
	require.Len(t, targets, 1)
}

func TestGetAllLoadBalancers_Error(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	_, err := getAllLoadBalancers(context.Background(), rg)
	require.Error(t, err)
}
