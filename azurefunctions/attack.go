package azurefunctions

import (
	"context"
	"os"
	"strings"
	"time"

	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-azure/common"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extutil"
)

type azureFunctionAction struct {
	description    action_kit_api.ActionDescription
	configProvider func(request action_kit_api.PrepareActionRequestBody) (*FaultInjectionConfig, error)
}

var _ action_kit_sdk.Action[AzureFunctionActionState] = (*azureFunctionAction)(nil)
var _ action_kit_sdk.ActionWithStop[AzureFunctionActionState] = (*azureFunctionAction)(nil)

type AzureFunctionActionState struct {
	Account          string                `json:"account"`
	Region           string                `json:"region"`
	DiscoveredByRole *string               `json:"discoveredByRole"`
	Param            string                `json:"param"`
	Config           *FaultInjectionConfig `json:"config"`
	ExperimentKey    *string               `json:"experimentKey"`
	ExecutionId      *int                  `json:"executionId"`
}

type FaultInjectionConfig struct {
	Injection    string         `json:"failureMode"`
	Rate         int            `json:"rate"`
	Enabled      bool           `json:"isEnabled"`
	StatusCode   *int           `json:"statusCode,omitempty"`
	MinLatency   *time.Duration `json:"minLatency,omitempty"`
	MaxLatency   *time.Duration `json:"maxLatency,omitempty"`
	ExceptionMsg *string        `json:"exceptionMsg,omitempty"`
	BlockedHosts *[]string      `json:"denylist,omitempty"`
	DiskSpace    *int           `json:"diskSpace,omitempty"`
}

func (config *FaultInjectionConfig) ToAppConfigKeyValuePairs() map[string]*string {
	appConfigMapping := make(map[string]*string)

	appConfigMapping["Steadybit:FaultInjection:Injection"] = &config.Injection
	appConfigMapping["Steadybit:FaultInjection:Rate"] = extutil.Ptr(fmt.Sprint(config.Rate))
	appConfigMapping["Steadybit:FaultInjection:Enabled"] = extutil.Ptr(fmt.Sprint(config.Enabled))

	if config.StatusCode != nil {
		appConfigMapping["Steadybit:FaultInjection:StatusCode"] = extutil.Ptr(fmt.Sprint(*config.StatusCode))
	}

	if config.MinLatency != nil {
		log.Info().Msgf("Setting minimum latency to %d ms", config.MinLatency)
		appConfigMapping["Steadybit:FaultInjection:Delay:MinimumLatency"] = extutil.Ptr(fmt.Sprint(config.MinLatency.Milliseconds()))
	}

	if config.MaxLatency != nil {
		log.Info().Msgf("Setting minimum latency to %d ms", config.MinLatency)
		appConfigMapping["Steadybit:FaultInjection:Delay:MaximumLatency"] = extutil.Ptr(fmt.Sprint(config.MaxLatency.Milliseconds()))
	}

	if config.ExceptionMsg != nil {
		appConfigMapping["Steadybit:FaultInjection:Exception:Message"] = config.ExceptionMsg
	}

	if config.BlockedHosts != nil && len(*config.BlockedHosts) > 0 {
		denylistStr := strings.Join(*config.BlockedHosts, ",")
		appConfigMapping["Steadybit:FaultInjection:Block:Hosts"] = &denylistStr
	}

	if config.DiskSpace != nil {
		appConfigMapping["Steadybit:FaultInjection:FillDisk:Megabytes"] = extutil.Ptr(fmt.Sprint(*config.DiskSpace))
	}

	return appConfigMapping
}

func (a *azureFunctionAction) Describe() action_kit_api.ActionDescription {
	return a.description
}

func (a *azureFunctionAction) NewEmptyState() AzureFunctionActionState {
	return AzureFunctionActionState{}
}

func (a *azureFunctionAction) Prepare(ctx context.Context, state *AzureFunctionActionState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	config, err := a.configProvider(request)
	if err != nil {
		return nil, extension_kit.ToError("Failed to create config", err)
	}

	state.ExperimentKey = request.ExecutionContext.ExperimentKey
	state.ExecutionId = request.ExecutionContext.ExecutionId
	state.Config = config
	return nil, nil
}

func GetAppConfigEndpoint() (string, error) {
	appConfigEndpoint, exists := os.LookupEnv("AZURE_APP_CONFIG_ENDPOINT")
	if !exists {
		return "", fmt.Errorf("AZURE_APP_CONFIG_ENDPOINT environment variable is not set")
	}
	return appConfigEndpoint, nil
}

func (a *azureFunctionAction) Start(ctx context.Context, state *AzureFunctionActionState) (*action_kit_api.StartResult, error) {
	cred, err := common.ConnectionAzure()

	if err != nil {
		log.Error().Msgf("Failed to create Azure credential: %v", err)
	}

	appConfigEndpoint, err := GetAppConfigEndpoint()

	if err != nil {
		return nil, err
	}

	client, err := azappconfig.NewClient(appConfigEndpoint, cred, nil)

	if err != nil {
		log.Error().Msgf("Failed to create Azure App Configuration client: %v", err)
		return nil, extension_kit.ToError("Failed to create Azure App Configuration client.", err)
	}

	client.SetSetting(ctx, "Steadybit:FaultInjection:Enabled", extutil.Ptr("Yes"), nil)
	client.SetSetting(ctx, "Steadybit:FaultInjection:Revision", extutil.Ptr(uuid.New().String()), nil)

	for key, value := range state.Config.ToAppConfigKeyValuePairs() {
		if _, err := client.SetSetting(ctx, key, value, nil); err != nil {
			log.Error().Msgf("Failed to set setting %s: %v", key, err)
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to set setting %s", key), err)
		}
	}

	return nil, nil
}

func (a *azureFunctionAction) Stop(ctx context.Context, state *AzureFunctionActionState) (*action_kit_api.StopResult, error) {
	cred, err := common.ConnectionAzure()
	if err != nil {
		log.Error().Msgf("Failed to create Azure credential: %v", err)
	}

	appConfigEndpoint, err := GetAppConfigEndpoint()

	if err != nil {
		return nil, err
	}

	client, err := azappconfig.NewClient(appConfigEndpoint, cred, nil)

	if err != nil {
		log.Error().Msgf("Failed to create Azure App Configuration client: %v", err)
		return nil, extension_kit.ToError("Failed to create Azure App Configuration client.", err)
	}

	client.SetSetting(ctx, "Steadybit:FaultInjection:Enabled", extutil.Ptr("No"), nil)
	client.SetSetting(ctx, "Steadybit:FaultInjection:Revision", extutil.Ptr(uuid.New().String()), nil)

	return nil, nil
}
