/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package register

import (
	"net/http"
	"os"
	"testing"

	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-azure/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterHandlers_WithEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name                   string
		envVars                map[string]string
		expectedDiscoveryCount int
		expectedActionCount    int
		description            string
	}{
		{
			name: "all features disabled",
			envVars: map[string]string{
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_VIRTUAL_MACHINES":        "false",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_SCALE_INSTANCES":         "false",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_AZURE_FUNCTIONS":         "false",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_NETWORK_SECURITY_GROUPS": "false",
			},
			expectedDiscoveryCount: 0,
			expectedActionCount:    0,
			description:            "When all discovery features are disabled, no handlers should be registered",
		},
		{
			name: "only virtual machines enabled",
			envVars: map[string]string{
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_VIRTUAL_MACHINES":        "true",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_SCALE_INSTANCES":         "false",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_AZURE_FUNCTIONS":         "false",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_NETWORK_SECURITY_GROUPS": "false",
			},
			expectedDiscoveryCount: 1,
			expectedActionCount:    1,
			description:            "When only VMs are enabled, should register VM discovery and state action",
		},
		{
			name: "only scale instances enabled",
			envVars: map[string]string{
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_VIRTUAL_MACHINES":        "false",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_SCALE_INSTANCES":         "true",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_AZURE_FUNCTIONS":         "false",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_NETWORK_SECURITY_GROUPS": "false",
			},
			expectedDiscoveryCount: 1,
			expectedActionCount:    1,
			description:            "When only scale instances are enabled, should register scale set discovery and state action",
		},
		{
			name: "only azure functions enabled",
			envVars: map[string]string{
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_VIRTUAL_MACHINES":        "false",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_SCALE_INSTANCES":         "false",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_AZURE_FUNCTIONS":         "true",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_NETWORK_SECURITY_GROUPS": "false",
			},
			expectedDiscoveryCount: 1,
			expectedActionCount:    4,
			description:            "When only Azure Functions are enabled, should register function discovery and 4 actions",
		},
		{
			name: "only network security groups enabled",
			envVars: map[string]string{
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_VIRTUAL_MACHINES":        "false",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_SCALE_INSTANCES":         "false",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_AZURE_FUNCTIONS":         "false",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_NETWORK_SECURITY_GROUPS": "true",
			},
			expectedDiscoveryCount: 1,
			expectedActionCount:    1,
			description:            "When only NSGs are enabled, should register NSG discovery and block action",
		},
		{
			name: "all features enabled",
			envVars: map[string]string{
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_VIRTUAL_MACHINES":        "true",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_SCALE_INSTANCES":         "true",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_AZURE_FUNCTIONS":         "true",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_NETWORK_SECURITY_GROUPS": "true",
			},
			expectedDiscoveryCount: 4,
			expectedActionCount:    7,
			description:            "When all features are enabled, should register all discoveries and actions",
		},
		{
			name:                   "default values (VMs and scale instances enabled by default)",
			envVars:                map[string]string{},
			expectedDiscoveryCount: 2,
			expectedActionCount:    2,
			description:            "With default config, VMs and scale instances should be enabled",
		},
		{
			name: "mixed configuration",
			envVars: map[string]string{
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_VIRTUAL_MACHINES":        "true",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_SCALE_INSTANCES":         "false",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_AZURE_FUNCTIONS":         "true",
				"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_NETWORK_SECURITY_GROUPS": "false",
			},
			expectedDiscoveryCount: 2,
			expectedActionCount:    5,
			description:            "Mixed configuration should register only enabled features",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearRegistrations()
			http.DefaultServeMux = http.NewServeMux()

			clearEnvironmentVariables()
			setEnvironmentVariables(tt.envVars)

			config.ParseConfiguration()
			config.ValidateConfiguration()

			err := RegisterHandlers()
			require.NoError(t, err, "registerHandlers should not return an error")

			discoveryList := discovery_kit_sdk.GetDiscoveryList()
			actionList := action_kit_sdk.GetActionList()

			assert.Equal(t, tt.expectedDiscoveryCount, len(discoveryList.Discoveries),
				"Discovery count mismatch for test: %s. Description: %s", tt.name, tt.description)
			assert.Equal(t, tt.expectedActionCount, len(actionList.Actions),
				"Action count mismatch for test: %s. Description: %s", tt.name, tt.description)

			t.Logf("Test %s: Registered %d discoveries and %d actions",
				tt.name, len(discoveryList.Discoveries), len(actionList.Actions))
		})
	}
}

func clearRegistrations() {
	discovery_kit_sdk.ClearRegisteredDiscoveries()
	action_kit_sdk.ClearRegisteredActions()
}

func clearEnvironmentVariables() {
	envVarsToClean := []string{
		"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_VIRTUAL_MACHINES",
		"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_SCALE_INSTANCES",
		"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_AZURE_FUNCTIONS",
		"STEADYBIT_EXTENSION_DISCOVERY_ENABLE_NETWORK_SECURITY_GROUPS",
	}

	for _, envVar := range envVarsToClean {
		os.Unsetenv(envVar)
	}
}

func setEnvironmentVariables(envVars map[string]string) {
	for key, value := range envVars {
		os.Setenv(key, value)
	}
}
