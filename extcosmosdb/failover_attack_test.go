/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extcosmosdb

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cosmos/armcosmos/v3"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- mock ---

type cosmosAccountsApiMock struct {
	mock.Mock
}

func (m *cosmosAccountsApiMock) Get(ctx context.Context, resourceGroupName string, accountName string, options *armcosmos.DatabaseAccountsClientGetOptions) (armcosmos.DatabaseAccountsClientGetResponse, error) {
	args := m.Called(ctx, resourceGroupName, accountName, options)
	if args.Get(0) == nil {
		return armcosmos.DatabaseAccountsClientGetResponse{}, args.Error(1)
	}
	return *args.Get(0).(*armcosmos.DatabaseAccountsClientGetResponse), args.Error(1)
}

func (m *cosmosAccountsApiMock) BeginFailoverPriorityChange(ctx context.Context, resourceGroupName string, accountName string, failoverParameters armcosmos.FailoverPolicies, options *armcosmos.DatabaseAccountsClientBeginFailoverPriorityChangeOptions) (*runtime.Poller[armcosmos.DatabaseAccountsClientFailoverPriorityChangeResponse], error) {
	args := m.Called(ctx, resourceGroupName, accountName, failoverParameters, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*runtime.Poller[armcosmos.DatabaseAccountsClientFailoverPriorityChangeResponse]), args.Error(1)
}

// --- helpers ---

func newAttack(client *cosmosAccountsApiMock) *cosmosFailoverAttack {
	return &cosmosFailoverAttack{
		clientProvider: func(string) (cosmosDatabaseAccountsApi, error) { return client, nil },
	}
}

func prepareReq() action_kit_api.PrepareActionRequestBody {
	return extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{},
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"azure.subscription.id":         {"sub-1"},
				"azure.resource-group.name":     {"rg-1"},
				"azure.cosmosdb.account.name":   {"cosmos-1"},
			},
		}),
	})
}

func accountWithPolicies(policies ...struct {
	region   string
	priority int32
}) *armcosmos.DatabaseAccountsClientGetResponse {
	fp := make([]*armcosmos.FailoverPolicy, 0, len(policies))
	for _, p := range policies {
		fp = append(fp, &armcosmos.FailoverPolicy{
			LocationName:     to.Ptr(p.region),
			FailoverPriority: to.Ptr(p.priority),
		})
	}
	return &armcosmos.DatabaseAccountsClientGetResponse{
		DatabaseAccountGetResults: armcosmos.DatabaseAccountGetResults{
			Properties: &armcosmos.DatabaseAccountGetProperties{
				FailoverPolicies: fp,
			},
		},
	}
}

func policy(region string, priority int32) struct {
	region   string
	priority int32
} {
	return struct {
		region   string
		priority int32
	}{region, priority}
}

// --- tests ---

func TestPrepare_HappyPath_SelectsLowestSecondaryAsPromoted(t *testing.T) {
	client := new(cosmosAccountsApiMock)
	// Write region (priority 0) + 2 secondaries (priority 1 + 2). Lowest-priority secondary is the
	// one we promote — that's the region Cosmos itself would pick on automatic failover.
	client.On("Get", mock.Anything, "rg-1", "cosmos-1", mock.Anything).
		Return(accountWithPolicies(policy("westeurope", 0), policy("northeurope", 1), policy("francecentral", 2)), nil)

	a := newAttack(client)
	state := CosmosDbFailoverState{}
	_, err := a.Prepare(context.Background(), &state, prepareReq())
	require.NoError(t, err)
	assert.Equal(t, "northeurope", state.PromotedRegion)
	// NewPolicies must put northeurope at priority 0; westeurope must get the priority that
	// northeurope vacated (1); francecentral stays at 2.
	priorities := map[string]int32{}
	for _, p := range state.NewPolicies {
		priorities[p.LocationName] = p.Priority
	}
	assert.Equal(t, int32(0), priorities["northeurope"])
	assert.Equal(t, int32(1), priorities["westeurope"])
	assert.Equal(t, int32(2), priorities["francecentral"])
}

