package validators

import (
	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	ovirtsdk "github.com/ovirt/go-ovirt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"fmt"
)

var (
	podType          = "pod"
	multusType       = "multus"
	networkName      = "some-net"
	networkID        = "some-net-id"
	wrongNetworkID   = "some-net-bad-id"
	wrongNetworkName = "some-net-bad"

	targetNetworkName      = "targetNetwork"
	targetNetworkNamespace = "targetNamespace"
	findNetAttachDefMock   func() (*netv1.NetworkAttachmentDefinition, error)

	namespace = "default"
)

var _ = Describe("Validating Network mapping", func() {
	validator := NetworkMappingValidator{
		provider: &mockNetAttachDefProvider{},
	}
	BeforeEach(func() {
		findNetAttachDefMock = func() (*netv1.NetworkAttachmentDefinition, error) {
			netAttachDef := netv1.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      targetNetworkName,
					Namespace: targetNetworkNamespace,
				},
			}
			return &netAttachDef, nil
		}
	})
	table.DescribeTable("should reject missing mapping for: ", func(nic *ovirtsdk.Nic, networkName *string, networkID *string) {
		nics := []*ovirtsdk.Nic{
			nic,
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			v2vv1alpha1.ResourceMappingItem{
				Type: &podType,
				Source: v2vv1alpha1.Source{
					Name: networkName,
					ID:   networkID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: "targetNetwork",
				},
			},
		}

		failures := validator.ValidateNetworkMapping(nics, &mapping, namespace)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(NetworkMappingID))
	},
		table.Entry("Vnic profile network with no mapping", createNic(&networkName, &networkID), nil, nil),
		table.Entry("Vnic profile network with ID mismatch", createNic(&networkName, &networkID), nil, &wrongNetworkID),
		table.Entry("Vnic profile network with name mismatch", createNic(&networkName, &networkID), &wrongNetworkName, nil),
		table.Entry("Vnic profile network with both name and ID wrong", createNic(&networkName, &networkID), &wrongNetworkName, &wrongNetworkID),
	)
	table.DescribeTable("should accept mapping for: ", func(nic *ovirtsdk.Nic, networkName *string, networkID *string) {
		nics := []*ovirtsdk.Nic{
			nic,
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			v2vv1alpha1.ResourceMappingItem{
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
		table.Entry("Vnic profile network with mapping with name", createNic(&networkName, &networkID), &networkName, nil),
		table.Entry("Vnic profile network with mapping with ID", createNic(&networkName, &networkID), nil, &networkID),
		table.Entry("Vnic profile network with mapping with both name and ID", createNic(&networkName, &networkID), &networkName, &networkID),
	)
	It("should accept mapping for no type", func() {
		nics := []*ovirtsdk.Nic{
			createNic(&networkName, &networkID),
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			v2vv1alpha1.ResourceMappingItem{
				Source: v2vv1alpha1.Source{
					ID: &networkID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: "targetNetwork",
				},
			},
		}

		failures := validator.ValidateNetworkMapping(nics, &mapping, namespace)

		Expect(failures).To(BeEmpty())
	})
	It("should accept mapping for multus type", func() {
		nics := []*ovirtsdk.Nic{
			createNic(&networkName, &networkID),
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			v2vv1alpha1.ResourceMappingItem{
				Type: &multusType,
				Source: v2vv1alpha1.Source{
					ID: &networkID,
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
			createNic(&networkName, &networkID),
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			v2vv1alpha1.ResourceMappingItem{
				Type: &multusType,
				Source: v2vv1alpha1.Source{
					ID: &networkID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name:      targetNetworkName,
					Namespace: &targetNetworkNamespace,
				},
			},
		}

		findNetAttachDefMock = func() (*netv1.NetworkAttachmentDefinition, error) {
			return nil, fmt.Errorf("boom")
		}

		failures := validator.ValidateNetworkMapping(nics, &mapping, namespace)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(NetworkTargetID))
	})
	It("should reject genie type", func() {
		nics := []*ovirtsdk.Nic{
			createNic(&networkName, &networkID),
		}
		genieType := "genie"
		mapping := []v2vv1alpha1.ResourceMappingItem{
			v2vv1alpha1.ResourceMappingItem{
				Type: &genieType,
				Source: v2vv1alpha1.Source{
					ID:   &networkID,
					Name: &networkName,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: "targetNetwork",
				},
			},
		}

		failures := validator.ValidateNetworkMapping(nics, &mapping, namespace)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(NetworkTypeID))
	})
	It("should reject nil mapping", func() {
		nics := []*ovirtsdk.Nic{
			createNic(&networkName, &networkID),
		}

		failures := validator.ValidateNetworkMapping(nics, nil, namespace)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(NetworkMappingID))
	})
	It("should accept nil mapping and one nic with no vnic profile", func() {
		nic := ovirtsdk.Nic{}
		nics := []*ovirtsdk.Nic{
			&nic,
		}

		failures := validator.ValidateNetworkMapping(nics, nil, namespace)

		Expect(failures).To(BeEmpty())
	})
})

func createNic(networkName *string, networkID *string) *ovirtsdk.Nic {
	nic := ovirtsdk.Nic{}
	network := ovirtsdk.Network{}
	if networkID != nil {
		network.SetId(*networkID)
	}
	if networkName != nil {
		network.SetName(*networkName)
	}
	profile := ovirtsdk.VnicProfile{}
	profile.SetNetwork(&network)
	nic.SetVnicProfile(&profile)
	return &nic
}

type mockNetAttachDefProvider struct{}

func (m *mockNetAttachDefProvider) Find(name string, namespace string) (*netv1.NetworkAttachmentDefinition, error) {
	return findNetAttachDefMock()
}
