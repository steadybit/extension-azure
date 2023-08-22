/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extvm

import (
  "context"
  "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
  "github.com/rs/zerolog/log"
  "github.com/steadybit/discovery-kit/go/discovery_kit_api"
  "github.com/steadybit/extension-azure/utils"
  "github.com/steadybit/extension-kit/extbuild"
  "github.com/steadybit/extension-kit/exthttp"
  "github.com/steadybit/extension-kit/extutil"
  "net/http"
  "strconv"
)

const discoveryBasePath = "/" + TargetIDVM + "/discovery"

func RegisterDiscoveryHandlers() {
  exthttp.RegisterHttpHandler(discoveryBasePath, exthttp.GetterAsHandler(getDiscoveryDescription))
  exthttp.RegisterHttpHandler(discoveryBasePath+"/target-description", exthttp.GetterAsHandler(getTargetDescription))
  exthttp.RegisterHttpHandler(discoveryBasePath+"/attribute-descriptions", exthttp.GetterAsHandler(getAttributeDescriptions))
  exthttp.RegisterHttpHandler(discoveryBasePath+"/discovered-targets", getDiscoveredTargets)
}

var (
  virtualMachinesClient *armcompute.VirtualMachinesClient
)

func GetDiscoveryList() discovery_kit_api.DiscoveryList {
  return discovery_kit_api.DiscoveryList{
    Discoveries: []discovery_kit_api.DescribingEndpointReference{
      {
        Method: "GET",
        Path:   discoveryBasePath,
      },
    },
    TargetTypes: []discovery_kit_api.DescribingEndpointReference{
      {
        Method: "GET",
        Path:   discoveryBasePath + "/target-description",
      },
    },
    TargetAttributes: []discovery_kit_api.DescribingEndpointReference{
      {
        Method: "GET",
        Path:   discoveryBasePath + "/attribute-descriptions",
      },
    },
  }
}

func getDiscoveryDescription() discovery_kit_api.DiscoveryDescription {
  return discovery_kit_api.DiscoveryDescription{
    Id:         TargetIDVM,
    RestrictTo: extutil.Ptr(discovery_kit_api.LEADER),
    Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
      Method:       "GET",
      Path:         discoveryBasePath + "/discovered-targets",
      CallInterval: extutil.Ptr("1m"),
    },
  }
}

