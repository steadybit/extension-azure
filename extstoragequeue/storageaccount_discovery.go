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
	TargetIDStorageQueue = "com.steadybit.extension_azure.storage-queue"
	targetIcon           = "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZmlsbC1ydWxlPSJldmVub2RkIiBjbGlwLXJ1bGU9ImV2ZW5vZGQiIGQ9Ik0yMC45OTk5IDE4Ljg3MjFDMjAuOTk5OSAxOS4wMzE0IDIwLjkzNjcgMTkuMTg1MSAyMC44MjQxIDE5LjI5NzlDMjAuNzExNCAxOS40MTA2IDIwLjU1NzggMTkuNDczNiAyMC4zOTgzIDE5LjQ3MzZIMy42MDE0NEMzLjQ0MTk1IDE5LjQ3MzYgMy4yODg0MyAxOS40MTA2IDMuMTc1NjYgMTkuMjk3OUMzLjA2Mjk2IDE5LjE4NTEgMi45OTk4OCAxOS4wMzE1IDIuOTk5ODggMTguODcyMVY4LjMxNjQxSDIwLjk5OTlWMTguODcyMVpNNS41MDY3MSA5LjUwNzgxQzUuMzQ1NzggOS41MDc4MSA1LjIxNDkyIDkuNjM3OTQgNS4yMTQ3MiA5Ljc5ODgzVjE3LjY1OTJDNS4yMTQ3MyAxNy44MjAyIDUuMzQ1NjUgMTcuOTUxMiA1LjUwNjcxIDE3Ljk1MTJIOC44NDI2NUM5LjAwMzY1IDE3Ljk1MTEgOS4xMzQ2NCAxNy44MjAyIDkuMTM0NjQgMTcuNjU5MlY5Ljc5ODgzQzkuMTM0NDQgOS42Mzc5OCA5LjAwMzUzIDkuNTA3ODggOC44NDI2NSA5LjUwNzgxSDUuNTA2NzFaTTEwLjMzMDkgOS41MDc4MUMxMC4xNyA5LjUwNzgzIDEwLjAzOTEgOS42Mzc5NCAxMC4wMzg5IDkuNzk4ODNWMTcuNjU5MkMxMC4wMzg5IDE3LjgyMDIgMTAuMTY5OSAxNy45NTEyIDEwLjMzMDkgMTcuOTUxMkgxMy42Njc4QzEzLjgyODggMTcuOTUxIDEzLjk1ODkgMTcuODIwMSAxMy45NTg5IDE3LjY1OTJWOS43OTg4M0MxMy45NTg3IDkuNjM4MDQgMTMuODI4NiA5LjUwNzk5IDEzLjY2NzggOS41MDc4MUgxMC4zMzA5Wk0xNS4xNTcxIDkuNTA3ODFDMTQuOTk2MiA5LjUwNzgxIDE0Ljg2NTMgOS42Mzc5MyAxNC44NjUxIDkuNzk4ODNWMTcuNjU5MkMxNC44NjUxIDE3LjgyMDIgMTQuOTk2IDE3Ljk1MTIgMTUuMTU3MSAxNy45NTEySDE4LjQ5M0MxOC42NTQgMTcuOTUxMSAxOC43ODUgMTcuODIwMiAxOC43ODUgMTcuNjU5MlY5Ljc5ODgzQzE4Ljc4NDggOS42Mzc5NyAxOC42NTM5IDkuNTA3ODggMTguNDkzIDkuNTA3ODFIMTUuMTU3MVoiIGZpbGw9ImN1cnJlbnRDb2xvciIvPgo8cGF0aCBkPSJNMjAuOTk2IDguMzE1NDNIMy4wMDI4MVY4LjIzMDQ3SDIwLjk5NlY4LjMxNTQzWiIgZmlsbD0iY3VycmVudENvbG9yIi8+CjxwYXRoIGQ9Ik0yMC4zOTU0IDVDMjAuNTU0NiA1LjAwMDEgMjAuNzA3NSA1LjA2MzI1IDIwLjgyMDIgNS4xNzU3OEMyMC45MzMgNS4yODg1NiAyMC45OTYgNS40NDIwOCAyMC45OTYgNS42MDE1NlY3LjIzMDQ3SDMuMDAyODFWNS42MDI1NEMzLjAwMjY3IDUuNTIzNDggMy4wMTg1NSA1LjQ0NTE1IDMuMDQ4NzEgNS4zNzIwN0MzLjA3ODg2IDUuMjk5MDcgMy4xMjI3OSA1LjIzMjY2IDMuMTc4NTkgNS4xNzY3NkMzLjIzNDQ0IDUuMTIwOCAzLjMwMDg3IDUuMDc2MTkgMy4zNzM5IDUuMDQ1OUMzLjQ0Njg4IDUuMDE1NjYgMy41MjUzOCA1IDMuNjA0MzcgNUgyMC4zOTU0WiIgZmlsbD0iY3VycmVudENvbG9yIi8+Cjwvc3ZnPgo="
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
	// We surface all storage accounts (not only those known to host queues) — most accounts can host queues, and the
	// exact list of queues is not relevant for reliability findings. Filter at agent / experiment level if needed.
	targets, err := common.DiscoverViaResourceGraph(ctx, client,
		"Resources | where type =~ 'Microsoft.Storage/storageAccounts' | project id, name, kind, type, resourceGroup, location, tags, properties, sku, subscriptionId",
		toStorageAccountTarget)
	if err != nil {
		log.Error().Err(err).Msg("failed to get Storage account results")
		return nil, err
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
	attributes["azure.subscription.id"] = []string{common.StringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{common.StringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{common.StringFromMap(items, "location")}

	if v := common.StringFromMap(items, "kind"); v != "" {
		attributes["azure.storage.kind"] = []string{v}
	}
	if v := common.StringFromMap(sku, "name"); v != "" {
		attributes["azure.storage.sku-name"] = []string{v}
	}
	if v := common.StringFromMap(sku, "tier"); v != "" {
		attributes["azure.storage.sku-tier"] = []string{v}
	}
	if v := common.StringFromMap(properties, "publicNetworkAccess"); v != "" {
		attributes["azure.storage.public-network-access"] = []string{v}
	}
	if v, ok := properties["allowBlobPublicAccess"].(bool); ok {
		attributes["azure.storage.allow-blob-public-access"] = []string{strconv.FormatBool(v)}
	}
	if v, ok := properties["allowSharedKeyAccess"].(bool); ok {
		attributes["azure.storage.allow-shared-key-access"] = []string{strconv.FormatBool(v)}
	}
	if v := common.StringFromMap(properties, "minimumTlsVersion"); v != "" {
		attributes["azure.storage.minimum-tls-version"] = []string{v}
	}
	if v, ok := properties["supportsHttpsTrafficOnly"].(bool); ok {
		attributes["azure.storage.supports-https-traffic-only"] = []string{strconv.FormatBool(v)}
	}
	if v := common.StringFromMap(encryption, "keySource"); v != "" {
		attributes["azure.storage.encryption.key-source"] = []string{v}
	}
	if v, ok := encryption["requireInfrastructureEncryption"].(bool); ok {
		attributes["azure.storage.encryption.require-infrastructure-encryption"] = []string{strconv.FormatBool(v)}
	}
	if v := common.StringFromMap(primaryEndpoints, "queue"); v != "" {
		attributes["azure.storage.queue.endpoint"] = []string{v}
	}
	if v := common.StringFromMap(properties, "provisioningState"); v != "" {
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
