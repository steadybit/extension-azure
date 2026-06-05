/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extstoragequeue

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

func TestStorageQueueDescribe(t *testing.T) {
	assert.Equal(t, TargetIDStorageQueue, (&accountDiscovery{}).Describe().Id)
}

func TestStorageQueueDescribeTarget(t *testing.T) {
	td := (&accountDiscovery{}).DescribeTarget()
	assert.Equal(t, TargetIDStorageQueue, td.Id)
	assert.Contains(t, td.Label.One, "Storage Queue")
}

func TestStorageQueueDescribeAttributes(t *testing.T) {
	attrs := (&accountDiscovery{}).DescribeAttributes()
	require.NotEmpty(t, attrs)
}

func TestNewStorageAccountDiscovery(t *testing.T) {
	require.NotNil(t, NewStorageAccountDiscovery())
}

func TestToStorageAccountTarget_HappyPath(t *testing.T) {
	in := map[string]any{
		"id":             "/sub/s/rg/r/Microsoft.Storage/storageAccounts/sa1",
		"name":           "sa1",
		"kind":           "StorageV2",
		"subscriptionId": "s",
		"resourceGroup":  "r",
		"location":       "westeurope",
		"sku":            map[string]any{"name": "Standard_LRS", "tier": "Standard"},
		"properties": map[string]any{
			"provisioningState":      "Succeeded",
			"publicNetworkAccess":    "Enabled",
			"allowBlobPublicAccess":  false,
			"allowSharedKeyAccess":   true,
			"minimumTlsVersion":      "TLS1_2",
			"supportsHttpsTrafficOnly": true,
			"primaryEndpoints":       map[string]any{"queue": "https://sa1.queue.core.windows.net/"},
			"encryption":             map[string]any{"keySource": "Microsoft.Storage", "requireInfrastructureEncryption": true},
		},
	}
	got := toStorageAccountTarget(in)
	assert.Equal(t, "sa1", got.Label)
	assert.Equal(t, TargetIDStorageQueue, got.TargetType)
	assert.Equal(t, []string{"sa1"}, got.Attributes["azure.storage.account.name"])
	assert.Equal(t, []string{"StorageV2"}, got.Attributes["azure.storage.kind"])
	assert.Equal(t, []string{"Standard_LRS"}, got.Attributes["azure.storage.sku-name"])
	assert.Equal(t, []string{"TLS1_2"}, got.Attributes["azure.storage.minimum-tls-version"])
}

func TestGetAllStorageAccounts_HappyPath(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponse(map[string]any{"id": "/sub/s/rg/r/Microsoft.Storage/storageAccounts/x", "name": "x", "kind": "StorageV2", "subscriptionId": "s", "resourceGroup": "r", "location": "westeurope", "sku": map[string]any{}, "properties": map[string]any{}}), nil)
	targets, err := getAllStorageAccounts(context.Background(), rg)
	require.NoError(t, err)
	require.Len(t, targets, 1)
}

func TestGetAllStorageAccounts_Error(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	_, err := getAllStorageAccounts(context.Background(), rg)
	require.Error(t, err)
}
