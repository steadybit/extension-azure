/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package exteventgrid

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

// ---------- Topic ----------

func TestTopicDescribe(t *testing.T) {
	assert.Equal(t, TargetIDTopic, (&topicDiscovery{}).Describe().Id)
}

func TestTopicDescribeTarget(t *testing.T) {
	td := (&topicDiscovery{}).DescribeTarget()
	assert.Equal(t, TargetIDTopic, td.Id)
	assert.Contains(t, td.Label.One, "Event Grid topic")
}

func TestTopicDescribeAttributes(t *testing.T) {
	require.NotEmpty(t, (&topicDiscovery{}).DescribeAttributes())
}

func TestNewTopicDiscovery(t *testing.T) {
	require.NotNil(t, NewTopicDiscovery())
}

func TestToTopicTarget_HappyPath(t *testing.T) {
	in := map[string]any{
		"id":             "/sub/s/rg/r/Microsoft.EventGrid/topics/t1",
		"name":           "t1",
		"kind":           "Azure",
		"subscriptionId": "s",
		"resourceGroup":  "r",
		"location":       "westeurope",
		"properties": map[string]any{
			"provisioningState":   "Succeeded",
			"inputSchema":         "EventGridSchema",
			"publicNetworkAccess": "Enabled",
			"endpoint":            "https://t1.westeurope-1.eventgrid.azure.net/api/events",
		},
	}
	got := toTopicTarget(in)
	assert.Equal(t, "t1", got.Label)
	assert.Equal(t, TargetIDTopic, got.TargetType)
	assert.Equal(t, []string{"t1"}, got.Attributes["azure.eventgrid.topic.name"])
}

func TestGetAllTopics_HappyPath(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponse(map[string]any{"id": "/sub/s/rg/r/Microsoft.EventGrid/topics/x", "name": "x", "subscriptionId": "s", "resourceGroup": "r", "location": "westeurope", "properties": map[string]any{}}), nil)
	targets, err := getAllTopics(context.Background(), rg)
	require.NoError(t, err)
	require.Len(t, targets, 1)
}

func TestGetAllTopics_Error(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	_, err := getAllTopics(context.Background(), rg)
	require.Error(t, err)
}

// ---------- Subscription ----------

func TestSubscriptionDescribe(t *testing.T) {
	assert.Equal(t, TargetIDSubscription, (&subscriptionDiscovery{}).Describe().Id)
}

func TestSubscriptionDescribeTarget(t *testing.T) {
	td := (&subscriptionDiscovery{}).DescribeTarget()
	assert.Equal(t, TargetIDSubscription, td.Id)
	assert.Contains(t, td.Label.One, "event subscription")
}

func TestSubscriptionDescribeAttributes(t *testing.T) {
	require.NotEmpty(t, (&subscriptionDiscovery{}).DescribeAttributes())
}

func TestNewSubscriptionDiscovery(t *testing.T) {
	require.NotNil(t, NewSubscriptionDiscovery())
}

func TestToSubscriptionTarget_HappyPath(t *testing.T) {
	in := map[string]any{
		"id":             "/sub/s/rg/r/Microsoft.EventGrid/eventSubscriptions/sub1",
		"name":           "sub1",
		"subscriptionId": "s",
		"resourceGroup":  "r",
		"location":       "westeurope",
		"properties": map[string]any{
			"provisioningState":  "Succeeded",
			"topic":              "/sub/s/rg/r/Microsoft.EventGrid/topics/t1",
			"destination":        map[string]any{"endpointType": "WebHook"},
			"eventDeliverySchema": "EventGridSchema",
			"retryPolicy":        map[string]any{"maxDeliveryAttempts": float64(30), "eventTimeToLiveInMinutes": float64(1440)},
		},
	}
	got := toSubscriptionTarget(in)
	assert.Equal(t, "sub1", got.Label)
	assert.Equal(t, TargetIDSubscription, got.TargetType)
	assert.Equal(t, []string{"sub1"}, got.Attributes["azure.eventgrid.subscription.name"])
}

func TestGetAllSubscriptions_Error(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	_, err := getAllSubscriptions(context.Background(), rg)
	require.Error(t, err)
}
