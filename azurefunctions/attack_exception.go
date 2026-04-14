package azurefunctions

import (
	"errors"
	"fmt"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-azure/appconfig"
	"github.com/steadybit/extension-kit/extbuild"
)

func NewAzureFunctionExceptionAction() action_kit_sdk.Action[appconfig.AppConfigurationActionState] {
	return &appconfig.AppConfigurationAction{
		Description:    getInjectExceptionDescription(),
		ConfigProvider: injectException,
	}
}

func getInjectExceptionDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:              fmt.Sprintf("%s.exception", TargetIDAzureFunction),
		Version:         extbuild.GetSemverVersionStringOrUnknown(),
		Label:           "Inject Exception",
		Description:     "Injects exception into the function.",
		Icon:            new(string(targetIcon)),
		TargetSelection: &azureFunctionTargetSelection,
		Technology:      new("Azure"),
		Category:        new("Azure Function"),
		Kind:            action_kit_api.Attack,
		TimeControl:     action_kit_api.TimeControlExternal,
		Parameters: []action_kit_api.ActionParameter{
			{
				Label:        "Duration",
				Name:         "duration",
				Type:         action_kit_api.ActionParameterTypeDuration,
				Description:  new("The duration of the attack."),
				Advanced:     new(false),
				Required:     new(true),
				DefaultValue: new("30s"),
				Order:        new(0),
			},
			{
				Name:         "rate",
				Label:        "Rate",
				Description:  new("The rate of invocations to affect."),
				Type:         action_kit_api.ActionParameterTypePercentage,
				DefaultValue: new("100"),
				Required:     new(true),
				Order:        new(1),
			},
			{
				Name:         "exceptionMsg",
				Label:        "Message",
				Description:  new("Message of the thrown exception."),
				Type:         action_kit_api.ActionParameterTypeString,
				DefaultValue: new("Injected exception"),
				Required:     new(true),
				Order:        new(2),
			},
		},
		Stop: new(action_kit_api.MutatingEndpointReference{}),
	}
}

func injectException(request action_kit_api.PrepareActionRequestBody) (*appconfig.FaultInjectionConfig, error) {
	appConfigurationEndpoints := request.Target.Attributes["azure-function.app-configuration.endpoint"]

	if len(appConfigurationEndpoints) == 0 {
		return nil, errors.New("no app configuration endpoint found, check if 'STEADYBIT_FAULT_INJECTION_ENDPOINT' environment variable is pointing to the correct app configuration and you are using the Steadybit .NET middleware (https://github.com/steadybit/failure-azure-functions-net)")
	}

	return &appconfig.FaultInjectionConfig{
		Injection:                "Exception",
		Rate:                     int(request.Config["rate"].(float64)),
		ExceptionMsg:             new(request.Config["exceptionMsg"].(string)),
		Enabled:                  true,
		AppConfigurationEndpoint: new(request.Target.Attributes["azure-function.app-configuration.endpoint"][0]),
		AppConfigurationSuffix:   new(request.Target.Attributes["steadybit.label"][0]),
	}, nil
}
