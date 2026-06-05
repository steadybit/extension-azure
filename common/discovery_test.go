/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package common

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type rgClientMock struct {
	mock.Mock
}

func (m *rgClientMock) Resources(ctx context.Context, q armresourcegraph.QueryRequest, o *armresourcegraph.ClientResourcesOptions) (armresourcegraph.ClientResourcesResponse, error) {
	args := m.Called(ctx, q, o)
	if r := args.Get(0); r != nil {
		return *(r.(*armresourcegraph.ClientResourcesResponse)), args.Error(1)
	}
	return armresourcegraph.ClientResourcesResponse{}, args.Error(1)
}

func rgResponseWith(rows ...map[string]any) *armresourcegraph.ClientResourcesResponse {
	var total = int64(len(rows))
	data := make([]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, r)
	}
	return &armresourcegraph.ClientResourcesResponse{
		QueryResponse: armresourcegraph.QueryResponse{
			TotalRecords: &total,
			Data:         data,
		},
	}
}

func TestDiscoverViaResourceGraph_HappyPath(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponseWith(
			map[string]any{"id": "/sub/1/r1", "name": "r1"},
			map[string]any{"id": "/sub/1/r2", "name": "r2"},
		), nil)

	toTarget := func(m map[string]any) discovery_kit_api.Target {
		return discovery_kit_api.Target{Id: m["id"].(string), Label: m["name"].(string), TargetType: "x"}
	}

	targets, err := DiscoverViaResourceGraph(context.Background(), rg, "Resources | take 1", toTarget)
	require.NoError(t, err)
	require.Len(t, targets, 2)
	assert.Equal(t, "r1", targets[0].Label)
	assert.Equal(t, "r2", targets[1].Label)
}

func TestDiscoverViaResourceGraph_ResourceGraphError(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("rg blew up"))

	_, err := DiscoverViaResourceGraph(context.Background(), rg, "q", func(map[string]any) discovery_kit_api.Target {
		return discovery_kit_api.Target{}
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rg blew up")
}

func TestDiscoverViaResourceGraph_EmptyRows(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(rgResponseWith(), nil)

	targets, err := DiscoverViaResourceGraph(context.Background(), rg, "q", func(map[string]any) discovery_kit_api.Target {
		return discovery_kit_api.Target{}
	})
	require.NoError(t, err)
	assert.NotNil(t, targets, "should return empty slice, not nil")
	assert.Len(t, targets, 0)
}

func TestDiscoverViaResourceGraph_NonSliceDataIsTolerated(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(&armresourcegraph.ClientResourcesResponse{
			QueryResponse: armresourcegraph.QueryResponse{Data: "not a slice"},
		}, nil)

	targets, err := DiscoverViaResourceGraph(context.Background(), rg, "q", func(map[string]any) discovery_kit_api.Target {
		return discovery_kit_api.Target{}
	})
	require.NoError(t, err)
	assert.NotNil(t, targets)
	assert.Len(t, targets, 0)
}

func TestDiscoverViaResourceGraph_SkipsNonMapRows(t *testing.T) {
	rg := new(rgClientMock)
	var total int64 = 2
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).
		Return(&armresourcegraph.ClientResourcesResponse{
			QueryResponse: armresourcegraph.QueryResponse{
				TotalRecords: &total,
				Data: []any{
					"not-a-map",
					map[string]any{"id": "/sub/1/r1", "name": "r1"},
				},
			},
		}, nil)

	targets, err := DiscoverViaResourceGraph(context.Background(), rg, "q", func(m map[string]any) discovery_kit_api.Target {
		return discovery_kit_api.Target{Id: m["id"].(string)}
	})
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Equal(t, "/sub/1/r1", targets[0].Id)
}

func TestDiscoverViaResourceGraph_ScopesToSubscriptionEnvVar(t *testing.T) {
	t.Setenv("AZURE_SUBSCRIPTION_ID", "sub-explicit")
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.MatchedBy(func(q armresourcegraph.QueryRequest) bool {
		return len(q.Subscriptions) == 1 && *q.Subscriptions[0] == "sub-explicit"
	}), mock.Anything).Return(rgResponseWith(), nil)

	_, err := DiscoverViaResourceGraph(context.Background(), rg, "q", func(map[string]any) discovery_kit_api.Target {
		return discovery_kit_api.Target{}
	})
	require.NoError(t, err)
	rg.AssertExpectations(t)
}

func TestDiscoverViaResourceGraph_NoSubscriptionEnvVarRunsCrossSubscription(t *testing.T) {
	t.Setenv("AZURE_SUBSCRIPTION_ID", "")
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.MatchedBy(func(q armresourcegraph.QueryRequest) bool {
		return q.Subscriptions == nil
	}), mock.Anything).Return(rgResponseWith(), nil)

	_, err := DiscoverViaResourceGraph(context.Background(), rg, "q", func(map[string]any) discovery_kit_api.Target {
		return discovery_kit_api.Target{}
	})
	require.NoError(t, err)
	rg.AssertExpectations(t)
}

func TestStringFromMap(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want string
	}{
		{"present and string", map[string]any{"k": "v"}, "k", "v"},
		{"present but not string", map[string]any{"k": 42}, "k", ""},
		{"absent", map[string]any{}, "k", ""},
		{"nil value", map[string]any{"k": nil}, "k", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, StringFromMap(tt.m, tt.key))
		})
	}
}

func TestStringSliceFromMap(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want []string
	}{
		{"happy path", map[string]any{"k": []any{"a", "b"}}, "k", []string{"a", "b"}},
		{"skips empty and non-string entries", map[string]any{"k": []any{"a", "", 42, "b"}}, "k", []string{"a", "b"}},
		{"absent key returns nil", map[string]any{}, "k", nil},
		{"not a slice returns nil", map[string]any{"k": "scalar"}, "k", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, StringSliceFromMap(tt.m, tt.key))
		})
	}
}
