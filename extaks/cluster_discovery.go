/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extaks

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-azure/common"
	"github.com/steadybit/extension-azure/config"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type clusterDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber          = (*clusterDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber       = (*clusterDiscovery)(nil)
	_ discovery_kit_sdk.EnrichmentRulesDescriber = (*clusterDiscovery)(nil)
)

// aksEnrichmentTargetTypes are the extension-kubernetes target types that get enriched with AKS cluster
// reliability config (kubernetes version, API-server public-access posture, network plugin/policy, AAD/RBAC,
// SKU tier, etc.). The K8s extension already carries k8s.cluster-name on its targets; we join on that.
var aksEnrichmentTargetTypes = []string{
	"com.steadybit.extension_kubernetes.kubernetes-deployment",
	"com.steadybit.extension_kubernetes.kubernetes-pod",
	"com.steadybit.extension_kubernetes.kubernetes-statefulset",
	"com.steadybit.extension_kubernetes.kubernetes-daemonset",
	"com.steadybit.extension_kubernetes.kubernetes-node",
	"com.steadybit.extension_kubernetes.argo-rollout",
}

// aksEnrichmentAttributes are the AKS cluster attributes copied onto matching Kubernetes targets.
// Stable, reliability-relevant config only — no labels (high cardinality) and no volatile state
// (power-state, provisioning-state).
var aksEnrichmentAttributes = []discovery_kit_api.Attribute{
	{Matcher: discovery_kit_api.Equals, Name: "azure.subscription.id"},
	{Matcher: discovery_kit_api.Equals, Name: "azure.resource-group.name"},
	{Matcher: discovery_kit_api.Equals, Name: "azure.location"},
	{Matcher: discovery_kit_api.Equals, Name: "azure.aks.cluster.name"},
	{Matcher: discovery_kit_api.Equals, Name: "azure.aks.cluster.kubernetes-version"},
	{Matcher: discovery_kit_api.Equals, Name: "azure.aks.cluster.private-cluster"},
	{Matcher: discovery_kit_api.Equals, Name: "azure.aks.cluster.authorized-ip-ranges"},
	{Matcher: discovery_kit_api.Equals, Name: "azure.aks.cluster.api-server-open-to-internet"},
	{Matcher: discovery_kit_api.Equals, Name: "azure.aks.cluster.network-plugin"},
	{Matcher: discovery_kit_api.Equals, Name: "azure.aks.cluster.network-policy"},
	{Matcher: discovery_kit_api.Equals, Name: "azure.aks.cluster.sku.tier"},
	{Matcher: discovery_kit_api.Equals, Name: "azure.aks.cluster.disable-local-accounts"},
	{Matcher: discovery_kit_api.Equals, Name: "azure.aks.cluster.aad-managed"},
	{Matcher: discovery_kit_api.Equals, Name: "azure.aks.cluster.azure-rbac-enabled"},
	{Matcher: discovery_kit_api.Equals, Name: "azure.aks.cluster.node-resource-group"},
}

func NewClusterDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&clusterDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *clusterDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: TargetIDCluster,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("60s"),
		},
	}
}

