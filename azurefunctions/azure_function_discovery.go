package azurefunctions

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

type azureFunctionDiscovery struct {
}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*azureFunctionDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*azureFunctionDiscovery)(nil)
)

func NewAzureFunctionDiscovery() discovery_kit_sdk.TargetDiscovery {
	discovery := &azureFunctionDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 30*time.Second),
	)
}

func (a *azureFunctionDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDAzureFunction,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure Function", Other: "Azure Functions"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure-function.state"},
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

func (a *azureFunctionDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "azure-function.state",
			Label: discovery_kit_api.PluralLabel{
				One:   "Function State",
				Other: "Function States",
			},
		},
		{
			Attribute: "azure-function.default-hostname",
			Label: discovery_kit_api.PluralLabel{
				One:   "Default Hostname",
				Other: "Default Hostnames",
			},
		},
		{
			Attribute: "azure-function.resource.id",
			Label: discovery_kit_api.PluralLabel{
				One:   "Resource ID",
				Other: "Resource IDs",
			},
		},
	}
}

func (a *azureFunctionDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: TargetIDAzureFunction,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("30s"),
		},
	}
}

func (a *azureFunctionDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllAzureFunctions(ctx, client)
}

func getAllAzureFunctions(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{to.Ptr(subscriptionId)}
	}
	results, err := client.Resources(ctx,
		armresourcegraph.QueryRequest{
			Query: to.Ptr("resources | where type =~ 'microsoft.web/sites' and kind has 'functionapp' | project name, type, ['id'], resourceGroup, location, tags, properties, subscriptionId"),
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
				attributes["azure-function.resource.id"] = []string{items["id"].(string)}

				// Add tags as labels
				for k, v := range common.GetMapValue(items, "tags") {
					attributes[fmt.Sprintf("azure-function.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
				}

				// Add properties if available
				properties := common.GetMapValue(items, "properties")
				if state, ok := properties["state"]; ok {
					attributes["azure-function.state"] = []string{extutil.ToString(state)}
				}
				if hostNames, ok := properties["defaultHostName"]; ok {
					attributes["azure-function.default-hostname"] = []string{extutil.ToString(hostNames)}
				}

				targets = append(targets, discovery_kit_api.Target{
					Id:         items["id"].(string),
					TargetType: TargetIDAzureFunction,
					Label:      items["name"].(string),
					Attributes: attributes,
				})
			}
		}
		return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesAzureFunction), nil
	}
}
