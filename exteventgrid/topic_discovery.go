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
	targetIcon           = "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTQuNjcyNzMgMTUuODA0QzQuNzU0MDUgMTUuODA0IDQuODMyMDIgMTUuODM2MyA0Ljg4OTUzIDE1Ljg5MzhDNC45NDcwMiAxNS45NTEzIDQuOTc5MzQgMTYuMDI5MiA0Ljk3OTM3IDE2LjExMDZWMTcuOTk1NEgxOS4wMzAyVjE2LjA3OTNDMTkuMDMyMyAxNi4wNTg1IDE5LjAzNjUgMTYuMDM3OSAxOS4wNDI4IDE2LjAxNzhDMTkuMDU1NiAxNS45Nzc2IDE5LjA3NjYgMTUuOTQwMiAxOS4xMDQ0IDE1LjkwODRDMTkuMTMyMiAxNS44NzY2IDE5LjE2NjcgMTUuODUxMyAxOS4yMDUgMTUuODMzM0MxOS4yNDMxIDE1LjgxNTMgMTkuMjg0OCAxNS44MDUyIDE5LjMyNyAxNS44MDRIMjAuNjkyM0MyMC43NzM3IDE1LjgwNCAyMC44NTI0IDE1LjgzNjIgMjAuOTEgMTUuODkzOEMyMC45Njc0IDE1Ljk1MTMgMjAuOTk5OSAxNi4wMjk0IDIwLjk5OTkgMTYuMTEwNlYxOS4zMTg2QzIwLjk5OTkgMTkuNDc4NSAyMC45MzYxIDE5LjYzMjIgMjAuODIzMSAxOS43NDU0QzIwLjcxIDE5Ljg1ODQgMjAuNTU2MyAxOS45MjIgMjAuMzk2NCAxOS45MjIxSDMuNjAzMzlMMy40ODQyNSAxOS45MTA0QzMuNDQ1NjkgMTkuOTAyNyAzLjQwODEyIDE5Ljg5MTIgMy4zNzE5NSAxOS44NzYyQzMuMjk4ODEgMTkuODQ1OSAzLjIzMjYzIDE5LjgwMTMgMy4xNzY2NCAxOS43NDU0QzMuMDYzNDUgMTkuNjMyMiAyLjk5OTg4IDE5LjQ3ODcgMi45OTk4OCAxOS4zMTg2VjE2LjA3ODRDMy4wMDIxNSAxNi4wNTcgMy4wMDU4NCAxNi4wMzU1IDMuMDEyNTcgMTYuMDE0OUMzLjAyNTk3IDE1Ljk3NCAzLjA0ODE3IDE1LjkzNjQgMy4wNzcwMyAxNS45MDQ1QzMuMTA2IDE1Ljg3MjYgMy4xNDIwMyAxNS44NDY3IDMuMTgxNTIgMTUuODI5M0MzLjIyMDkgMTUuODEyMSAzLjI2MzUyIDE1LjgwMzcgMy4zMDY1MiAxNS44MDRINC42NzI3M1oiIGZpbGw9ImN1cnJlbnRDb2xvciIvPgo8cGF0aCBkPSJNMTMuOTY4NiA3LjM0Mzk5QzE0LjU4MjYgNy4zNDM5OSAxNS4wODA5IDcuODQxMzUgMTUuMDgwOSA4LjQ1NTMyQzE1LjA4MDkgOS4wNjkzMyAxNC41ODI2IDkuNTY3NjMgMTMuOTY4NiA5LjU2NzYzQzEzLjQ0OTggOS41Njc1IDEzLjAxNSA5LjIxMTM2IDEyLjg5MjUgOC43MzA3MUgxMS4wNjczTDcuMjM1MjMgMTIuNjA1N0g4Ljg4NjZMMTEuMTMxNyAxMC4zNzIzSDE1LjEzNzZDMTUuMjc0MSA5LjkxMjkxIDE1LjY5OTMgOS41NzczOSAxNi4yMDMgOS41NzczOUMxNi44MTcgOS41NzczOSAxNy4zMTQzIDEwLjA3NTcgMTcuMzE0MyAxMC42ODk3QzE3LjMxNDIgMTEuMzAzNiAxNi44MTcgMTEuODAxIDE2LjIwMyAxMS44MDFDMTUuNjk5NSAxMS44MDEgMTUuMjc1MyAxMS40NjYyIDE1LjEzODUgMTEuMDA3MUgxMS4zODU2TDkuNzk2NzUgMTIuNjA1N0gxMS45NzI1QzEyLjEwOTQgMTIuMTQ2OCAxMi41MzM2IDExLjgxMTggMTMuMDM3IDExLjgxMThDMTMuNjUxIDExLjgxMTggMTQuMTQ5MyAxMi4zMTAxIDE0LjE0OTMgMTIuOTI0MUMxNC4xNDkgMTMuNTM3OSAxMy42NTA4IDE0LjAzNTQgMTMuMDM3IDE0LjAzNTRDMTIuNTMzNiAxNC4wMzUzIDEyLjEwOTQgMTMuNzAwNCAxMS45NzI1IDEzLjI0MTVIOC41MTU1TDEwLjExNTEgMTQuODQwMUgxNC4yMTY3QzE0LjM1MzQgMTQuMzgxIDE0Ljc3ODUgMTQuMDQ2MSAxNS4yODIxIDE0LjA0NjFDMTUuODk1OSAxNC4wNDYzIDE2LjM5MzMgMTQuNTQzNyAxNi4zOTM0IDE1LjE1NzVDMTYuMzkzNCAxNS43NzE0IDE1Ljg5NiAxNi4yNjk2IDE1LjI4MjEgMTYuMjY5OEMxNC43Nzg3IDE2LjI2OTggMTQuMzUzNSAxNS45MzQ4IDE0LjIxNjcgMTUuNDc1OEg5Ljg4MTcxTDcuNjE2MDkgMTMuMjQxNUg1LjY5OTFWMTIuNjA1N0g2LjMzNDg0TDEwLjgwMjYgOC4xMjcySDEyLjkwNjFDMTMuMDQ2MSA3LjY3MzQ1IDEzLjQ2OSA3LjM0NDExIDEzLjk2ODYgNy4zNDM5OVoiIGZpbGw9ImN1cnJlbnRDb2xvciIvPgo8cGF0aCBkPSJNMjAuNDYwOCAzLjg5NzcxQzIwLjQ4MDUgMy44OTk4NCAyMC41MDAyIDMuOTAwNTIgMjAuNTE5NCAzLjkwNDU0QzIwLjU0MjIgMy45MDkzIDIwLjU2MzggMy45MTc3MyAyMC41ODU4IDMuOTI1MDVDMjAuNjAwMSAzLjkyOTggMjAuNjE0OSAzLjkzMjkyIDIwLjYyODggMy45Mzg3MkMyMC42NTMgMy45NDg4NiAyMC42NzU0IDMuOTYyNiAyMC42OTgxIDMuOTc1ODNDMjAuNzA5NSAzLjk4MjQzIDIwLjcyMTQgMy45ODgwMSAyMC43MzIzIDMuOTk1MzZDMjAuNzQ0OCA0LjAwMzc3IDIwLjc1NTYgNC4wMTQzMyAyMC43Njc1IDQuMDIzNjhDMjAuNzg2MSA0LjAzODMzIDIwLjgwNjIgNC4wNTE2OSAyMC44MjMxIDQuMDY4NkMyMC45MzYyIDQuMTgxNzYgMjAuOTk5OSA0LjMzNTQxIDIwLjk5OTkgNC40OTUzNlY3LjY3MjEyQzIwLjk5OTggNy43NTM0NyAyMC45NjY2IDcuODMxMzkgMjAuOTA5MSA3Ljg4ODkyQzIwLjg1MTUgNy45NDYzOSAyMC43NzM2IDcuOTc4NzYgMjAuNjkyMyA3Ljk3ODc2SDE5LjMyNkMxOS4yNDY2IDcuOTc1OSAxOS4xNzEzIDcuOTQyMTkgMTkuMTE2MSA3Ljg4NTAxQzE5LjA2MSA3LjgyNzggMTkuMDMwMiA3Ljc1MTU2IDE5LjAzMDIgNy42NzIxMlY1LjgxODZINC45Njg2M1Y3LjY3MjEyQzQuOTY4NTYgNy43NTM0NiA0LjkzNjMgNy44MzE0IDQuODc4NzggNy44ODg5MkM0LjgyMTI0IDcuOTQ2NDMgNC43NDMzNCA3Ljk3ODcyIDQuNjYxOTkgNy45Nzg3NkgzLjMwNjUyQzMuMjI1MTUgNy45Nzg3NSAzLjE0NzI5IDcuOTQ2NDIgMy4wODk3MiA3Ljg4ODkyQzMuMDMyMiA3LjgzMTQgMi45OTk5NSA3Ljc1MzQ2IDIuOTk5ODggNy42NzIxMlY0LjQ5NTM2QzIuOTk5ODggNC4zMzUzIDMuMDYzNDYgNC4xODE3OCAzLjE3NjY0IDQuMDY4NkMzLjI4OTggMy45NTU1NCAzLjQ0MzQyIDMuODkxODUgMy42MDMzOSAzLjg5MTg1SDIwLjQwNjFDMjAuNDI0NCAzLjg5MjE3IDIwLjQ0MjggMy44OTU3NSAyMC40NjA4IDMuODk3NzFaIiBmaWxsPSJjdXJyZW50Q29sb3IiLz4KPC9zdmc+Cg=="
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