func getTargetDescription() discovery_kit_api.TargetDescription {
  return discovery_kit_api.TargetDescription{
    Id:      TargetIDVM,
    Version: extbuild.GetSemverVersionStringOrUnknown(),
    Icon:    extutil.Ptr(targetIcon),

    // Labels used in the UI
    Label: discovery_kit_api.PluralLabel{One: "Azure Virtual Machine", Other: "Azure Virtual Machines"},

    // Category for the targets to appear in
    Category: extutil.Ptr("cloud"),

    // Specify attributes shown in table columns and to be used for sorting
    Table: discovery_kit_api.Table{
      Columns: []discovery_kit_api.Column{
        {Attribute: "steadybit.label"},
        {Attribute: "azure-vm.location"},
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

func getAttributeDescriptions() discovery_kit_api.AttributeDescriptions {
  return discovery_kit_api.AttributeDescriptions{
    Attributes: []discovery_kit_api.AttributeDescription{
      {
        Attribute: "azure-vm.computer.name",
        Label: discovery_kit_api.PluralLabel{
          One:   "Computer name",
          Other: "Computer names",
        },
      },
      {
        Attribute: "azure-vm.location",
        Label: discovery_kit_api.PluralLabel{
          One:   "Location",
          Other: "Locations",
        },
      },
      {
        Attribute: "azure-vm.network.id",
        Label: discovery_kit_api.PluralLabel{
          One:   "Network ID",
          Other: "Network IDs",
        },
      },
      {
        Attribute: "azure-vm.os.name",
        Label: discovery_kit_api.PluralLabel{
          One:   "OS name",
          Other: "OS names",
        },
      },
      {
        Attribute: "azure-vm.os.type",
        Label: discovery_kit_api.PluralLabel{
          One:   "OS type",
          Other: "OS types",
        },
      },
      {
        Attribute: "azure-vm.os.version",
        Label: discovery_kit_api.PluralLabel{
          One:   "OS version",
          Other: "OS versions",
        },
      },
      {
        Attribute: "azure-vm.power.state",
        Label: discovery_kit_api.PluralLabel{
          One:   "Power state",
          Other: "Power states",
        },
      },
      {
        Attribute: "azure-vm.tags",
        Label: discovery_kit_api.PluralLabel{
          One:   "Tags",
          Other: "Tags",
        },
      },
      {
        Attribute: "azure-vm.vm.id",
        Label: discovery_kit_api.PluralLabel{
          One:   "VM ID",
          Other: "VM IDs",
        },
      },
      {
        Attribute: "azure-vm.vm.size",
        Label: discovery_kit_api.PluralLabel{
          One:   "VM size",
          Other: "VM sizes",
        },
      },
      {
        Attribute: "azure-vm.vm.name",
        Label: discovery_kit_api.PluralLabel{
          One:   "VM name",
          Other: "VM names",
        },
      },
      {
        Attribute: "azure-vm.subscription.id",
        Label: discovery_kit_api.PluralLabel{
          One:   "Subscription ID",
          Other: "Subscription IDs",
        },
      },
      {
        Attribute: "azure-vm.resource-group.name",
        Label: discovery_kit_api.PluralLabel{
          One:   "Resource group name",
          Other: "Resource group names",
        },
      },
    },
  }
}

func getDiscoveredTargets(w http.ResponseWriter, _ *http.Request, _ []byte) {
  targets := make([]discovery_kit_api.Target, 0)
  //subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")

  ctx := context.Background()
  client, err := utils.GetClientByCredentials()
  if err != nil {
    log.Error().Msgf("failed to get client: %v", err)
    return
  }
  results, err := client.Resources(ctx,
    armresourcegraph.QueryRequest{
      Query: to.Ptr("Resources | where type =~ 'Microsoft.Compute/virtualMachines' | project name, type, resourceGroup, location, tags, properties, subscriptionId | limit 10"),
      Options: &armresourcegraph.QueryRequestOptions{
        ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
      },
      //Subscriptions: []*string{
      //  to.Ptr(subscriptionId)},
    },
    nil)
  if err != nil {
    log.Error().Msgf("failed to get results: %v", err)
  } else {
    // Print the obtained query results
    log.Debug().Msgf("Resources found: " + strconv.FormatInt(*results.TotalRecords, 10))
    if m, ok := results.Data.([]interface{}); ok {
      for _, r := range m {
        items := r.(map[string]interface{})
        attributes := make(map[string][]string)

        properties := getMapValue(items, "properties")
        extended := getMapValue(properties, "extended")
        networkProfile := getMapValue(properties, "networkProfile")
        networkInterfaces := getMapValue(networkProfile, "networkInterfaces")
        instanceView := getMapValue(extended, "instanceView")
        hardwareProfile := getMapValue(properties, "hardwareProfile")
        powerState := getMapValue(instanceView, "powerState")
        storageProfile := getMapValue(properties, "storageProfile")
        osDisk := getMapValue(storageProfile, "osDisk")

        attributes["azure-vm.vm.name"] = []string{items["name"].(string)}
        attributes["azure-vm.subscription.id"] = []string{items["subscriptionId"].(string)}
        attributes["azure-vm.vm.id"] = []string{getPropertyValue(properties, "vmId")}
        attributes["azure-vm.vm.size"] = []string{getPropertyValue(hardwareProfile, "vmSize")}
        attributes["azure-vm.os.name"] = []string{getPropertyValue(instanceView, "osName")}
        attributes["azure-vm.computer.name"] = []string{getPropertyValue(instanceView, "computerName")}
        attributes["azure-vm.os.version"] = []string{getPropertyValue(instanceView, "osVersion")}
        attributes["azure-vm.os.type"] = []string{getPropertyValue(osDisk, "osType")}
        attributes["azure-vm.power.state"] = []string{getPropertyValue(powerState, "code")}
        attributes["azure-vm.network.id"] = []string{getPropertyValue(networkInterfaces, "id")}
        attributes["azure-vm.location"] = []string{getPropertyValue(items, "location")}
        attributes["azure-vm.resource-group.name"] = []string{getPropertyValue(items, "resourceGroup")}
        attributes["azure-vm.tags"] = parseTags(getMapValue(items, "tags"))

        targets = append(targets, discovery_kit_api.Target{
          Id:         properties["vmId"].(string),
          TargetType: TargetIDVM,
          Label:      items["name"].(string),
          Attributes: attributes,
        })
      }
    }
  }

  exthttp.WriteBody(w, discovery_kit_api.DiscoveredTargets{Targets: targets})
}

func parseTags(value map[string]interface{}) []string {
  tags := make([]string, 0)
  for k, v := range value {
    tags = append(tags, k+":"+v.(string))
  }
  return tags
}

func getPropertyValue(properties map[string]interface{}, key string) string {
  if value, ok := properties[key]; ok {
    return value.(string)
  }
  return ""
}

func getMapValue(properties map[string]interface{}, key string) map[string]interface{} {
  if value, ok := properties[key]; ok {
    if m, ok := value.(map[string]interface{}); ok {
      return m
    } else if n, ok := value.([]interface{}); ok {
      if len(n) > 0 {
        if o, ok := n[0].(map[string]interface{}); ok {
          return o
        }
      }
    }
  }
  return make(map[string]interface{})
}
