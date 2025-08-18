package nsg

import (
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
)

const (
	TargetIDNetworkSG = "com.steadybit.extension_azure.nsg"
	targetIcon        = "data:image/svg+xml,%3Csvg%20xmlns=%22http://www.w3.org/2000/svg%22%20width=%2224%22%20height=%2224%22%20viewBox=%220%200%2024%2024%22%20fill=%22none%22%20stroke=%22currentColor%22%20stroke-width=%222%22%20stroke-linecap=%22round%22%20stroke-linejoin=%22round%22%20class=%22lucide%20lucide-shield-icon%20lucide-shield%22%3E%3Cpath%20d=%22M20%2013c0%205-3.5%207.5-7.66%208.95a1%201%200%200%201-.67-.01C7.5%2020.5%204%2018%204%2013V6a1%201%200%200%201%201-1c2%200%204.5-1.2%206.24-2.72a1.17%201.17%200%200%201%201.52%200C14.51%203.81%2017%205%2019%205a1%201%200%200%201%201%201z%22/%3E%3C/svg%3E"
)

var (
	networkSecurityGroupTargetSelection = action_kit_api.TargetSelection{
		TargetType: TargetIDNetworkSG,
		SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
			{
				Label: "network security group id",
				Query: "network-security-group.id=\"\"",
			},
		}),
	}
)
