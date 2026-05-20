/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extcosmosdb

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
	TargetIDCosmosDbAccount = "com.steadybit.extension_azure.cosmosdb.account"
	targetIcon              = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNMTIgMmw5IDV2MTBsLTkgNS05LTVWN2w5LTV6IiBmaWxsPSJjdXJyZW50Q29sb3IiLz48L3N2Zz4="
)

type accountDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*accountDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*accountDiscovery)(nil)
)

func NewAccountDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&accountDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *accountDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDCosmosDbAccount,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *accountDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDCosmosDbAccount,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure Cosmos DB", Other: "Azure Cosmos DBs"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.cosmosdb.api-kind"},
				{Attribute: "azure.cosmosdb.consistency-level"},
				{Attribute: "azure.cosmosdb.public-network-access"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *accountDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.cosmosdb.account.name", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB account name", Other: "Cosmos DB account names"}},
		{Attribute: "azure.cosmosdb.api-kind", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB API kind", Other: "Cosmos DB API kinds"}},
		{Attribute: "azure.cosmosdb.consistency-level", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB consistency level", Other: "Cosmos DB consistency levels"}},
		{Attribute: "azure.cosmosdb.enable-multiple-write-locations", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB multi-write locations", Other: "Cosmos DB multi-write locations"}},
		{Attribute: "azure.cosmosdb.enable-automatic-failover", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB automatic failover", Other: "Cosmos DB automatic failover"}},
		{Attribute: "azure.cosmosdb.locations", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB location", Other: "Cosmos DB locations"}},
		{Attribute: "azure.cosmosdb.write-locations", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB write location", Other: "Cosmos DB write locations"}},
		{Attribute: "azure.cosmosdb.read-locations", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB read location", Other: "Cosmos DB read locations"}},
		{Attribute: "azure.cosmosdb.public-network-access", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB public network access", Other: "Cosmos DB public network access"}},
		{Attribute: "azure.cosmosdb.is-virtual-network-filter-enabled", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB VNet filter", Other: "Cosmos DB VNet filter"}},
		{Attribute: "azure.cosmosdb.disable-key-based-metadata-write-access", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB key-based metadata write disabled", Other: "Cosmos DB key-based metadata write disabled"}},
		{Attribute: "azure.cosmosdb.disable-local-auth", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB local auth disabled", Other: "Cosmos DB local auth disabled"}},
		{Attribute: "azure.cosmosdb.backup-policy.type", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB backup policy type", Other: "Cosmos DB backup policy types"}},
		{Attribute: "azure.cosmosdb.is-zone-redundant", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB zone-redundant", Other: "Cosmos DB zone-redundant"}},
		{Attribute: "azure.cosmosdb.provisioning-state", Label: discovery_kit_api.PluralLabel{One: "Cosmos DB provisioning state", Other: "Cosmos DB provisioning states"}},
	}
}

func (d *accountDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllCosmosDbAccounts(ctx, client)
}

func getAllCosmosDbAccounts(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	results, err := client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: extutil.Ptr("Resources | where type =~ 'Microsoft.DocumentDB/databaseAccounts' | project id, name, kind, type, resourceGroup, location, tags, properties, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get Cosmos DB results")
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
		targets = append(targets, toCosmosDbAccountTarget(items))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesCosmosDb), nil
}

func toCosmosDbAccountTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")
	consistency := common.GetMapValue(properties, "consistencyPolicy")
	backupPolicy := common.GetMapValue(properties, "backupPolicy")

	id, _ := items["id"].(string)
	name, _ := items["name"].(string)

	attributes := make(map[string][]string)
	attributes["azure.cosmosdb.account.name"] = []string{name}
	attributes["azure.subscription.id"] = []string{stringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{stringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{stringFromMap(items, "location")}

	if v := stringFromMap(items, "kind"); v != "" {
		attributes["azure.cosmosdb.api-kind"] = []string{v}
	}
	if v := stringFromMap(consistency, "defaultConsistencyLevel"); v != "" {
		attributes["azure.cosmosdb.consistency-level"] = []string{v}
	}
	if v, ok := properties["enableMultipleWriteLocations"].(bool); ok {
		attributes["azure.cosmosdb.enable-multiple-write-locations"] = []string{strconv.FormatBool(v)}
	}
	if v, ok := properties["enableAutomaticFailover"].(bool); ok {
		attributes["azure.cosmosdb.enable-automatic-failover"] = []string{strconv.FormatBool(v)}
	}
	if v := stringFromMap(properties, "publicNetworkAccess"); v != "" {
		attributes["azure.cosmosdb.public-network-access"] = []string{v}
	}
	if v, ok := properties["isVirtualNetworkFilterEnabled"].(bool); ok {
		attributes["azure.cosmosdb.is-virtual-network-filter-enabled"] = []string{strconv.FormatBool(v)}
	}
	if v, ok := properties["disableKeyBasedMetadataWriteAccess"].(bool); ok {
		attributes["azure.cosmosdb.disable-key-based-metadata-write-access"] = []string{strconv.FormatBool(v)}
	}
	if v, ok := properties["disableLocalAuth"].(bool); ok {
		attributes["azure.cosmosdb.disable-local-auth"] = []string{strconv.FormatBool(v)}
	}
	if v := stringFromMap(backupPolicy, "type"); v != "" {
		attributes["azure.cosmosdb.backup-policy.type"] = []string{v}
	}
	if v := stringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.cosmosdb.provisioning-state"] = []string{v}
	}

	if locs := locationNames(properties, "locations"); len(locs) > 0 {
		sort.Strings(locs)
		attributes["azure.cosmosdb.locations"] = locs
	}
	if locs := locationNames(properties, "writeLocations"); len(locs) > 0 {
		sort.Strings(locs)
		attributes["azure.cosmosdb.write-locations"] = locs
	}
	if locs := locationNames(properties, "readLocations"); len(locs) > 0 {
		sort.Strings(locs)
		attributes["azure.cosmosdb.read-locations"] = locs
	}

	if isZoneRedundant := anyZoneRedundant(properties); isZoneRedundant != nil {
		attributes["azure.cosmosdb.is-zone-redundant"] = []string{strconv.FormatBool(*isZoneRedundant)}
	}

	for k, v := range common.GetMapValue(items, "tags") {
		attributes[fmt.Sprintf("azure.cosmosdb.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDCosmosDbAccount,
		Label:      name,
		Attributes: attributes,
	}
}

func locationNames(properties map[string]any, key string) []string {
	v, ok := properties[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		loc, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := loc["locationName"].(string); ok && name != "" {
			out = append(out, name)
		}
	}
	return out
}

// anyZoneRedundant returns true iff at least one of the configured locations has isZoneRedundant=true.
func anyZoneRedundant(properties map[string]any) *bool {
	v, ok := properties["locations"]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	hasAny := false
	seen := false
	for _, e := range arr {
		loc, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if zr, ok := loc["isZoneRedundant"].(bool); ok {
			seen = true
			if zr {
				hasAny = true
				break
			}
		}
	}
	if !seen {
		return nil
	}
	return &hasAny
}

func stringFromMap(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
