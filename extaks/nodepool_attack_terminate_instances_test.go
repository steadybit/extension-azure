/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extaks

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- mocks ---

type machinesApiMock struct {
	mock.Mock
}

func (m *machinesApiMock) NewListPager(resourceGroupName string, resourceName string, agentPoolName string, options *armcontainerservice.MachinesClientListOptions) *runtime.Pager[armcontainerservice.MachinesClientListResponse] {
	args := m.Called(resourceGroupName, resourceName, agentPoolName, options)
	return runtime.NewPager(runtime.PagingHandler[armcontainerservice.MachinesClientListResponse]{
		More: func(page armcontainerservice.MachinesClientListResponse) bool {
			return page.NextLink != nil && len(*page.NextLink) > 0
		},
		Fetcher: func(_ context.Context, _ *armcontainerservice.MachinesClientListResponse) (armcontainerservice.MachinesClientListResponse, error) {
			if args.Get(0) == nil {
				return armcontainerservice.MachinesClientListResponse{}, args.Error(1)
			}
			return *args.Get(0).(*armcontainerservice.MachinesClientListResponse), nil
		},
	})
}

type agentPoolsApiMock struct {
	mock.Mock
}

func (m *agentPoolsApiMock) BeginDeleteMachines(ctx context.Context, resourceGroupName string, resourceName string, agentPoolName string, machines armcontainerservice.AgentPoolDeleteMachinesParameter, options *armcontainerservice.AgentPoolsClientBeginDeleteMachinesOptions) (*runtime.Poller[armcontainerservice.AgentPoolsClientDeleteMachinesResponse], error) {
	args := m.Called(ctx, resourceGroupName, resourceName, agentPoolName, machines, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*runtime.Poller[armcontainerservice.AgentPoolsClientDeleteMachinesResponse]), args.Error(1)
}

// --- helpers ---

// identityPerm returns a fake rng that yields the identity permutation [0,1,2,...,n-1]. Makes the
// random sampling deterministic for the tests: the first `sampleSize` names are always picked.
func identityPerm(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func machinesPageWithNames(names ...string) *armcontainerservice.MachinesClientListResponse {
	machines := make([]*armcontainerservice.Machine, 0, len(names))
	for _, n := range names {
		machines = append(machines, &armcontainerservice.Machine{Name: to.Ptr(n)})
	}
	return &armcontainerservice.MachinesClientListResponse{
		MachineListResult: armcontainerservice.MachineListResult{Value: machines},
	}
}

func newAttack(machines *machinesApiMock, agentPools *agentPoolsApiMock) *nodePoolTerminateInstancesAttack {
	return &nodePoolTerminateInstancesAttack{
		machinesProvider:   func(string) (MachinesApi, error) { return machines, nil },
		agentPoolsProvider: func(string) (AgentPoolsApi, error) { return agentPools, nil },
		rng:                identityPerm,
	}
}

func prepareReq(percentage int, mode string) action_kit_api.PrepareActionRequestBody {
	return extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{"percentage": percentage},
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"azure.subscription.id":     {"sub-1"},
				"azure.resource-group.name": {"rg-1"},
				"azure.aks.cluster.name":    {"cluster-1"},
				"azure.aks.nodepool.name":   {"pool-1"},
				"azure.aks.nodepool.mode":   {mode},
			},
		}),
	})
}

// --- tests ---

func TestPrepare_HappyPath_SamplesByPercentage(t *testing.T) {
	machines := new(machinesApiMock)
	machines.On("NewListPager", "rg-1", "cluster-1", "pool-1", mock.Anything).
		Return(machinesPageWithNames("a", "b", "c", "d"), nil)
	a := newAttack(machines, new(agentPoolsApiMock))

	state := NodePoolTerminateInstancesState{}
	_, err := a.Prepare(context.Background(), &state, prepareReq(50, "User"))
	require.NoError(t, err)
	// 50% of 4 = 2 machines. identityPerm picks the first 2 (after sort: a,b,c,d).
	assert.Equal(t, []string{"a", "b"}, state.MachineNames)
	assert.Equal(t, 50, state.Percentage)
	assert.Equal(t, "sub-1", state.SubscriptionId)
	assert.Equal(t, "cluster-1", state.ClusterName)
}

