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
	TargetIDNamespace = "com.steadybit.extension_azure.servicebus.namespace"
	targetIcon        = "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTMuMzAyNjIgMTUuODAxM0g0LjY2ODQ4QzQuNzA4MjYgMTUuODAxMyA0Ljc0NzYgMTUuODA5MSA0Ljc4NDI5IDE1LjgyNDNDNC44MjA5OSAxNS44Mzk2IDQuODU0NDEgMTUuODYxOSA0Ljg4MjYyIDE1Ljg5QzQuOTEwNzIgMTUuOTE4MSA0LjkzMzEyIDE1Ljk1MTUgNC45NDgyNiAxNS45ODgzQzQuOTYzNSAxNi4wMjUgNC45NzEzMyAxNi4wNjQ1IDQuOTcxMzMgMTYuMTA0MVYxOC4wMDA2SDE5LjAyODRWMTYuMTExNUMxOS4wMjc1IDE2LjA3MTIgMTkuMDM0NyAxNi4wMzEgMTkuMDQ5NSAxNS45OTM1QzE5LjA2NDIgMTUuOTU1OSAxOS4wODYyIDE1LjkyMTYgMTkuMTE0NiAxNS44OTI3QzE5LjE0MjggMTUuODYzOCAxOS4xNzY1IDE1Ljg0MDcgMTkuMjEzNyAxNS44MjVDMTkuMjUwOSAxNS44MDkzIDE5LjI5MSAxNS44MDEzIDE5LjMzMTQgMTUuODAxM0gyMC42OTY5QzIwLjc3NzMgMTUuODAxMyAyMC44NTQzIDE1LjgzMzIgMjAuOTExMiAxNS44OTAxQzIwLjk2NzkgMTUuOTQ2OSAyMC45OTk5IDE2LjAyNCAyMC45OTk5IDE2LjEwNDJWMTkuMzE3N0MyMC45OTk5IDE5LjMxODcgMjAuOTk5OCAxOS4zMTk4IDIwLjk5OTggMTkuMzIwOUwyMC45OTkgMTkuMzUwN0MyMC45OTE2IDE5LjQ5OTEgMjAuOTI5NSAxOS42Mzk5IDIwLjgyMzkgMTkuNzQ1NEMyMC43ODg0IDE5Ljc4MSAyMC43NDg2IDE5LjgxMTUgMjAuNzA2MSAxOS44MzY4QzIwLjYxMjcgMTkuODkyOSAyMC41MDUxIDE5LjkyMzMgMjAuMzk0MyAxOS45MjMzSDE5LjAyODRWMTkuOTIxM0gzLjYwMDMyQzMuNDQxMTQgMTkuOTIxMyAzLjI4ODM0IDE5Ljg1OCAzLjE3NTgzIDE5Ljc0NTRDMy4wNjMyMSAxOS42MzI5IDIuOTk5ODggMTkuNDgwMSAyLjk5OTg4IDE5LjMyMDlWMTYuMTA0MUMyLjk5OTg4IDE2LjAyMzggMy4wMzE4MSAxNS45NDY3IDMuMDg4NTggMTUuODlDMy4xNDUzNCAxNS44MzMxIDMuMjIyMzYgMTUuODAxMyAzLjMwMjYyIDE1LjgwMTNaIiBmaWxsPSJjdXJyZW50Q29sb3IiLz4KPHBhdGggZD0iTTE1LjgzNjcgMTYuMzQ3NlYxNi4zNTJIOC4xNTIyOVYxNi4zNDc2TDExLjk5NDUgMTMuODA4NUwxNS44MzY3IDE2LjM0NzZaIiBmaWxsPSJjdXJyZW50Q29sb3IiLz4KPHBhdGggZD0iTTcuMzQ3NDQgMTAuODQ1OEwxMS4wMzUgMTMuMjgyNEw3LjM1NDA1IDE1LjcxOUM3LjM1NDA1IDE1LjYyIDcuMzQ3NTUgMTAuOTI1NiA3LjM0NzMzIDEwLjg0NTZMNy4zNDc0NCAxMC44NDU4WiIgZmlsbD0iY3VycmVudENvbG9yIi8+CjxwYXRoIGQ9Ik0xNi42NDE4IDEwLjg0ODhDMTYuNjQxOCAxMC43NjUxIDE2LjYzNSAxNS42MTc4IDE2LjYzNSAxNS43MTU5TDEyLjk1ODYgMTMuMjgyMkwxNi42NDE3IDEwLjg0ODdMMTYuNjQxOCAxMC44NDg4WiIgZmlsbD0iY3VycmVudENvbG9yIi8+CjxwYXRoIGQ9Ik0xMS43MDc3IDcuNzc3NDhDMTEuODgxOSA3LjY1ODY3IDEyLjExMTIgNy42NTg2OCAxMi4yODU0IDcuNzc3NDhDMTMuMTc1NCA4LjM4NDcxIDE1LjgxNjkgMTAuMTg3MiAxNS44MjczIDEwLjE5NDJMMTUuODQ4NSAxMC4yMDgyTDExLjk5NDggMTIuNzUyN0w4LjE0NCAxMC4yMDgyQzguMTY4MjUgMTAuMTkxNyAxMC44MTY1IDguMzg1NCAxMS43MDc3IDcuNzc3NDhaIiBmaWxsPSJjdXJyZW50Q29sb3IiLz4KPHBhdGggZD0iTTIwLjM5MzkgMy44OTA2OUMyMC4zOTU0IDMuODkwNjkgMjAuMzk2OCAzLjg5MDc5IDIwLjM5ODIgMy44OTA4SDIwLjQwMzZDMjAuNTYyMSAzLjg5MTk1IDIwLjcxMzcgMy45NTU3MSAyMC44MjUzIDQuMDY4MUMyMC45MzcgNC4xODA2MSAyMC45OTk4IDQuMzMyNjMgMjAuOTk5OCA0LjQ5MTEzTDIwLjk5OTcgNy43MDc3QzIwLjk5OTYgNy43ODc5NyAyMC45Njc3IDcuODY1MDggMjAuOTEwOSA3LjkyMTg1QzIwLjg1NDEgNy45Nzg2IDIwLjc3NzEgOC4wMTA0NCAyMC42OTY3IDguMDEwNDRIMTkuMzMwOEMxOS4yNTA2IDguMDEwNDMgMTkuMTczNiA3Ljk3ODU4IDE5LjExNjggNy45MjE4NUMxOS4wNiA3Ljg2NTA4IDE5LjAyOCA3Ljc4Nzk3IDE5LjAyOCA3LjcwNzdWNS44MTE0SDQuOTcxMzNWNy43MDI0NEM0Ljk3MTMyIDcuNzgyNjkgNC45MzkzNyA3Ljg1OTcxIDQuODgyNjIgNy45MTY0N0M0LjgyNTg1IDcuOTczMjQgNC43NDg3NiA4LjAwNTE4IDQuNjY4NDggOC4wMDUxOEgzLjMwMjYyQzMuMjIzMjcgOC4wMDUxNyAzLjE0NzAyIDcuOTczOTggMy4wOTAzOCA3LjkxODM4QzMuMDMzNzMgNy44NjI3NiAzLjAwMTI2IDcuNzg3MDYgMi45OTk4OCA3LjcwNzdWNC40OTExM0MyLjk5OTg4IDQuMzMxOTQgMy4wNjMxIDQuMTc5MTUgMy4xNzU3MiA0LjA2NjY0QzMuMjg4MzUgMy45NTQwMiAzLjQ0MTAyIDMuODkwOCAzLjYwMDIxIDMuODkwOEgzLjYwMTIxQzMuNjAyNjIgMy44OTA3OSAzLjYwNDA2IDMuODkwNjkgMy42MDU0NyAzLjg5MDY5SDIwLjM5MzlaIiBmaWxsPSJjdXJyZW50Q29sb3IiLz4KPC9zdmc+Cg=="
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
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: new("60s")},
	}
}

