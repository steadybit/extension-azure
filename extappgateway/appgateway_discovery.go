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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-azure/common"
	"github.com/steadybit/extension-azure/config"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"os"
)

const (
	TargetIDAppGateway = "com.steadybit.extension_azure.application-gateway"
	targetIcon         = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNNCA0aDE2djJINFY0em0wIDdoMTZ2Mkg0di0yem0wIDdoMTZ2Mkg0di0yeiIgZmlsbD0iY3VycmVudENvbG9yIi8+PC9zdmc+"
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
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *appGatewayDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDAppGateway,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure Application Gateway", Other: "Azure Application Gateways"},
		Category: extutil.Ptr("cloud"),
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
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	results, err := client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: extutil.Ptr("Resources | where type =~ 'Microsoft.Network/applicationGateways' | project id, name, type, resourceGroup, location, tags, properties, zones, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get Application Gateway results")
		return nil, err
	}
	targets := make([]discovery_kit_api.Target, 0)
	rows, ok := results.Data.([]any)
	if !ok {
		return targets, nil
	}
	for _, r := range rows {
		items, ok := r.(map[string]any)
		if !ok {
			continue
		}
		targets = append(targets, toAppGatewayTarget(items))
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
	attributes["azure.subscription.id"] = []string{stringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{stringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{stringFromMap(items, "location")}

	if v := stringFromMap(sku, "name"); v != "" {
		attributes["azure.application-gateway.sku-name"] = []string{v}
	}
	if v := stringFromMap(sku, "tier"); v != "" {
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
	if v := stringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.application-gateway.provisioning-state"] = []string{v}
	}

	wafEnabled := false
	if v, ok := wafConfig["enabled"].(bool); ok {
		wafEnabled = v
	}
	attributes["azure.application-gateway.waf-enabled"] = []string{strconv.FormatBool(wafEnabled)}
	if wafEnabled {
		if v := stringFromMap(wafConfig, "firewallMode"); v != "" {
			attributes["azure.application-gateway.waf.firewall-mode"] = []string{v}
		}
		if v := stringFromMap(wafConfig, "ruleSetType"); v != "" {
			attributes["azure.application-gateway.waf.rule-set-type"] = []string{v}
		}
		if v := stringFromMap(wafConfig, "ruleSetVersion"); v != "" {
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

func stringFromMap(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
