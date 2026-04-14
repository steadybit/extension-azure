package nsg

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
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
	NetworkSecurityGroupName string            `json:"networkSecurityGroupName"`
	NetworkSecurityRuleNames []string          `json:"networkSecurityRuleNames"`
}

type BlockHostsConfig struct {
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
		Id:              fmt.Sprintf("%s.block", TargetIDNetworkSG),
		Version:         extbuild.GetSemverVersionStringOrUnknown(),
		Label:           "Block Hosts",
		Description:     "Block specific inbound or outbound traffic for specific hosts.",
		Icon:            new(string(targetIcon)),
		TargetSelection: &networkSecurityGroupTargetSelection,
		Technology:      new("Azure"),
		Category:        new("Network Security Groups"),
		Kind:            action_kit_api.Attack,
		TimeControl:     action_kit_api.TimeControlExternal,
		Parameters: []action_kit_api.ActionParameter{
			{
				Label:        "Duration",
				Name:         "duration",
				Type:         action_kit_api.ActionParameterTypeDuration,
				Description:  new("The duration of the attack."),
				Advanced:     new(false),
				Required:     new(true),
				DefaultValue: new("30s"),
				Order:        new(0),
			},
			{
				Name:         "hosts",
				Label:        "Hosts To Block",
				Description:  new("Hosts to block from executing the function."),
				Type:         action_kit_api.ActionParameterTypeStringArray,
				DefaultValue: new(""),
				Required:     new(true),
				Order:        new(2),
			},
			{
				Name:         "direction",
				Label:        "Block direction",
				Description:  new("Direction in which to block traffic"),
				Type:         action_kit_api.ActionParameterTypeString,
				Required:     new(true),
				Order:        new(3),
				DefaultValue: new(string(BlockOutbound)),
				Options: new([]action_kit_api.ParameterOption{
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
		Stop: new(action_kit_api.MutatingEndpointReference{}),
	}
}

func injectBlock(request action_kit_api.PrepareActionRequestBody) (*BlockHostsConfig, error) {
	hostsInterface := request.Config["hosts"].([]any)
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
		BlockedIPs:     new(ipHosts),
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
	resourceId := request.Target.Attributes["network-security-group.id"]

	if len(resourceId) == 0 {
		return nil, fmt.Errorf("target is missing 'network-security-group.id' attribute")
	}

	resource := resourceId[0]

	parts := strings.Split(strings.TrimPrefix(resource, "/"), "/")

	if len(parts) < 8 {
		return nil, fmt.Errorf("invalid resource id format")
	}

	resourceGroup := parts[3]
	nsgName := parts[7]

	config, err := b.configProvider(request)

	if err != nil {
		return nil, err
	}

	state.Config = config
	state.ResourceId = resource
	state.ResourceGroupName = resourceGroup
	state.NetworkSecurityGroupName = nsgName

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

	securityGroup, err := client.Get(ctx, state.ResourceGroupName, state.NetworkSecurityGroupName, nil)

	if err != nil {
		return nil, fmt.Errorf("unable to retrieve security group '%s' in the resource group '%s' with error %s", state.NetworkSecurityGroupName, state.ResourceGroupName, err)
	}

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
		var sourcePrefix, destinationPrefix *string
		if state.Config.BlockDirection == armnetwork.SecurityRuleDirectionInbound {
			sourcePrefix = new(ip)
			destinationPrefix = new("*")
		} else {
			sourcePrefix = new("*")
			destinationPrefix = new(ip)
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
					SourcePortRange:          new("*"),
					DestinationPortRange:     new("*"),
					SourceAddressPrefix:      sourcePrefix,
					DestinationAddressPrefix: destinationPrefix,
					Access:                   to.Ptr(armnetwork.SecurityRuleAccessDeny),
					Direction:                new(state.Config.BlockDirection),
					Priority:                 new(priority),
					Description:              new("Blocked by steadybit"),
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
