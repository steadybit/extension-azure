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

func TestGetAllTopics_HappyPath(t *testing.T) {
	rg := new(azureResourceGraphClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponseWithOneNamespace(), nil)

	lister := func(ctx context.Context, sub, rgName, ns string) ([]*armservicebus.SBTopic, error) {
		assert.Equal(t, "sub-1", sub)
		assert.Equal(t, "rg-1", rgName)
		assert.Equal(t, "sb-test-ns", ns)
		return []*armservicebus.SBTopic{
			{
				ID:   to.Ptr("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.ServiceBus/namespaces/sb-test-ns/topics/t1"),
				Name: to.Ptr("t1"),
				Properties: &armservicebus.SBTopicProperties{
					Status:                     to.Ptr(armservicebus.EntityStatusActive),
					RequiresDuplicateDetection: to.Ptr(false),
					SupportOrdering:            to.Ptr(true),
					EnablePartitioning:         to.Ptr(false),
					DefaultMessageTimeToLive:   to.Ptr("P14D"),
					SubscriptionCount:          to.Ptr[int32](3),
				},
			},
		}, nil
	}

	targets, err := getAllTopics(context.Background(), rg, lister)
	require.NoError(t, err)
	require.Len(t, targets, 1)

	tgt := targets[0]
	assert.Equal(t, TargetIDTopic, tgt.TargetType)
	assert.Equal(t, "sb-test-ns/t1", tgt.Label)
	assert.Equal(t, []string{"t1"}, tgt.Attributes["azure.servicebus.topic.name"])
	assert.Equal(t, []string{"sb-test-ns"}, tgt.Attributes["azure.servicebus.namespace.name"])
	assert.Equal(t, []string{"sub-1"}, tgt.Attributes["azure.subscription.id"])
	assert.Equal(t, []string{"Active"}, tgt.Attributes["azure.servicebus.topic.status"])
	assert.Equal(t, []string{"true"}, tgt.Attributes["azure.servicebus.topic.support-ordering"])
	assert.Equal(t, []string{"false"}, tgt.Attributes["azure.servicebus.topic.enable-partitioning"])
	assert.Equal(t, []string{"3"}, tgt.Attributes["azure.servicebus.topic.subscription-count"])
}

func TestGetAllTopics_PerNamespaceListerErrorIsTolerated(t *testing.T) {
	rg := new(azureResourceGraphClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponseWithOneNamespace(), nil)

	lister := func(ctx context.Context, _, _, _ string) ([]*armservicebus.SBTopic, error) {
		return nil, errors.New("transient ARM 503")
	}

	targets, err := getAllTopics(context.Background(), rg, lister)
	require.NoError(t, err, "discovery should not surface per-namespace list errors as fatal")
	assert.Empty(t, targets)
}

func TestTopicToTarget_OptionalPropertiesOmittedWhenNil(t *testing.T) {
	ns := serviceBusNamespaceRef{name: "ns", resourceGroup: "rg", subscriptionId: "sub", location: "loc"}
	tp := &armservicebus.SBTopic{
		ID:         to.Ptr("/.../t"),
		Name:       to.Ptr("t"),
		Properties: &armservicebus.SBTopicProperties{Status: to.Ptr(armservicebus.EntityStatusDisabled)},
	}
	tgt := topicToTarget(tp, ns)
	assert.Equal(t, []string{"Disabled"}, tgt.Attributes["azure.servicebus.topic.status"])
	_, hasSubCount := tgt.Attributes["azure.servicebus.topic.subscription-count"]
	assert.False(t, hasSubCount, "subscription-count should be absent when SDK value is nil")
}

func TestListServiceBusNamespaceRefs_ParsesRGRows(t *testing.T) {
	// Also covers queue_discovery's shared helper.
	var total int64 = 2
	resp := &armresourcegraph.ClientResourcesResponse{
		QueryResponse: armresourcegraph.QueryResponse{
			TotalRecords: &total,
			Data: []any{
				map[string]any{"name": "ns-a", "resourceGroup": "rg-a", "location": "westeurope", "subscriptionId": "sub-a"},
				map[string]any{"name": "ns-b", "resourceGroup": "rg-b", "location": "eastus", "subscriptionId": "sub-b"},
			},
		},
	}
	rg := new(azureResourceGraphClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)

	refs, err := listServiceBusNamespaceRefs(context.Background(), rg)
	require.NoError(t, err)
	require.Len(t, refs, 2)
	assert.Equal(t, "ns-a", refs[0].name)
	assert.Equal(t, "rg-a", refs[0].resourceGroup)
	assert.Equal(t, "sub-a", refs[0].subscriptionId)
	assert.Equal(t, "westeurope", refs[0].location)
	assert.Equal(t, "ns-b", refs[1].name)
	assert.Equal(t, "eastus", refs[1].location)
}
