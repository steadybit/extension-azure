package extvm

import (
	"context"
	"errors"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/steadybit/extension-azure/config"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

type azureResourceGraphClientMock struct {
	mock.Mock
}

func (m *azureResourceGraphClientMock) Resources(ctx context.Context, query armresourcegraph.QueryRequest, options *armresourcegraph.ClientResourcesOptions) (armresourcegraph.ClientResourcesResponse, error) {
	args := m.Called(ctx, query, options)
	if args.Get(0) == nil {
		return armresourcegraph.ClientResourcesResponse{}, args.Error(1)
	}
	return *args.Get(0).(*armresourcegraph.ClientResourcesResponse), args.Error(1)
}

func TestGetAllAzureVirtualMachines(t *testing.T) {
	// Given
	mockedApi := new(azureResourceGraphClientMock)
	var totalRecords int64 = 1
	mockedReturnValue := armresourcegraph.ClientResourcesResponse{
		QueryResponse: armresourcegraph.QueryResponse{
			TotalRecords: extutil.Ptr(totalRecords),
			Data: []any{
				map[string]any{
					"name":           "myVm",
					"type":           "Microsoft.Compute/virtualMachines",
					"location":       "westeurope",
					"extendedLocation": map[string]any{
						"name": "westeurope-1",
						"type": "EdgeZone",
					},
					"subscriptionId": "42",
					"resourceGroup":  "rg-1",
					"tags": map[string]any{
						"tag1": "Value1",
						"tag2": "Value2",
					},
					"properties": map[string]any{
						"hardwareProfile": map[string]any{
							"vmSize": "Standard_D2s_v3",
						},
						"extended": map[string]any{
							"instanceView": map[string]any{
								"osVersion":    "18.04.5 LTS",
								"computerName": "dev-demo",
								"osName":       "Ubuntu 18.04.5 LTS",
								"vmAgent": map[string]any{
									"vmAgentVersion": "2.7.0",
								},
								"powerState": map[string]any{
									"code": "PowerState/running",
								},
							},
						},
						"storageProfile": map[string]any{
							"osDisk": map[string]any{
								"osType": "Linux",
							},
						},
						"osProfile": map[string]any{
							"computerName":  "dev-demo-ngroup2",
							"adminUsername": "ubuntu",
						},
						"networkProfile": map[string]any{
							"networkInterfaces": []any{
								map[string]any{
									"id": "/subscriptions/42/resourceGroups/rg-1/providers/Microsoft.Network/networkInterfaces/i-0ef9adc9fbd3b19c5",
								},
							},
						},
						"provisioningState": "Succeeded",
						"vmId":              "/subscriptions/42/resourceGroups/rg-1/providers/Microsoft.Compute/virtualMachines/i-0ef9adc9fbd3b19c5",
					},
				},
			},
		},
	}
	mockedApi.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)

	config.Config.DiscoveryAttributesExcludesVM = []string{"azure-vm.label.tag1"}

	// When
	targets, err := getAllVirtualMachines(context.Background(), mockedApi)

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, TargetIDVM, target.TargetType)
	assert.Equal(t, "myVm", target.Label)
	assert.Equal(t, []string{"myVm"}, target.Attributes["azure-vm.vm.name"])
	assert.Equal(t, []string{"42"}, target.Attributes["azure.subscription.id"])
	assert.Equal(t, []string{"/subscriptions/42/resourceGroups/rg-1/providers/Microsoft.Compute/virtualMachines/i-0ef9adc9fbd3b19c5"}, target.Attributes["azure-vm.vm.id"])
	assert.Equal(t, []string{"Standard_D2s_v3"}, target.Attributes["azure-vm.vm.size"])
	assert.Equal(t, []string{"Ubuntu 18.04.5 LTS"}, target.Attributes["azure-vm.os.name"])
	assert.Equal(t, []string{"dev-demo"}, target.Attributes["azure-vm.hostname"])
	assert.Equal(t, []string{"18.04.5 LTS"}, target.Attributes["azure-vm.os.version"])
	assert.Equal(t, []string{"Linux"}, target.Attributes["azure-vm.os.type"])
	assert.Equal(t, []string{"PowerState/running"}, target.Attributes["azure-vm.power.state"])
	assert.Equal(t, []string{"/subscriptions/42/resourceGroups/rg-1/providers/Microsoft.Network/networkInterfaces/i-0ef9adc9fbd3b19c5"}, target.Attributes["azure-vm.network.id"])
	assert.Equal(t, []string{"westeurope"}, target.Attributes["azure.location"])
	assert.Equal(t, []string{"westeurope-1"}, target.Attributes["azure.zone"])
	assert.Equal(t, []string{"rg-1"}, target.Attributes["azure.resource-group.name"])
	assert.Equal(t, []string{"Value2"}, target.Attributes["azure-vm.label.tag2"])
	assert.NotContains(t, target.Attributes, "azure-vm.label.tag1")
	_, present := target.Attributes["label.name"]
	assert.False(t, present)
}

func TestGetAllError(t *testing.T) {
	// Given
	mockedApi := new(azureResourceGraphClientMock)

	mockedApi.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("expected"))

	// When
	_, err := getAllVirtualMachines(context.Background(), mockedApi)

	// Then
	assert.Equal(t, err.Error(), "expected")
}
