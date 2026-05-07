/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extservicebus

import (
	"context"
	"fmt"
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
	TargetIDNamespace = "com.steadybit.extension_azure.servicebus.namespace"
	targetIcon        = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNNCA0aDE2djJINFY0em0wIDdoMTZ2Mkg0di0yem0wIDdoMTZ2Mkg0di0yeiIgZmlsbD0iY3VycmVudENvbG9yIi8+PC9zdmc+"
)

type namespaceDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*namespaceDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*namespaceDiscovery)(nil)
)

func NewNamespaceDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&namespaceDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *namespaceDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDNamespace,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *namespaceDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDNamespace,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure Service Bus namespace", Other: "Azure Service Bus namespaces"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.servicebus.sku-name"},
				{Attribute: "azure.servicebus.zone-redundant"},
				{Attribute: "azure.servicebus.public-network-access"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *namespaceDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.servicebus.namespace.name", Label: discovery_kit_api.PluralLabel{One: "Service Bus namespace name", Other: "Service Bus namespace names"}},
		{Attribute: "azure.servicebus.sku-name", Label: discovery_kit_api.PluralLabel{One: "Service Bus SKU name", Other: "Service Bus SKU names"}},
		{Attribute: "azure.servicebus.sku-tier", Label: discovery_kit_api.PluralLabel{One: "Service Bus SKU tier", Other: "Service Bus SKU tiers"}},
		{Attribute: "azure.servicebus.sku-capacity", Label: discovery_kit_api.PluralLabel{One: "Service Bus SKU capacity", Other: "Service Bus SKU capacities"}},
		{Attribute: "azure.servicebus.zone-redundant", Label: discovery_kit_api.PluralLabel{One: "Service Bus zone-redundant", Other: "Service Bus zone-redundant"}},
		{Attribute: "azure.servicebus.minimum-tls-version", Label: discovery_kit_api.PluralLabel{One: "Service Bus minimum TLS version", Other: "Service Bus minimum TLS versions"}},
		{Attribute: "azure.servicebus.public-network-access", Label: discovery_kit_api.PluralLabel{One: "Service Bus public network access", Other: "Service Bus public network access"}},
		{Attribute: "azure.servicebus.disable-local-auth", Label: discovery_kit_api.PluralLabel{One: "Service Bus local auth disabled", Other: "Service Bus local auth disabled"}},
		{Attribute: "azure.servicebus.provisioning-state", Label: discovery_kit_api.PluralLabel{One: "Service Bus provisioning state", Other: "Service Bus provisioning states"}},
		{Attribute: "azure.servicebus.endpoint", Label: discovery_kit_api.PluralLabel{One: "Service Bus endpoint", Other: "Service Bus endpoints"}},
	}
}

func (d *namespaceDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllNamespaces(ctx, client)
}

func getAllNamespaces(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	results, err := client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: extutil.Ptr("Resources | where type =~ 'Microsoft.ServiceBus/namespaces' | project id, name, type, resourceGroup, location, tags, properties, sku, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get Service Bus namespace results")
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
		targets = append(targets, toNamespaceTarget(items))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesServiceBus), nil
}

func toNamespaceTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")
	sku := common.GetMapValue(items, "sku")

	id, _ := items["id"].(string)
	name, _ := items["name"].(string)

	attributes := make(map[string][]string)
	attributes["azure.servicebus.namespace.name"] = []string{name}
	attributes["azure.subscription.id"] = []string{stringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{stringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{stringFromMap(items, "location")}

	if v := stringFromMap(sku, "name"); v != "" {
		attributes["azure.servicebus.sku-name"] = []string{v}
	}
	if v := stringFromMap(sku, "tier"); v != "" {
		attributes["azure.servicebus.sku-tier"] = []string{v}
	}
	if v, ok := sku["capacity"].(float64); ok {
		attributes["azure.servicebus.sku-capacity"] = []string{strconv.Itoa(int(v))}
	}
	if v, ok := properties["zoneRedundant"].(bool); ok {
		attributes["azure.servicebus.zone-redundant"] = []string{strconv.FormatBool(v)}
	}
	if v := stringFromMap(properties, "minimumTlsVersion"); v != "" {
		attributes["azure.servicebus.minimum-tls-version"] = []string{v}
	}
	if v := stringFromMap(properties, "publicNetworkAccess"); v != "" {
		attributes["azure.servicebus.public-network-access"] = []string{v}
	}
	if v, ok := properties["disableLocalAuth"].(bool); ok {
		attributes["azure.servicebus.disable-local-auth"] = []string{strconv.FormatBool(v)}
	}
	if v := stringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.servicebus.provisioning-state"] = []string{v}
	}
	if v := stringFromMap(properties, "serviceBusEndpoint"); v != "" {
		attributes["azure.servicebus.endpoint"] = []string{v}
	}

	for k, v := range common.GetMapValue(items, "tags") {
		attributes[fmt.Sprintf("azure.servicebus.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDNamespace,
		Label:      name,
		Attributes: attributes,
	}
}

func stringFromMap(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
