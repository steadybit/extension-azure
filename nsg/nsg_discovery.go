package nsg

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-azure/common"
	"github.com/steadybit/extension-azure/config"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type nsgDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*nsgDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*nsgDiscovery)(nil)
)

func NewNsgDiscovery() discovery_kit_sdk.TargetDiscovery {
	discovery := &nsgDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 30*time.Second),
	)
}

func (a *nsgDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDNetworkSG,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Network Security Group", Other: "Network Security Groups"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.location"},
				{Attribute: "network-security-group.state"},
				{Attribute: "azure.resource-group.name"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: "steadybit.label",
					Direction: "ASC",
				},
			},
		},
	}
}

func (a *nsgDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{}
}

func (a *nsgDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: TargetIDNetworkSG,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("30s"),
		},
	}
}

func (a *nsgDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllNSGs(ctx, client)
}

func getAllNSGs(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{to.Ptr(subscriptionId)}
	}
	results, err := client.Resources(ctx,
		armresourcegraph.QueryRequest{
			Query: to.Ptr("resources | where type =~ 'microsoft.network/networksecuritygroups' | project name, type, id, resourceGroup, location, tags, properties, subscriptionId"),
			Options: &armresourcegraph.QueryRequestOptions{
				ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
			},
			Subscriptions: subscriptions,
		},
		nil)
	if err != nil {
		log.Error().Msgf("failed to get results: %v", err)
		return nil, err
	} else {
		log.Debug().Msgf("Network Security Groups found: %s", strconv.FormatInt(*results.TotalRecords, 10))

		targets := make([]discovery_kit_api.Target, 0)
		if m, ok := results.Data.([]interface{}); ok {
			for _, r := range m {
				items := r.(map[string]interface{})
				attributes := make(map[string][]string)

				// Add basic attributes
				attributes["azure.subscription.id"] = []string{items["subscriptionId"].(string)}
				attributes["azure.resource-group.name"] = []string{items["resourceGroup"].(string)}
				attributes["azure.location"] = []string{items["location"].(string)}
				attributes["network-security-group.id"] = []string{items["id"].(string)}

				// Add tags as labels
				for k, v := range common.GetMapValue(items, "tags") {
					attributes[fmt.Sprintf("network-security-group.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
				}

				properties := common.GetMapValue(items, "properties")
				if state, ok := properties["provisioningState"]; ok {
					attributes["network-security-group.state"] = []string{extutil.ToString(state)}
				}

				targets = append(targets, discovery_kit_api.Target{
					Id:         items["id"].(string),
					TargetType: TargetIDNetworkSG,
					Label:      items["name"].(string),
					Attributes: attributes,
				})
			}
		}
		return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesNetworkSecurityGroup), nil
	}
}