func (d *clusterDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDCluster,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure AKS cluster", Other: "Azure AKS clusters"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.aks.cluster.kubernetes-version"},
				{Attribute: "azure.aks.cluster.power-state"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *clusterDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.aks.cluster.name", Label: discovery_kit_api.PluralLabel{One: "AKS cluster name", Other: "AKS cluster names"}},
		{Attribute: "azure.aks.cluster.kubernetes-version", Label: discovery_kit_api.PluralLabel{One: "AKS Kubernetes version", Other: "AKS Kubernetes versions"}},
		{Attribute: "azure.aks.cluster.power-state", Label: discovery_kit_api.PluralLabel{One: "AKS cluster power state", Other: "AKS cluster power states"}},
		{Attribute: "azure.aks.cluster.provisioning-state", Label: discovery_kit_api.PluralLabel{One: "AKS cluster provisioning state", Other: "AKS cluster provisioning states"}},
		{Attribute: "azure.aks.cluster.dns-prefix", Label: discovery_kit_api.PluralLabel{One: "AKS DNS prefix", Other: "AKS DNS prefixes"}},
		{Attribute: "azure.aks.cluster.fqdn", Label: discovery_kit_api.PluralLabel{One: "AKS FQDN", Other: "AKS FQDNs"}},
		{Attribute: "azure.aks.cluster.private-cluster", Label: discovery_kit_api.PluralLabel{One: "AKS private cluster", Other: "AKS private clusters"}},
		{Attribute: "azure.aks.cluster.authorized-ip-ranges", Label: discovery_kit_api.PluralLabel{One: "AKS authorized IP range", Other: "AKS authorized IP ranges"}},
		{Attribute: "azure.aks.cluster.api-server-open-to-internet", Label: discovery_kit_api.PluralLabel{One: "AKS API server open to internet", Other: "AKS API server open to internet"}},
		{Attribute: "azure.aks.cluster.network-plugin", Label: discovery_kit_api.PluralLabel{One: "AKS network plugin", Other: "AKS network plugins"}},
		{Attribute: "azure.aks.cluster.network-policy", Label: discovery_kit_api.PluralLabel{One: "AKS network policy", Other: "AKS network policies"}},
		{Attribute: "azure.aks.cluster.sku.tier", Label: discovery_kit_api.PluralLabel{One: "AKS SKU tier", Other: "AKS SKU tiers"}},
		{Attribute: "azure.aks.cluster.disable-local-accounts", Label: discovery_kit_api.PluralLabel{One: "AKS disable local accounts", Other: "AKS disable local accounts"}},
		{Attribute: "azure.aks.cluster.aad-managed", Label: discovery_kit_api.PluralLabel{One: "AKS AAD managed", Other: "AKS AAD managed"}},
		{Attribute: "azure.aks.cluster.azure-rbac-enabled", Label: discovery_kit_api.PluralLabel{One: "AKS Azure RBAC", Other: "AKS Azure RBAC"}},
		{Attribute: "azure.aks.cluster.node-resource-group", Label: discovery_kit_api.PluralLabel{One: "AKS node resource group", Other: "AKS node resource groups"}},
		{Attribute: "azure.location", Label: discovery_kit_api.PluralLabel{One: "Location", Other: "Locations"}},
		{Attribute: "azure.subscription.id", Label: discovery_kit_api.PluralLabel{One: "Subscription ID", Other: "Subscription IDs"}},
		{Attribute: "azure.resource-group.name", Label: discovery_kit_api.PluralLabel{One: "Resource group name", Other: "Resource group names"}},
		{Attribute: "k8s.cluster-name", Label: discovery_kit_api.PluralLabel{One: "Kubernetes cluster name", Other: "Kubernetes cluster names"}},
	}
}

func (d *clusterDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllAksClusters(ctx, client)
}

func getAllAksClusters(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	targets, err := common.DiscoverViaResourceGraph(ctx, client,
		"Resources | where type =~ 'Microsoft.ContainerService/managedClusters' | project id, name, type, resourceGroup, location, tags, properties, sku, subscriptionId",
		toClusterTarget)
	if err != nil {
		log.Error().Err(err).Msg("failed to get AKS cluster results")
		return nil, err
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesAksCluster), nil
}

func toClusterTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")
	powerState := common.GetMapValue(properties, "powerState")
	apiServerAccessProfile := common.GetMapValue(properties, "apiServerAccessProfile")
	networkProfile := common.GetMapValue(properties, "networkProfile")
	aadProfile := common.GetMapValue(properties, "aadProfile")
	sku := common.GetMapValue(items, "sku")

	id, _ := items["id"].(string)
	name, _ := items["name"].(string)

	attributes := make(map[string][]string)
	attributes["azure.aks.cluster.name"] = []string{name}
	// The AKS cluster name IS the Kubernetes cluster name (1:1). Surface it under the cluster-wide
	// k8s.cluster-name attribute that extension-kubernetes carries on its discovered targets, so enrichment
	// rules below can join AKS reliability config onto Kubernetes targets discovered by that extension.
	attributes["k8s.cluster-name"] = []string{name}
	attributes["azure.subscription.id"] = []string{common.StringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{common.StringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{common.StringFromMap(items, "location")}

	if v := common.StringFromMap(properties, "kubernetesVersion"); v != "" {
		attributes["azure.aks.cluster.kubernetes-version"] = []string{v}
	}
	if v := common.StringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.aks.cluster.provisioning-state"] = []string{v}
	}
	if v := common.StringFromMap(powerState, "code"); v != "" {
		attributes["azure.aks.cluster.power-state"] = []string{v}
	}
	if v := common.StringFromMap(properties, "dnsPrefix"); v != "" {
		attributes["azure.aks.cluster.dns-prefix"] = []string{v}
	}
	if v := common.StringFromMap(properties, "fqdn"); v != "" {
		attributes["azure.aks.cluster.fqdn"] = []string{v}
	}
	if v := common.StringFromMap(properties, "nodeResourceGroup"); v != "" {
		attributes["azure.aks.cluster.node-resource-group"] = []string{v}
	}
	if v, ok := properties["disableLocalAccounts"].(bool); ok {
		attributes["azure.aks.cluster.disable-local-accounts"] = []string{strconv.FormatBool(v)}
	}

	private := false
	if v, ok := apiServerAccessProfile["enablePrivateCluster"].(bool); ok {
		private = v
	}
	attributes["azure.aks.cluster.private-cluster"] = []string{strconv.FormatBool(private)}

	authorizedRanges := common.StringSliceFromMap(apiServerAccessProfile, "authorizedIPRanges")
	if len(authorizedRanges) > 0 {
		sort.Strings(authorizedRanges)
		attributes["azure.aks.cluster.authorized-ip-ranges"] = authorizedRanges
	}
	attributes["azure.aks.cluster.api-server-open-to-internet"] = []string{strconv.FormatBool(isAksApiServerOpenToInternet(private, authorizedRanges))}

	if v := common.StringFromMap(networkProfile, "networkPlugin"); v != "" {
		attributes["azure.aks.cluster.network-plugin"] = []string{v}
	}
	if v := common.StringFromMap(networkProfile, "networkPolicy"); v != "" {
		attributes["azure.aks.cluster.network-policy"] = []string{v}
	}
	if v := common.StringFromMap(sku, "tier"); v != "" {
		attributes["azure.aks.cluster.sku.tier"] = []string{v}
	}
	if v, ok := aadProfile["managed"].(bool); ok {
		attributes["azure.aks.cluster.aad-managed"] = []string{strconv.FormatBool(v)}
	}
	if v, ok := aadProfile["enableAzureRBAC"].(bool); ok {
		attributes["azure.aks.cluster.azure-rbac-enabled"] = []string{strconv.FormatBool(v)}
	}

	for k, v := range common.GetMapValue(items, "tags") {
		attributes[fmt.Sprintf("azure.aks.cluster.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDCluster,
		Label:      name,
		Attributes: attributes,
	}
}

// isAksApiServerOpenToInternet mirrors the EKS public-access-open-to-internet semantics for the AKS API server.
// True iff the cluster is NOT private AND has no authorizedIPRanges restriction.
func isAksApiServerOpenToInternet(private bool, authorizedIPRanges []string) bool {
	if private {
		return false
	}
	return len(authorizedIPRanges) == 0
}

func (d *clusterDiscovery) DescribeEnrichmentRules() []discovery_kit_api.TargetEnrichmentRule {
	rules := make([]discovery_kit_api.TargetEnrichmentRule, 0, len(aksEnrichmentTargetTypes))
	for _, t := range aksEnrichmentTargetTypes {
		rules = append(rules, aksClusterToK8sEnrichmentRule(t))
	}
	return rules
}

func aksClusterToK8sEnrichmentRule(destTargetType string) discovery_kit_api.TargetEnrichmentRule {
	return discovery_kit_api.TargetEnrichmentRule{
		Id:      fmt.Sprintf("com.steadybit.extension_azure.aks.cluster-to-%s", destTargetType),
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Src: discovery_kit_api.SourceOrDestination{
			Type: TargetIDCluster,
			Selector: map[string]string{
				"k8s.cluster-name": "${dest.k8s.cluster-name}",
			},
		},
		Dest: discovery_kit_api.SourceOrDestination{
			Type: destTargetType,
			Selector: map[string]string{
				"k8s.cluster-name": "${src.k8s.cluster-name}",
			},
		},
		Attributes: aksEnrichmentAttributes,
	}
}
