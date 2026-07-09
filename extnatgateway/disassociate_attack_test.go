/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extnatgateway

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- mock ---

type subnetsApiMock struct {
	mock.Mock
}

func (m *subnetsApiMock) Get(ctx context.Context, resourceGroupName string, virtualNetworkName string, subnetName string, options *armnetwork.SubnetsClientGetOptions) (armnetwork.SubnetsClientGetResponse, error) {
	args := m.Called(ctx, resourceGroupName, virtualNetworkName, subnetName, options)
	if args.Get(0) == nil {
		return armnetwork.SubnetsClientGetResponse{}, args.Error(1)
	}
	return *args.Get(0).(*armnetwork.SubnetsClientGetResponse), args.Error(1)
}

func (m *subnetsApiMock) BeginCreateOrUpdate(ctx context.Context, resourceGroupName string, virtualNetworkName string, subnetName string, subnetParameters armnetwork.Subnet, options *armnetwork.SubnetsClientBeginCreateOrUpdateOptions) (*runtime.Poller[armnetwork.SubnetsClientCreateOrUpdateResponse], error) {
	args := m.Called(ctx, resourceGroupName, virtualNetworkName, subnetName, subnetParameters, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*runtime.Poller[armnetwork.SubnetsClientCreateOrUpdateResponse]), args.Error(1)
}

// --- helpers ---

func newAttack(client *subnetsApiMock) *natGatewayDisassociateAttack {
	return &natGatewayDisassociateAttack{
		subnetsClientProvider: func(string) (subnetsApi, error) { return client, nil },
	}
}

func subnetIDFor(subnet string) string {
	return "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Network/virtualNetworks/vnet-1/subnets/" + subnet
}

func prepareReqWithSubnets(natGwName string, subnetIDs ...string) action_kit_api.PrepareActionRequestBody {
	return extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]any{"duration": "60s"},
		Target: new(action_kit_api.Target{
			Attributes: map[string][]string{
				"azure.subscription.id":     {"sub-1"},
				"azure.resource-group.name": {"rg-1"},
				"azure.nat-gateway.name":    {natGwName},
				"azure.nat-gateway.subnets": subnetIDs,
			},
		}),
	})
}

// subnetResp helps build a SubnetsClientGetResponse with a NatGateway reference set.
func subnetRespWithNatGateway(natGwID string) *armnetwork.SubnetsClientGetResponse {
	return &armnetwork.SubnetsClientGetResponse{
		Subnet: armnetwork.Subnet{
			Properties: &armnetwork.SubnetPropertiesFormat{
				NatGateway: &armnetwork.SubResource{ID: new(natGwID)},
			},
		},
	}
}

// --- tests ---

func TestPrepare_HappyPath_CapturesSubnetRefs(t *testing.T) {
	a := newAttack(new(subnetsApiMock))
	state := NatGatewayDisassociateState{}
	_, err := a.Prepare(context.Background(), &state, prepareReqWithSubnets("ngw-1", subnetIDFor("snet-a"), subnetIDFor("snet-b")))
	require.NoError(t, err)
	assert.Equal(t, "ngw-1", state.NatGatewayName)
	assert.Equal(t, "sub-1", state.SubscriptionId)
	assert.Equal(t, "rg-1", state.ResourceGroupName)
	assert.Equal(t, []string{subnetIDFor("snet-a"), subnetIDFor("snet-b")}, state.SubnetRefs)
	assert.Equal(t,
		"/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Network/natGateways/ngw-1",
		state.NatGatewayId)
}

