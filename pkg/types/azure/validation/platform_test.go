package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/azure"
)

func validPlatform() *azure.Platform {
	return &azure.Platform{
		Region:                      "eastus",
		BaseDomainResourceGroupName: "group",
		OutboundType:                azure.LoadbalancerOutboundType,
		CloudName:                   azure.PublicCloud,
	}
}

func validNetworkPlatform() *azure.Platform {
	p := validPlatform()
	p.NetworkResourceGroupName = "networkresourcegroup"
	p.VirtualNetwork = "virtualnetwork"
	p.ComputeSubnet = "computesubnet"
	p.ControlPlaneSubnet = "controlplanesubnet"
	return p
}

func TestValidatePlatform(t *testing.T) {
	cases := []struct {
		name     string
		platform *azure.Platform
		wantSkip func(p *azure.Platform) bool
		expected string
	}{
		{
			name: "invalid region",
			platform: func() *azure.Platform {
				p := validPlatform()
				p.Region = ""
				return p
			}(),
			expected: `^test-path\.region: Required value: region should be set to one of the supported Azure regions$`,
		},
		{
			name: "invalid baseDomainResourceGroupName",
			wantSkip: func(p *azure.Platform) bool {
				// This test case doesn't apply to ARO
				// so we want to skip it when run tests for ARO build
				return p.IsARO()
			},
			platform: func() *azure.Platform {
				p := validPlatform()
				p.BaseDomainResourceGroupName = ""
				return p
			}(),
			expected: `^test-path\.baseDomainResourceGroupName: Required value: baseDomainResourceGroupName is the resource group name where the azure dns zone is deployed$`,
		},
		{
			name: "do not require baseDomainResourceGroupName on ARO",
			wantSkip: func(p *azure.Platform) bool {
				// This is a ARO-specific test case
				// so want to skip when running non-ARO builds
				return !p.IsARO()
			},
			platform: func() *azure.Platform {
				p := validPlatform()
				p.BaseDomainResourceGroupName = ""
				return p
			}(),
		},
		{
			name:     "minimal",
			platform: validPlatform(),
		},
		{
			name: "valid machine pool",
			platform: func() *azure.Platform {
				p := validPlatform()
				p.DefaultMachinePlatform = &azure.MachinePool{}
				return p
			}(),
		},
		{
			name:     "valid subnets & virtual network",
			platform: validNetworkPlatform(),
		},
		{
			name: "missing subnets",
			platform: func() *azure.Platform {
				p := validNetworkPlatform()
				p.ControlPlaneSubnet = ""
				return p
			}(),
			expected: `^test-path\.controlPlaneSubnet: Required value: must provide a control plane subnet when a virtual network is specified$`,
		},
		{
			name: "subnets missing virtual network",
			platform: func() *azure.Platform {
				p := validNetworkPlatform()
				p.ControlPlaneSubnet = ""
				p.VirtualNetwork = ""
				return p
			}(),
			expected: `^test-path\.virtualNetwork: Required value: must provide a virtual network when supplying subnets$`,
		},
		{
			name: "missing network resource group",
			platform: func() *azure.Platform {
				p := validNetworkPlatform()
				p.NetworkResourceGroupName = ""
				return p
			}(),
			expected: `^\[test-path\.networkResourceGroupName: Required value: must provide a network resource group when a virtual network is specified, test-path\.networkResourceGroupName: Required value: must provide a network resource group when supplying subnets\]$`,
		},
		{
			name: "missing cloud name",
			platform: func() *azure.Platform {
				p := validPlatform()
				p.CloudName = ""
				return p
			}(),
			expected: `^test-path\.cloudName: Unsupported value: "": supported values:`,
		},
		{
			name: "invalid cloud name",
			platform: func() *azure.Platform {
				p := validPlatform()
				p.CloudName = azure.CloudEnvironment("AzureOtherCloud")
				return p
			}(),
			expected: `^test-path\.cloudName: Unsupported value: "AzureOtherCloud": supported values:`,
		},
		{
			name: "invalid outbound type",
			platform: func() *azure.Platform {
				p := validNetworkPlatform()
				p.OutboundType = "random-egress"
				return p
			}(),
			expected: `^test-path\.outboundType: Unsupported value: "random-egress": supported values: "Loadbalancer", "UserDefinedRouting"$`,
		},
		{
			name: "invalid user defined type",
			platform: func() *azure.Platform {
				p := validPlatform()
				p.OutboundType = azure.UserDefinedRoutingOutboundType
				return p
			}(),
			expected: `^test-path\.outboundType: Invalid value: "UserDefinedRouting": UserDefinedRouting is only allowed when installing to pre-existing network$`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.wantSkip != nil && tc.wantSkip(tc.platform) {
				t.Skip()
			}

			err := ValidatePlatform(tc.platform, types.ExternalPublishingStrategy, field.NewPath("test-path")).ToAggregate()
			if tc.expected == "" {
				assert.NoError(t, err)
			} else {
				assert.Regexp(t, tc.expected, err)
			}
		})
	}
}

