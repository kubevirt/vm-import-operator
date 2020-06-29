package validators_test

import (
	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/validation/validators"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	ovirtsdk "github.com/ovirt/go-ovirt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"fmt"
)

var (
	podType                  = "pod"
	multusType               = "multus"
	networkName              = "some-net"
	vnicProfileName          = "some-vnic-profile-name"
	srcNetMappingName        = networkName + "/" + vnicProfileName
	vnicProfileID            = "some-vnic-profile-id"
	wrongNetworkID           = "some-net-bad-id"
	wrongNetworkName         = "some-net-bad"
	wrongVnicProfileName     = "some-vnic-bad"
	wrongSrcNetMappingName   = wrongNetworkName + "/" + wrongVnicProfileName
	invalidSrcNetMappingName = "bad-name-without-slash"

	targetNetworkName      = "targetNetwork"
	targetNetworkNamespace = "targetNamespace"
	findNetAttachDefMock   func(string) (*netv1.NetworkAttachmentDefinition, error)

	namespace = "default"
)

var _ = Describe("Validating Network mapping", func() {
	validator := validators.NewNetworkMappingValidator(&mockNetAttachDefProvider{})
	BeforeEach(func() {
		findNetAttachDefMock = func(name string) (*netv1.NetworkAttachmentDefinition, error) {
			netAttachDef := netv1.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      targetNetworkName,
					Namespace: targetNetworkNamespace,
				},
			}
			return &netAttachDef, nil
		}
	})
	table.DescribeTable("should reject missing mapping for: ", func(nic *ovirtsdk.Nic, srcNetMappingName *string, vnicProfileID *string) {
		nics := []*ovirtsdk.Nic{
			nic,
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Type: &podType,
				Source: v2vv1alpha1.Source{
					Name: srcNetMappingName,
					ID:   vnicProfileID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: "targetNetwork",
				},
			},
		}

		failures := validator.ValidateNetworkMapping(nics, &mapping, namespace)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.NetworkMappingID))
	},
		table.Entry("Vnic profile network with no mapping", createNic(&networkName, &vnicProfileName, &vnicProfileID), nil, nil),
		table.Entry("Vnic profile network with ID mismatch", createNic(&networkName, &vnicProfileName, &vnicProfileID), nil, &wrongNetworkID),
		table.Entry("Vnic profile network with name mismatch", createNic(&networkName, &vnicProfileName, &vnicProfileID), &wrongSrcNetMappingName, nil),
		table.Entry("Vnic profile network with both name and ID wrong", createNic(&networkName, &vnicProfileName, &vnicProfileID), &wrongSrcNetMappingName, &wrongNetworkID),
		table.Entry("Source network mapping format is illegal", createNic(&networkName, &vnicProfileName, &vnicProfileID), &invalidSrcNetMappingName, nil),
	)
	table.DescribeTable("should accept mapping for: ", func(nic *ovirtsdk.Nic, networkName *string, networkID *string) {
		nics := []*ovirtsdk.Nic{
			nic,
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Type: &podType,
				Source: v2vv1alpha1.Source{
					ID:   networkID,
					Name: networkName,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: "targetNetwork",
				},
			},
		}

		failures := validator.ValidateNetworkMapping(nics, &mapping, namespace)

		Expect(failures).To(BeEmpty())
	},
		table.Entry("Vnic profile network with mapping with name", createNic(&networkName, &vnicProfileName, &vnicProfileID), &srcNetMappingName, nil),
		table.Entry("Vnic profile network with mapping with ID", createNic(&networkName, &vnicProfileName, &vnicProfileID), nil, &vnicProfileID),
		table.Entry("Vnic profile network with mapping with both name and ID",
			createNic(&networkName, &vnicProfileName, &vnicProfileID),
			&srcNetMappingName,
			&vnicProfileID,
		),
	)
	It("should accept mapping for no type", func() {
		nics := []*ovirtsdk.Nic{
			createNic(&networkName, &vnicProfileName, &vnicProfileID),
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Source: v2vv1alpha1.Source{
					ID: &vnicProfileID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: "targetNetwork",
				},
			},
		}

		failures := validator.ValidateNetworkMapping(nics, &mapping, namespace)

		Expect(failures).To(BeEmpty())
	})
	It("should accept mapping with non-existing, not-required network attachment definition", func() {
		nics := []*ovirtsdk.Nic{
			createNic(&networkName, &vnicProfileName, &vnicProfileID),
		}
		otherNetwork := "other-net"

		mapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Source: v2vv1alpha1.Source{
					ID: &vnicProfileID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetNetworkName,
				},
				Type: &multusType,
			},
			{
				Source: v2vv1alpha1.Source{
					ID: &otherNetwork,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: wrongSrcNetMappingName,
				},
				Type: &multusType,
			},
		}

		findNetAttachDefMock = func(name string) (*netv1.NetworkAttachmentDefinition, error) {
			if name == targetNetworkName {
				netAttachDef := netv1.NetworkAttachmentDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name:      targetNetworkName,
						Namespace: targetNetworkNamespace,
					},
				}
				return &netAttachDef, nil
			}
			return nil, fmt.Errorf("Not found: %s", name)
		}

		failures := validator.ValidateNetworkMapping(nics, &mapping, namespace)

		Expect(failures).To(BeEmpty())
	})
	It("should accept mapping for multus type", func() {
		nics := []*ovirtsdk.Nic{
			createNic(&networkName, &vnicProfileName, &vnicProfileID),
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Type: &multusType,
				Source: v2vv1alpha1.Source{
					ID: &vnicProfileID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name:      targetNetworkName,
					Namespace: &targetNetworkNamespace,
				},
			},
		}

		failures := validator.ValidateNetworkMapping(nics, &mapping, namespace)

		Expect(failures).To(BeEmpty())
	})
	It("should reject mapping for multus type for retrieval error", func() {
		nics := []*ovirtsdk.Nic{
			createNic(&networkName, &vnicProfileName, &vnicProfileID),
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Type: &multusType,
				Source: v2vv1alpha1.Source{
					ID: &vnicProfileID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name:      targetNetworkName,
					Namespace: &targetNetworkNamespace,
				},
			},
		}

		findNetAttachDefMock = func(name string) (*netv1.NetworkAttachmentDefinition, error) {
			return nil, fmt.Errorf("boom")
		}

		failures := validator.ValidateNetworkMapping(nics, &mapping, namespace)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.NetworkTargetID))
	})
	It("should reject genie type", func() {
		nics := []*ovirtsdk.Nic{
			createNic(&networkName, &vnicProfileName, &vnicProfileID),
		}
		genieType := "genie"
		mapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Type: &genieType,
				Source: v2vv1alpha1.Source{
					ID:   &vnicProfileID,
					Name: &srcNetMappingName,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: "targetNetwork",
				},
			},
		}

		failures := validator.ValidateNetworkMapping(nics, &mapping, namespace)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.NetworkTypeID))
	})
	It("should reject nil type and target namespace present", func() {
		nics := []*ovirtsdk.Nic{
			createNic(&networkName, &vnicProfileName, &vnicProfileID),
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Type: nil,
				Source: v2vv1alpha1.Source{
					ID:   &vnicProfileID,
					Name: &srcNetMappingName,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name:      "targetNetwork",
					Namespace: &namespace,
				},
			},
		}

		failures := validator.ValidateNetworkMapping(nics, &mapping, namespace)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.NetworkTypeID))
	})
	It("should reject nil mapping", func() {
		nics := []*ovirtsdk.Nic{
			createNic(&networkName, &vnicProfileName, &vnicProfileID),
		}

		failures := validator.ValidateNetworkMapping(nics, nil, namespace)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.NetworkMappingID))
	})
	It("should accept nil mapping and one nic with no vnic profile", func() {
		nic := ovirtsdk.Nic{}
		nics := []*ovirtsdk.Nic{
			&nic,
		}

		failures := validator.ValidateNetworkMapping(nics, nil, namespace)

		Expect(failures).To(BeEmpty())
	})
	It("should reject mapping of two nics of the same vnic profile to a pod network", func() {
		nics := []*ovirtsdk.Nic{
			createNic(&networkName, &vnicProfileName, &vnicProfileID),
			createNic(&networkName, &vnicProfileName, &vnicProfileID),
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Source: v2vv1alpha1.Source{
					ID: &vnicProfileID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: "targetNetwork",
				},
				Type: &podType,
			},
		}

		failures := validator.ValidateNetworkMapping(nics, &mapping, namespace)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.NetworkMultiplePodTargetsID))
	})
	It("should reject mapping of two nics of different vnic profiles to a pod network", func() {
		networkName2 := "net-2"
		vnicProfileName2 := "vnic-2"
		vnicProfileID2 := "vnic-2-id"
		nics := []*ovirtsdk.Nic{
			createNic(&networkName, &vnicProfileName, &vnicProfileID),
			createNic(&networkName2, &vnicProfileName2, &vnicProfileID2),
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Source: v2vv1alpha1.Source{
					ID: &vnicProfileID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: "targetNetwork",
				},
				Type: &podType,
			},
			{
				Source: v2vv1alpha1.Source{
					ID: &vnicProfileID2,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: "targetNetwork2",
				},
				Type: &podType,
			},
		}

		failures := validator.ValidateNetworkMapping(nics, &mapping, namespace)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.NetworkMultiplePodTargetsID))
	})
})

func createNic(networkName *string, vnicProfileName *string, vnicProfileID *string) *ovirtsdk.Nic {
	nic := ovirtsdk.Nic{}
	network := ovirtsdk.Network{}
	if networkName != nil {
		network.SetName(*networkName)
	}
	profile := ovirtsdk.VnicProfile{}
	if vnicProfileID != nil {
		profile.SetId(*vnicProfileID)
	}
	if vnicProfileName != nil {
		profile.SetName(*vnicProfileName)
	}
	profile.SetNetwork(&network)
	nic.SetVnicProfile(&profile)
	return &nic
}

type mockNetAttachDefProvider struct{}

func (m *mockNetAttachDefProvider) Find(name string, _ string) (*netv1.NetworkAttachmentDefinition, error) {
	return findNetAttachDefMock(name)
}
