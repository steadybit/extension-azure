package appconfig

import (
	"context"
	"errors"
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

type AppConfigurationAction struct {
	Description    action_kit_api.ActionDescription
	ConfigProvider func(request action_kit_api.PrepareActionRequestBody) (*FaultInjectionConfig, error)
}

var _ action_kit_sdk.Action[AppConfigurationActionState] = (*AppConfigurationAction)(nil)
var _ action_kit_sdk.ActionWithStop[AppConfigurationActionState] = (*AppConfigurationAction)(nil)

type AppConfigurationActionState struct {
	Account          string                `json:"account"`
	Region           string                `json:"region"`
	DiscoveredByRole *string               `json:"discoveredByRole"`
	Param            string                `json:"param"`
	Config           *FaultInjectionConfig `json:"config"`
	ExperimentKey    *string               `json:"experimentKey"`
	ExecutionId      *int                  `json:"executionId"`
}

type AttackType int

type FaultInjectionConfig struct {
	Injection                string         `json:"failureMode"`
	Rate                     int            `json:"rate"`
	Enabled                  bool           `json:"isEnabled"`
	AppConfigurationId       *string        `json:"configurationId"`
	AppConfigurationEndpoint *string        `json:"configurationEndpoint"`
	AppConfigurationSuffix   *string        `json:"configurationSuffix"`
	StatusCode               *int           `json:"statusCode,omitempty"`
	MinLatency               *time.Duration `json:"minLatency,omitempty"`
	MaxLatency               *time.Duration `json:"maxLatency,omitempty"`
	ExceptionMsg             *string        `json:"exceptionMsg,omitempty"`
	DiskSpace                *int           `json:"diskSpace,omitempty"`
}

func (config *FaultInjectionConfig) ToAppConfigKeyValuePairs(azureFunctionName string) map[string]*string {
	appConfigMapping := make(map[string]*string)

	appConfigMapping[fmt.Sprintf("Steadybit:FaultInjection:%s:Injection", azureFunctionName)] = &config.Injection
	appConfigMapping[fmt.Sprintf("Steadybit:FaultInjection:%s:Rate", azureFunctionName)] = extutil.Ptr(fmt.Sprint(config.Rate))
	appConfigMapping[fmt.Sprintf("Steadybit:FaultInjection:%s:Enabled", azureFunctionName)] = extutil.Ptr(fmt.Sprint(config.Enabled))

	if config.StatusCode != nil {
		appConfigMapping[fmt.Sprintf("Steadybit:FaultInjection:%s:StatusCode", azureFunctionName)] = extutil.Ptr(fmt.Sprint(*config.StatusCode))
	}

	if config.MinLatency != nil {
		log.Debug().Msgf("Setting minimum latency to %d ms", config.MinLatency.Milliseconds())
		appConfigMapping[fmt.Sprintf("Steadybit:FaultInjection:%s:Delay:MinimumLatency", azureFunctionName)] = extutil.Ptr(fmt.Sprint(config.MinLatency.Milliseconds()))
	}

	if config.MaxLatency != nil {
		log.Debug().Msgf("Setting maximum latency to %d ms", config.MaxLatency.Milliseconds())
		appConfigMapping[fmt.Sprintf("Steadybit:FaultInjection:%s:Delay:MaximumLatency", azureFunctionName)] = extutil.Ptr(fmt.Sprint(config.MaxLatency.Milliseconds()))
	}

	if config.ExceptionMsg != nil {
		appConfigMapping[fmt.Sprintf("Steadybit:FaultInjection:%s:Exception:Message", azureFunctionName)] = config.ExceptionMsg
	}

	if config.DiskSpace != nil {
		appConfigMapping[fmt.Sprintf("Steadybit:FaultInjection:%s:FillDisk:Megabytes", azureFunctionName)] = extutil.Ptr(fmt.Sprint(*config.DiskSpace))
	}

	return appConfigMapping
}

func (a *AppConfigurationAction) Describe() action_kit_api.ActionDescription {
	return a.Description
}

func (a *AppConfigurationAction) NewEmptyState() AppConfigurationActionState {
	return AppConfigurationActionState{}
}

func (a *AppConfigurationAction) Prepare(ctx context.Context, state *AppConfigurationActionState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	config, err := a.ConfigProvider(request)
	if err != nil {
		return nil, extension_kit.ToError("Failed to create config", err)
	}

	state.ExperimentKey = request.ExecutionContext.ExperimentKey
	state.ExecutionId = request.ExecutionContext.ExecutionId
	state.Config = config
	return nil, nil
}

func GetAppConfigEndpoint(appConfigurationId string) (string, error) {
	splitId := strings.Split(appConfigurationId, "/")

	if len(splitId) != 9 {
		return "", fmt.Errorf("invalid app configuration id format")
	}

	appConfigurationName := splitId[len(splitId)-1]

	return fmt.Sprintf("https://%s.azconfig.io", appConfigurationName), nil
}

func getAppConfigEndpoint(config FaultInjectionConfig) (string, error) {
	if config.AppConfigurationId != nil {
		appConfigEndpoint, err := GetAppConfigEndpoint(*config.AppConfigurationId)

		if err != nil {
			return "", err
		}

		return appConfigEndpoint, nil
	} else if config.AppConfigurationEndpoint != nil {
		appConfigEndpoint := *config.AppConfigurationEndpoint
		return appConfigEndpoint, nil
	} else {
		return "", fmt.Errorf("missing app configuration endpoint")
	}
}

func getAppConfigName(endpoint string) (string, error) {
	splitEndpoint := strings.Split(endpoint, ".")
	if len(splitEndpoint) != 3 {
		return "", errors.New("invalid app configuration endpoint")
	}

	if strings.Contains(splitEndpoint[0], "https://") {
		return strings.Replace(splitEndpoint[0], "https://", "", 1), nil
	}

	return "", errors.New("invalid app configuration endpoint")
}

func (a *AppConfigurationAction) Start(ctx context.Context, state *AppConfigurationActionState) (*action_kit_api.StartResult, error) {
	cred, err := common.ConnectionAzure()

	if err != nil {
		log.Error().Msgf("Failed to create Azure credential: %v", err)
	}

	appConfigEndpoint, err := getAppConfigEndpoint(*state.Config)

	if err != nil {
		return nil, extension_kit.ToError("Failed to get App Configuration endpoint.", err)
	}

	client, err := azappconfig.NewClient(appConfigEndpoint, cred, nil)

	if err != nil {
		log.Error().Msgf("Failed to create Azure App Configuration client: %v", err)
		return nil, extension_kit.ToError("Failed to create Azure App Configuration client.", err)
	}

	client.SetSetting(ctx, fmt.Sprintf("Steadybit:FaultInjection:%s:Enabled", *state.Config.AppConfigurationSuffix), extutil.Ptr("Yes"), nil)
	client.SetSetting(ctx, fmt.Sprintf("Steadybit:FaultInjection:%s:Revision", *state.Config.AppConfigurationSuffix), extutil.Ptr(uuid.New().String()), nil)

	for key, value := range state.Config.ToAppConfigKeyValuePairs(*state.Config.AppConfigurationSuffix) {
		if _, err := client.SetSetting(ctx, key, value, nil); err != nil {
			log.Error().Msgf("Failed to set setting %s: %v", key, err)
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to set setting %s", key), err)
		}
	}

	return nil, nil
}

func (a *AppConfigurationAction) Stop(ctx context.Context, state *AppConfigurationActionState) (*action_kit_api.StopResult, error) {
	cred, err := common.ConnectionAzure()
	if err != nil {
		log.Error().Msgf("Failed to create Azure credential: %v", err)
	}

	appConfigEndpoint, err := getAppConfigEndpoint(*state.Config)

	if err != nil {
		return nil, extension_kit.ToError("Failed to get App Configuration endpoint.", err)
	}

	client, err := azappconfig.NewClient(appConfigEndpoint, cred, nil)

	if err != nil {
		log.Error().Msgf("Failed to create Azure App Configuration client: %v", err)
		return nil, extension_kit.ToError("Failed to create Azure App Configuration client.", err)
	}

	filter := *state.Config.AppConfigurationSuffix
	pager := client.NewListSettingsPager(azappconfig.SettingSelector{
		KeyFilter: extutil.Ptr(fmt.Sprintf("Steadybit:FaultInjection:%s:*", filter)),
	}, &azappconfig.ListSettingsOptions{})

	var keysToDelete []string

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list settings: %v", err)
		}

		for _, setting := range page.Settings {
			keysToDelete = append(keysToDelete, *setting.Key)
		}
	}

	for _, key := range keysToDelete {
		_, err := client.DeleteSetting(ctx, key, nil)
		if err != nil {
			return nil, extension_kit.ToError(fmt.Sprintf("Failed to delete setting %s", key), err)
		}
	}

	return nil, nil
}
