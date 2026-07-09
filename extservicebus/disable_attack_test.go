/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extservicebus

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/servicebus/armservicebus"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- mocks ---

type queuesApiMock struct {
	mock.Mock
}

func (m *queuesApiMock) Get(ctx context.Context, rg, ns, q string, options *armservicebus.QueuesClientGetOptions) (armservicebus.QueuesClientGetResponse, error) {
	args := m.Called(ctx, rg, ns, q, options)
	if args.Get(0) == nil {
		return armservicebus.QueuesClientGetResponse{}, args.Error(1)
	}
	return *args.Get(0).(*armservicebus.QueuesClientGetResponse), args.Error(1)
}

func (m *queuesApiMock) CreateOrUpdate(ctx context.Context, rg, ns, q string, params armservicebus.SBQueue, options *armservicebus.QueuesClientCreateOrUpdateOptions) (armservicebus.QueuesClientCreateOrUpdateResponse, error) {
	args := m.Called(ctx, rg, ns, q, params, options)
	if args.Get(0) == nil {
		return armservicebus.QueuesClientCreateOrUpdateResponse{}, args.Error(1)
	}
	return *args.Get(0).(*armservicebus.QueuesClientCreateOrUpdateResponse), args.Error(1)
}

type topicsApiMock struct {
	mock.Mock
}

func (m *topicsApiMock) Get(ctx context.Context, rg, ns, tp string, options *armservicebus.TopicsClientGetOptions) (armservicebus.TopicsClientGetResponse, error) {
	args := m.Called(ctx, rg, ns, tp, options)
	if args.Get(0) == nil {
		return armservicebus.TopicsClientGetResponse{}, args.Error(1)
	}
	return *args.Get(0).(*armservicebus.TopicsClientGetResponse), args.Error(1)
}

func (m *topicsApiMock) CreateOrUpdate(ctx context.Context, rg, ns, tp string, params armservicebus.SBTopic, options *armservicebus.TopicsClientCreateOrUpdateOptions) (armservicebus.TopicsClientCreateOrUpdateResponse, error) {
	args := m.Called(ctx, rg, ns, tp, params, options)
	if args.Get(0) == nil {
		return armservicebus.TopicsClientCreateOrUpdateResponse{}, args.Error(1)
	}
	return *args.Get(0).(*armservicebus.TopicsClientCreateOrUpdateResponse), args.Error(1)
}

// --- helpers ---

func newQueueAttack(client *queuesApiMock) *queueDisableAttack {
	return &queueDisableAttack{
		clientProvider: func(string) (queuesApi, error) { return client, nil },
	}
}

func newTopicAttack(client *topicsApiMock) *topicDisableAttack {
	return &topicDisableAttack{
		clientProvider: func(string) (topicsApi, error) { return client, nil },
	}
}

func queuePrepareReq() action_kit_api.PrepareActionRequestBody {
	return extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]any{"duration": "60s"},
		Target: new(action_kit_api.Target{
			Attributes: map[string][]string{
				"azure.subscription.id":           {"sub-1"},
				"azure.resource-group.name":       {"rg-1"},
				"azure.servicebus.namespace.name": {"ns-1"},
				"azure.servicebus.queue.name":     {"queue-1"},
			},
		}),
	})
}

func topicPrepareReq() action_kit_api.PrepareActionRequestBody {
	return extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]any{"duration": "60s"},
		Target: new(action_kit_api.Target{
			Attributes: map[string][]string{
				"azure.subscription.id":           {"sub-1"},
				"azure.resource-group.name":       {"rg-1"},
				"azure.servicebus.namespace.name": {"ns-1"},
				"azure.servicebus.topic.name":     {"topic-1"},
			},
		}),
	})
}

func queueResp(status armservicebus.EntityStatus) *armservicebus.QueuesClientGetResponse {
	return &armservicebus.QueuesClientGetResponse{
		SBQueue: armservicebus.SBQueue{
			Properties: &armservicebus.SBQueueProperties{Status: new(status)},
		},
	}
}

func topicResp(status armservicebus.EntityStatus) *armservicebus.TopicsClientGetResponse {
	return &armservicebus.TopicsClientGetResponse{
		SBTopic: armservicebus.SBTopic{
			Properties: &armservicebus.SBTopicProperties{Status: new(status)},
		},
	}
}

// --- queue tests ---

func TestQueue_Prepare_CapturesOriginalStatus(t *testing.T) {
	client := new(queuesApiMock)
	client.On("Get", mock.Anything, "rg-1", "ns-1", "queue-1", mock.Anything).
		Return(queueResp(armservicebus.EntityStatusActive), nil)
	a := newQueueAttack(client)
	state := EntityDisableState{}
	_, err := a.Prepare(context.Background(), &state, queuePrepareReq())
	require.NoError(t, err)
	assert.Equal(t, "Active", state.OriginalStatus)
	assert.Equal(t, "ns-1", state.NamespaceName)
	assert.Equal(t, "queue-1", state.EntityName)
}

