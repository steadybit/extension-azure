package extscalesetinstance

import (
	"context"
	"errors"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
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

type azureVirtualMachineScaleSetVMsClientApiMock struct {
	mock.Mock
}

func (m *azureVirtualMachineScaleSetVMsClientApiMock) NewListPager(resourceGroupName string, virtualMachineScaleSetName string, options *armcompute.VirtualMachineScaleSetVMsClientListOptions) *runtime.Pager[armcompute.VirtualMachineScaleSetVMsClientListResponse] {
	args := m.Called(resourceGroupName, virtualMachineScaleSetName, options)

	return runtime.NewPager(runtime.PagingHandler[armcompute.VirtualMachineScaleSetVMsClientListResponse]{
		More: func(page armcompute.VirtualMachineScaleSetVMsClientListResponse) bool {
			return page.NextLink != nil && len(*page.NextLink) > 0
		},
		Fetcher: func(ctx context.Context, page *armcompute.VirtualMachineScaleSetVMsClientListResponse) (armcompute.VirtualMachineScaleSetVMsClientListResponse, error) {
			if args.Get(0) == nil {
				return armcompute.VirtualMachineScaleSetVMsClientListResponse{}, args.Error(1)
			}
			return *args.Get(0).(*armcompute.VirtualMachineScaleSetVMsClientListResponse), nil
		},
	})
}

func TestGetAllAzureScaleSets(t *testing.T) {
	// Given
	mockedApi := new(azureResourceGraphClientMock)
	var totalRecords int64 = 1
	mockedReturnValue := armresourcegraph.ClientResourcesResponse{
		QueryResponse: armresourcegraph.QueryResponse{
			TotalRecords: extutil.Ptr(totalRecords),
			Data: []any{
				map[string]any{
					"id":             "myScalesetId",
					"name":           "myScaleset",
					"type":           "Microsoft.compute/virtualmachinescalesets",
					"location":       "westeurope",
					"subscriptionId": "42",
					"resourceGroup":  "rg-1",
					"tags": map[string]any{
						"tag1": "Value1",
						"tag2": "Value2",
					},
				},
			},
		},
	}
	mockedApi.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)

	// When
	scaleSets, err := GetAllScaleSets(context.Background(), mockedApi)

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(scaleSets))

	scaleSet := scaleSets[0]
	assert.Equal(t, "myScalesetId", scaleSet.Id)
	assert.Equal(t, "westeurope", scaleSet.Location)
	assert.Equal(t, "myScaleset", scaleSet.Name)
	assert.Equal(t, "42", scaleSet.SubscriptionId)
	assert.Equal(t, "rg-1", scaleSet.ResourceGroupName)
	assert.Equal(t, []string{"Value1"}, scaleSet.Attributes["azure-scale-set.label.tag1"])
	assert.Equal(t, []string{"Value2"}, scaleSet.Attributes["azure-scale-set.label.tag2"])
	_, present := scaleSet.Attributes["label.name"]
	assert.False(t, present)
}

