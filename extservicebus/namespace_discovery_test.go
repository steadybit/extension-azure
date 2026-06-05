/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extservicebus

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNamespaceDescribe(t *testing.T) {
	assert.Equal(t, TargetIDNamespace, (&namespaceDiscovery{}).Describe().Id)
}

func TestNamespaceDescribeTarget(t *testing.T) {
	td := (&namespaceDiscovery{}).DescribeTarget()
	assert.Equal(t, TargetIDNamespace, td.Id)
	assert.Contains(t, td.Label.One, "Service Bus namespace")
}

func TestNamespaceDescribeAttributes(t *testing.T) {
	require.NotEmpty(t, (&namespaceDiscovery{}).DescribeAttributes())
}

func TestNewNamespaceDiscovery(t *testing.T) {
	require.NotNil(t, NewNamespaceDiscovery())
}

func TestToNamespaceTarget_HappyPath(t *testing.T) {
	in := map[string]any{
		"id":             "/sub/s/rg/r/Microsoft.ServiceBus/namespaces/sbn1",
		"name":           "sbn1",
		"subscriptionId": "s",
		"resourceGroup":  "r",
		"location":       "westeurope",
		"sku":            map[string]any{"name": "Standard", "tier": "Standard", "capacity": float64(1)},
		"properties": map[string]any{
			"provisioningState":      "Succeeded",
			"zoneRedundant":          true,
			"publicNetworkAccess":    "Enabled",
			"minimumTlsVersion":      "1.2",
			"disableLocalAuth":       false,
			"serviceBusEndpoint":     "https://sbn1.servicebus.windows.net:443/",
			"status":                 "Active",
		},
	}
	got := toNamespaceTarget(in)
	assert.Equal(t, "sbn1", got.Label)
	assert.Equal(t, TargetIDNamespace, got.TargetType)
	assert.Equal(t, []string{"sbn1"}, got.Attributes["azure.servicebus.namespace.name"])
	assert.Equal(t, []string{"Standard"}, got.Attributes["azure.servicebus.sku-name"])
	assert.Equal(t, []string{"true"}, got.Attributes["azure.servicebus.zone-redundant"])
}

func TestGetAllNamespaces_HappyPath(t *testing.T) {
	rg := new(azureResourceGraphClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponseWithOneNamespace(), nil)

	targets, err := getAllNamespaces(context.Background(), rg)
	require.NoError(t, err)
	require.Len(t, targets, 1)
}

func TestGetAllNamespaces_Error(t *testing.T) {
	rg := new(azureResourceGraphClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	_, err := getAllNamespaces(context.Background(), rg)
	require.Error(t, err)
}
