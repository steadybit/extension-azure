/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extapim

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
	TargetIDApiManagement = "com.steadybit.extension_azure.apim.service"
	targetIcon            = "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTE4LjcwMzUgNy45OTY3MUMxOC42NTYxIDYuMzQxMDUgMTcuOTU2MSA0Ljc3MTI2IDE2Ljc1NjIgMy42Mjk1QzE1LjU1NjIgMi40ODc3NCAxMy45NTM2IDEuODY2NTggMTIuMjk3NiAxLjkwMTQxQzEwLjk1ODQgMS44ODc2NCA5LjY0Nzg2IDIuMjg5OCA4LjU0Njg0IDMuMDUyNEM3LjQ0NTgyIDMuODE1IDYuNjA4NTcgNC45MDA0NyA2LjE1MDU5IDYuMTU5MDZDNC43MzExNyA2LjM1NDgzIDMuNDI5NjEgNy4wNTQ5MiAyLjQ4Mzg5IDguMTMxMzRDMS41MzgxNiA5LjIwNzc2IDEuMDExNDMgMTAuNTg4NiAxIDEyLjAyMTRDMS4wNTc0MiAxMy42MTMxIDEuNzQwODEgMTUuMTE3NyAyLjkwMTYgMTYuMjA4MkM0LjA2MjM4IDE3LjI5ODcgNS42MDY3IDE3Ljg4NjkgNy4xOTg4MiAxNy44NDQ5QzcuMzc5NzggMTcuODU3NiA3LjU2MTQgMTcuODU3NiA3Ljc0MjM1IDE3Ljg0NDlIOS4yOTUyOUM5LjIwMDk2IDE3LjYxNSA5LjE1MjU5IDE3LjM2ODggOS4xNTI5NCAxNy4xMjAyQzkuMTU3NjEgMTYuNzg4NSA5LjI0NjY5IDE2LjQ2MzQgOS40MTE3NiAxNi4xNzU1SDcuNkg3LjE5ODgyQzYuMDQ3NTYgMTYuMjE0NiA0LjkyNjY0IDE1LjgwMiA0LjA3NTQ5IDE1LjAyNThDMy4yMjQzNCAxNC4yNDk2IDIuNzEwNDMgMTMuMTcxNCAyLjY0MzUzIDEyLjAyMTRDMi42NTM5OCAxMC45ODAxIDMuMDQxMTYgOS45Nzc3NyAzLjczMzQ2IDkuMTk5ODJDNC40MjU3NiA4LjQyMTg4IDUuMzc2MzQgNy45MjA5NCA2LjQwOTQxIDcuNzg5NjVMNy4zOTI5NCA3LjYzNDM2TDcuNzE2NDcgNi42ODk2NUM4LjA2MzA1IDUuNzU3MzYgOC42ODkxNyA0Ljk1NDkgOS41MDkxOSA0LjM5MkMxMC4zMjkyIDMuODI5MTEgMTEuMzAzMSAzLjUzMzI3IDEyLjI5NzYgMy41NDQ5NEMxMy41MTgxIDMuNTE2NzcgMTQuNzAwOSAzLjk2ODc4IDE1LjU5MTUgNC44MDM3NEMxNi40ODIyIDUuNjM4NyAxNy4wMDk1IDYuNzg5ODkgMTcuMDYgOC4wMDk2NVY5LjQzMzE4TDE4LjQ3MDYgOS42MjczQzE5LjI2MTggOS43Mjg1NiAxOS45OTA5IDEwLjEwODcgMjAuNTI3IDEwLjY5OTNDMjEuMDYzMSAxMS4yODk5IDIxLjM3MSAxMi4wNTI0IDIxLjM5NTMgMTIuODQ5NkMyMS4zNjUyIDEzLjcyOTcgMjAuOTk1MSAxNC41NjM5IDIwLjM2MjggMTUuMTc2OUMxOS43MzA2IDE1Ljc4OTggMTguODg1MyAxNi4xMzM5IDE4LjAwNDcgMTYuMTM2N0gxNy44MTA2SDE3LjcwNzFIMTYuNDEyOUMxNi4xODk1IDE0Ljk3NDIgMTUuNTY2NSAxMy45MjY0IDE0LjY1MTggMTMuMTc0OEMxMy43MzcyIDEyLjQyMzMgMTIuNTg4NSAxMi4wMTUyIDExLjQwNDcgMTIuMDIxNEMxMS4yODgxIDEyLjAwNjggMTEuMTY5OCAxMi4wMTcxIDExLjA1NzUgMTIuMDUxN0MxMC45NDUyIDEyLjA4NjMgMTAuODQxNiAxMi4xNDQzIDEwLjc1MzUgMTIuMjIyMUMxMC42NjU0IDEyLjI5OTggMTAuNTk0OCAxMi4zOTU0IDEwLjU0NjUgMTIuNTAyNUMxMC40OTgxIDEyLjYwOTUgMTAuNDczMSAxMi43MjU3IDEwLjQ3MzEgMTIuODQzMkMxMC40NzMxIDEyLjk2MDcgMTAuNDk4MSAxMy4wNzY4IDEwLjU0NjUgMTMuMTgzOUMxMC41OTQ4IDEzLjI5MSAxMC42NjU0IDEzLjM4NjYgMTAuNzUzNSAxMy40NjQzQzEwLjg0MTYgMTMuNTQyIDEwLjk0NTIgMTMuNjAwMSAxMS4wNTc1IDEzLjYzNDdDMTEuMTY5OCAxMy42NjkzIDExLjI4ODEgMTMuNjc5NiAxMS40MDQ3IDEzLjY2NDlDMTIuMjc2MSAxMy43MTg2IDEzLjA5NDMgMTQuMTAyNSAxMy42OTI0IDE0LjczODZDMTQuMjkwNSAxNS4zNzQ2IDE0LjYyMzYgMTYuMjE0OCAxNC42MjM2IDE3LjA4NzlDMTQuNjIzNiAxNy45NjEgMTQuMjkwNSAxOC44MDEyIDEzLjY5MjQgMTkuNDM3MkMxMy4wOTQzIDIwLjA3MzMgMTIuMjc2MSAyMC40NTcyIDExLjQwNDcgMjAuNTEwOEMxMS4yODgxIDIwLjQ5NjIgMTEuMTY5OCAyMC41MDY1IDExLjA1NzUgMjAuNTQxMUMxMC45NDUyIDIwLjU3NTcgMTAuODQxNiAyMC42MzM4IDEwLjc1MzUgMjAuNzExNUMxMC42NjU0IDIwLjc4OTIgMTAuNTk0OCAyMC44ODQ4IDEwLjU0NjUgMjAuOTkxOUMxMC40OTgxIDIxLjA5OSAxMC40NzMxIDIxLjIxNTEgMTAuNDczMSAyMS4zMzI2QzEwLjQ3MzEgMjEuNDUwMSAxMC40OTgxIDIxLjU2NjIgMTAuNTQ2NSAyMS42NzMzQzEwLjU5NDggMjEuNzgwNCAxMC42NjU0IDIxLjg3NiAxMC43NTM1IDIxLjk1MzdDMTAuODQxNiAyMi4wMzE0IDEwLjk0NTIgMjIuMDg5NSAxMS4wNTc1IDIyLjEyNDFDMTEuMTY5OCAyMi4xNTg3IDExLjI4ODEgMjIuMTY5IDExLjQwNDcgMjIuMTU0NEMxMi42MTg1IDIyLjE1MjIgMTMuNzkxNCAyMS43MTQ5IDE0LjcxMDMgMjAuOTIxOUMxNS42MjkyIDIwLjEyODggMTYuMjMzMyAxOS4wMzI1IDE2LjQxMjkgMTcuODMySDE3Ljc3MThDMTcuODU3NSAxNy44NDU2IDE3Ljk0NDggMTcuODQ1NiAxOC4wMzA2IDE3LjgzMkMxOS4zNDM1IDE3LjgwODYgMjAuNTk1OSAxNy4yNzU4IDIxLjUyMzIgMTYuMzQ2MUMyMi40NTA1IDE1LjQxNjQgMjIuOTgwMSAxNC4xNjI2IDIzIDEyLjg0OTZDMjIuOTc3OCAxMS42NjIgMjIuNTMzMSAxMC41MjExIDIxLjc0NTcgOS42MzE3NUMyMC45NTgzIDguNzQyMzYgMTkuODc5NyA4LjE2MjY4IDE4LjcwMzUgNy45OTY3MVoiIGZpbGw9ImN1cnJlbnRDb2xvciIvPgo8L3N2Zz4K"
)

type apimDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*apimDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*apimDiscovery)(nil)
)

func NewApiManagementDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&apimDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *apimDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDApiManagement,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *apimDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDApiManagement,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure API Management service", Other: "Azure API Management services"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.apim.sku-name"},
				{Attribute: "azure.apim.zones"},
				{Attribute: "azure.apim.public-network-access"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *apimDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.apim.service.name", Label: discovery_kit_api.PluralLabel{One: "API Management service name", Other: "API Management service names"}},
		{Attribute: "azure.apim.sku-name", Label: discovery_kit_api.PluralLabel{One: "API Management SKU name", Other: "API Management SKU names"}},
		{Attribute: "azure.apim.sku-capacity", Label: discovery_kit_api.PluralLabel{One: "API Management SKU capacity", Other: "API Management SKU capacities"}},
		{Attribute: "azure.apim.zones", Label: discovery_kit_api.PluralLabel{One: "API Management zone", Other: "API Management zones"}},
		{Attribute: "azure.apim.gateway-url", Label: discovery_kit_api.PluralLabel{One: "API Management gateway URL", Other: "API Management gateway URLs"}},
		{Attribute: "azure.apim.developer-portal-url", Label: discovery_kit_api.PluralLabel{One: "API Management developer portal URL", Other: "API Management developer portal URLs"}},
		{Attribute: "azure.apim.virtual-network-type", Label: discovery_kit_api.PluralLabel{One: "API Management VNet type", Other: "API Management VNet types"}},
		{Attribute: "azure.apim.public-network-access", Label: discovery_kit_api.PluralLabel{One: "API Management public network access", Other: "API Management public network access"}},
		{Attribute: "azure.apim.disable-gateway", Label: discovery_kit_api.PluralLabel{One: "API Management gateway disabled", Other: "API Management gateway disabled"}},
		{Attribute: "azure.apim.additional-locations", Label: discovery_kit_api.PluralLabel{One: "API Management additional location", Other: "API Management additional locations"}},
		{Attribute: "azure.apim.provisioning-state", Label: discovery_kit_api.PluralLabel{One: "API Management provisioning state", Other: "API Management provisioning states"}},
	}
}