func TestGetAllAzureScaleSetInstances(t *testing.T) {
	mockedApi := new(azureVirtualMachineScaleSetVMsClientApiMock)

	mockedReturnValue := armcompute.VirtualMachineScaleSetVMsClientListResponse{
		VirtualMachineScaleSetVMListResult: armcompute.VirtualMachineScaleSetVMListResult{
			Value: []*armcompute.VirtualMachineScaleSetVM{&armcompute.VirtualMachineScaleSetVM{
				Location:   extutil.Ptr("westeurope"),
				Tags:       map[string]*string{"tag1": extutil.Ptr("Value1"), "tag2": extutil.Ptr("Value2")},
				ID:         extutil.Ptr("/subscriptions/42/resourceGroups/rg-1/providers/Microsoft.Compute/virtualMachineScaleSets/myScaleSet/virtualMachines/myVm"),
				InstanceID: extutil.Ptr("0"),
				Name:       extutil.Ptr("myVm"),
				Zones:      []*string{extutil.Ptr("1"), extutil.Ptr("2")},
				Properties: &armcompute.VirtualMachineScaleSetVMProperties{
					OSProfile: &armcompute.OSProfile{
						ComputerName: extutil.Ptr("dev-demo"),
					},
					HardwareProfile: &armcompute.HardwareProfile{
						VMSize: extutil.Ptr(armcompute.VirtualMachineSizeTypesBasicA0),
					},
					InstanceView: &armcompute.VirtualMachineScaleSetVMInstanceView{
						OSName:    extutil.Ptr("Ubuntu 18.04.5 LTS"),
						OSVersion: extutil.Ptr("18.04.5 LTS"),
					},
					StorageProfile: &armcompute.StorageProfile{
						OSDisk: &armcompute.OSDisk{
							OSType: extutil.Ptr(armcompute.OperatingSystemTypesLinux),
						},
					},
					ProvisioningState: extutil.Ptr("Succeeded"),
				},
			},
			},
		},
	}
	// Given
	mockedApi.On("NewListPager", mock.Anything, mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)

	var scaleSet ScaleSet
	scaleSet.Name = "myScaleSet"
	scaleSet.Location = "westeurope"
	scaleSet.SubscriptionId = "42"
	scaleSet.ResourceGroupName = "rg-1"
	scaleSet.Id = "/subscriptions/42/resourceGroups/rg-1/providers/Microsoft.Compute/virtualMachineScaleSets/myScaleSet"
	scaleSet.Attributes = make(map[string][]string)
	scaleSet.Attributes["azure-scale-set.label.tag1"] = []string{"value1"}
	// When
	targets, err := GetAllScaleSetInstances(context.Background(), mockedApi, scaleSet)

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, TargetIDScaleSetInstance, target.TargetType)
	assert.Equal(t, "myVm", target.Label)
	assert.Equal(t, []string{"myScaleSet"}, target.Attributes["azure-scale-set.name"])
	assert.Equal(t, []string{"myVm"}, target.Attributes["azure-scale-set-instance.name"])
	assert.Equal(t, []string{"42"}, target.Attributes["azure.subscription.id"])
	assert.Equal(t, []string{"/subscriptions/42/resourceGroups/rg-1/providers/Microsoft.Compute/virtualMachineScaleSets/myScaleSet/virtualMachines/myVm"}, target.Attributes["azure-scale-set-instance.resource.id"])
	assert.Equal(t, []string{"0"}, target.Attributes["azure-scale-set-instance.id"])
	assert.Equal(t, []string{"westeurope"}, target.Attributes["azure.location"])
	assert.Equal(t, []string{"rg-1"}, target.Attributes["azure.resource-group.name"])
	assert.Equal(t, []string{"dev-demo"}, target.Attributes["azure-scale-set-instance.hostname"])
	assert.Equal(t, []string{"Basic_A0"}, target.Attributes["azure-scale-set-instance.vm.size"])
	assert.Equal(t, []string{"Ubuntu 18.04.5 LTS"}, target.Attributes["azure-scale-set-instance.os.name"])
	assert.Equal(t, []string{"18.04.5 LTS"}, target.Attributes["azure-scale-set-instance.os.version"])
	assert.Equal(t, []string{"Linux"}, target.Attributes["azure-scale-set-instance.os.type"])
	assert.Equal(t, []string{"Succeeded"}, target.Attributes["azure-scale-set-instance.provisioning.state"])
	assert.Equal(t, []string{"Value1"}, target.Attributes["azure-scale-set-instance.label.tag1"])
	assert.Equal(t, []string{"Value2"}, target.Attributes["azure-scale-set-instance.label.tag2"])
	_, present := target.Attributes["label.name"]
	assert.False(t, present)
}

func TestGetAllError(t *testing.T) {
	// Given
	mockedApi := new(azureResourceGraphClientMock)

	mockedApi.On("Resources", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("expected"))

	// When
	_, err := GetAllScaleSets(context.Background(), mockedApi)

	// Then
	assert.Equal(t, err.Error(), "expected")
}

func TestGetAttributeDescriptions(t *testing.T) {
	// just cover this static code
	descriptions := getAttributeDescriptions()
	assert.Greater(t, len(descriptions.Attributes), 8)
}

func TestGetToContainerEnrichmentRule(t *testing.T) {
	// just cover this static code
	enrichmentRule := getToContainerEnrichmentRule()
	assert.Greater(t, len(enrichmentRule.Attributes), 8)
}

func TestGetToHostEnrichmentRule(t *testing.T) {
	// just cover this static code
	enrichmentRule := getToHostEnrichmentRule()
	assert.Greater(t, len(enrichmentRule.Attributes), 8)
}
