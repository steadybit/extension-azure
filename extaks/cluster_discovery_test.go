/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extaks

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

func TestClusterDescribe(t *testing.T) {
	assert.Equal(t, TargetIDCluster, (&clusterDiscovery{}).Describe().Id)
}

func TestClusterDescribeTarget(t *testing.T) {
	td := (&clusterDiscovery{}).DescribeTarget()
	assert.Equal(t, TargetIDCluster, td.Id)
	assert.Contains(t, td.Label.One, "AKS cluster")
}

func TestClusterDescribeAttributes(t *testing.T) {
	require.NotEmpty(t, (&clusterDiscovery{}).DescribeAttributes())
}

func TestNewClusterDiscovery(t *testing.T) {
	require.NotNil(t, NewClusterDiscovery())
}

func TestToClusterTarget_HappyPath(t *testing.T) {
	in := map[string]any{
		"id":             "/sub/s/rg/r/Microsoft.ContainerService/managedClusters/aks1",
		"name":           "aks1",
		"subscriptionId": "s",
		"resourceGroup":  "r",
		"location":       "westeurope",
		"properties": map[string]any{
			"kubernetesVersion":   "1.29.0",
			"powerState":          map[string]any{"code": "Running"},
			"provisioningState":   "Succeeded",
			"dnsPrefix":           "aks1-dns",
			"fqdn":                "aks1-dns.hcp.westeurope.azmk8s.io",
			"enablePrivateCluster": false,
			"sku":                  map[string]any{"tier": "Free"},
		},
	}
	got := toClusterTarget(in)
	assert.Equal(t, "aks1", got.Label)
	assert.Equal(t, TargetIDCluster, got.TargetType)
	assert.Equal(t, []string{"aks1"}, got.Attributes["azure.aks.cluster.name"])
	assert.Equal(t, []string{"1.29.0"}, got.Attributes["azure.aks.cluster.kubernetes-version"])
}

func TestGetAllClusters_HappyPath(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponse(map[string]any{"id": "/sub/s/rg/r/Microsoft.ContainerService/managedClusters/x", "name": "x", "subscriptionId": "s", "resourceGroup": "r", "location": "westeurope", "properties": map[string]any{}}), nil)
	targets, err := getAllAksClusters(context.Background(), rg)
	require.NoError(t, err)
	require.Len(t, targets, 1)
}

func TestGetAllClusters_Error(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	_, err := getAllAksClusters(context.Background(), rg)
	require.Error(t, err)
}
