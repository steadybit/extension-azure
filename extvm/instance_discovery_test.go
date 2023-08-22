package extvm

import (
	"context"
	"errors"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

type azureVirtualMachineClientMock struct {
	mock.Mock
}

func (m *azureVirtualMachineClientMock) Resources(ctx context.Context, query armresourcegraph.QueryRequest, options *armresourcegraph.ClientResourcesOptions) (armresourcegraph.ClientResourcesResponse, error) {
	args := m.Called(ctx, query, options)
	if args.Get(0) == nil {
		return armresourcegraph.ClientResourcesResponse{}, args.Error(1)
	}
	return *args.Get(0).(*armresourcegraph.ClientResourcesResponse), args.Error(1)
}

func TestGetAllAzureVirtualMachines(t *testing.T) {
	// Given
	mockedApi := new(azureVirtualMachineClientMock)
	var totalRecords int64 = 1
	mockedReturnValue := armresourcegraph.ClientResourcesResponse{
		QueryResponse: armresourcegraph.QueryResponse{
			TotalRecords: extutil.Ptr(totalRecords),
			Data: []any{
				map[string]any{
					"name":           "myVm",
					"type":           "Microsoft.Compute/virtualMachines",
					"id":             "/subscriptions/42/resourceGroups/rg-1/providers/Microsoft.Compute/virtualMachines/i-0ef9adc9fbd3b19c5",
					"location":       "westeurope",
					"subscriptionId": "42",
					"tags": map[string]any{
						"tag1": "Value1",
					},
					"properties": map[string]any{
						"hardwareProfile": map[string]any{
							"vmSize": "Standard_D2s_v3",
						},
						"storageProfile": map[string]any{
							"imageReference": map[string]any{
								"publisher": "Canonical",
								"offer":     "UbuntuServer",
								"sku":       "18.04-LTS",
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
						"vmId":              "0815",
					},
				},
			},
		},
	}
	mockedApi.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)

	// When
	targets, err := GetAllVirtualMachines(context.Background(), mockedApi)

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, TargetIDVM, target.TargetType)
	assert.Equal(t, "myVm", target.Label)
	assert.Equal(t, []string{"myVm"}, target.Attributes["azure-vm.vm.name"])
	assert.Equal(t, []string{"42"}, target.Attributes["azure-vm.subscription.id"])
	_, present := target.Attributes["label.name"]
	assert.False(t, present)
}

func TestGetAllAvailabilityZonesError(t *testing.T) {
	// Given
	mockedApi := new(azureVirtualMachineClientMock)

	mockedApi.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("expected"))

	// When
	_, err := GetAllVirtualMachines(context.Background(), mockedApi)

	// Then
	assert.Equal(t, err.Error(), "expected")
}