func (d *apimDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllApimServices(ctx, client)
}

func getAllApimServices(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	targets, err := common.DiscoverViaResourceGraph(ctx, client,
		"Resources | where type =~ 'Microsoft.ApiManagement/service' | project id, name, type, resourceGroup, location, tags, properties, sku, zones, subscriptionId",
		toApimTarget)
	if err != nil {
		log.Error().Err(err).Msg("failed to get API Management results")
		return nil, err
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesApiManagement), nil
}

func toApimTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")
	sku := common.GetMapValue(items, "sku")

	id, _ := items["id"].(string)
	name, _ := items["name"].(string)

	attributes := make(map[string][]string)
	attributes["azure.apim.service.name"] = []string{name}
	attributes["azure.subscription.id"] = []string{common.StringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{common.StringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{common.StringFromMap(items, "location")}

	if v := common.StringFromMap(sku, "name"); v != "" {
		attributes["azure.apim.sku-name"] = []string{v}
	}
	if v, ok := sku["capacity"].(float64); ok {
		attributes["azure.apim.sku-capacity"] = []string{strconv.Itoa(int(v))}
	}
	if zones := topLevelStringSlice(items, "zones"); len(zones) > 0 {
		sort.Strings(zones)
		attributes["azure.apim.zones"] = zones
	}
	if v := common.StringFromMap(properties, "gatewayUrl"); v != "" {
		attributes["azure.apim.gateway-url"] = []string{v}
	}
	if v := common.StringFromMap(properties, "developerPortalUrl"); v != "" {
		attributes["azure.apim.developer-portal-url"] = []string{v}
	}
	if v := common.StringFromMap(properties, "virtualNetworkType"); v != "" {
		attributes["azure.apim.virtual-network-type"] = []string{v}
	}
	if v := common.StringFromMap(properties, "publicNetworkAccess"); v != "" {
		attributes["azure.apim.public-network-access"] = []string{v}
	}
	if v, ok := properties["disableGateway"].(bool); ok {
		attributes["azure.apim.disable-gateway"] = []string{strconv.FormatBool(v)}
	}
	if v := common.StringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.apim.provisioning-state"] = []string{v}
	}

	if locs, ok := properties["additionalLocations"].([]any); ok && len(locs) > 0 {
		names := make([]string, 0, len(locs))
		for _, e := range locs {
			loc, ok := e.(map[string]any)
			if !ok {
				continue
			}
			if n := common.StringFromMap(loc, "location"); n != "" {
				names = append(names, n)
			}
		}
		if len(names) > 0 {
			sort.Strings(names)
			attributes["azure.apim.additional-locations"] = names
		}
	}

	for k, v := range common.GetMapValue(items, "tags") {
		attributes[fmt.Sprintf("azure.apim.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDApiManagement,
		Label:      name,
		Attributes: attributes,
	}
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