func TestPrepare_RejectsEmptySubnets(t *testing.T) {
	// If the NAT GW is not currently associated with any subnet, the attack would be a no-op —
	// reject at Prepare with a clear error.
	a := newAttack(new(subnetsApiMock))
	state := NatGatewayDisassociateState{}
	_, err := a.Prepare(context.Background(), &state, prepareReqWithSubnets("ngw-1"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no associated subnets")
}

func TestStart_DisassociatesEachSubnet(t *testing.T) {
	client := new(subnetsApiMock)
	// Get returns current subnet (with NAT GW set); BeginCreateOrUpdate must be called with NatGateway=nil.
	client.On("Get", mock.Anything, "rg-1", "vnet-1", "snet-a", mock.Anything).
		Return(subnetRespWithNatGateway("ngw-id"), nil)
	client.On("BeginCreateOrUpdate", mock.Anything, "rg-1", "vnet-1", "snet-a",
		mock.MatchedBy(func(s armnetwork.Subnet) bool {
			return s.Properties != nil && s.Properties.NatGateway == nil
		}), mock.Anything).Return(nil, nil)

	a := newAttack(client)
	state := NatGatewayDisassociateState{
		SubscriptionId:    "sub-1",
		ResourceGroupName: "rg-1",
		NatGatewayName:    "ngw-1",
		SubnetRefs:        []string{subnetIDFor("snet-a")},
	}
	_, err := a.Start(context.Background(), &state)
	require.NoError(t, err)
	client.AssertExpectations(t)
}

func TestStop_ReassociatesEachSubnet(t *testing.T) {
	client := new(subnetsApiMock)
	// Subnet has been disassociated (NatGateway=nil) at Start; Stop must put it back.
	client.On("Get", mock.Anything, "rg-1", "vnet-1", "snet-a", mock.Anything).
		Return(&armnetwork.SubnetsClientGetResponse{Subnet: armnetwork.Subnet{Properties: &armnetwork.SubnetPropertiesFormat{}}}, nil)
	wantNgID := "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Network/natGateways/ngw-1"
	client.On("BeginCreateOrUpdate", mock.Anything, "rg-1", "vnet-1", "snet-a",
		mock.MatchedBy(func(s armnetwork.Subnet) bool {
			return s.Properties != nil &&
				s.Properties.NatGateway != nil &&
				s.Properties.NatGateway.ID != nil &&
				*s.Properties.NatGateway.ID == wantNgID
		}), mock.Anything).Return(nil, nil)

	a := newAttack(client)
	state := NatGatewayDisassociateState{
		SubscriptionId:    "sub-1",
		ResourceGroupName: "rg-1",
		NatGatewayName:    "ngw-1",
		NatGatewayId:      wantNgID,
		SubnetRefs:        []string{subnetIDFor("snet-a")},
	}
	_, err := a.Stop(context.Background(), &state)
	require.NoError(t, err)
	client.AssertExpectations(t)
}

func TestStart_PropagatesError(t *testing.T) {
	client := new(subnetsApiMock)
	client.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("ARM 403 Forbidden"))

	a := newAttack(client)
	state := NatGatewayDisassociateState{
		SubscriptionId: "sub-1", ResourceGroupName: "rg-1",
		NatGatewayName: "ngw-1",
		SubnetRefs:     []string{subnetIDFor("snet-a")},
	}
	_, err := a.Start(context.Background(), &state)
	require.Error(t, err)
}

func TestStart_SkipsUnparseableSubnetRefs(t *testing.T) {
	// If we somehow end up with a malformed subnet ID, Start should skip it gracefully rather than
	// fail the whole attack — at least the valid subnets get disassociated.
	client := new(subnetsApiMock)
	client.On("Get", mock.Anything, "rg-1", "vnet-1", "snet-valid", mock.Anything).
		Return(subnetRespWithNatGateway("ngw-id"), nil)
	client.On("BeginCreateOrUpdate", mock.Anything, "rg-1", "vnet-1", "snet-valid", mock.Anything, mock.Anything).
		Return(nil, nil)

	a := newAttack(client)
	state := NatGatewayDisassociateState{
		SubscriptionId:    "sub-1",
		ResourceGroupName: "rg-1",
		NatGatewayName:    "ngw-1",
		SubnetRefs:        []string{"not-a-valid-arm-id", subnetIDFor("snet-valid")},
	}
	_, err := a.Start(context.Background(), &state)
	require.NoError(t, err)
	client.AssertExpectations(t)
}
