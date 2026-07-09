/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extappgateway

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-azure/common"
	"github.com/steadybit/extension-azure/config"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

const (
	TargetIDAppGateway = "com.steadybit.extension_azure.application-gateway"
	targetIcon         = "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTEwLjA4MTYgOC45MDQzNkMxMC4xOTQ0IDguOTQ1MDIgMTAuMzY1OSA4Ljk2NjMzIDEwLjQ3NDQgOC45NDU2QzEwLjY0MjQgOS4xMTYzNSAxMC44MjE0IDkuMjc1OTYgMTEuMDEwNCA5LjQyMzAzQzExLjI1MDkgOS41OTkxMSAxMS41MDcgOS43NTI3MiAxMS43NzU0IDkuODgyMDFWOS43OTk1NUMxMS43NzU2IDEwLjE2MzQgMTIuMDcxMiAxMC40NTgyIDEyLjQzNTEgMTAuNDU4MkMxMi42MzQ3IDEwLjQ1ODEgMTIuODEzNCAxMC4zNjk0IDEyLjkzNDIgMTAuMjI5MkMxMy4zNDIyIDEwLjMxNTcgMTMuNzgyMSAxMC4zNDk3IDE0LjE5ODMgMTAuMzIxNUMxNC4yMjM5IDEwLjMyMDggMTMuOTUzMSAxMC41NjQ1IDEzLjk1MzEgMTAuNTY0NUMxMy4yODI2IDExLjA0OCAxMi40NTA0IDExLjI1MiAxMS42MzIyIDExLjEzNDJDMTAuODc4MiAxMS4wMjU1IDEwLjE4OTggMTAuNjUwOSA5LjY4OTg4IDEwLjA4MTdDOS43NzQ1IDkuNzExOTEgOS45MzgxOSA5LjI1NTUyIDEwLjA4MTYgOC45MDQzNloiIGZpbGw9ImN1cnJlbnRDb2xvciIvPgo8cGF0aCBkPSJNMTEuODgxNyA2LjkxNTQzQzEyLjMwMTUgNy4zNDAxOSAxMi43NTYgNy43MzAxNyAxMy4yNDAyIDguMDc5NzFDMTMuMjM3IDguMTA3MDcgMTMuMjM0OCA4LjEzNDk5IDEzLjIzNDggOC4xNjMyNkMxMy4yMzQ4IDguNTUzMjQgMTMuNTUxMyA4Ljg2OTY0IDEzLjk0MTIgOC44Njk2NEMxNC4wMzk0IDguODY5NjIgMTQuMTMyOSA4Ljg0OTQ2IDE0LjIxNzkgOC44MTMyMkwxNC4yMDM4IDguODIxOUwxNC4zNzc0IDguOTA0MzZMMTQuNTM2OSA4Ljk3NDg5TDE0LjkwMTUgOS4xMjM1NUMxNC44MjE5IDkuMzcwNzIgMTQuNzEyMyA5LjYwNzc0IDE0LjU3NiA5LjgyNzc2QzE0LjU3NiA5LjgyNzc2IDEzLjQzIDkuODM5MDggMTMuMDkzNyA5Ljc2Mzc0VjkuNzgyMTlDMTMuMDg0NSA5LjQyNjMxIDEyLjc5MzEgOS4xNDAwNyAxMi40MzUxIDkuMTM5ODJDMTIuMjYyMyA5LjEzOTgyIDEyLjEwNDYgOS4yMDY4MSAxMS45ODcgOS4zMTU2MUMxMS42Mzk2IDkuMTMzNjMgMTEuMzE4NiA4LjkwNDQxIDExLjAzNDMgOC42MzQxOEMxMS4wMjE2IDguNjQ3NjUgMTEuMDA4NSA4LjY2MDU4IDEwLjk5NTIgOC42NzMyNUMxMS4xODYxIDguNDg3MjQgMTEuMzA0NCA4LjIyNzM0IDExLjMwNDUgNy45Mzk3NEMxMS4zMDQ1IDcuNzY2MDcgMTEuMjYxNSA3LjYwMjM0IDExLjE4NTEgNy40NTkwNUMxMS4zOTU3IDcuMjUxOTggMTEuNjI5NiA3LjA2OTM1IDExLjg4MTcgNi45MTU0M1oiIGZpbGw9ImN1cnJlbnRDb2xvciIvPgo8cGF0aCBkPSJNOS4yNjc3OSA2LjQ4NTc1QzkuMzE0OTQgNi43NTk5NiA5LjM4ODg5IDcuMDI5MzkgOS40OTAyMyA3LjI4ODdDOS4zNDQ0IDcuNDY1NjYgOS4yNTY5NCA3LjY5MjU1IDkuMjU2OTQgNy45Mzk3NEM5LjI1Njk4IDguMjA3MzEgOS4zNjAxNSA4LjQ1MDcxIDkuNTI4MjEgOC42MzMxQzkuNDM0ODQgOC45MTE5MyA5LjM1NjYzIDkuMjMyMDkgOS4yOTQ5MiA5LjUxOTZDOC45NzkzMyA4Ljk0ODE3IDguODQ0ODIgOC4yODk0OSA4LjkxNjIzIDcuNjMyNjZDOC45NjAyOCA3LjIyNzk3IDkuMDgwODcgNi44MzkwMyA5LjI2Nzc5IDYuNDg1NzVaIiBmaWxsPSJjdXJyZW50Q29sb3IiLz4KPHBhdGggZD0iTTE0LjM5OCA2LjIwNzk3QzE0Ljg5NjUgNi44NTk1NSAxNS4xMjQzIDcuNjc4NjYgMTUuMDM0OSA4LjQ5NDIxQzE1LjAyNzMgOC41NjMyIDE1LjAxNzcgOC42MzI0MiAxNS4wMDU2IDguNzAwMzdMMTQuNjM3OCA4LjUzOTc4TDE0LjU2MDggOC41MDI4OUMxNC42MTU5IDguNDAyMjQgMTQuNjQ3NiA4LjI4NjEgMTQuNjQ3NiA4LjE2MzI2QzE0LjY0NzUgNy43NzMzIDE0LjMzMTIgNy40NTY5IDEzLjk0MTIgNy40NTY4OEMxMy43NDI3IDcuNDU2ODggMTMuNTYyOCA3LjUzODc0IDEzLjQzNDUgNy42NzA2NEMxMy4wMTQ4IDcuMzQ4OTEgMTIuNjIxNCA2Ljk5Mzg3IDEyLjI1ODIgNi42MDk0NEMxMi45MDQ4IDYuMzAwMTggMTMuNjA5OCA2LjEzMjc3IDE0LjMyNTMgNi4xMTY4MkMxNC4zNSA2LjE0Njc4IDE0LjM3NDQgNi4xNzcwOSAxNC4zOTggNi4yMDc5N1oiIGZpbGw9ImN1cnJlbnRDb2xvciIvPgo8cGF0aCBkPSJNMTAuNjU2NyA1LjE5Nzc3QzEwLjg2NjggNS42MTY1NyAxMS4xNDgxIDYuMDU5ODYgMTEuNDQ1NSA2LjQyMTczQzExLjE4MjMgNi41OTMwMiAxMC45MzczIDYuNzkwOTcgMTAuNzE0MiA3LjAxMkMxMC41ODI3IDYuOTUwNTcgMTAuNDM2IDYuOTE1NDggMTAuMjgxMiA2LjkxNTQzQzEwLjE2MTUgNi45MTU0MyAxMC4wNDYyIDYuOTM2MjUgOS45Mzk0NSA2Ljk3NDAzQzkuODA1MTMgNi42MzA2MyA5LjcxMTMzIDYuMjcyNTEgOS42NjA1OSA1LjkwNzRDOS43NzcwNyA1Ljc3MDkzIDkuOTA2MDggNS42NDI5MSAxMC4wNDU4IDUuNTI2NTRDMTAuMjM3OCA1LjM5MzIgMTAuNDQzNSA1LjI4NTEgMTAuNjU2NyA1LjE5Nzc3WiIgZmlsbD0iY3VycmVudENvbG9yIi8+CjxwYXRoIGQ9Ik0xMS4wNjE0IDUuMDU5OTZDMTEuNDgxIDQuOTUwNzcgMTEuOTIyIDQuOTI3NjUgMTIuMzU4MSA0Ljk5NTk1QzEyLjg3ODIgNS4wNzc0MSAxMy4zNjU1IDUuMjg1MTggMTMuNzc5NSA1LjU5NTk5QzEzLjIyMzggNS42NDQwMSAxMS45NDI1IDYuMTI2ODIgMTEuODIzMSA2LjE4NjI3QzExLjU0MTQgNS44NDYwNyAxMS4yNzI5IDUuNDQ3NzcgMTEuMDYxNCA1LjA1OTk2WiIgZmlsbD0iY3VycmVudENvbG9yIi8+CjxwYXRoIGQ9Ik0xMS4wMTQ4IDUuMDcxOUMxMC44OTI4IDUuMTA1NjMgMTAuNzc0MSA1LjE0OTY4IDEwLjY1NjcgNS4xOTc3N0wxMC42NTU2IDUuMTk1NkMxMC42NTg3IDUuMTk0MzYgMTAuNjgxMSA1LjE4NDk1IDEwLjcwNjYgNS4xNzQ5OEMxMC43MjgyIDUuMTY2NTYgMTAuNzUyNSA1LjE1Njk5IDEwLjc2ODQgNS4xNTExMUMxMC44MDMyIDUuMTM4MzUgMTAuODcyNiA1LjExNTMgMTAuODcyNiA1LjExNTNMMTAuOTc0NiA1LjA4Mzg0TDExLjAxNDggNS4wNzE5WiIgZmlsbD0iY3VycmVudENvbG9yIi8+CjxwYXRoIGZpbGwtcnVsZT0iZXZlbm9kZCIgY2xpcC1ydWxlPSJldmVub2RkIiBkPSJNMTEuNTI1OCAyLjE5NjQ3QzExLjc4NzggMS45MzQ2MiAxMi4yMTIyIDEuOTM0NzMgMTIuNDc0MiAyLjE5NjQ3TDIxLjgwMzYgMTEuNTI1OUMyMi4wNjU1IDExLjc4NzkgMjIuMDY1NCAxMi4yMTIzIDIxLjgwMzYgMTIuNDc0MkwxMi40NzQyIDIxLjgwMzdDMTIuMjEyMiAyMi4wNjU1IDExLjc4NzggMjIuMDY1NiAxMS41MjU4IDIxLjgwMzdMMi4xOTY0IDEyLjQ3NDJDMS45MzQ1NyAxMi4yMTIzIDEuOTM0NSAxMS43ODc5IDIuMTk2NCAxMS41MjU5TDExLjUyNTggMi4xOTY0N1pNMTIuMzk4MiA0LjY3NTg1QzExLjUwMzIgNC41MzU2OSAxMC41ODgxIDQuNzQ0OTkgOS44NDM5NiA1LjI2MTc5QzkuMTQyNTQgNS44NDYwNCA4LjY5NTkgNi42ODA2IDguNTk3MjIgNy41ODgxOEM4LjUxOTUgOC4zMDMyMSA4LjY2Mjg3IDkuMDE5NzkgOS4wMDA4NyA5LjY0NDM4TDcuNDE5OTIgMTEuMjEyM0M3LjM5ODExIDExLjIzMzQgNy4zNjg3OSAxMS4yNDQ5IDcuMzM4NTQgMTEuMjQ0OUM3LjMwNzk0IDExLjI0NDggNy4yNzc5NyAxMS4yMzM1IDcuMjU2MDcgMTEuMjEyM1YxMS4xNTI2TDYuMzI2MTcgMTAuMjM0N0M2LjMwOTgzIDEwLjIxODcgNi4yODg4NiAxMC4yMDc2IDYuMjY2NDkgMTAuMjAzMkM2LjI0Mzg0IDEwLjE5ODggNi4yMTk1MSAxMC4yMDA5IDYuMTk4MTMgMTAuMjA5N0M2LjE3NzM5IDEwLjIxODQgNi4xNiAxMC4yMzM1IDYuMTQ3MTMgMTAuMjUyQzYuMTM0MzEgMTAuMjcxIDYuMTI2ODYgMTAuMjk0MiA2LjEyNjUyIDEwLjMxNzFWMTMuNjQ3MkM2LjEyNTQ4IDEzLjY3NjMgNi4xMzMyOSAxMy43MDYxIDYuMTUwMzkgMTMuNzI5N0M2LjE2NzM4IDEzLjc1MjggNi4xOTIyIDEzLjc2OTQgNi4yMTk4MyAxMy43Nzc0SDkuNTYyOTNDOS41ODExNSAxMy43Njc1IDkuNTk2NzggMTMuNzUzNCA5LjYwODUgMTMuNzM2MkM5LjYyMDI4IDEzLjcxODkgOS42Mjc4OSAxMy42OTg0IDkuNjMwMjEgMTMuNjc3NkM5LjYzMjUxIDEzLjY1NzEgOS42MzAwNyAxMy42MzYxIDkuNjIyNjEgMTMuNjE2OEM5LjYxNDk4IDEzLjU5NzMgOS42MDE1NSAxMy41Nzk2IDkuNTg1NzIgMTMuNTY1OEw4LjY2ODgzIDEyLjY0NzlWMTIuNTg5M0M4LjY0NjUzIDEyLjU2NTMgOC42MzQxMSAxMi41MzMgOC42MzQxMSAxMi41MDAzQzguNjM0MiAxMi40Njc2IDguNjQ2NjEgMTIuNDM2MiA4LjY2ODgzIDEyLjQxMjRMMTAuMTY2MiAxMC45MTM5QzEwLjQzNDQgMTEuMDg5OCAxMC43MjY5IDExLjIyODcgMTEuMDM0MyAxMS4zMjYyVjE2LjQ3MjdDMTEuMDM0MyAxNi41MDM3IDExLjAyMTQgMTYuNTMzMyAxMC45OTk2IDE2LjU1NTJDMTAuOTc3NyAxNi41NzcxIDEwLjk0OCAxNi41ODk3IDEwLjkxNzEgMTYuNTg5OUg5LjUyNzEyQzkuNTA0MzggMTYuNTkwNCA5LjQ4MTk3IDE2LjU5NzggOS40NjMxIDE2LjYxMDVDOS40NDQxNiAxNi42MjM1IDkuNDI5NTcgMTYuNjQyNiA5LjQyMDc5IDE2LjY2MzdDOS40MTIxOCAxNi42ODQ3IDkuNDA5MDQgMTYuNzA3NSA5LjQxMzE5IDE2LjcyOTlDOS40MTc1NyAxNi43NTI1IDkuNDI5NzYgMTYuNzczMiA5LjQ0NTc0IDE2Ljc4OTZMMTEuNzk5MyAxOS4xNDQyQzExLjgyMTEgMTkuMTY1NCAxMS44NTAyIDE5LjE3ODcgMTEuODgwNiAxOS4xNzg5QzExLjkxMTUgMTkuMTc4OSAxMS45NDIyIDE5LjE2NTcgMTEuOTY0MiAxOS4xNDQyTDE0LjMxNzcgMTYuNzg5NkMxNC4zMzM2IDE2Ljc3MzEgMTQuMzQ0OCAxNi43NTIyIDE0LjM0OTIgMTYuNzI5OUMxNC4zNTM0IDE2LjcwNzYgMTQuMzUxMSAxNi42ODQ4IDE0LjM0MjcgMTYuNjYzN0MxNC4zMzQgMTYuNjQyNCAxNC4zMTgzIDE2LjYyMzUgMTQuMjk5MyAxNi42MTA1QzE0LjI4MDQgMTYuNTk3OCAxNC4yNTc5IDE2LjU5MDMgMTQuMjM1MiAxNi41ODk5SDEyLjg1ODNDMTIuODI4IDE2LjU4NzMgMTIuNzk5NSAxNi41NzQ1IDEyLjc3OCAxNi41NTNDMTIuNzU2NSAxNi41MzE1IDEyLjc0MzggMTYuNTAzIDEyLjc0MTEgMTYuNDcyN1YxMS40MzE1QzEzLjEwMzYgMTEuMzY0MiAxMy40NTUzIDExLjI0IDEzLjc4MTcgMTEuMDYyNkwxNS4xMzA0IDEyLjQxMjRDMTUuMTUyMyAxMi40MzYyIDE1LjE2NTEgMTIuNDY3OCAxNS4xNjUxIDEyLjUwMDNDMTUuMTY1MSAxMi41MzMgMTUuMTUyNiAxMi41NjUzIDE1LjEzMDQgMTIuNTg5M0wxNS4wNzA3IDEyLjY0NzlMMTQuMTUzOSAxMy41NjU4QzE0LjEzOSAxMy41ODM1IDE0LjEyODQgMTMuNjA0OSAxNC4xMjQ2IDEzLjYyNzdDMTQuMTIwOCAxMy42NTA2IDE0LjEyMjcgMTMuNjc0MyAxNC4xMzExIDEzLjY5NkMxNC4xMzk0IDEzLjcxNzUgMTQuMTUzMSAxMy43MzcxIDE0LjE3MTIgMTMuNzUxNEMxNC4xODk1IDEzLjc2NTcgMTQuMjEyMSAxMy43NzQ0IDE0LjIzNTIgMTMuNzc3NEgxNy41NzczQzE3LjYwOTQgMTMuNzc0NSAxNy42NDAxIDEzLjc1ODggMTcuNjYxOSAxMy43MzUxQzE3LjY4MzMgMTMuNzExMiAxNy42OTU2IDEzLjY3OTMgMTcuNjk1NSAxMy42NDcyVjEwLjMxNzFDMTcuNjk4MiAxMC4yOTE2IDE3LjY5MjYgMTAuMjY1NCAxNy42NzkyIDEwLjI0MzNDMTcuNjY1OCAxMC4yMjEzIDE3LjY0NDggMTAuMjAzOSAxNy42MjA3IDEwLjE5NDVDMTcuNTk2NiAxMC4xODUzIDE3LjU2OTMgMTAuMTg0MSAxNy41NDQ3IDEwLjE5MTNDMTcuNTIwMiAxMC4xOTg2IDE3LjQ5OTEgMTAuMjE0MiAxNy40ODM5IDEwLjIzNDdMMTYuNTY2IDExLjE1MjZWMTEuMjEyM0MxNi41NDQxIDExLjIzMzYgMTYuNTE0IDExLjI0NDcgMTYuNDgzNSAxMS4yNDQ5QzE2LjQ1MjggMTEuMjQ0OSAxNi40MjE5IDExLjIzMzggMTYuNCAxMS4yMTIzTDE0Ljk2NzcgOS43OTk1NUMxNS4xNzI3IDkuNDExMSAxNS4zMDUxIDguOTg0MzUgMTUuMzUzOSA4LjUzOTc4QzE1LjQ1MjcgNy42MzkxOSAxNS4yMDEzIDYuNzM0MzcgMTQuNjUwOCA2LjAxNDgzQzE0LjEwMDMgNS4yOTUzIDEzLjI5MzIgNC44MTYwNiAxMi4zOTgyIDQuNjc1ODVaIiBmaWxsPSJjdXJyZW50Q29sb3IiLz4KPC9zdmc+Cg=="
)

type appGatewayDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*appGatewayDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*appGatewayDiscovery)(nil)
)

func NewAppGatewayDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&appGatewayDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *appGatewayDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDAppGateway,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: new("60s")},
	}
}

func (d *appGatewayDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDAppGateway,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     new(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure Application Gateway", Other: "Azure Application Gateways"},
		Category: new("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.application-gateway.sku-name"},
				{Attribute: "azure.application-gateway.zones"},
				{Attribute: "azure.application-gateway.waf-enabled"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *appGatewayDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.application-gateway.name", Label: discovery_kit_api.PluralLabel{One: "Application Gateway name", Other: "Application Gateway names"}},
		{Attribute: "azure.application-gateway.sku-name", Label: discovery_kit_api.PluralLabel{One: "Application Gateway SKU name", Other: "Application Gateway SKU names"}},
		{Attribute: "azure.application-gateway.sku-tier", Label: discovery_kit_api.PluralLabel{One: "Application Gateway SKU tier", Other: "Application Gateway SKU tiers"}},
		{Attribute: "azure.application-gateway.sku-capacity", Label: discovery_kit_api.PluralLabel{One: "Application Gateway SKU capacity", Other: "Application Gateway SKU capacities"}},
		{Attribute: "azure.application-gateway.autoscale.min-capacity", Label: discovery_kit_api.PluralLabel{One: "Application Gateway autoscale min capacity", Other: "Application Gateway autoscale min capacities"}},
		{Attribute: "azure.application-gateway.autoscale.max-capacity", Label: discovery_kit_api.PluralLabel{One: "Application Gateway autoscale max capacity", Other: "Application Gateway autoscale max capacities"}},
		{Attribute: "azure.application-gateway.zones", Label: discovery_kit_api.PluralLabel{One: "Application Gateway zone", Other: "Application Gateway zones"}},
		{Attribute: "azure.application-gateway.frontend.public-exposed", Label: discovery_kit_api.PluralLabel{One: "Application Gateway public frontend", Other: "Application Gateway public frontend"}},
		{Attribute: "azure.application-gateway.http2-enabled", Label: discovery_kit_api.PluralLabel{One: "Application Gateway HTTP/2", Other: "Application Gateway HTTP/2"}},
		{Attribute: "azure.application-gateway.waf-enabled", Label: discovery_kit_api.PluralLabel{One: "Application Gateway WAF enabled", Other: "Application Gateway WAF enabled"}},
		{Attribute: "azure.application-gateway.waf.firewall-mode", Label: discovery_kit_api.PluralLabel{One: "Application Gateway WAF mode", Other: "Application Gateway WAF modes"}},
		{Attribute: "azure.application-gateway.waf.rule-set-type", Label: discovery_kit_api.PluralLabel{One: "Application Gateway WAF rule set type", Other: "Application Gateway WAF rule set types"}},
		{Attribute: "azure.application-gateway.waf.rule-set-version", Label: discovery_kit_api.PluralLabel{One: "Application Gateway WAF rule set version", Other: "Application Gateway WAF rule set versions"}},
		{Attribute: "azure.application-gateway.listener-count", Label: discovery_kit_api.PluralLabel{One: "Application Gateway listener count", Other: "Application Gateway listener counts"}},
		{Attribute: "azure.application-gateway.backend-pool-count", Label: discovery_kit_api.PluralLabel{One: "Application Gateway backend pool count", Other: "Application Gateway backend pool counts"}},
		{Attribute: "azure.application-gateway.routing-rule-count", Label: discovery_kit_api.PluralLabel{One: "Application Gateway routing rule count", Other: "Application Gateway routing rule counts"}},
		{Attribute: "azure.application-gateway.provisioning-state", Label: discovery_kit_api.PluralLabel{One: "Application Gateway provisioning state", Other: "Application Gateway provisioning states"}},
	}
}

func (d *appGatewayDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllAppGateways(ctx, client)
}

func getAllAppGateways(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	targets, err := common.DiscoverViaResourceGraph(ctx, client,
		"Resources | where type =~ 'Microsoft.Network/applicationGateways' | project id, name, type, resourceGroup, location, tags, properties, zones, subscriptionId",
		toAppGatewayTarget)
	if err != nil {
		log.Error().Err(err).Msg("failed to get Application Gateway results")
		return nil, err
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesApplicationGateway), nil
}

func toAppGatewayTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")
	sku := common.GetMapValue(properties, "sku")
	autoscale := common.GetMapValue(properties, "autoscaleConfiguration")
	wafConfig := common.GetMapValue(properties, "webApplicationFirewallConfiguration")

	id, _ := items["id"].(string)
	name, _ := items["name"].(string)

	attributes := make(map[string][]string)
	attributes["azure.application-gateway.name"] = []string{name}
	attributes["azure.subscription.id"] = []string{common.StringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{common.StringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{common.StringFromMap(items, "location")}

	if v := common.StringFromMap(sku, "name"); v != "" {
		attributes["azure.application-gateway.sku-name"] = []string{v}
	}
	if v := common.StringFromMap(sku, "tier"); v != "" {
		attributes["azure.application-gateway.sku-tier"] = []string{v}
	}
	if v, ok := sku["capacity"].(float64); ok {
		attributes["azure.application-gateway.sku-capacity"] = []string{strconv.Itoa(int(v))}
	}
	if v, ok := autoscale["minCapacity"].(float64); ok {
		attributes["azure.application-gateway.autoscale.min-capacity"] = []string{strconv.Itoa(int(v))}
	}
	if v, ok := autoscale["maxCapacity"].(float64); ok {
		attributes["azure.application-gateway.autoscale.max-capacity"] = []string{strconv.Itoa(int(v))}
	}
	if zones := topLevelStringSlice(items, "zones"); len(zones) > 0 {
		sort.Strings(zones)
		attributes["azure.application-gateway.zones"] = zones
	}
	if v, ok := properties["enableHttp2"].(bool); ok {
		attributes["azure.application-gateway.http2-enabled"] = []string{strconv.FormatBool(v)}
	}
	if v := common.StringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.application-gateway.provisioning-state"] = []string{v}
	}

	wafEnabled := false
	if v, ok := wafConfig["enabled"].(bool); ok {
		wafEnabled = v
	}
	attributes["azure.application-gateway.waf-enabled"] = []string{strconv.FormatBool(wafEnabled)}
	if wafEnabled {
		if v := common.StringFromMap(wafConfig, "firewallMode"); v != "" {
			attributes["azure.application-gateway.waf.firewall-mode"] = []string{v}
		}
		if v := common.StringFromMap(wafConfig, "ruleSetType"); v != "" {
			attributes["azure.application-gateway.waf.rule-set-type"] = []string{v}
		}
		if v := common.StringFromMap(wafConfig, "ruleSetVersion"); v != "" {
			attributes["azure.application-gateway.waf.rule-set-version"] = []string{v}
		}
	}

	attributes["azure.application-gateway.listener-count"] = []string{strconv.Itoa(arrayLen(properties, "httpListeners"))}
	attributes["azure.application-gateway.backend-pool-count"] = []string{strconv.Itoa(arrayLen(properties, "backendAddressPools"))}
	attributes["azure.application-gateway.routing-rule-count"] = []string{strconv.Itoa(arrayLen(properties, "requestRoutingRules"))}

	publicExposed := false
	if v, ok := properties["frontendIPConfigurations"].([]any); ok {
		for _, e := range v {
			fc, ok := e.(map[string]any)
			if !ok {
				continue
			}
			fcProps := common.GetMapValue(fc, "properties")
			pip := common.GetMapValue(fcProps, "publicIPAddress")
			if id, ok := pip["id"].(string); ok && id != "" {
				publicExposed = true
				break
			}
		}
	}
	attributes["azure.application-gateway.frontend.public-exposed"] = []string{strconv.FormatBool(publicExposed)}

	for k, v := range common.GetMapValue(items, "tags") {
		attributes[fmt.Sprintf("azure.application-gateway.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDAppGateway,
		Label:      name,
		Attributes: attributes,
	}
}

func arrayLen(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	arr, ok := v.([]any)
	if !ok {
		return 0
	}
	return len(arr)
}

func topLevelStringSlice(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