func TestValidateUserTags(t *testing.T) {
	fieldPath := "spec.platform.azure.userTags"
	cases := []struct {
		name     string
		userTags map[string]string
		wantErr  bool
	}{
		{
			name:     "userTags not configured",
			userTags: map[string]string{},
			wantErr:  false,
		},
		{
			name: "userTags configured",
			userTags: map[string]string{
				"key1": "value1", "key_2": "value_2", "key.3": "value.3", "key=4": "value=4", "key+5": "value+5",
				"key-6": "value-6", "key@7": "value@7", "key8_": "value8-", "key9=": "value9+", "key10-": "value10@"},
			wantErr: false,
		},
		{
			name: "userTags configured is more than max limit",
			userTags: map[string]string{
				"key1": "value1", "key2": "value2", "key3": "value3", "key4": "value4", "key5": "value5",
				"key6": "value6", "key7": "value7", "key8": "value8", "key9": "value9", "key10": "value10",
				"key11": "value11"},
			wantErr: true,
		},
		{
			name:     "userTags contains key starting a number",
			userTags: map[string]string{"1key": "1value"},
			wantErr:  true,
		},
		{
			name:     "userTags contains empty key",
			userTags: map[string]string{"": "value"},
			wantErr:  true,
		},
		{
			name: "userTags contains key length greater than 128",
			userTags: map[string]string{
				"thisisaverylongkeywithmorethan128characterswhichisnotallowedforazureresourcetagkeysandthetagkeyvalidationshouldfailwithinvalidfieldvalueerror": "value"},
			wantErr: true,
		},
		{
			name:     "userTags contains key with invalid character",
			userTags: map[string]string{"key/test": "value"},
			wantErr:  true,
		},
		{
			name:     "userTags contains value length greater than 256",
			userTags: map[string]string{"key": "thisisaverylongvaluewithmorethan256characterswhichisnotallowedforazureresourcetagvaluesandthetagvaluevalidationshouldfailwithinvalidfieldvalueerrorrepeatthisisaverylongvaluewithmorethan256characterswhichisnotallowedforazureresourcetagvaluesandthetagvaluevalidationshouldfailwithinvalidfieldvalueerror"},
			wantErr:  true,
		},
		{
			name:     "userTags contains empty value",
			userTags: map[string]string{"key": ""},
			wantErr:  true,
		},
		{
			name:     "userTags contains value with invalid character",
			userTags: map[string]string{"key": "value*^%"},
			wantErr:  true,
		},
		{
			name:     "userTags contains key as name",
			userTags: map[string]string{"name": "value"},
			wantErr:  true,
		},
		{
			name:     "userTags contains allowed key name123",
			userTags: map[string]string{"name123": "value"},
			wantErr:  false,
		},
		{
			name:     "userTags contains key with prefix kubernetes.io",
			userTags: map[string]string{"kubernetes.io_cluster": "value"},
			wantErr:  true,
		},
		{
			name:     "userTags contains allowed key prefix for_openshift.io",
			userTags: map[string]string{"for_openshift.io": "azure"},
			wantErr:  false,
		},
		{
			name:     "userTags contains key with prefix azure",
			userTags: map[string]string{"azure": "microsoft"},
			wantErr:  true,
		},
		{
			name:     "userTags contains allowed key resourcename",
			userTags: map[string]string{"resourcename": "value"},
			wantErr:  false,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUserTags(tt.userTags, field.NewPath(fieldPath))
			if (len(err) > 0) != tt.wantErr {
				t.Errorf("unexpected error, err: %v", err)
			}
		})
	}
}
