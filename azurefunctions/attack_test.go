package azurefunctions

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
)

func TestFaultInjectionConfig_ToAppConfigKeyValuePairs(t *testing.T) {
	tests := []struct {
		name     string
		config   FaultInjectionConfig
		expected map[string]*string
	}{
		{
			name: "basic config with minimal fields",
			config: FaultInjectionConfig{
				Injection: "exception",
				Rate:      50,
				Enabled:   true,
			},
			expected: map[string]*string{
				"Steadybit:FaultInjection:Injection": extutil.Ptr("exception"),
				"Steadybit:FaultInjection:Rate":      extutil.Ptr("50"),
				"Steadybit:FaultInjection:Enabled":   extutil.Ptr("true"),
			},
		},
		{
			name: "config with status code",
			config: FaultInjectionConfig{
				Injection:  "status_code",
				Rate:       75,
				Enabled:    true,
				StatusCode: extutil.Ptr(500),
			},
			expected: map[string]*string{
				"Steadybit:FaultInjection:Injection":  extutil.Ptr("status_code"),
				"Steadybit:FaultInjection:Rate":       extutil.Ptr("75"),
				"Steadybit:FaultInjection:Enabled":    extutil.Ptr("true"),
				"Steadybit:FaultInjection:StatusCode": extutil.Ptr("500"),
			},
		},
		{
			name: "config with latency",
			config: FaultInjectionConfig{
				Injection:  "latency",
				Rate:       100,
				Enabled:    true,
				MinLatency: extutil.Ptr(100 * time.Millisecond),
				MaxLatency: extutil.Ptr(500 * time.Millisecond),
			},
			expected: map[string]*string{
				"Steadybit:FaultInjection:Injection":            extutil.Ptr("latency"),
				"Steadybit:FaultInjection:Rate":                 extutil.Ptr("100"),
				"Steadybit:FaultInjection:Enabled":              extutil.Ptr("true"),
				"Steadybit:FaultInjection:Delay:MinimumLatency": extutil.Ptr("100"),
				"Steadybit:FaultInjection:Delay:MaximumLatency": extutil.Ptr("500"),
			},
		},
		{
			name: "config with exception message",
			config: FaultInjectionConfig{
				Injection:    "exception",
				Rate:         25,
				Enabled:      true,
				ExceptionMsg: extutil.Ptr("Test exception message"),
			},
			expected: map[string]*string{
				"Steadybit:FaultInjection:Injection":         extutil.Ptr("exception"),
				"Steadybit:FaultInjection:Rate":              extutil.Ptr("25"),
				"Steadybit:FaultInjection:Enabled":           extutil.Ptr("true"),
				"Steadybit:FaultInjection:Exception:Message": extutil.Ptr("Test exception message"),
			},
		},
		{
			name: "config with disk space",
			config: FaultInjectionConfig{
				Injection: "fill_disk",
				Rate:      90,
				Enabled:   true,
				DiskSpace: extutil.Ptr(1024),
			},
			expected: map[string]*string{
				"Steadybit:FaultInjection:Injection":          extutil.Ptr("fill_disk"),
				"Steadybit:FaultInjection:Rate":               extutil.Ptr("90"),
				"Steadybit:FaultInjection:Enabled":            extutil.Ptr("true"),
				"Steadybit:FaultInjection:FillDisk:Megabytes": extutil.Ptr("1024"),
			},
		},
		{
			name: "config with all fields",
			config: FaultInjectionConfig{
				Injection:    "mixed",
				Rate:         60,
				Enabled:      false,
				StatusCode:   extutil.Ptr(404),
				MinLatency:   extutil.Ptr(200 * time.Millisecond),
				MaxLatency:   extutil.Ptr(800 * time.Millisecond),
				ExceptionMsg: extutil.Ptr("All fields test"),
				DiskSpace:    extutil.Ptr(512),
			},
			expected: map[string]*string{
				"Steadybit:FaultInjection:Injection":            extutil.Ptr("mixed"),
				"Steadybit:FaultInjection:Rate":                 extutil.Ptr("60"),
				"Steadybit:FaultInjection:Enabled":              extutil.Ptr("false"),
				"Steadybit:FaultInjection:StatusCode":           extutil.Ptr("404"),
				"Steadybit:FaultInjection:Delay:MinimumLatency": extutil.Ptr("200"),
				"Steadybit:FaultInjection:Delay:MaximumLatency": extutil.Ptr("800"),
				"Steadybit:FaultInjection:Exception:Message":    extutil.Ptr("All fields test"),
				"Steadybit:FaultInjection:FillDisk:Megabytes":   extutil.Ptr("512"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.ToAppConfigKeyValuePairs()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAppConfigEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		setEnv      bool
		expected    string
		expectError bool
	}{
		{
			name:        "environment variable is set",
			envValue:    "https://test-config.azconfig.io",
			setEnv:      true,
			expected:    "https://test-config.azconfig.io",
			expectError: false,
		},
		{
			name:        "environment variable is not set",
			setEnv:      false,
			expected:    "",
			expectError: true,
		},
		{
			name:        "environment variable is empty",
			envValue:    "",
			setEnv:      true,
			expected:    "",
			expectError: false,
		},
	}

	originalValue := os.Getenv("AZURE_APP_CONFIG_ENDPOINT")
	defer func() {
		if originalValue != "" {
			os.Setenv("AZURE_APP_CONFIG_ENDPOINT", originalValue)
		} else {
			os.Unsetenv("AZURE_APP_CONFIG_ENDPOINT")
		}
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("AZURE_APP_CONFIG_ENDPOINT")

			if tt.setEnv {
				os.Setenv("AZURE_APP_CONFIG_ENDPOINT", tt.envValue)
			}

			result, err := GetAppConfigEndpoint()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "AZURE_APP_CONFIG_ENDPOINT environment variable is not set")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestAzureFunctionAction_Prepare(t *testing.T) {
	tests := []struct {
		name           string
		configProvider func(request action_kit_api.PrepareActionRequestBody) (*FaultInjectionConfig, error)
		request        action_kit_api.PrepareActionRequestBody
		expectedState  AzureFunctionActionState
		expectError    bool
	}{
		{
			name: "successful prepare with valid config",
			configProvider: func(request action_kit_api.PrepareActionRequestBody) (*FaultInjectionConfig, error) {
				return &FaultInjectionConfig{
					Injection: "exception",
					Rate:      50,
					Enabled:   true,
				}, nil
			},
			request: action_kit_api.PrepareActionRequestBody{
				ExecutionContext: &action_kit_api.ExecutionContext{
					ExperimentKey: extutil.Ptr("test-experiment"),
					ExecutionId:   extutil.Ptr(123),
				},
			},
			expectedState: AzureFunctionActionState{
				ExperimentKey: extutil.Ptr("test-experiment"),
				ExecutionId:   extutil.Ptr(123),
				Config: &FaultInjectionConfig{
					Injection: "exception",
					Rate:      50,
					Enabled:   true,
				},
			},
			expectError: false,
		},
		{
			name: "prepare with config provider error",
			configProvider: func(request action_kit_api.PrepareActionRequestBody) (*FaultInjectionConfig, error) {
				return nil, assert.AnError
			},
			request: action_kit_api.PrepareActionRequestBody{
				ExecutionContext: &action_kit_api.ExecutionContext{
					ExperimentKey: extutil.Ptr("test-experiment"),
					ExecutionId:   extutil.Ptr(123),
				},
			},
			expectedState: AzureFunctionActionState{},
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := azureFunctionAction{
				configProvider: tt.configProvider,
			}

			state := AzureFunctionActionState{}
			result, err := action.Prepare(context.Background(), &state, tt.request)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Nil(t, result)
			assert.Equal(t, tt.expectedState, state)
		})
	}
}
