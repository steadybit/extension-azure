/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extapim

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

func TestApimDescribe(t *testing.T) {
	d := &apimDiscovery{}
	assert.Equal(t, TargetIDApiManagement, d.Describe().Id)
}

func TestApimDescribeTarget(t *testing.T) {
	td := (&apimDiscovery{}).DescribeTarget()
	assert.Equal(t, TargetIDApiManagement, td.Id)
	assert.Equal(t, "Azure API Management service", td.Label.One)
	assert.NotEmpty(t, td.Table.Columns)
}

func TestApimDescribeAttributes(t *testing.T) {
	attrs := (&apimDiscovery{}).DescribeAttributes()
	require.NotEmpty(t, attrs)
	for _, a := range attrs {
		assert.NotEmpty(t, a.Attribute)
		assert.NotEmpty(t, a.Label.One)
	}
}

func TestNewApimDiscovery(t *testing.T) {
	require.NotNil(t, NewApiManagementDiscovery())
}

func TestToApimTarget_HappyPath(t *testing.T) {
	in := map[string]any{
		"id":             "/sub/s/rg/r/Microsoft.ApiManagement/service/apim1",
		"name":           "apim1",
		"subscriptionId": "s",
		"resourceGroup":  "r",
		"location":       "westeurope",
		"sku":            map[string]any{"name": "Developer", "capacity": float64(1)},
		"zones":          []any{"1"},
		"properties": map[string]any{
			"provisioningState":      "Succeeded",
			"gatewayUrl":             "https://apim1.azure-api.net",
			"developerPortalUrl":     "https://apim1.developer.azure-api.net",
			"publicNetworkAccess":    "Enabled",
			"virtualNetworkType":     "None",
		},
	}
	got := toApimTarget(in)
	assert.Equal(t, "apim1", got.Label)
	assert.Equal(t, TargetIDApiManagement, got.TargetType)
	assert.Equal(t, []string{"apim1"}, got.Attributes["azure.apim.service.name"])
	assert.Equal(t, []string{"Developer"}, got.Attributes["azure.apim.sku-name"])
	assert.Equal(t, []string{"1"}, got.Attributes["azure.apim.sku-capacity"])
	assert.Equal(t, []string{"Succeeded"}, got.Attributes["azure.apim.provisioning-state"])
}

func TestToApimTarget_NilProperties(t *testing.T) {
	in := map[string]any{
		"id":             "/sub/s/rg/r/Microsoft.ApiManagement/service/apim2",
		"name":           "apim2",
		"subscriptionId": "s",
		"resourceGroup":  "r",
		"location":       "northeurope",
		"sku":            map[string]any{},
		"properties":     map[string]any{},
	}
	got := toApimTarget(in)
	assert.Equal(t, []string{"apim2"}, got.Attributes["azure.apim.service.name"])
}

func TestGetAllApim_HappyPath(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponse(map[string]any{"id": "/sub/s/rg/r/Microsoft.ApiManagement/service/x", "name": "x", "subscriptionId": "s", "resourceGroup": "r", "location": "westeurope", "sku": map[string]any{}, "properties": map[string]any{}}), nil)
	targets, err := getAllApimServices(context.Background(), rg)
	require.NoError(t, err)
	require.Len(t, targets, 1)
}

func TestGetAllApim_Error(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	_, err := getAllApimServices(context.Background(), rg)
	require.Error(t, err)
}
