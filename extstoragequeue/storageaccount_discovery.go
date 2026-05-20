/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

// Package extstoragequeue discovers Azure Storage accounts that have the queue service enabled.
// Modeling the Storage account as the target keeps things simple — Storage Queues themselves are nested resources
// and surfacing every individual queue would explode target counts. Reliability-relevant config (sku, replication,
// network access, encryption, soft-delete, infrastructure encryption, public access, allow-shared-key-access) all
// lives at the Storage account level.
package extstoragequeue

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
	TargetIDStorageQueue = "com.steadybit.extension_azure.storage-queue"
	targetIcon           = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNMyA3aDE0djJIM3YxMmgxOFY3aC00VjVoNHYyaDFWMjFoLTIwVjVoMnYyeiIgZmlsbD0iY3VycmVudENvbG9yIi8+PC9zdmc+"
)

type accountDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*accountDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*accountDiscovery)(nil)
)

func NewStorageAccountDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&accountDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *accountDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDStorageQueue,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *accountDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDStorageQueue,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure Storage Queue", Other: "Azure Storage Queues"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.storage.sku-name"},
				{Attribute: "azure.storage.kind"},
				{Attribute: "azure.storage.public-network-access"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *accountDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.storage.account.name", Label: discovery_kit_api.PluralLabel{One: "Storage account name", Other: "Storage account names"}},
		{Attribute: "azure.storage.kind", Label: discovery_kit_api.PluralLabel{One: "Storage account kind", Other: "Storage account kinds"}},
		{Attribute: "azure.storage.sku-name", Label: discovery_kit_api.PluralLabel{One: "Storage SKU name", Other: "Storage SKU names"}},
		{Attribute: "azure.storage.sku-tier", Label: discovery_kit_api.PluralLabel{One: "Storage SKU tier", Other: "Storage SKU tiers"}},
		{Attribute: "azure.storage.public-network-access", Label: discovery_kit_api.PluralLabel{One: "Storage public network access", Other: "Storage public network access"}},
		{Attribute: "azure.storage.allow-blob-public-access", Label: discovery_kit_api.PluralLabel{One: "Storage allow blob public access", Other: "Storage allow blob public access"}},
		{Attribute: "azure.storage.allow-shared-key-access", Label: discovery_kit_api.PluralLabel{One: "Storage allow shared key access", Other: "Storage allow shared key access"}},
		{Attribute: "azure.storage.minimum-tls-version", Label: discovery_kit_api.PluralLabel{One: "Storage minimum TLS version", Other: "Storage minimum TLS versions"}},
		{Attribute: "azure.storage.supports-https-traffic-only", Label: discovery_kit_api.PluralLabel{One: "Storage HTTPS only", Other: "Storage HTTPS only"}},
		{Attribute: "azure.storage.encryption.key-source", Label: discovery_kit_api.PluralLabel{One: "Storage encryption key source", Other: "Storage encryption key sources"}},
		{Attribute: "azure.storage.encryption.require-infrastructure-encryption", Label: discovery_kit_api.PluralLabel{One: "Storage requires infrastructure encryption", Other: "Storage requires infrastructure encryption"}},
		{Attribute: "azure.storage.queue.endpoint", Label: discovery_kit_api.PluralLabel{One: "Storage queue endpoint", Other: "Storage queue endpoints"}},
		{Attribute: "azure.storage.provisioning-state", Label: discovery_kit_api.PluralLabel{One: "Storage provisioning state", Other: "Storage provisioning states"}},
	}
}

func (d *accountDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllStorageAccounts(ctx, client)
}

func getAllStorageAccounts(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	// We surface all storage accounts (not only those known to host queues) — most accounts can host queues, and the
	// exact list of queues is not relevant for reliability findings. Filter at agent / experiment level if needed.
	results, err := client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: extutil.Ptr("Resources | where type =~ 'Microsoft.Storage/storageAccounts' | project id, name, kind, type, resourceGroup, location, tags, properties, sku, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get Storage account results")
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
		targets = append(targets, toStorageAccountTarget(items))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesStorageQueue), nil
}

func toStorageAccountTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")
	encryption := common.GetMapValue(properties, "encryption")
	primaryEndpoints := common.GetMapValue(properties, "primaryEndpoints")
	sku := common.GetMapValue(items, "sku")

	id, _ := items["id"].(string)
	name, _ := items["name"].(string)

	attributes := make(map[string][]string)
	attributes["azure.storage.account.name"] = []string{name}
	attributes["azure.subscription.id"] = []string{stringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{stringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{stringFromMap(items, "location")}

	if v := stringFromMap(items, "kind"); v != "" {
		attributes["azure.storage.kind"] = []string{v}
	}
	if v := stringFromMap(sku, "name"); v != "" {
		attributes["azure.storage.sku-name"] = []string{v}
	}
	if v := stringFromMap(sku, "tier"); v != "" {
		attributes["azure.storage.sku-tier"] = []string{v}
	}
	if v := stringFromMap(properties, "publicNetworkAccess"); v != "" {
		attributes["azure.storage.public-network-access"] = []string{v}
	}
	if v, ok := properties["allowBlobPublicAccess"].(bool); ok {
		attributes["azure.storage.allow-blob-public-access"] = []string{strconv.FormatBool(v)}
	}
	if v, ok := properties["allowSharedKeyAccess"].(bool); ok {
		attributes["azure.storage.allow-shared-key-access"] = []string{strconv.FormatBool(v)}
	}
	if v := stringFromMap(properties, "minimumTlsVersion"); v != "" {
		attributes["azure.storage.minimum-tls-version"] = []string{v}
	}
	if v, ok := properties["supportsHttpsTrafficOnly"].(bool); ok {
		attributes["azure.storage.supports-https-traffic-only"] = []string{strconv.FormatBool(v)}
	}
	if v := stringFromMap(encryption, "keySource"); v != "" {
		attributes["azure.storage.encryption.key-source"] = []string{v}
	}
	if v, ok := encryption["requireInfrastructureEncryption"].(bool); ok {
		attributes["azure.storage.encryption.require-infrastructure-encryption"] = []string{strconv.FormatBool(v)}
	}
	if v := stringFromMap(primaryEndpoints, "queue"); v != "" {
		attributes["azure.storage.queue.endpoint"] = []string{v}
	}
	if v := stringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.storage.provisioning-state"] = []string{v}
	}

	for k, v := range common.GetMapValue(items, "tags") {
		attributes[fmt.Sprintf("azure.storage.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDStorageQueue,
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
