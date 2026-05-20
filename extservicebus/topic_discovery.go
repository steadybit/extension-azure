/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extservicebus

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/servicebus/armservicebus"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-azure/common"
	"github.com/steadybit/extension-azure/config"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
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
	rgClient, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get Resource Graph client: %w", err)
	}
	return getAllTopics(ctx, rgClient, common.GetServiceBusTopicsClient)
}

// getAllTopics lists Service Bus topics via direct ARM. See getAllQueues' comment for the rationale
// on direct ARM vs. Resource Graph for Service Bus child resources.
func getAllTopics(
	ctx context.Context,
	rgClient common.ArmResourceGraphApi,
	topicsClientProvider func(subscriptionId string) (*armservicebus.TopicsClient, error),
) ([]discovery_kit_api.Target, error) {
	namespaces, err := listServiceBusNamespaceRefs(ctx, rgClient)
	if err != nil {
		return nil, err
	}

	targets := make([]discovery_kit_api.Target, 0)
	for _, ns := range namespaces {
		client, err := topicsClientProvider(ns.subscriptionId)
		if err != nil {
			log.Warn().Err(err).Msgf("failed to create Service Bus topics client for subscription %s; skipping", ns.subscriptionId)
			continue
		}
		pager := client.NewListByNamespacePager(ns.resourceGroup, ns.name, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				log.Warn().Err(err).Msgf("failed to list topics for namespace %s/%s; skipping rest of namespace", ns.subscriptionId, ns.name)
				break
			}
			for _, t := range page.Value {
				if t == nil {
					continue
				}
				targets = append(targets, topicToTarget(t, ns))
			}
		}
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesServiceBus), nil
}

func topicToTarget(t *armservicebus.SBTopic, ns serviceBusNamespaceRef) discovery_kit_api.Target {
	topicName := ""
	if t.Name != nil {
		topicName = *t.Name
	}
	id := ""
	if t.ID != nil {
		id = *t.ID
	}

	attributes := map[string][]string{
		"azure.servicebus.topic.name":     {topicName},
		"azure.servicebus.namespace.name": {ns.name},
		"azure.subscription.id":           {ns.subscriptionId},
		"azure.resource-group.name":       {ns.resourceGroup},
		"azure.location":                  {ns.location},
	}

	if p := t.Properties; p != nil {
		if p.Status != nil {
			attributes["azure.servicebus.topic.status"] = []string{string(*p.Status)}
		}
		if p.RequiresDuplicateDetection != nil {
			attributes["azure.servicebus.topic.requires-duplicate-detection"] = []string{strconv.FormatBool(*p.RequiresDuplicateDetection)}
		}
		if p.SupportOrdering != nil {
			attributes["azure.servicebus.topic.support-ordering"] = []string{strconv.FormatBool(*p.SupportOrdering)}
		}
		if p.EnablePartitioning != nil {
			attributes["azure.servicebus.topic.enable-partitioning"] = []string{strconv.FormatBool(*p.EnablePartitioning)}
		}
		if p.DefaultMessageTimeToLive != nil {
			attributes["azure.servicebus.topic.default-message-time-to-live"] = []string{*p.DefaultMessageTimeToLive}
		}
		if p.SubscriptionCount != nil {
			attributes["azure.servicebus.topic.subscription-count"] = []string{strconv.Itoa(int(*p.SubscriptionCount))}
		}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDTopic,
		Label:      fmt.Sprintf("%s/%s", ns.name, topicName),
		Attributes: attributes,
	}
}
