/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package exteventgrid

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
	TargetIDTopic        = "com.steadybit.extension_azure.eventgrid.topic"
	TargetIDSubscription = "com.steadybit.extension_azure.eventgrid.subscription"
	targetIcon           = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNMTIgMmw0IDQtNCA0LTQtNCA0LTR6IiBmaWxsPSJjdXJyZW50Q29sb3IiLz48L3N2Zz4="
)

type topicDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*topicDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*topicDiscovery)(nil)
)

func NewTopicDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&topicDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *topicDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDTopic,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *topicDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDTopic,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure Event Grid topic", Other: "Azure Event Grid topics"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.eventgrid.topic.kind"},
				{Attribute: "azure.eventgrid.topic.public-network-access"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *topicDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.eventgrid.topic.name", Label: discovery_kit_api.PluralLabel{One: "Event Grid topic name", Other: "Event Grid topic names"}},
		{Attribute: "azure.eventgrid.topic.kind", Label: discovery_kit_api.PluralLabel{One: "Event Grid topic kind", Other: "Event Grid topic kinds"}},
		{Attribute: "azure.eventgrid.topic.input-schema", Label: discovery_kit_api.PluralLabel{One: "Event Grid topic input schema", Other: "Event Grid topic input schemas"}},
		{Attribute: "azure.eventgrid.topic.public-network-access", Label: discovery_kit_api.PluralLabel{One: "Event Grid topic public network access", Other: "Event Grid topic public network access"}},
		{Attribute: "azure.eventgrid.topic.local-auth-disabled", Label: discovery_kit_api.PluralLabel{One: "Event Grid topic local auth disabled", Other: "Event Grid topic local auth disabled"}},
		{Attribute: "azure.eventgrid.topic.endpoint", Label: discovery_kit_api.PluralLabel{One: "Event Grid topic endpoint", Other: "Event Grid topic endpoints"}},
		{Attribute: "azure.eventgrid.topic.provisioning-state", Label: discovery_kit_api.PluralLabel{One: "Event Grid topic provisioning state", Other: "Event Grid topic provisioning states"}},
	}
}

func (d *topicDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllTopics(ctx, client)
}

func getAllTopics(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	results, err := client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: extutil.Ptr("Resources | where type =~ 'Microsoft.EventGrid/topics' or type =~ 'Microsoft.EventGrid/systemTopics' or type =~ 'Microsoft.EventGrid/domains' | project id, name, type, kind, resourceGroup, location, tags, properties, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get Event Grid topic results")
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
		targets = append(targets, toTopicTarget(items))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesEventGrid), nil
}

func toTopicTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")

	id, _ := items["id"].(string)
	name, _ := items["name"].(string)
	rawType, _ := items["type"].(string)
	// Map ARM resource type to a simpler "kind" for the agent: "topic" / "system-topic" / "domain".
	kind := "topic"
	switch strings.ToLower(rawType) {
	case "microsoft.eventgrid/systemtopics":
		kind = "system-topic"
	case "microsoft.eventgrid/domains":
		kind = "domain"
	}

	attributes := make(map[string][]string)
	attributes["azure.eventgrid.topic.name"] = []string{name}
	attributes["azure.subscription.id"] = []string{stringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{stringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{stringFromMap(items, "location")}
	attributes["azure.eventgrid.topic.kind"] = []string{kind}

	if v := stringFromMap(properties, "inputSchema"); v != "" {
		attributes["azure.eventgrid.topic.input-schema"] = []string{v}
	}
	if v := stringFromMap(properties, "publicNetworkAccess"); v != "" {
		attributes["azure.eventgrid.topic.public-network-access"] = []string{v}
	}
	if v, ok := properties["disableLocalAuth"].(bool); ok {
		attributes["azure.eventgrid.topic.local-auth-disabled"] = []string{strconv.FormatBool(v)}
	}
	if v := stringFromMap(properties, "endpoint"); v != "" {
		attributes["azure.eventgrid.topic.endpoint"] = []string{v}
	}
	if v := stringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.eventgrid.topic.provisioning-state"] = []string{v}
	}

	for k, v := range common.GetMapValue(items, "tags") {
		attributes[fmt.Sprintf("azure.eventgrid.topic.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDTopic,
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
