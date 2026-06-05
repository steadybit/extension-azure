/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package appcontainers

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

func TestAppContainerDescribe(t *testing.T) {
	assert.Equal(t, TargetIDContainerApp, (&appContainerDiscovery{}).Describe().Id)
}

func TestAppContainerDescribeTarget(t *testing.T) {
	td := (&appContainerDiscovery{}).DescribeTarget()
	assert.Equal(t, TargetIDContainerApp, td.Id)
}

func TestAppContainerDescribeAttributes(t *testing.T) {
	require.NotEmpty(t, (&appContainerDiscovery{}).DescribeAttributes())
}

func TestNewAppContainerDiscovery(t *testing.T) {
	require.NotNil(t, NewAppContainerDiscovery())
}

func TestSafeToString(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"bool", true, "true"},
		{"nil", nil, ""},
		{"float", 3.14, "3.14"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, safeToString(tt.in))
		})
	}
}

func TestGetAllContainerApps_Error(t *testing.T) {
	rg := new(rgClientMock)
	rg.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	_, err := getAllContainerApps(context.Background(), rg)
	require.Error(t, err)
}

func TestInjectExceptionDescription(t *testing.T) {
	d := getInjectExceptionDescription()
	assert.Equal(t, TargetIDContainerApp, d.TargetSelection.TargetType)
	assert.NotEmpty(t, d.Id)
	assert.NotEmpty(t, d.Label)
}

func TestInjectLatencyDescription(t *testing.T) {
	d := getInjectLatencyDescription()
	assert.Equal(t, TargetIDContainerApp, d.TargetSelection.TargetType)
	assert.NotEmpty(t, d.Id)
}

func TestInjectFillDiskDescription(t *testing.T) {
	d := getInjectFillDiskDescription()
	assert.Equal(t, TargetIDContainerApp, d.TargetSelection.TargetType)
}

func TestInjectStatusCodeDescription(t *testing.T) {
	d := getInjectStatusCodeDescription()
	assert.Equal(t, TargetIDContainerApp, d.TargetSelection.TargetType)
}

func TestNewAppContainerExceptionAction(t *testing.T) {
	require.NotNil(t, NewAppContainerExceptionAction())
}

func TestNewAppContainerLatencyAction(t *testing.T) {
	require.NotNil(t, NewAppContainerLatencyAction())
}

func TestNewAppContainerFillDiskAction(t *testing.T) {
	require.NotNil(t, NewAppContainerFillDiskAction())
}

func TestNewAppContainerStatusCodeAction(t *testing.T) {
	require.NotNil(t, NewAppContainerStatusCodeAction())
}
