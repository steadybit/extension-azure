package azurefunctions

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/appservice/armappservice"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-azure/common"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type BlockDirection string

const (
	BlockInbound  BlockDirection = "inbound"
	BlockOutbound BlockDirection = "outbound"
)

type blockAction struct {
	description    action_kit_api.ActionDescription
	configProvider func(request action_kit_api.PrepareActionRequestBody) (*BlockHostsConfig, error)
}

var _ action_kit_sdk.Action[BlockActionState] = (*blockAction)(nil)
var _ action_kit_sdk.ActionWithStop[BlockActionState] = (*blockAction)(nil)

type BlockActionState struct {
	ResourceId               string            `json:"resourceId"`
	Config                   *BlockHostsConfig `json:"config"`
	ResourceGroupName        string            `json:"resourceGroupName"`
	SubnetId                 string            `json:"subnetId"`
	NetworkSecurityGroupName string            `json:"networkSecurityGroupName"`
	NetworkSecurityRuleNames []string          `json:"networkSecurityRuleNames"`
}

type BlockHostsConfig struct {
	Rate           int                              `json:"rate"`
	BlockedIPs     *[]string                        `json:"denylist,omitempty"`
	BlockDirection armnetwork.SecurityRuleDirection `json:"direction"`
}

func NewBlockAction() action_kit_sdk.Action[BlockActionState] {
	return &blockAction{
		description:    getInjectBlockDescription(),
		configProvider: injectBlock,
	}
}

func getInjectBlockDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:              fmt.Sprintf("%s.block", TargetIDAzureFunction),
		Version:         extbuild.GetSemverVersionStringOrUnknown(),
		Label:           "Inject Block Hosts",
		Description:     "Block specific hosts from executing the function.",
		Icon:            extutil.Ptr(string(targetIcon)),
		TargetSelection: &azureFunctionTargetSelection,
		Technology:      extutil.Ptr("Azure"),
		Category:        extutil.Ptr("Azure Functions"),
		Kind:            action_kit_api.Attack,
		TimeControl:     action_kit_api.TimeControlExternal,
		Parameters: []action_kit_api.ActionParameter{
			{
				Label:        "Duration",
				Name:         "duration",
				Type:         action_kit_api.ActionParameterTypeDuration,
				Description:  extutil.Ptr("The duration of the attack."),
				Advanced:     extutil.Ptr(false),
				Required:     extutil.Ptr(true),
				DefaultValue: extutil.Ptr("30s"),
				Order:        extutil.Ptr(0),
			},
			{
				Name:         "rate",
				Label:        "Rate",
				Description:  extutil.Ptr("The rate of invocations to affect."),
				Type:         action_kit_api.ActionParameterTypePercentage,
				DefaultValue: extutil.Ptr("100"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
			},
			{
				Name:         "hosts",
				Label:        "Hosts To Block",
				Description:  extutil.Ptr("Hosts to block from executing the function."),
				Type:         action_kit_api.ActionParameterTypeStringArray,
				DefaultValue: extutil.Ptr(""),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(2),
			},
			{
				Name:         "direction",
				Label:        "Block direction",
				Description:  extutil.Ptr("Direction in which to block traffic"),
				Type:         action_kit_api.ActionParameterTypeString,
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(3),
				DefaultValue: extutil.Ptr(string(BlockOutbound)),
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "Block inbound traffic",
						Value: string(BlockInbound),
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Block outbound traffic",
						Value: string(BlockOutbound),
					},
				}),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func injectBlock(request action_kit_api.PrepareActionRequestBody) (*BlockHostsConfig, error) {
	hostsInterface := request.Config["hosts"].([]interface{})
	ipHosts := make([]string, 0)
	hosts := make([]string, len(hostsInterface))
	for i, h := range hostsInterface {
		hosts[i] = fmt.Sprintf("%v", h)
	}

	for _, host := range hosts {
		ip := net.ParseIP(host)
		if ip == nil {
			ipList, err := net.LookupIP(host)
			if err != nil {
				return nil, fmt.Errorf("the following entry is not a resolvable domain: %s", host)
			}

			for _, ipEntry := range ipList {
				ipHosts = append(ipHosts, ipEntry.String())
			}
		} else {
			ipHosts = append(ipHosts, host)
		}
	}

	blockDirectionRequest := extutil.ToString(request.Config["direction"])

	var blockDirection armnetwork.SecurityRuleDirection

	if blockDirectionRequest == string(BlockInbound) {
		blockDirection = armnetwork.SecurityRuleDirectionInbound
	} else if blockDirectionRequest == string(BlockOutbound) {
		blockDirection = armnetwork.SecurityRuleDirectionOutbound
	} else {
		return nil, fmt.Errorf("invalid direction %s is specified, please select one of the following: %s, %s", blockDirectionRequest, string(BlockInbound), string(BlockOutbound))
	}

	return &BlockHostsConfig{
		Rate:           int(request.Config["rate"].(float64)),
		BlockedIPs:     extutil.Ptr(ipHosts),
		BlockDirection: blockDirection,
	}, nil
}