func (d *namespaceDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDNamespace,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     new(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure Service Bus namespace", Other: "Azure Service Bus namespaces"},
		Category: new("cloud"),
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
	targets, err := common.DiscoverViaResourceGraph(ctx, client,
		"Resources | where type =~ 'Microsoft.ServiceBus/namespaces' | project id, name, type, resourceGroup, location, tags, properties, sku, subscriptionId",
		toNamespaceTarget)
	if err != nil {
		log.Error().Err(err).Msg("failed to get Service Bus namespace results")
		return nil, err
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
	attributes["azure.subscription.id"] = []string{common.StringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{common.StringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{common.StringFromMap(items, "location")}

	if v := common.StringFromMap(sku, "name"); v != "" {
		attributes["azure.servicebus.sku-name"] = []string{v}
	}
	if v := common.StringFromMap(sku, "tier"); v != "" {
		attributes["azure.servicebus.sku-tier"] = []string{v}
	}
	if v, ok := sku["capacity"].(float64); ok {
		attributes["azure.servicebus.sku-capacity"] = []string{strconv.Itoa(int(v))}
	}
	if v, ok := properties["zoneRedundant"].(bool); ok {
		attributes["azure.servicebus.zone-redundant"] = []string{strconv.FormatBool(v)}
	}
	if v := common.StringFromMap(properties, "minimumTlsVersion"); v != "" {
		attributes["azure.servicebus.minimum-tls-version"] = []string{v}
	}
	if v := common.StringFromMap(properties, "publicNetworkAccess"); v != "" {
		attributes["azure.servicebus.public-network-access"] = []string{v}
	}
	if v, ok := properties["disableLocalAuth"].(bool); ok {
		attributes["azure.servicebus.disable-local-auth"] = []string{strconv.FormatBool(v)}
	}
	if v := common.StringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.servicebus.provisioning-state"] = []string{v}
	}
	if v := common.StringFromMap(properties, "serviceBusEndpoint"); v != "" {
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
