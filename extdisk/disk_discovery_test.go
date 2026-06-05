/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extdisk

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
	return &armresourcegraph.ClientResourcesResponse{
		QueryResponse: armresourcegraph.QueryResponse{TotalRecords: &total, Data: data},
	}
}

func TestDiskDescribe(t *testing.T) {
	d := &diskDiscovery{}
	desc := d.Describe()
	assert.Equal(t, TargetIDDisk, desc.Id)
	assert.NotNil(t, desc.Discover.CallInterval)
}

func TestDiskDescribeTarget(t *testing.T) {
	d := &diskDiscovery{}
	td := d.DescribeTarget()
	assert.Equal(t, TargetIDDisk, td.Id)
	assert.Equal(t, "Azure Managed Disk", td.Label.One)
	assert.Equal(t, "Azure Managed Disks", td.Label.Other)
	require.NotNil(t, td.Category)
	assert.Equal(t, "cloud", *td.Category)
	assert.NotEmpty(t, td.Table.Columns)
}

func TestDiskDescribeAttributes(t *testing.T) {
	d := &diskDiscovery{}
	attrs := d.DescribeAttributes()
	assert.NotEmpty(t, attrs)
	for _, a := range attrs {
		assert.True(t, len(a.Attribute) > 0)
		assert.NotEmpty(t, a.Label.One)
		assert.NotEmpty(t, a.Label.Other)
	}
}

func TestNewDiskDiscovery(t *testing.T) {
	d := NewDiskDiscovery()
	require.NotNil(t, d)
}

func TestToDiskTarget_FullProperties(t *testing.T) {
	in := map[string]any{
		"id":             "/subscriptions/s1/resourceGroups/rg1/providers/Microsoft.Compute/disks/d1",
		"name":           "d1",
		"subscriptionId": "s1",
		"resourceGroup":  "rg1",
		"location":       "westeurope",
		"managedBy":      "/subscriptions/s1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1",
		"zones":          []any{"2", "1"},
		"sku":            map[string]any{"name": "Premium_LRS"},
		"tags":           map[string]any{"Owner": "Antoine"},
		"properties": map[string]any{
			"diskSizeGB":          float64(128),
			"diskIOPSReadWrite":   float64(500),
			"diskMBpsReadWrite":   float64(100),
			"osType":              "Linux",
			"diskState":           "Attached",
			"publicNetworkAccess": "Disabled",
			"networkAccessPolicy": "DenyAll",
			"maxShares":           float64(1),
			"burstingEnabled":     true,
			"encryption":          map[string]any{"type": "EncryptionAtRestWithCustomerKey", "diskEncryptionSetId": "/des/1"},
		},
	}
	got := toDiskTarget(in)
	assert.Equal(t, "/subscriptions/s1/resourceGroups/rg1/providers/Microsoft.Compute/disks/d1", got.Id)
	assert.Equal(t, "d1", got.Label)
	assert.Equal(t, TargetIDDisk, got.TargetType)
	assert.Equal(t, []string{"d1"}, got.Attributes["azure.disk.name"])
	assert.Equal(t, []string{"s1"}, got.Attributes["azure.subscription.id"])
	assert.Equal(t, []string{"Premium_LRS"}, got.Attributes["azure.disk.sku-name"])
	assert.Equal(t, []string{"128"}, got.Attributes["azure.disk.size-gb"])
	assert.Equal(t, []string{"500"}, got.Attributes["azure.disk.iops-read-write"])
	assert.Equal(t, []string{"100"}, got.Attributes["azure.disk.mbps-read-write"])
	assert.Equal(t, []string{"Linux"}, got.Attributes["azure.disk.os-type"])
	assert.Equal(t, []string{"Attached"}, got.Attributes["azure.disk.disk-state"])
	assert.Equal(t, []string{"EncryptionAtRestWithCustomerKey"}, got.Attributes["azure.disk.encryption.type"])
	assert.Equal(t, []string{"/des/1"}, got.Attributes["azure.disk.encryption.set-id"])
	assert.Equal(t, []string{"true"}, got.Attributes["azure.disk.bursting-enabled"])
	assert.Equal(t, []string{"1", "2"}, got.Attributes["azure.disk.zones"], "zones must be sorted")
	assert.Equal(t, []string{"Antoine"}, got.Attributes["azure.disk.label.owner"])
}

func TestToDiskTarget_OptionalFieldsOmittedWhenAbsent(t *testing.T) {
	in := map[string]any{
		"id":             "/sub/s/rg/r/Microsoft.Compute/disks/d2",
		"name":           "d2",
		"subscriptionId": "s",
		"resourceGroup":  "r",
		"location":       "northeurope",
		"properties":     map[string]any{},
		"sku":            map[string]any{},
	}
	got := toDiskTarget(in)
	assert.Equal(t, []string{"d2"}, got.Attributes["azure.disk.name"])
	_, hasSize := got.Attributes["azure.disk.size-gb"]
	assert.False(t, hasSize, "size-gb omitted when properties.diskSizeGB absent")
	_, hasIops := got.Attributes["azure.disk.iops-read-write"]
	assert.False(t, hasIops)
	_, hasEncType := got.Attributes["azure.disk.encryption.type"]
	assert.False(t, hasEncType)
}

func TestGetAllDisks_HappyPath(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponse(
			map[string]any{"id": "/sub/s/rg/r/Microsoft.Compute/disks/d1", "name": "d1", "subscriptionId": "s", "resourceGroup": "r", "location": "westeurope", "properties": map[string]any{}, "sku": map[string]any{}},
		), nil)

	targets, err := getAllDisks(context.Background(), rg)
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Equal(t, "d1", targets[0].Label)
}

func TestGetAllDisks_ResourceGraphError(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))

	_, err := getAllDisks(context.Background(), rg)
	require.Error(t, err)
}