// Describe implements action_kit_sdk.Action.
func (b *blockAction) Describe() action_kit_api.ActionDescription {
	return b.description
}

// NewEmptyState implements action_kit_sdk.Action.
func (b *blockAction) NewEmptyState() BlockActionState {
	return BlockActionState{}
}

// Prepare implements action_kit_sdk.Action.
func (b *blockAction) Prepare(ctx context.Context, state *BlockActionState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	resourceId := request.Target.Attributes["azure-function.resource.id"]

	if len(resourceId) == 0 {
		return nil, fmt.Errorf("target is missing 'azure-function.resource.id' attribute")
	}

	resource := resourceId[0]

	parts := strings.Split(strings.TrimPrefix(resource, "/"), "/")

	if len(parts) < 8 {
		return nil, fmt.Errorf("invalid resource id format")
	}

	resourceGroup := parts[3]
	functionName := parts[7]

	config, err := b.configProvider(request)

	if err != nil {
		return nil, err
	}

	cred, err := common.ConnectionAzure()

	if err != nil {
		return nil, fmt.Errorf("failed to create azure credential: %s", err)
	}

	subscriptionId, found := os.LookupEnv("AZURE_SUBSCRIPTION_ID")

	if !found {
		return nil, fmt.Errorf("'AZURE_SUBSCRIPTION_ID' environment variable is missing")
	}

	appServiceClient, err := armappservice.NewWebAppsClient(subscriptionId, cred, nil)

	if err != nil {
		return nil, fmt.Errorf("unable to create app service client: %s", err)
	}

	functionApp, err := appServiceClient.Get(ctx, resourceGroup, functionName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get function app: %s", err)
	}

	state.Config = config
	state.ResourceId = resource
	state.ResourceGroupName = resourceGroup
	state.SubnetId = *functionApp.Properties.VirtualNetworkSubnetID

	return nil, nil
}

