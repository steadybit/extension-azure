/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extcosmosdb

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

func TestCosmosDescribe(t *testing.T) {
	assert.Equal(t, TargetIDCosmosDbAccount, (&accountDiscovery{}).Describe().Id)
}

func TestCosmosDescribeTarget(t *testing.T) {
	td := (&accountDiscovery{}).DescribeTarget()
	assert.Equal(t, TargetIDCosmosDbAccount, td.Id)
	assert.Contains(t, td.Label.One, "Cosmos DB")
}

func TestCosmosDescribeAttributes(t *testing.T) {
	attrs := (&accountDiscovery{}).DescribeAttributes()
	require.NotEmpty(t, attrs)
}

func TestNewAccountDiscovery(t *testing.T) {
	require.NotNil(t, NewAccountDiscovery())
}

func TestToCosmosDbTarget_HappyPath(t *testing.T) {
	in := map[string]any{
		"id":             "/sub/s/rg/r/Microsoft.DocumentDB/databaseAccounts/c1",
		"name":           "c1",
		"subscriptionId": "s",
		"resourceGroup":  "r",
		"location":       "westeurope",
		"kind":           "GlobalDocumentDB",
		"properties": map[string]any{
			"provisioningState":            "Succeeded",
			"consistencyPolicy":            map[string]any{"defaultConsistencyLevel": "Session"},
			"enableMultipleWriteLocations": false,
			"enableAutomaticFailover":      true,
			"locations": []any{
				map[string]any{"locationName": "West Europe", "failoverPriority": float64(0)},
				map[string]any{"locationName": "North Europe", "failoverPriority": float64(1)},
			},
		},
	}
	got := toCosmosDbAccountTarget(in)
	assert.Equal(t, "c1", got.Label)
	assert.Equal(t, TargetIDCosmosDbAccount, got.TargetType)
	assert.Equal(t, []string{"c1"}, got.Attributes["azure.cosmosdb.account.name"])
	assert.Equal(t, []string{"Session"}, got.Attributes["azure.cosmosdb.consistency-level"])
	assert.Equal(t, []string{"true"}, got.Attributes["azure.cosmosdb.enable-automatic-failover"])
}

func TestGetAllCosmosDbAccounts_HappyPath(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponse(map[string]any{"id": "/sub/s/rg/r/Microsoft.DocumentDB/databaseAccounts/x", "name": "x", "subscriptionId": "s", "resourceGroup": "r", "location": "westeurope", "properties": map[string]any{}}), nil)
	targets, err := getAllCosmosDbAccounts(context.Background(), rg)
	require.NoError(t, err)
	require.Len(t, targets, 1)
}

func TestGetAllCosmosDbAccounts_Error(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	_, err := getAllCosmosDbAccounts(context.Background(), rg)
	require.Error(t, err)
}