func TestPrepare_RejectsSingleRegion(t *testing.T) {
	client := new(cosmosAccountsApiMock)
	client.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(accountWithPolicies(policy("westeurope", 0)), nil)
	a := newAttack(client)
	state := CosmosDbFailoverState{}
	_, err := a.Prepare(context.Background(), &state, prepareReq())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fewer than 2 regions")
}

func TestPrepare_RejectsMissingTargetAttributes(t *testing.T) {
	a := newAttack(new(cosmosAccountsApiMock))
	state := CosmosDbFailoverState{}
	req := extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{Attributes: map[string][]string{}}),
	})
	_, err := a.Prepare(context.Background(), &state, req)
	require.Error(t, err)
}

func TestPrepare_PropagatesGetError(t *testing.T) {
	client := new(cosmosAccountsApiMock)
	client.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("ARM 404"))
	a := newAttack(client)
	state := CosmosDbFailoverState{}
	_, err := a.Prepare(context.Background(), &state, prepareReq())
	require.Error(t, err)
}

func TestStart_SubmitsNewPolicies(t *testing.T) {
	client := new(cosmosAccountsApiMock)
	client.On("BeginFailoverPriorityChange", mock.Anything, "rg-1", "cosmos-1",
		mock.MatchedBy(func(fp armcosmos.FailoverPolicies) bool {
			// Expect the two policies we put in state, with priorities swapped.
			byRegion := map[string]int32{}
			for _, p := range fp.FailoverPolicies {
				if p.LocationName == nil || p.FailoverPriority == nil {
					return false
				}
				byRegion[*p.LocationName] = *p.FailoverPriority
			}
			return byRegion["northeurope"] == 0 && byRegion["westeurope"] == 1
		}), mock.Anything).
		Return(nil, nil)

	a := newAttack(client)
	state := CosmosDbFailoverState{
		SubscriptionId:    "sub-1",
		ResourceGroupName: "rg-1",
		AccountName:       "cosmos-1",
		PromotedRegion:    "northeurope",
		NewPolicies: []failoverPolicy{
			{LocationName: "northeurope", Priority: 0},
			{LocationName: "westeurope", Priority: 1},
		},
	}
	_, err := a.Start(context.Background(), &state)
	require.NoError(t, err)
	client.AssertExpectations(t)
}

func TestStart_PropagatesError(t *testing.T) {
	client := new(cosmosAccountsApiMock)
	client.On("BeginFailoverPriorityChange", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("ARM 409 Conflict"))

	a := newAttack(client)
	state := CosmosDbFailoverState{
		SubscriptionId: "sub-1", ResourceGroupName: "rg-1", AccountName: "cosmos-1",
		PromotedRegion: "northeurope",
		NewPolicies:    []failoverPolicy{{LocationName: "northeurope", Priority: 0}, {LocationName: "westeurope", Priority: 1}},
	}
	_, err := a.Start(context.Background(), &state)
	require.Error(t, err)
}

func TestPromotePolicies_PreservesUnrelatedRegions(t *testing.T) {
	// Pure helper test: the swap should only touch the current primary and the promoted secondary.
	original := []failoverPolicy{
		{LocationName: "westeurope", Priority: 0},
		{LocationName: "northeurope", Priority: 1},
		{LocationName: "francecentral", Priority: 2},
	}
	out := promotePolicies(original, "northeurope")
	byRegion := map[string]int32{}
	for _, p := range out {
		byRegion[p.LocationName] = p.Priority
	}
	assert.Equal(t, int32(0), byRegion["northeurope"])
	assert.Equal(t, int32(1), byRegion["westeurope"]) // gets the vacated priority
	assert.Equal(t, int32(2), byRegion["francecentral"]) // untouched
}
