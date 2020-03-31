package validators_test

import (
	"fmt"

	v2vv1alpha1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	"github.com/kubevirt/vm-import-operator/pkg/providers/ovirt/validation/validators"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	ovirtsdk "github.com/ovirt/go-ovirt"
	v1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	findStorageClassMock func() (*v1.StorageClass, error)
	targetStorageClass   = "myStorageClass"

	domainName      = "domain"
	domainID        = "domain-id"
	wrongDomainName = "wrong-domain"
	wrongDomainID   = "wrong-domain-id"
)

var _ = Describe("Validating Storage mapping", func() {
	validator := validators.NewStorageMappingValidator(&mockStorageClassProvider{})
	BeforeEach(func() {
		findStorageClassMock = func() (*v1.StorageClass, error) {
			sc := v1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: targetNetworkName,
				},
			}
			return &sc, nil
		}
	})
	table.DescribeTable("should reject missing mapping for: ", func(sd *ovirtsdk.StorageDomain, domainName *string, domainID *string) {
		da := createDiskAttachment(sd)
		das := []*ovirtsdk.DiskAttachment{
			da,
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			v2vv1alpha1.ResourceMappingItem{
				Source: v2vv1alpha1.Source{
					Name: domainName,
					ID:   domainID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
		}

		failures := validator.ValidateStorageMapping(das, &mapping)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.StorageMappingID))
	},
		table.Entry("No mapping", createDomain(&domainName, &domainID), nil, nil),
		table.Entry("ID mismatch", createDomain(&domainName, &domainID), nil, &wrongDomainID),
		table.Entry("name mismatch", createDomain(&domainName, &domainID), &wrongDomainName, nil),
		table.Entry("both name and ID wrong", createDomain(&domainName, &domainID), &wrongDomainName, &wrongDomainID),
	)
	table.DescribeTable("should accept mapping for: ", func(sd *ovirtsdk.StorageDomain, domainName *string, domainID *string) {
		da := createDiskAttachment(sd)
		das := []*ovirtsdk.DiskAttachment{
			da,
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			v2vv1alpha1.ResourceMappingItem{
				Source: v2vv1alpha1.Source{
					Name: domainName,
					ID:   domainID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
		}

		failures := validator.ValidateStorageMapping(das, &mapping)

		Expect(failures).To(BeEmpty())
	},
		table.Entry("mapping with ID", createDomain(&domainName, &domainID), nil, &domainID),
		table.Entry("mapping with name", createDomain(&domainName, &domainID), &domainName, nil),
		table.Entry("mapping with both name and ID", createDomain(&domainName, &domainID), &domainName, &domainID),
	)
	It("should reject mapping for storage class retrieval error", func() {
		sd := createDomain(&domainName, &domainID)
		da := createDiskAttachment(sd)
		das := []*ovirtsdk.DiskAttachment{
			da,
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			v2vv1alpha1.ResourceMappingItem{
				Source: v2vv1alpha1.Source{
					Name: &domainName,
					ID:   &domainID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
		}

		findStorageClassMock = func() (*v1.StorageClass, error) {
			return nil, fmt.Errorf("boom")
		}

		failures := validator.ValidateStorageMapping(das, &mapping)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.StorageTargetID))
	})
	It("should reject nil mapping", func() {
		sd := createDomain(&domainName, &domainID)
		da := createDiskAttachment(sd)
		das := []*ovirtsdk.DiskAttachment{
			da,
		}

		failures := validator.ValidateStorageMapping(das, nil)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.StorageMappingID))
	})
})

func createDiskAttachment(sd *ovirtsdk.StorageDomain) *ovirtsdk.DiskAttachment {
	disk := ovirtsdk.Disk{}
	disk.SetStorageDomain(sd)
	da := ovirtsdk.DiskAttachment{}
	da.SetDisk(&disk)
	return &da
}
func createDomain(name *string, id *string) *ovirtsdk.StorageDomain {
	sd := ovirtsdk.StorageDomain{}
	if name != nil {
		sd.SetName(*name)
	}
	if id != nil {
		sd.SetId(*id)
	}
	return &sd
}

type mockStorageClassProvider struct{}

func (m *mockStorageClassProvider) Find(name string) (*v1.StorageClass, error) {
	return findStorageClassMock()
}
