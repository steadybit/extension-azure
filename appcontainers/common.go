package appcontainers

import (
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
)

const (
	TargetIDContainerApp = "com.steadybit.extension_azure.container_app"
	targetIcon           = "data:image/svg+xml,%3Csvg%20xmlns=%22http://www.w3.org/2000/svg%22%20width=%2224%22%20height=%2224%22%20viewBox=%220%200%2024%2024%22%20fill=%22none%22%20stroke=%22currentColor%22%20stroke-width=%222%22%20stroke-linecap=%22round%22%20stroke-linejoin=%22round%22%20class=%22lucide%20lucide-container-icon%20lucide-container%22%3E%3Cpath%20d=%22M22%207.7c0-.6-.4-1.2-.8-1.5l-6.3-3.9a1.72%201.72%200%200%200-1.7%200l-10.3%206c-.5.2-.9.8-.9%201.4v6.6c0%20.5.4%201.2.8%201.5l6.3%203.9a1.72%201.72%200%200%200%201.7%200l10.3-6c.5-.3.9-1%20.9-1.5Z%22/%3E%3Cpath%20d=%22M10%2021.9V14L2.1%209.1%22/%3E%3Cpath%20d=%22m10%2014%2011.9-6.9%22/%3E%3Cpath%20d=%22M14%2019.8v-8.1%22/%3E%3Cpath%20d=%22M18%2017.5V9.4%22/%3E%3C/svg%3E"
)

var (
	azureFunctionTargetSelection = action_kit_api.TargetSelection{
		TargetType: TargetIDContainerApp,
		SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
			{
				Label: "function name",
				Query: "container-app.resource.id=\"\"",
			},
		}),
	}
)