func (b *blockAction) Start(ctx context.Context, state *BlockActionState) (*action_kit_api.StartResult, error) {
	cred, err := common.ConnectionAzure()

	if err != nil {
		return nil, fmt.Errorf("failed to create azure credential: %s", err)
	}

	subscriptionId, found := os.LookupEnv("AZURE_SUBSCRIPTION_ID")

	if !found {
		return nil, fmt.Errorf("'AZURE_SUBSCRIPTION_ID' environment variable is missing")
	}

	client, err := armnetwork.NewSecurityGroupsClient(subscriptionId, cred, nil)

	if err != nil {
		return nil, fmt.Errorf("unable to create security groups client: %s", err)
	}

	nsgPages := client.NewListPager(state.ResourceGroupName, nil)

	if !nsgPages.More() {
		return nil, fmt.Errorf("azure functions resource group does not have any network security groups")
	}

	page, err := nsgPages.NextPage(ctx)

	if err != nil {
		return nil, fmt.Errorf("unable to retrieve network security groups: %s", err)
	}

	securityGroups := page.Value

	var securityGroup *armnetwork.SecurityGroup = nil

	for _, sg := range securityGroups {
		for _, snet := range sg.Properties.Subnets {
			if *snet.ID == state.SubnetId {
				securityGroup = sg
				break
			}
		}
	}

	if securityGroup == nil {
		return nil, fmt.Errorf("no security group is attached to the function app subnet")
	}

	state.NetworkSecurityGroupName = *securityGroup.Name

	securityRulesClient, err := armnetwork.NewSecurityRulesClient(subscriptionId, cred, nil)

	if err != nil {
		return nil, fmt.Errorf("unable to retrieve security rules client: %s", err)
	}

	existingRules := securityGroup.Properties.SecurityRules
	usedPriorities := make(map[int32]bool)
	for _, rule := range existingRules {
		if rule.Properties != nil && rule.Properties.Priority != nil {
			usedPriorities[*rule.Properties.Priority] = true
		}
	}

	for i, ip := range *state.Config.BlockedIPs {
		log.Info().Msgf("Processing IP: %s", ip)
		var sourcePrefix, destinationPrefix *string
		if state.Config.BlockDirection == armnetwork.SecurityRuleDirectionInbound {
			sourcePrefix = to.Ptr(ip)
			destinationPrefix = to.Ptr("*")
		} else {
			sourcePrefix = to.Ptr("*")
			destinationPrefix = to.Ptr(ip)
		}

		priority := 100 + int32(i)
		for usedPriorities[priority] {
			priority++
		}
		usedPriorities[priority] = true

		sg, err := securityRulesClient.BeginCreateOrUpdate(ctx,
			state.ResourceGroupName,
			*securityGroup.Name,
			fmt.Sprintf("SteadybitBlockRule-%d", i),
			armnetwork.SecurityRule{
				Properties: &armnetwork.SecurityRulePropertiesFormat{
					Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
					SourcePortRange:          to.Ptr("*"),
					DestinationPortRange:     to.Ptr("*"),
					SourceAddressPrefix:      sourcePrefix,
					DestinationAddressPrefix: destinationPrefix,
					Access:                   to.Ptr(armnetwork.SecurityRuleAccessDeny),
					Direction:                to.Ptr(state.Config.BlockDirection),
					Priority:                 to.Ptr(priority),
					Description:              to.Ptr("Blocked by steadybit"),
				},
			}, nil)

		if err != nil {
			err = cleanupRules(ctx, state, securityRulesClient)

			if err != nil {
				return nil, fmt.Errorf("failed to create a security rule; additionally failed to clean up security rules: %s", err)
			}

			return nil, fmt.Errorf("failed to create a security rule: %s", err)
		}

		rule, err := sg.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{
			Frequency: time.Second * 5,
		})

		state.NetworkSecurityRuleNames = append(state.NetworkSecurityRuleNames, *rule.Name)

		if err != nil {
			err = cleanupRules(ctx, state, securityRulesClient)

			if err != nil {
				return nil, fmt.Errorf("failed to create a security rule; additionally failed to clean up security rules: %s", err)
			}
			return nil, fmt.Errorf("failed to create a security rule (timeout): %s", err)
		}
	}

	return nil, nil
}

func (b *blockAction) Stop(ctx context.Context, state *BlockActionState) (*action_kit_api.StopResult, error) {
	cred, err := common.ConnectionAzure()

	if err != nil {
		return nil, fmt.Errorf("failed to create azure credential: %s", err)
	}

	subscriptionId, found := os.LookupEnv("AZURE_SUBSCRIPTION_ID")

	if !found {
		return nil, fmt.Errorf("'AZURE_SUBSCRIPTION_ID' environment variable is missing")
	}

	client, err := armnetwork.NewSecurityRulesClient(subscriptionId, cred, nil)

	if err != nil {
		return nil, fmt.Errorf("unable to create security groups client: %s", err)
	}

	err = cleanupRules(ctx, state, client)

	if err != nil {
		return nil, fmt.Errorf("failed to clean up security rules: %s", err)
	}

	return nil, nil
}

func cleanupRules(ctx context.Context, state *BlockActionState, client *armnetwork.SecurityRulesClient) error {
	for _, ruleName := range state.NetworkSecurityRuleNames {
		poller, err := client.BeginDelete(ctx, state.ResourceGroupName, state.NetworkSecurityGroupName, ruleName, nil)

		if err != nil {
			return fmt.Errorf("unable to create a delete poller: %s", err)
		}

		_, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{
			Frequency: time.Second * 5,
		})

		if err != nil {
			return fmt.Errorf("failed to delete a security rule (timeout): %s", err)
		}
	}
	return nil
}
