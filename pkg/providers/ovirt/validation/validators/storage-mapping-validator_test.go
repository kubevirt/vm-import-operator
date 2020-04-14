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
	findStorageClassMock func(name string) (*v1.StorageClass, error)
	targetStorageClass   = "myStorageClass"

	diskID        = "disk-id"
	diskName      = "disk-name"
	wrongDiskID   = "wrong-disk-id"
	wrongDiskName = "wrong-disk-name"

	domainName      = "domain"
	domainID        = "domain-id"
	wrongDomainName = "wrong-domain"
	wrongDomainID   = "wrong-domain-id"
)

var _ = Describe("Validating Storage mapping", func() {
	validator := validators.NewStorageMappingValidator(&mockStorageClassProvider{})
	BeforeEach(func() {
		findStorageClassMock = func(name string) (*v1.StorageClass, error) {
			sc := v1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: targetNetworkName,
				},
			}
			return &sc, nil
		}
	})
	table.DescribeTable("should ignore missing mapping for: ", func(sd *ovirtsdk.StorageDomain, domainName *string, domainID *string) {
		da := createDiskAttachment(sd)
		das := []*ovirtsdk.DiskAttachment{
			da,
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Source: v2vv1alpha1.Source{
					Name: domainName,
					ID:   domainID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
		}

		failures := validator.ValidateStorageMapping(das, &mapping, nil)

		Expect(failures).To(BeEmpty())
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
			{
				Source: v2vv1alpha1.Source{
					Name: domainName,
					ID:   domainID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
		}

		failures := validator.ValidateStorageMapping(das, &mapping, nil)

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
			{
				Source: v2vv1alpha1.Source{
					Name: &domainName,
					ID:   &domainID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
		}

		findStorageClassMock = func(name string) (*v1.StorageClass, error) {
			return nil, fmt.Errorf("boom")
		}

		failures := validator.ValidateStorageMapping(das, &mapping, nil)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.StorageTargetID))
	})
	It("should reject multiple mapping for storage class retrieval error", func() {
		otherDomainName := "other-domain"
		otherDomainID := "other-domain-id"
		das := []*ovirtsdk.DiskAttachment{
			createDiskAttachment(createDomain(&domainName, &domainID)),
			createDiskAttachment(createDomain(&otherDomainName, &otherDomainID)),
		}

		mapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Source: v2vv1alpha1.Source{
					Name: &domainName,
					ID:   &domainID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
			{
				Source: v2vv1alpha1.Source{
					Name: &otherDomainName,
					ID:   &otherDomainID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
		}

		findStorageClassMock = func(name string) (*v1.StorageClass, error) {
			return nil, fmt.Errorf("boom")
		}

		failures := validator.ValidateStorageMapping(das, &mapping, nil)

		Expect(failures).To(HaveLen(2))
		Expect(failures[0].ID).To(Equal(validators.StorageTargetID))
		Expect(failures[0].Message).To(ContainSubstring(domainID))
		Expect(failures[0].Message).To(ContainSubstring(domainName))
		Expect(failures[1].ID).To(Equal(validators.StorageTargetID))
		Expect(failures[1].Message).To(ContainSubstring(otherDomainID))
		Expect(failures[1].Message).To(ContainSubstring(otherDomainName))
	})
	It("should accept nil mapping", func() {
		sd := createDomain(&domainName, &domainID)
		da := createDiskAttachment(sd)
		das := []*ovirtsdk.DiskAttachment{
			da,
		}

		failures := validator.ValidateStorageMapping(das, nil, nil)

		Expect(failures).To(BeEmpty())
	})
})

var _ = Describe("Validating Disk mapping", func() {
	validator := validators.NewStorageMappingValidator(&mockStorageClassProvider{})
	BeforeEach(func() {
		findStorageClassMock = func(name string) (*v1.StorageClass, error) {
			sc := v1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: targetNetworkName,
				},
			}
			return &sc, nil
		}
	})
	table.DescribeTable("should accept missing mapping for: ", func(diskName *string, diskID *string) {
		da := createDiskAttachment(createDomain(&domainName, &domainID))
		das := []*ovirtsdk.DiskAttachment{
			da,
		}

		diskMapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Source: v2vv1alpha1.Source{
					Name: diskName,
					ID:   diskID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
		}

		failures := validator.ValidateStorageMapping(das, nil, &diskMapping)

		Expect(failures).To(BeEmpty())
	},
		table.Entry("No mapping", nil, nil),
		table.Entry("ID mismatch", nil, &wrongDiskID),
		table.Entry("name mismatch", &wrongDiskName, nil),
		table.Entry("both name and ID wrong", &wrongDiskName, &wrongDiskID),
	)
	table.DescribeTable("should accept mapping for: ", func(diskName *string, diskID *string) {
		da := createDiskAttachment(createDomain(&domainName, &domainID))
		das := []*ovirtsdk.DiskAttachment{
			da,
		}

		diskMapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Source: v2vv1alpha1.Source{
					Name: diskName,
					ID:   diskID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
		}

		failures := validator.ValidateStorageMapping(das, nil, &diskMapping)

		Expect(failures).To(BeEmpty())
	},
		table.Entry("mapping with ID", nil, &diskID),
		table.Entry("mapping with name", &diskName, nil),
		table.Entry("mapping with both name and ID", &diskName, &diskID),
	)
	It("should reject disk mapping for storage class retrieval error", func() {
		da := createDiskAttachment(createDomain(&domainName, &domainID))
		das := []*ovirtsdk.DiskAttachment{
			da,
		}

		diskMapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Source: v2vv1alpha1.Source{
					Name: &diskName,
					ID:   &diskID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
		}

		findStorageClassMock = func(name string) (*v1.StorageClass, error) {
			return nil, fmt.Errorf("boom")
		}

		failures := validator.ValidateStorageMapping(das, nil, &diskMapping)

		Expect(failures).To(HaveLen(1))
		Expect(failures[0].ID).To(Equal(validators.DiskTargetID))
	})
	It("should reject multiple mapping for storage class retrieval error", func() {
		otherDiskID := "disk-id"
		otherDiskName := "disk-name"
		das := []*ovirtsdk.DiskAttachment{
			createCustomDiskAttachment(createDomain(&domainName, &domainID), diskID, diskName),
			createCustomDiskAttachment(createDomain(&domainName, &domainID), otherDiskID, otherDiskName),
		}
		diskMapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Source: v2vv1alpha1.Source{
					Name: &diskName,
					ID:   &diskID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
			{
				Source: v2vv1alpha1.Source{
					Name: &otherDiskName,
					ID:   &otherDiskID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
		}

		findStorageClassMock = func(name string) (*v1.StorageClass, error) {
			return nil, fmt.Errorf("boom")
		}

		failures := validator.ValidateStorageMapping(das, nil, &diskMapping)

		Expect(failures).To(HaveLen(2))
		Expect(failures[0].ID).To(Equal(validators.DiskTargetID))
		Expect(failures[0].Message).To(ContainSubstring(diskID))
		Expect(failures[0].Message).To(ContainSubstring(diskName))
		Expect(failures[1].ID).To(Equal(validators.DiskTargetID))
		Expect(failures[1].Message).To(ContainSubstring(otherDiskID))
		Expect(failures[1].Message).To(ContainSubstring(otherDiskName))
	})
	It("should accept nil mapping", func() {
		da := createDiskAttachment(createDomain(&domainName, &domainID))
		das := []*ovirtsdk.DiskAttachment{
			da,
		}

		failures := validator.ValidateStorageMapping(das, &[]v2vv1alpha1.ResourceMappingItem{}, nil)

		Expect(failures).To(BeEmpty())
	})
	It("should accept mapping for storage class and disk mapping", func() {
		sd := createDomain(&domainName, &domainID)
		da := createDiskAttachment(sd)
		customID := "custom-disk-id"
		customName := "custom-disk-name"
		customSDID := "custom-domain-id"
		customSDName := "custom-domain-name"
		customTargetStorageClass := "customStorageClass"
		csd := createDomain(&customSDName, &customSDID)
		cda := createCustomDiskAttachment(csd, customID, customName)
		das := []*ovirtsdk.DiskAttachment{
			da,
			cda,
		}

		storageMapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Source: v2vv1alpha1.Source{
					Name: &domainName,
					ID:   &domainID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: targetStorageClass,
				},
			},
		}

		diskMapping := []v2vv1alpha1.ResourceMappingItem{
			{
				Source: v2vv1alpha1.Source{
					Name: &customName,
					ID:   &customID,
				},
				Target: v2vv1alpha1.ObjectIdentifier{
					Name: customTargetStorageClass,
				},
			},
		}

		findStorageClassMock = func(name string) (*v1.StorageClass, error) {
			if name == targetStorageClass {
				sc := v1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: targetStorageClass,
					},
				}
				return &sc, nil
			}
			if name == customTargetStorageClass {
				sc := v1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: customTargetStorageClass,
					},
				}
				return &sc, nil
			}
			return nil, fmt.Errorf("boom")
		}

		failures := validator.ValidateStorageMapping(das, &storageMapping, &diskMapping)

		Expect(failures).To(BeEmpty())
	})
})

func createDiskAttachment(sd *ovirtsdk.StorageDomain) *ovirtsdk.DiskAttachment {
	return createCustomDiskAttachment(sd, diskID, diskName)
}

func createCustomDiskAttachment(sd *ovirtsdk.StorageDomain, diskID string, diskName string) *ovirtsdk.DiskAttachment {
	disk := ovirtsdk.Disk{}
	disk.SetStorageDomain(sd)
	disk.SetId(diskID)
	disk.SetAlias(diskName)
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
	return findStorageClassMock(name)
}
