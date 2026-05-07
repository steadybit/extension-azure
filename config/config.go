/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package config

import (
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

// Specification is the configuration specification for the extension. Configuration values can be applied
// through environment variables. Learn more through the documentation of the envconfig package.
// https://github.com/kelseyhightower/envconfig
type Specification struct {
	AzureCertificatePath                            string   `json:"azureCertificatePath" required:"false" split_words:"true"`
	AzureCertificatePassword                        string   `json:"azureCertificatePassword" required:"false" split_words:"true"`
	AzureUserAssertionString                        string   `json:"azureUserAssertionString" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesScaleSetInstance     []string `json:"discoveryAttributesExcludesScaleSetInstance" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesVM                   []string `json:"discoveryAttributesExcludesVM" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesAzureFunction        []string `json:"discoveryAttributesExcludesAzureFunction" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesNetworkSecurityGroup []string `json:"discoveryAttributesExcludesNetworkSecurityGroup" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesContainerApp         []string `json:"discoveryAttributesExcludesContainerApp" required:"false" split_words:"true"`
	EnrichScaleSetVMDataForTargetTypes              []string `json:"EnrichScaleSetVMDataForTargetTypes" split_words:"true" default:"com.steadybit.extension_jvm.jvm-instance,com.steadybit.extension_container.container,com.steadybit.extension_kubernetes.argo-rollout,com.steadybit.extension_kubernetes.kubernetes-deployment,com.steadybit.extension_kubernetes.kubernetes-pod,com.steadybit.extension_kubernetes.kubernetes-daemonset,com.steadybit.extension_kubernetes.kubernetes-statefulset,com.steadybit.extension_http.client-location,com.steadybit.extension_jmeter.location,com.steadybit.extension_k6.location,com.steadybit.extension_gatling.location"`
	DiscoveryEnableVirtualMachines                  bool     `json:"discoveryEnableVirtualMachines" split_words:"true" required:"false" default:"true"`
	DiscoveryEnableScaleInstances                   bool     `json:"discoveryEnableScaleInstances" split_words:"true" required:"false" default:"true"`
	DiscoveryEnableAzureFunctions                   bool     `json:"discoveryEnableAzureFunctions" split_words:"true" required:"false" default:"false"`
	DiscoveryEnableNetworkSecurityGroups            bool     `json:"discoveryEnableNetworkSecurityGroups" split_words:"true" required:"false" default:"false"`
	DiscoveryEnableContainerApps                    bool     `json:"discoveryEnableContainerApps" split_words:"true" required:"false" default:"false"`

	// Modules added in feat/expand-azure-targets-and-attacks; default to disabled to keep the smallest IAM/cost
	// footprint for users upgrading from a previous version.
	DiscoveryEnableAksCluster         bool `json:"discoveryEnableAksCluster" split_words:"true" required:"false" default:"false"`
	DiscoveryEnableAksNodePool        bool `json:"discoveryEnableAksNodePool" split_words:"true" required:"false" default:"false"`
	DiscoveryEnableScaleSet           bool `json:"discoveryEnableScaleSet" split_words:"true" required:"false" default:"false"`
	DiscoveryEnableManagedDisk        bool `json:"discoveryEnableManagedDisk" split_words:"true" required:"false" default:"false"`
	DiscoveryEnableNatGateway         bool `json:"discoveryEnableNatGateway" split_words:"true" required:"false" default:"false"`
	DiscoveryEnableCosmosDb           bool `json:"discoveryEnableCosmosDb" split_words:"true" required:"false" default:"false"`
	DiscoveryEnableEventGrid          bool `json:"discoveryEnableEventGrid" split_words:"true" required:"false" default:"false"`
	DiscoveryEnableServiceBus         bool `json:"discoveryEnableServiceBus" split_words:"true" required:"false" default:"false"`
	DiscoveryEnableServiceBusQueue    bool `json:"discoveryEnableServiceBusQueue" split_words:"true" required:"false" default:"false"`
	DiscoveryEnableServiceBusTopic    bool `json:"discoveryEnableServiceBusTopic" split_words:"true" required:"false" default:"false"`
	DiscoveryEnableStorageQueue       bool `json:"discoveryEnableStorageQueue" split_words:"true" required:"false" default:"false"`
	DiscoveryEnableLoadBalancer       bool `json:"discoveryEnableLoadBalancer" split_words:"true" required:"false" default:"false"`
	DiscoveryEnableApplicationGateway bool `json:"discoveryEnableApplicationGateway" split_words:"true" required:"false" default:"false"`
	DiscoveryEnableApiManagement      bool `json:"discoveryEnableApiManagement" split_words:"true" required:"false" default:"false"`

	DiscoveryAttributesExcludesAksCluster         []string `json:"discoveryAttributesExcludesAksCluster" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesAksNodePool        []string `json:"discoveryAttributesExcludesAksNodePool" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesScaleSet           []string `json:"discoveryAttributesExcludesScaleSet" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesManagedDisk        []string `json:"discoveryAttributesExcludesManagedDisk" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesNatGateway         []string `json:"discoveryAttributesExcludesNatGateway" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesCosmosDb           []string `json:"discoveryAttributesExcludesCosmosDb" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesEventGrid          []string `json:"discoveryAttributesExcludesEventGrid" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesServiceBus         []string `json:"discoveryAttributesExcludesServiceBus" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesStorageQueue       []string `json:"discoveryAttributesExcludesStorageQueue" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesLoadBalancer       []string `json:"discoveryAttributesExcludesLoadBalancer" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesApplicationGateway []string `json:"discoveryAttributesExcludesApplicationGateway" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesApiManagement      []string `json:"discoveryAttributesExcludesApiManagement" required:"false" split_words:"true"`
}

var (
	Config Specification
)

func ParseConfiguration() {
	err := envconfig.Process("steadybit_extension", &Config)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to parse configuration from environment.")
	}
}

func ValidateConfiguration() {
	// You may optionally validate the configuration here.
}
