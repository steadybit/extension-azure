/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extvm

import (
  "context"
  "fmt"
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

const discoveryBasePath = "/"+TargetIDVM + "/discovery"

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
		Label: discovery_kit_api.PluralLabel{One: "Robot", Other: "Robots"},

		// Category for the targets to appear in
		Category: extutil.Ptr("example"),

		// Specify attributes shown in table columns and to be used for sorting
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "robot.reportedBy"},
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
				Attribute: "robot.reportedBy",
				Label: discovery_kit_api.PluralLabel{
					One:   "Reported by",
					Other: "Reported by",
				},
			},
		},
	}
}

func getDiscoveredTargets2(w http.ResponseWriter, _ *http.Request, _ []byte) {

  //client := utils.GetVirtualMachinesClient()
  // list all virtual machines in the subscription
}
func getDiscoveredTargets(w http.ResponseWriter, _ *http.Request, _ []byte) {
	targets := make([]discovery_kit_api.Target, 0)
  //subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")

  ctx := context.Background()
  client := utils.GetClientByCredentials()
  // Create the query request, Run the query and get the results. Update the Tags and subscriptionId details below.
  results, err := client.Resources(ctx,
    armresourcegraph.QueryRequest{
      Query: to.Ptr("Resources | where type =~ 'Microsoft.Compute/virtualMachines' | project name, type, location, properties"),
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
    fmt.Printf("Resources found: " + strconv.FormatInt(*results.TotalRecords, 10) + "\n")
    fmt.Printf("Results: " + fmt.Sprint(results.Data) + "\n")
    if m, ok := results.Data.([]interface{}); ok {
      for _, r := range m {
        items := r.(map[string]interface{})
        attributes := make(map[string][]string)
        properties := items["properties"].(map[string]interface{})
        extended := properties["extended"].(map[string]interface{})
        instanceView := extended["instanceView"].(map[string]interface{})
        hardwareProfile := properties["hardwareProfile"].(map[string]interface{})
        powerState := instanceView["powerState"].(map[string]interface{})
        for k, v := range properties {
          fmt.Printf("k: %s, v: %+v\n", k, v)
        }
        attributes["azure.vmId"] = []string{properties["vmId"].(string)}
        attributes["azure.vmSize,"] = []string{hardwareProfile["vmSize"].(string)}
        attributes["azure.osName,"] = []string{instanceView["osName"].(string)}
        attributes["azure.computerName,"] = []string{instanceView["computerName"].(string)}
        attributes["azure.osVersion,"] = []string{instanceView["osVersion"].(string)}
        attributes["azure.powerState,"] = []string{powerState["code"].(string)}

        targets = append(targets, discovery_kit_api.Target{
          Id:         properties["vmId"].(string),
          TargetType: TargetIDVM,
          Label:      items["name"].(string),
          Attributes: attributes,
        })
      }
    }
    //for _, result := range results.Data {
    //  fmt.Printf("Result: " + fmt.Sprint(result) + "\n")
    // targets = append(targets, discovery_kit_api.Target{
    //   Id:         *result["name"].(string),
    //   TargetType: TargetIDVM,
    //   Label:      *result["name"].(string),
    //   Attributes: map[string][]string{"robot.reportedBy": {"extension-azure"}},
    // })
    //}
  }


	//for i, name := range config.Config.RobotNames {
	//	targets[i] = discovery_kit_api.Target{
	//		Id:         name,
	//		TargetType: TargetIDVM,
	//		Label:      name,
	//		Attributes: map[string][]string{"robot.reportedBy": {"extension-azure"}},
	//	}
	//}
	exthttp.WriteBody(w, discovery_kit_api.DiscoveredTargets{Targets: targets})
}
