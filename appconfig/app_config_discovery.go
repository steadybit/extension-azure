package appconfig

import (
	"context"
	"fmt"
	"os"
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
)

type appConfigurationDiscovery struct {
}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*appConfigurationDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*appConfigurationDiscovery)(nil)
)

func NewAppConfigurationDiscovery() discovery_kit_sdk.TargetDiscovery {
	discovery := &appConfigurationDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 30*time.Second),
	)
}

func (a *appConfigurationDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDAzureAppConfiguration,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure App Configuration", Other: "Azure App Configurations"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "app-configuration.provisioning-state"},
				{Attribute: "azure.location"},
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

func (a *appConfigurationDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{}

}

func (a *appConfigurationDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: TargetIDAzureAppConfiguration,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("30s"),
		},
	}
}

func (a *appConfigurationDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllAppConfigurations(ctx, client)
}

func getAllAppConfigurations(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{to.Ptr(subscriptionId)}
	}
	results, err := client.Resources(ctx,
		armresourcegraph.QueryRequest{
			Query: to.Ptr("resources | where type =~ 'microsoft.appconfiguration/configurationstores' | project name, type, ['id'], resourceGroup, location, tags, properties, subscriptionId"),
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
		targets := make([]discovery_kit_api.Target, 0)
		if m, ok := results.Data.([]interface{}); ok {
			for _, r := range m {
				items := r.(map[string]interface{})
				attributes := make(map[string][]string)

				// Add basic attributes
				attributes["azure.subscription.id"] = []string{items["subscriptionId"].(string)}
				attributes["azure.resource-group.name"] = []string{items["resourceGroup"].(string)}
				attributes["azure.location"] = []string{items["location"].(string)}
				attributes["app-configuration.resource.id"] = []string{items["id"].(string)}

				// Add tags as labels
				for k, v := range common.GetMapValue(items, "tags") {
					attributes[fmt.Sprintf("app-configuration.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
				}

				// Add properties if available
				properties := common.GetMapValue(items, "properties")
				if provisioningState, ok := properties["provisioningState"]; ok {
					attributes["app-configuration.provisioning-state"] = []string{extutil.ToString(provisioningState)}
				}
				if endpoint, ok := properties["endpoint"]; ok {
					attributes["app-configuration.endpoint"] = []string{extutil.ToString(endpoint)}
				}
				if sku, ok := properties["sku"]; ok {
					if skuMap, ok := sku.(map[string]interface{}); ok {
						if name, ok := skuMap["name"]; ok {
							attributes["app-configuration.sku.name"] = []string{extutil.ToString(name)}
						}
					}
				}

				targets = append(targets, discovery_kit_api.Target{
					Id:         items["id"].(string),
					TargetType: TargetIDAzureAppConfiguration,
					Label:      items["name"].(string),
					Attributes: attributes,
				})
			}
		}
		return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesAppConfiguration), nil
	}
}
