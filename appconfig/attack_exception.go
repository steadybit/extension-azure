package appconfig

import (
	"fmt"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

func NewExceptionAction() action_kit_sdk.Action[AppConfigurationActionState] {
	return &azureFunctionAction{
		description:    getInjectExceptionDescription(),
		configProvider: injectException,
	}
}

func getInjectExceptionDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:              fmt.Sprintf("%s.exception", TargetIDAzureAppConfiguration),
		Version:         extbuild.GetSemverVersionStringOrUnknown(),
		Label:           "Inject Exception",
		Description:     "Injects exception into the function.",
		Icon:            extutil.Ptr(string(targetIcon)),
		TargetSelection: &azureFunctionTargetSelection,
		Technology:      extutil.Ptr("Azure"),
		Category:        extutil.Ptr("Azure App Configuration"),
		Kind:            action_kit_api.Attack,
		TimeControl:     action_kit_api.TimeControlExternal,
		Parameters: []action_kit_api.ActionParameter{
			{
				Label:        "Duration",
				Name:         "duration",
				Type:         action_kit_api.ActionParameterTypeDuration,
				Description:  extutil.Ptr("The duration of the attack."),
				Advanced:     extutil.Ptr(false),
				Required:     extutil.Ptr(true),
				DefaultValue: extutil.Ptr("30s"),
				Order:        extutil.Ptr(0),
			},
			{
				Name:         "rate",
				Label:        "Rate",
				Description:  extutil.Ptr("The rate of invocations to affect."),
				Type:         action_kit_api.ActionParameterTypePercentage,
				DefaultValue: extutil.Ptr("100"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
			},
			{
				Name:         "exceptionMsg",
				Label:        "Message",
				Description:  extutil.Ptr("Message of the thrown exception."),
				Type:         action_kit_api.ActionParameterTypeString,
				DefaultValue: extutil.Ptr("Injected exception"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(2),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func injectException(request action_kit_api.PrepareActionRequestBody) (*FaultInjectionConfig, error) {
	return &FaultInjectionConfig{
		Injection:          "Exception",
		Rate:               int(request.Config["rate"].(float64)),
		ExceptionMsg:       extutil.Ptr(request.Config["exceptionMsg"].(string)),
		Enabled:            true,
		AppConfigurationId: request.Target.Name,
	}, nil
}
