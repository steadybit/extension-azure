/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extloadbalancer

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
	TargetIDLoadBalancer = "com.steadybit.extension_azure.load-balancer"
	targetIcon           = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNNCA0aDE2djJINFY0em0wIDdoMTZ2Mkg0di0yem0wIDdoMTZ2Mkg0di0yeiIgZmlsbD0iY3VycmVudENvbG9yIi8+PC9zdmc+"
)

type loadBalancerDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*loadBalancerDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*loadBalancerDiscovery)(nil)
)

func NewLoadBalancerDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&loadBalancerDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *loadBalancerDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDLoadBalancer,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *loadBalancerDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDLoadBalancer,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure Load Balancer", Other: "Azure Load Balancers"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.load-balancer.sku-name"},
				{Attribute: "azure.load-balancer.frontend.public-exposed"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *loadBalancerDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.load-balancer.name", Label: discovery_kit_api.PluralLabel{One: "Load Balancer name", Other: "Load Balancer names"}},
		{Attribute: "azure.load-balancer.sku-name", Label: discovery_kit_api.PluralLabel{One: "Load Balancer SKU name", Other: "Load Balancer SKU names"}},
		{Attribute: "azure.load-balancer.sku-tier", Label: discovery_kit_api.PluralLabel{One: "Load Balancer SKU tier", Other: "Load Balancer SKU tiers"}},
		{Attribute: "azure.load-balancer.frontend.public-exposed", Label: discovery_kit_api.PluralLabel{One: "Load Balancer has public frontend", Other: "Load Balancer has public frontend"}},
		{Attribute: "azure.load-balancer.frontend.zones", Label: discovery_kit_api.PluralLabel{One: "Load Balancer frontend zone", Other: "Load Balancer frontend zones"}},
		{Attribute: "azure.load-balancer.backend-pool-count", Label: discovery_kit_api.PluralLabel{One: "Load Balancer backend pool count", Other: "Load Balancer backend pool counts"}},
		{Attribute: "azure.load-balancer.load-balancing-rule-count", Label: discovery_kit_api.PluralLabel{One: "Load Balancer rule count", Other: "Load Balancer rule counts"}},
		{Attribute: "azure.load-balancer.outbound-rule-count", Label: discovery_kit_api.PluralLabel{One: "Load Balancer outbound rule count", Other: "Load Balancer outbound rule counts"}},
		{Attribute: "azure.load-balancer.probe-count", Label: discovery_kit_api.PluralLabel{One: "Load Balancer probe count", Other: "Load Balancer probe counts"}},
		{Attribute: "azure.load-balancer.provisioning-state", Label: discovery_kit_api.PluralLabel{One: "Load Balancer provisioning state", Other: "Load Balancer provisioning states"}},
	}
}

func (d *loadBalancerDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllLoadBalancers(ctx, client)
}

func getAllLoadBalancers(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	results, err := client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: extutil.Ptr("Resources | where type =~ 'Microsoft.Network/loadBalancers' | project id, name, type, resourceGroup, location, tags, properties, sku, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get Load Balancer results")
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
		targets = append(targets, toLoadBalancerTarget(items))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesLoadBalancer), nil
}

func toLoadBalancerTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")
	sku := common.GetMapValue(items, "sku")

	id, _ := items["id"].(string)
	name, _ := items["name"].(string)

	attributes := make(map[string][]string)
	attributes["azure.load-balancer.name"] = []string{name}
	attributes["azure.subscription.id"] = []string{stringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{stringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{stringFromMap(items, "location")}

	if v := stringFromMap(sku, "name"); v != "" {
		attributes["azure.load-balancer.sku-name"] = []string{v}
	}
	if v := stringFromMap(sku, "tier"); v != "" {
		attributes["azure.load-balancer.sku-tier"] = []string{v}
	}
	if v := stringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.load-balancer.provisioning-state"] = []string{v}
	}

	publicExposed, frontendZones := analyzeFrontendIps(properties)
	attributes["azure.load-balancer.frontend.public-exposed"] = []string{strconv.FormatBool(publicExposed)}
	if len(frontendZones) > 0 {
		sort.Strings(frontendZones)
		attributes["azure.load-balancer.frontend.zones"] = frontendZones
	}

	attributes["azure.load-balancer.backend-pool-count"] = []string{strconv.Itoa(arrayLen(properties, "backendAddressPools"))}
	attributes["azure.load-balancer.load-balancing-rule-count"] = []string{strconv.Itoa(arrayLen(properties, "loadBalancingRules"))}
	attributes["azure.load-balancer.outbound-rule-count"] = []string{strconv.Itoa(arrayLen(properties, "outboundRules"))}
	attributes["azure.load-balancer.probe-count"] = []string{strconv.Itoa(arrayLen(properties, "probes"))}

	for k, v := range common.GetMapValue(items, "tags") {
		attributes[fmt.Sprintf("azure.load-balancer.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDLoadBalancer,
		Label:      name,
		Attributes: attributes,
	}
}

// analyzeFrontendIps walks frontendIPConfigurations to determine if any of them have a publicIPAddress reference,
// and aggregates the unique availability zones declared on those frontend configs.
func analyzeFrontendIps(properties map[string]any) (publicExposed bool, zones []string) {
	v, ok := properties["frontendIPConfigurations"]
	if !ok {
		return false, nil
	}
	arr, ok := v.([]any)
	if !ok {
		return false, nil
	}
	zoneSet := make(map[string]struct{})
	for _, e := range arr {
		fc, ok := e.(map[string]any)
		if !ok {
			continue
		}
		fcProps := common.GetMapValue(fc, "properties")
		publicIP := common.GetMapValue(fcProps, "publicIPAddress")
		if id, ok := publicIP["id"].(string); ok && id != "" {
			publicExposed = true
		}
		if zArr, ok := fc["zones"].([]any); ok {
			for _, z := range zArr {
				if s, ok := z.(string); ok && s != "" {
					zoneSet[s] = struct{}{}
				}
			}
		}
	}
	for z := range zoneSet {
		zones = append(zones, z)
	}
	return publicExposed, zones
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

func stringFromMap(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
