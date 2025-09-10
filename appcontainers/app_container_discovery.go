package appcontainers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"strconv"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/appcontainers/armappcontainers"
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

type appContainerDiscovery struct {
}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*appContainerDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*appContainerDiscovery)(nil)
)

func NewAppContainerDiscovery() discovery_kit_sdk.TargetDiscovery {
	discovery := &appContainerDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 30*time.Second),
	)
}

func (a *appContainerDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDContainerApp,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Container App", Other: "Container Apps"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.location"},
				{Attribute: "azure.resource-group.name"},
				{Attribute: "container-app.provisioning-state"},
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

func (a *appContainerDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "container-app.resource.id",
			Label: discovery_kit_api.PluralLabel{
				One:   "Container App Resource ID",
				Other: "Container App Resource IDs",
			},
		},
		{
			Attribute: "container-app.provisioning-state",
			Label: discovery_kit_api.PluralLabel{
				One:   "Provisioning State",
				Other: "Provisioning States",
			},
		},
		{
			Attribute: "container-app.managed-environment.id",
			Label: discovery_kit_api.PluralLabel{
				One:   "Managed Environment ID",
				Other: "Managed Environment IDs",
			},
		},
		{
			Attribute: "container-app.ingress.fqdn",
			Label: discovery_kit_api.PluralLabel{
				One:   "Ingress FQDN",
				Other: "Ingress FQDNs",
			},
		},
	}
}

func (a *appContainerDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: TargetIDContainerApp,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("30s"),
		},
	}
}

func (a *appContainerDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllContainerApps(ctx, client)
}

// safeToString safely converts any value to string
func safeToString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func getAllContainerApps(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")

	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{to.Ptr(subscriptionId)}
	}

	cred, err := common.ConnectionAzure()

	if err != nil {
		return nil, fmt.Errorf("failed to get azure credentials")
	}

	appClient, err := armappcontainers.NewContainerAppsClient(subscriptionId, cred, nil)
	if err != nil {
		log.Error().Msgf("failed to create container apps client: %v", err)
		return nil, err
	}

	query := `resources
		| where type =~ 'Microsoft.App/containerApps'
		| project name, type, ['id'], resourceGroup, location, tags, properties, subscriptionId`

	results, err := client.Resources(ctx,
		armresourcegraph.QueryRequest{
			Query: to.Ptr(query),
			Options: &armresourcegraph.QueryRequestOptions{
				ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
			},
			Subscriptions: subscriptions,
		},
		nil)
	if err != nil {
		log.Error().Msgf("failed to get container apps: %v", err)
		return nil, err
	}

	targets := make([]discovery_kit_api.Target, 0)

	if m, ok := results.Data.([]interface{}); ok {
		for _, r := range m {
			items := r.(map[string]interface{})
			attributes := make(map[string][]string)

			// Add basic attributes
			attributes["azure.subscription.id"] = []string{items["subscriptionId"].(string)}
			attributes["azure.resource-group.name"] = []string{items["resourceGroup"].(string)}
			attributes["azure.location"] = []string{items["location"].(string)}
			attributes["container-app.resource.id"] = []string{items["id"].(string)}

			// Add tags as labels
			for k, v := range common.GetMapValue(items, "tags") {
				attributes[fmt.Sprintf("container-app.label.%s", strings.ToLower(k))] = []string{safeToString(v)}
			}

			// Add properties if available
			properties := common.GetMapValue(items, "properties")
			if provisioningState, ok := properties["provisioningState"]; ok {
				attributes["container-app.provisioning-state"] = []string{safeToString(provisioningState)}
			}

			// Add managed environment information
			if managedEnvironmentId, ok := properties["managedEnvironmentId"]; ok {
				attributes["container-app.managed-environment.id"] = []string{safeToString(managedEnvironmentId)}
			}

			// Add configuration information
			if configuration, ok := properties["configuration"].(map[string]interface{}); ok {
				// Add ingress information
				if ingress, ok := configuration["ingress"].(map[string]interface{}); ok {
					if fqdn, ok := ingress["fqdn"]; ok && fqdn != nil {
						attributes["container-app.ingress.fqdn"] = []string{safeToString(fqdn)}
					}
					if external, ok := ingress["external"]; ok {
						attributes["container-app.ingress.external"] = []string{safeToString(external)}
					}
					if targetPort, ok := ingress["targetPort"]; ok {
						attributes["container-app.ingress.target-port"] = []string{safeToString(targetPort)}
					}
				}

				// Add replica information
				if activeRevisionsMode, ok := configuration["activeRevisionsMode"]; ok {
					attributes["container-app.active-revisions-mode"] = []string{safeToString(activeRevisionsMode)}
				}
			}

			// Add template information
			if template, ok := properties["template"].(map[string]interface{}); ok {
				// Add scale information
				if scale, ok := template["scale"].(map[string]interface{}); ok {
					if minReplicas, ok := scale["minReplicas"]; ok {
						attributes["container-app.scale.min-replicas"] = []string{safeToString(minReplicas)}
					}
					if maxReplicas, ok := scale["maxReplicas"]; ok {
						attributes["container-app.scale.max-replicas"] = []string{safeToString(maxReplicas)}
					}
				}

				// Add container information
				if containers, ok := template["containers"].([]interface{}); ok && len(containers) > 0 {
					containerNames := make([]string, 0, len(containers))
					for _, container := range containers {
						if containerMap, ok := container.(map[string]interface{}); ok {
							if name, ok := containerMap["name"]; ok {
								containerNames = append(containerNames, safeToString(name))
							}
						}
					}
					if len(containerNames) > 0 {
						attributes["container-app.container.names"] = containerNames
					}
				}
			}

			// Add workload profile information
			if workloadProfileName, ok := properties["workloadProfileName"]; ok && workloadProfileName != nil {
				attributes["container-app.workload-profile.name"] = []string{safeToString(workloadProfileName)}
			}

			resp, err := appClient.Get(ctx, items["resourceGroup"].(string), items["name"].(string), nil)

			if err != nil {
				return nil, err
			}

			if !(resp.ContainerApp.Properties == nil || resp.ContainerApp.Properties.Template == nil ||
				resp.ContainerApp.Properties.Template.Containers == nil) {
				for _, container := range resp.ContainerApp.Properties.Template.Containers {
					if len(container.Env) == 0 {
						continue
					}

					for _, env := range container.Env {
						if env.Name != nil && env.Value != nil && *env.Name == "STEADYBIT_FAULT_INJECTION_ENDPOINT" {
							attributes["container-app.app-configuration.endpoint"] = []string{*env.Value}
						}
					}
				}
			}

			targets = append(targets, discovery_kit_api.Target{
				Id:         items["id"].(string),
				TargetType: TargetIDContainerApp,
				Label:      items["name"].(string),
				Attributes: attributes,
			})
		}
	}

	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesContainerApp), nil
}
