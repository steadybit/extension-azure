/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extservicebus

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/servicebus/armservicebus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// azureResourceGraphClientMock mirrors the mock used elsewhere in this codebase (see
// extvm/vm_discovery_test.go). Lets us pretend to be Azure Resource Graph.
type azureResourceGraphClientMock struct {
	mock.Mock
}

func (m *azureResourceGraphClientMock) Resources(ctx context.Context, query armresourcegraph.QueryRequest, options *armresourcegraph.ClientResourcesOptions) (armresourcegraph.ClientResourcesResponse, error) {
	args := m.Called(ctx, query, options)
	if args.Get(0) == nil {
		return armresourcegraph.ClientResourcesResponse{}, args.Error(1)
	}
	return *args.Get(0).(*armresourcegraph.ClientResourcesResponse), args.Error(1)
}

// rgResponseWithOneNamespace returns an RG response describing a single Service Bus namespace.
func rgResponseWithOneNamespace() *armresourcegraph.ClientResourcesResponse {
	var total int64 = 1
	return &armresourcegraph.ClientResourcesResponse{
		QueryResponse: armresourcegraph.QueryResponse{
			TotalRecords: &total,
			Data: []any{
				map[string]any{
					"name":           "sb-test-ns",
					"resourceGroup":  "rg-1",
					"location":       "westeurope",
					"subscriptionId": "sub-1",
				},
			},
		},
	}
}

func TestGetAllQueues_HappyPath(t *testing.T) {
	rg := new(azureResourceGraphClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponseWithOneNamespace(), nil)

	lister := func(ctx context.Context, sub, rg, ns string) ([]*armservicebus.SBQueue, error) {
		assert.Equal(t, "sub-1", sub)
		assert.Equal(t, "rg-1", rg)
		assert.Equal(t, "sb-test-ns", ns)
		return []*armservicebus.SBQueue{
			{
				ID:   to.Ptr("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.ServiceBus/namespaces/sb-test-ns/queues/q1"),
				Name: to.Ptr("q1"),
				Properties: &armservicebus.SBQueueProperties{
					Status:                           to.Ptr(armservicebus.EntityStatusActive),
					MaxDeliveryCount:                 to.Ptr[int32](10),
					DeadLetteringOnMessageExpiration: to.Ptr(true),
					RequiresDuplicateDetection:       to.Ptr(false),
					RequiresSession:                  to.Ptr(false),
					LockDuration:                     to.Ptr("PT1M"),
					DefaultMessageTimeToLive:         to.Ptr("P14D"),
				},
			},
		}, nil
	}

	targets, err := getAllQueues(context.Background(), rg, lister)
	require.NoError(t, err)
	require.Len(t, targets, 1)

	tgt := targets[0]
	assert.Equal(t, TargetIDQueue, tgt.TargetType)
	assert.Equal(t, "sb-test-ns/q1", tgt.Label)
	assert.Equal(t, []string{"q1"}, tgt.Attributes["azure.servicebus.queue.name"])
	assert.Equal(t, []string{"sb-test-ns"}, tgt.Attributes["azure.servicebus.namespace.name"])
	assert.Equal(t, []string{"sub-1"}, tgt.Attributes["azure.subscription.id"])
	assert.Equal(t, []string{"rg-1"}, tgt.Attributes["azure.resource-group.name"])
	assert.Equal(t, []string{"westeurope"}, tgt.Attributes["azure.location"])
	assert.Equal(t, []string{"Active"}, tgt.Attributes["azure.servicebus.queue.status"])
	assert.Equal(t, []string{"10"}, tgt.Attributes["azure.servicebus.queue.max-delivery-count"])
	assert.Equal(t, []string{"true"}, tgt.Attributes["azure.servicebus.queue.dead-lettering-on-message-expiration"])
	assert.Equal(t, []string{"PT1M"}, tgt.Attributes["azure.servicebus.queue.lock-duration"])
}

func TestGetAllQueues_MultipleNamespacesMixedSuccess(t *testing.T) {
	// Two namespaces; lister fails on the second one. We expect a degraded result (queues from the
	// first namespace) rather than an outright error — discovery should be resilient.
	var total int64 = 2
	resp := &armresourcegraph.ClientResourcesResponse{
		QueryResponse: armresourcegraph.QueryResponse{
			TotalRecords: &total,
			Data: []any{
				map[string]any{"name": "ns-a", "resourceGroup": "rg-1", "location": "westeurope", "subscriptionId": "sub-1"},
				map[string]any{"name": "ns-b", "resourceGroup": "rg-2", "location": "eastus", "subscriptionId": "sub-2"},
			},
		},
	}
	rg := new(azureResourceGraphClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)

	lister := func(ctx context.Context, sub, _, _ string) ([]*armservicebus.SBQueue, error) {
		if sub == "sub-1" {
			return []*armservicebus.SBQueue{
				{ID: to.Ptr("/.../q-a-1"), Name: to.Ptr("q-a-1"), Properties: &armservicebus.SBQueueProperties{Status: to.Ptr(armservicebus.EntityStatusActive)}},
			}, nil
		}
		return nil, errors.New("permission denied")
	}

	targets, err := getAllQueues(context.Background(), rg, lister)
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Equal(t, "ns-a/q-a-1", targets[0].Label)
}

func TestGetAllQueues_ResourceGraphError(t *testing.T) {
	rg := new(azureResourceGraphClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("ARG outage"))

	lister := func(ctx context.Context, _, _, _ string) ([]*armservicebus.SBQueue, error) {
		t.Fatal("lister should not be called when RG fails")
		return nil, nil
	}

	_, err := getAllQueues(context.Background(), rg, lister)
	require.Error(t, err)
}

func TestGetAllQueues_SkipsNilQueueEntries(t *testing.T) {
	// Defensive: the SDK can in theory return nil slice entries — discovery should skip them, not panic.
	rg := new(azureResourceGraphClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(rgResponseWithOneNamespace(), nil)
	lister := func(ctx context.Context, _, _, _ string) ([]*armservicebus.SBQueue, error) {
		return []*armservicebus.SBQueue{nil, {ID: to.Ptr("/.../q1"), Name: to.Ptr("q1"), Properties: &armservicebus.SBQueueProperties{Status: to.Ptr(armservicebus.EntityStatusActive)}}}, nil
	}
	targets, err := getAllQueues(context.Background(), rg, lister)
	require.NoError(t, err)
	require.Len(t, targets, 1)
}

func TestQueueToTarget_OptionalPropertiesOmittedWhenNil(t *testing.T) {
	// Only required fields set on Properties; other attributes should simply be absent rather than empty.
	ns := serviceBusNamespaceRef{name: "ns", resourceGroup: "rg", subscriptionId: "sub", location: "loc"}
	q := &armservicebus.SBQueue{
		ID:         to.Ptr("/.../q"),
		Name:       to.Ptr("q"),
		Properties: &armservicebus.SBQueueProperties{Status: to.Ptr(armservicebus.EntityStatusDisabled)},
	}
	tgt := queueToTarget(q, ns)
	assert.Equal(t, []string{"Disabled"}, tgt.Attributes["azure.servicebus.queue.status"])
	_, hasMaxDelivery := tgt.Attributes["azure.servicebus.queue.max-delivery-count"]
	assert.False(t, hasMaxDelivery, "max-delivery-count attribute should not be set when SDK value is nil")
	_, hasForward := tgt.Attributes["azure.servicebus.queue.forward-to"]
	assert.False(t, hasForward, "forward-to attribute should not be set when SDK value is nil")
}