func TestQueue_Prepare_DefaultsToActiveWhenStatusMissing(t *testing.T) {
	// Defensive: if the SDK returns a queue with no Status field (newly created etc.), default to
	// "Active" rather than empty so Stop can still restore to a valid status.
	client := new(queuesApiMock)
	client.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&armservicebus.QueuesClientGetResponse{SBQueue: armservicebus.SBQueue{}}, nil)
	a := newQueueAttack(client)
	state := EntityDisableState{}
	_, err := a.Prepare(context.Background(), &state, queuePrepareReq())
	require.NoError(t, err)
	assert.Equal(t, "Active", state.OriginalStatus)
}

func TestQueue_Start_SetsStatusDisabled(t *testing.T) {
	client := new(queuesApiMock)
	// Start re-fetches, then writes back with Status=Disabled.
	client.On("Get", mock.Anything, "rg-1", "ns-1", "queue-1", mock.Anything).
		Return(queueResp(armservicebus.EntityStatusActive), nil)
	client.On("CreateOrUpdate", mock.Anything, "rg-1", "ns-1", "queue-1",
		mock.MatchedBy(func(q armservicebus.SBQueue) bool {
			return q.Properties != nil && q.Properties.Status != nil && *q.Properties.Status == armservicebus.EntityStatusDisabled
		}), mock.Anything).Return(nil, nil)

	a := newQueueAttack(client)
	state := EntityDisableState{SubscriptionId: "sub-1", ResourceGroupName: "rg-1", NamespaceName: "ns-1", EntityName: "queue-1", OriginalStatus: "Active"}
	_, err := a.Start(context.Background(), &state)
	require.NoError(t, err)
	client.AssertExpectations(t)
}

func TestQueue_Stop_RestoresOriginalStatus(t *testing.T) {
	client := new(queuesApiMock)
	client.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(queueResp(armservicebus.EntityStatusDisabled), nil)
	client.On("CreateOrUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.MatchedBy(func(q armservicebus.SBQueue) bool {
			return q.Properties != nil && q.Properties.Status != nil && *q.Properties.Status == armservicebus.EntityStatusActive
		}), mock.Anything).Return(nil, nil)

	a := newQueueAttack(client)
	state := EntityDisableState{SubscriptionId: "sub-1", ResourceGroupName: "rg-1", NamespaceName: "ns-1", EntityName: "queue-1", OriginalStatus: "Active"}
	_, err := a.Stop(context.Background(), &state)
	require.NoError(t, err)
	client.AssertExpectations(t)
}

func TestQueue_Start_PropagatesError(t *testing.T) {
	client := new(queuesApiMock)
	client.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("ARM 403"))
	a := newQueueAttack(client)
	state := EntityDisableState{SubscriptionId: "sub-1", ResourceGroupName: "rg-1", NamespaceName: "ns-1", EntityName: "queue-1"}
	_, err := a.Start(context.Background(), &state)
	require.Error(t, err)
}

// --- topic tests ---

func TestTopic_Prepare_CapturesOriginalStatus(t *testing.T) {
	client := new(topicsApiMock)
	client.On("Get", mock.Anything, "rg-1", "ns-1", "topic-1", mock.Anything).
		Return(topicResp(armservicebus.EntityStatusActive), nil)
	a := newTopicAttack(client)
	state := EntityDisableState{}
	_, err := a.Prepare(context.Background(), &state, topicPrepareReq())
	require.NoError(t, err)
	assert.Equal(t, "Active", state.OriginalStatus)
	assert.Equal(t, "topic-1", state.EntityName)
}

func TestTopic_Start_SetsStatusDisabled(t *testing.T) {
	client := new(topicsApiMock)
	client.On("Get", mock.Anything, "rg-1", "ns-1", "topic-1", mock.Anything).
		Return(topicResp(armservicebus.EntityStatusActive), nil)
	client.On("CreateOrUpdate", mock.Anything, "rg-1", "ns-1", "topic-1",
		mock.MatchedBy(func(tp armservicebus.SBTopic) bool {
			return tp.Properties != nil && tp.Properties.Status != nil && *tp.Properties.Status == armservicebus.EntityStatusDisabled
		}), mock.Anything).Return(nil, nil)

	a := newTopicAttack(client)
	state := EntityDisableState{SubscriptionId: "sub-1", ResourceGroupName: "rg-1", NamespaceName: "ns-1", EntityName: "topic-1", OriginalStatus: "Active"}
	_, err := a.Start(context.Background(), &state)
	require.NoError(t, err)
	client.AssertExpectations(t)
}

func TestTopic_Stop_RestoresOriginalStatus(t *testing.T) {
	client := new(topicsApiMock)
	client.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(topicResp(armservicebus.EntityStatusDisabled), nil)
	client.On("CreateOrUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.MatchedBy(func(tp armservicebus.SBTopic) bool {
			return tp.Properties != nil && tp.Properties.Status != nil && *tp.Properties.Status == armservicebus.EntityStatusActive
		}), mock.Anything).Return(nil, nil)

	a := newTopicAttack(client)
	state := EntityDisableState{SubscriptionId: "sub-1", ResourceGroupName: "rg-1", NamespaceName: "ns-1", EntityName: "topic-1", OriginalStatus: "Active"}
	_, err := a.Stop(context.Background(), &state)
	require.NoError(t, err)
	client.AssertExpectations(t)
}
