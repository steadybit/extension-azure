/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extnatgateway

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
	TargetIDNatGateway = "com.steadybit.extension_azure.nat-gateway"
	targetIcon         = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNMTIgMmw0IDRoLTN2NGgtMlY2SDhsNC00em0wIDIwbC00LTRoM3YtNGgyVjE4aDNsLTQgNHpNMiAxMmw0LTR2M2g0djJINlYxNmwtNC00em0yMCAwbC00IDR2LTNoLTR2LTJoNFY4bDQgNHoiIGZpbGw9ImN1cnJlbnRDb2xvciIvPjwvc3ZnPg=="
)

type natGatewayDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*natGatewayDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*natGatewayDiscovery)(nil)
)

func NewNatGatewayDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&natGatewayDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *natGatewayDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDNatGateway,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *natGatewayDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDNatGateway,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure NAT Gateway", Other: "Azure NAT Gateways"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.nat-gateway.zones"},
				{Attribute: "azure.nat-gateway.idle-timeout-in-minutes"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *natGatewayDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.nat-gateway.name", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway name", Other: "NAT Gateway names"}},
		{Attribute: "azure.nat-gateway.sku-name", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway SKU", Other: "NAT Gateway SKUs"}},
		{Attribute: "azure.nat-gateway.zones", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway zone", Other: "NAT Gateway zones"}},
		{Attribute: "azure.nat-gateway.idle-timeout-in-minutes", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway idle timeout (minutes)", Other: "NAT Gateway idle timeouts (minutes)"}},
		{Attribute: "azure.nat-gateway.public-ip-addresses", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway Public IP", Other: "NAT Gateway Public IPs"}},
		{Attribute: "azure.nat-gateway.public-ip-prefixes", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway Public IP prefix", Other: "NAT Gateway Public IP prefixes"}},
		{Attribute: "azure.nat-gateway.subnets", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway subnet", Other: "NAT Gateway subnets"}},
		{Attribute: "azure.nat-gateway.provisioning-state", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway provisioning state", Other: "NAT Gateway provisioning states"}},
	}
}

func (d *natGatewayDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllNatGateways(ctx, client)
}

func getAllNatGateways(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	results, err := client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: extutil.Ptr("Resources | where type =~ 'Microsoft.Network/natGateways' | project id, name, type, resourceGroup, location, tags, properties, sku, zones, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get NAT Gateway results")
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
		targets = append(targets, toNatGatewayTarget(items))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesNatGateway), nil
}

func toNatGatewayTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")
	sku := common.GetMapValue(items, "sku")

	id, _ := items["id"].(string)
	name, _ := items["name"].(string)

	attributes := make(map[string][]string)
	attributes["azure.nat-gateway.name"] = []string{name}
	attributes["azure.subscription.id"] = []string{stringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{stringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{stringFromMap(items, "location")}

	if v := stringFromMap(sku, "name"); v != "" {
		attributes["azure.nat-gateway.sku-name"] = []string{v}
	}
	if zones := stringSliceFromMap(items, "zones"); len(zones) > 0 {
		sort.Strings(zones)
		attributes["azure.nat-gateway.zones"] = zones
	}
	if v, ok := properties["idleTimeoutInMinutes"].(float64); ok {
		attributes["azure.nat-gateway.idle-timeout-in-minutes"] = []string{strconv.Itoa(int(v))}
	}
	if v := stringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.nat-gateway.provisioning-state"] = []string{v}
	}

	if ips := referenceIds(properties, "publicIpAddresses"); len(ips) > 0 {
		sort.Strings(ips)
		attributes["azure.nat-gateway.public-ip-addresses"] = ips
	}
	if prefixes := referenceIds(properties, "publicIpPrefixes"); len(prefixes) > 0 {
		sort.Strings(prefixes)
		attributes["azure.nat-gateway.public-ip-prefixes"] = prefixes
	}
	if subnets := referenceIds(properties, "subnets"); len(subnets) > 0 {
		sort.Strings(subnets)
		attributes["azure.nat-gateway.subnets"] = subnets
	}

	for k, v := range common.GetMapValue(items, "tags") {
		attributes[fmt.Sprintf("azure.nat-gateway.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDNatGateway,
		Label:      name,
		Attributes: attributes,
	}
}

// referenceIds extracts the resource ARM IDs from a list of {id: ...} ARM references.
func referenceIds(m map[string]any, key string) []string {
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
		ref, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if id, ok := ref["id"].(string); ok && id != "" {
			out = append(out, id)
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

func stringSliceFromMap(m map[string]any, key string) []string {
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
