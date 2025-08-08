package nsg

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/stretchr/testify/assert"
)

func TestBlockAction_Prepare_Success(t *testing.T) {
	action := NewBlockAction().(*blockAction)
	state := &BlockActionState{}
	request := action_kit_api.PrepareActionRequestBody{
		Target: &action_kit_api.Target{
			Attributes: map[string][]string{
				"network-security-group.id": {"/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/networkSecurityGroups/test-nsg"},
			},
		},
		Config: map[string]interface{}{
			"hosts":     []interface{}{"192.168.1.1"},
			"direction": string(BlockInbound),
		},
	}

	result, err := action.Prepare(context.Background(), state, request)

	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/networkSecurityGroups/test-nsg", state.ResourceId)
	assert.Equal(t, "test-rg", state.ResourceGroupName)
	assert.Equal(t, "test-nsg", state.NetworkSecurityGroupName)
	assert.NotNil(t, state.Config)
	assert.Equal(t, armnetwork.SecurityRuleDirectionInbound, state.Config.BlockDirection)
}

func TestBlockAction_Prepare_MissingResourceId(t *testing.T) {
	action := NewBlockAction().(*blockAction)
	state := &BlockActionState{}
	request := action_kit_api.PrepareActionRequestBody{
		Target: &action_kit_api.Target{
			Attributes: map[string][]string{},
		},
		Config: map[string]interface{}{
			"hosts":     []interface{}{"192.168.1.1"},
			"direction": string(BlockInbound),
		},
	}

	result, err := action.Prepare(context.Background(), state, request)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "target is missing 'network-security-group.id' attribute")
}

func TestBlockAction_Prepare_InvalidResourceIdFormat(t *testing.T) {
	action := NewBlockAction().(*blockAction)
	state := &BlockActionState{}
	request := action_kit_api.PrepareActionRequestBody{
		Target: &action_kit_api.Target{
			Attributes: map[string][]string{
				"network-security-group.id": {"invalid-resource-id"},
			},
		},
		Config: map[string]interface{}{
			"hosts":     []interface{}{"192.168.1.1"},
			"direction": string(BlockInbound),
		},
	}

	result, err := action.Prepare(context.Background(), state, request)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid resource id format")
}

func TestBlockHostsConfig_BlockDirection(t *testing.T) {
	tests := []struct {
		name              string
		inputDirection    string
		expectedDirection armnetwork.SecurityRuleDirection
		expectError       bool
	}{
		{
			name:              "inbound direction",
			inputDirection:    string(BlockInbound),
			expectedDirection: armnetwork.SecurityRuleDirectionInbound,
			expectError:       false,
		},
		{
			name:              "outbound direction",
			inputDirection:    string(BlockOutbound),
			expectedDirection: armnetwork.SecurityRuleDirectionOutbound,
			expectError:       false,
		},
		{
			name:           "invalid direction",
			inputDirection: "sideways",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"hosts":     []interface{}{"192.168.1.1"},
					"direction": tt.inputDirection,
				},
			}

			config, err := injectBlock(request)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
				assert.Equal(t, tt.expectedDirection, config.BlockDirection)
			}
		})
	}
}

func TestBlockAction_IPAddressHandling(t *testing.T) {
	tests := []struct {
		name          string
		inputHosts    []interface{}
		expectError   bool
		expectedCount int
	}{
		{
			name:          "single IP address",
			inputHosts:    []interface{}{"192.168.1.1"},
			expectError:   false,
			expectedCount: 1,
		},
		{
			name:          "multiple IP addresses",
			inputHosts:    []interface{}{"192.168.1.1", "10.0.0.1", "172.16.0.1"},
			expectError:   false,
			expectedCount: 3,
		},
		{
			name:          "IPv6 address",
			inputHosts:    []interface{}{"2001:db8::1"},
			expectError:   false,
			expectedCount: 1,
		},
		{
			name:          "mixed IPv4 and IPv6",
			inputHosts:    []interface{}{"192.168.1.1", "2001:db8::1"},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name:        "invalid IP format",
			inputHosts:  []interface{}{"not.an.ip.address"},
			expectError: true,
		},
		{
			name:        "invalid Domain",
			inputHosts:  []interface{}{"not-a-domain"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"hosts":     tt.inputHosts,
					"direction": string(BlockInbound),
				},
			}

			config, err := injectBlock(request)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
				assert.NotNil(t, config.BlockedIPs)
				assert.Equal(t, tt.expectedCount, len(*config.BlockedIPs))
			}
		})
	}
}
