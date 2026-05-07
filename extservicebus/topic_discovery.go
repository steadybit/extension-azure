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

const TargetIDTopic = "com.steadybit.extension_azure.servicebus.topic"

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
		Label:    discovery_kit_api.PluralLabel{One: "Azure Service Bus topic", Other: "Azure Service Bus topics"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.servicebus.namespace.name"},
				{Attribute: "azure.servicebus.topic.status"},
				{Attribute: "azure.servicebus.topic.requires-duplicate-detection"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *topicDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.servicebus.topic.name", Label: discovery_kit_api.PluralLabel{One: "Service Bus topic name", Other: "Service Bus topic names"}},
		{Attribute: "azure.servicebus.namespace.name", Label: discovery_kit_api.PluralLabel{One: "Service Bus namespace name", Other: "Service Bus namespace names"}},
		{Attribute: "azure.servicebus.topic.status", Label: discovery_kit_api.PluralLabel{One: "Service Bus topic status", Other: "Service Bus topic statuses"}},
		{Attribute: "azure.servicebus.topic.requires-duplicate-detection", Label: discovery_kit_api.PluralLabel{One: "Service Bus topic requires deduplication", Other: "Service Bus topic requires deduplication"}},
		{Attribute: "azure.servicebus.topic.support-ordering", Label: discovery_kit_api.PluralLabel{One: "Service Bus topic supports ordering", Other: "Service Bus topic supports ordering"}},
		{Attribute: "azure.servicebus.topic.enable-partitioning", Label: discovery_kit_api.PluralLabel{One: "Service Bus topic partitioning", Other: "Service Bus topic partitioning"}},
		{Attribute: "azure.servicebus.topic.default-message-time-to-live", Label: discovery_kit_api.PluralLabel{One: "Service Bus topic default message TTL", Other: "Service Bus topic default message TTLs"}},
		{Attribute: "azure.servicebus.topic.subscription-count", Label: discovery_kit_api.PluralLabel{One: "Service Bus topic subscription count", Other: "Service Bus topic subscription counts"}},
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
		Query: extutil.Ptr("Resources | where type =~ 'Microsoft.ServiceBus/namespaces/topics' | project id, name, type, resourceGroup, location, properties, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get Service Bus topic results")
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
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesServiceBus), nil
}

func toTopicTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")

	id, _ := items["id"].(string)
	rawName, _ := items["name"].(string)
	namespace := ""
	topicName := rawName
	if i := strings.LastIndex(rawName, "/"); i >= 0 {
		namespace = rawName[:i]
		topicName = rawName[i+1:]
	}
	label := topicName
	if namespace != "" {
		label = fmt.Sprintf("%s/%s", namespace, topicName)
	}

	attributes := make(map[string][]string)
	attributes["azure.servicebus.topic.name"] = []string{topicName}
	if namespace != "" {
		attributes["azure.servicebus.namespace.name"] = []string{namespace}
	}
	attributes["azure.subscription.id"] = []string{stringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{stringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{stringFromMap(items, "location")}

	if v := stringFromMap(properties, "status"); v != "" {
		attributes["azure.servicebus.topic.status"] = []string{v}
	}
	if v, ok := properties["requiresDuplicateDetection"].(bool); ok {
		attributes["azure.servicebus.topic.requires-duplicate-detection"] = []string{strconv.FormatBool(v)}
	}
	if v, ok := properties["supportOrdering"].(bool); ok {
		attributes["azure.servicebus.topic.support-ordering"] = []string{strconv.FormatBool(v)}
	}
	if v, ok := properties["enablePartitioning"].(bool); ok {
		attributes["azure.servicebus.topic.enable-partitioning"] = []string{strconv.FormatBool(v)}
	}
	if v := stringFromMap(properties, "defaultMessageTimeToLive"); v != "" {
		attributes["azure.servicebus.topic.default-message-time-to-live"] = []string{v}
	}
	if v, ok := properties["subscriptionCount"].(float64); ok {
		attributes["azure.servicebus.topic.subscription-count"] = []string{strconv.Itoa(int(v))}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDTopic,
		Label:      label,
		Attributes: attributes,
	}
}