func TestPrepare_RoundsUpToAtLeastOne(t *testing.T) {
	machines := new(machinesApiMock)
	machines.On("NewListPager", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(machinesPageWithNames("a", "b", "c"), nil)
	a := newAttack(machines, new(agentPoolsApiMock))

	state := NodePoolTerminateInstancesState{}
	// 1% of 3 = 0.03 → must round up to 1.
	_, err := a.Prepare(context.Background(), &state, prepareReq(1, "User"))
	require.NoError(t, err)
	assert.Len(t, state.MachineNames, 1)
}

func TestPrepare_RejectsPercentageOutOfRange(t *testing.T) {
	a := newAttack(new(machinesApiMock), new(agentPoolsApiMock))

	state := NodePoolTerminateInstancesState{}
	_, err := a.Prepare(context.Background(), &state, prepareReq(0, "User"))
	require.Error(t, err)

	state = NodePoolTerminateInstancesState{}
	_, err = a.Prepare(context.Background(), &state, prepareReq(101, "User"))
	require.Error(t, err)
}

func TestPrepare_RejectsEmptyMachineList(t *testing.T) {
	machines := new(machinesApiMock)
	machines.On("NewListPager", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(machinesPageWithNames(), nil) // empty
	a := newAttack(machines, new(agentPoolsApiMock))

	state := NodePoolTerminateInstancesState{}
	_, err := a.Prepare(context.Background(), &state, prepareReq(33, "User"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no machines")
}

func TestPrepare_RejectsDeleteAllOfSystemPool(t *testing.T) {
	// 1 system node + 100% percentage → would terminate the only system node → AKS forbids it.
	// We catch it at Prepare so the user sees an actionable error before the API call fails.
	machines := new(machinesApiMock)
	machines.On("NewListPager", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(machinesPageWithNames("the-only-system-node"), nil)
	a := newAttack(machines, new(agentPoolsApiMock))

	state := NodePoolTerminateInstancesState{}
	_, err := a.Prepare(context.Background(), &state, prepareReq(100, "System"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "system node pool")
	assert.Contains(t, err.Error(), "Reduce the percentage")
}

func TestPrepare_AllowsDeleteAllOfUserPool(t *testing.T) {
	// AKS only forbids deleting all nodes of a *system* pool — user pools can be drained to zero.
	machines := new(machinesApiMock)
	machines.On("NewListPager", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(machinesPageWithNames("user-node"), nil)
	a := newAttack(machines, new(agentPoolsApiMock))

	state := NodePoolTerminateInstancesState{}
	_, err := a.Prepare(context.Background(), &state, prepareReq(100, "User"))
	require.NoError(t, err)
	assert.Equal(t, []string{"user-node"}, state.MachineNames)
}

func TestStart_DelegatesToBeginDeleteMachines(t *testing.T) {
	agentPools := new(agentPoolsApiMock)
	agentPools.On("BeginDeleteMachines", mock.Anything, "rg-1", "cluster-1", "pool-1",
		mock.MatchedBy(func(p armcontainerservice.AgentPoolDeleteMachinesParameter) bool {
			if len(p.MachineNames) != 2 {
				return false
			}
			got := []string{*p.MachineNames[0], *p.MachineNames[1]}
			return got[0] == "a" && got[1] == "b"
		}), mock.Anything).
		Return(nil, nil) // Poller can be nil; the attack only checks err.
	a := newAttack(new(machinesApiMock), agentPools)

	state := NodePoolTerminateInstancesState{
		SubscriptionId:    "sub-1",
		ResourceGroupName: "rg-1",
		ClusterName:       "cluster-1",
		NodePoolName:      "pool-1",
		MachineNames:      []string{"a", "b"},
	}
	_, err := a.Start(context.Background(), &state)
	require.NoError(t, err)
	agentPools.AssertExpectations(t)
}

func TestStart_PropagatesError(t *testing.T) {
	agentPools := new(agentPoolsApiMock)
	agentPools.On("BeginDeleteMachines", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("ARM 403 Forbidden"))
	a := newAttack(new(machinesApiMock), agentPools)

	state := NodePoolTerminateInstancesState{
		SubscriptionId: "sub-1", ResourceGroupName: "rg-1",
		ClusterName: "cluster-1", NodePoolName: "pool-1",
		MachineNames: []string{"a"},
	}
	_, err := a.Start(context.Background(), &state)
	require.Error(t, err)
}

func TestStart_RejectsEmptyMachineList(t *testing.T) {
	// Defensive: if state was constructed wrong, Start should refuse rather than send an empty delete.
	a := newAttack(new(machinesApiMock), new(agentPoolsApiMock))
	state := NodePoolTerminateInstancesState{
		SubscriptionId: "sub-1", ResourceGroupName: "rg-1",
		ClusterName: "cluster-1", NodePoolName: "pool-1",
		MachineNames: []string{},
	}
	_, err := a.Start(context.Background(), &state)
	require.Error(t, err)
}
