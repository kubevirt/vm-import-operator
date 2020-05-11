package os_test

import (
	"context"
	"fmt"
	"os"

	osmap "github.com/kubevirt/vm-import-operator/pkg/os"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	get func(context.Context, client.ObjectKey, runtime.Object) error
)

var _ = Describe("OS Map Provider ", func() {
	var (
		mClient  *mockClient
		osMapper *osmap.OSMaps
	)

	BeforeEach(func() {
		mClient = &mockClient{}
		osMapper = osmap.NewOSMapProvider(mClient)
	})

	Describe("Core OS Map only", func() {
		It("should load OS map", func() {
			guestOsToCommon, osInfoToCommon, err := osMapper.GetOSMaps()

			Expect(err).ToNot(HaveOccurred())
			Expect(guestOsToCommon).NotTo(BeEmpty())
			Expect(osInfoToCommon).NotTo(BeEmpty())
		})
	})

	Describe("User OS Map env vars set with empty value", func() {
		BeforeEach(func() {
			os.Setenv(osmap.OsConfigMapName, "")
			os.Setenv(osmap.OsConfigMapNamespace, "")
		})

		It("should load OS map", func() {
			guestOsToCommon, osInfoToCommon, err := osMapper.GetOSMaps()

			Expect(err).ToNot(HaveOccurred())
			Expect(guestOsToCommon).NotTo(BeEmpty())
			Expect(osInfoToCommon).NotTo(BeEmpty())
		})
	})

	Describe("User OS Map env vars set with non-empty value", func() {
		BeforeEach(func() {
			os.Setenv(osmap.OsConfigMapName, "osmap-name")
			os.Setenv(osmap.OsConfigMapNamespace, "osmap-namespace")
		})

		It("should fail when user OS map does not exist", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				return fmt.Errorf("Not found")
			}

			guestOsToCommon, osInfoToCommon, err := osMapper.GetOSMaps()

			Expect(err).To(HaveOccurred())
			// core values should be loaded prior to loading user OS map data
			Expect(guestOsToCommon).NotTo(BeEmpty())
			Expect(osInfoToCommon).NotTo(BeEmpty())
		})

		It("should prefer user OS map values", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				obj.(*corev1.ConfigMap).Data = make(map[string]string)
				obj.(*corev1.ConfigMap).Data[osmap.GuestOsToCommonKey] = `"Fedora": "myfedora"`
				obj.(*corev1.ConfigMap).Data[osmap.OsInfoToCommonKey] = `"rhel_6_9_plus_ppc64": "myos"`
				return nil
			}

			guestOsToCommon, osInfoToCommon, err := osMapper.GetOSMaps()

			Expect(err).NotTo(HaveOccurred())
			// core values should be loaded prior to loading user OS map data
			Expect(guestOsToCommon).NotTo(BeEmpty())
			Expect(osInfoToCommon).NotTo(BeEmpty())

			// verify user values overrides core values
			Expect(osInfoToCommon["rhel_6_9_plus_ppc64"]).To(Equal("myos"))
			Expect(guestOsToCommon["Fedora"]).To(Equal("myfedora"))

			// verify other core values remain intact
			Expect(len(osInfoToCommon)).To(BeNumerically(">", 2))
			Expect(osInfoToCommon["windows_8"]).To(Equal("win10"))

			Expect(len(guestOsToCommon)).To(BeNumerically(">", 2))
			Expect(guestOsToCommon["CentOS Linux"]).To(Equal("centos"))
		})

		It("should fail to parse user OS map values of Guest OS to Common", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				obj.(*corev1.ConfigMap).Data = make(map[string]string)
				obj.(*corev1.ConfigMap).Data[osmap.GuestOsToCommonKey] = `"Fedora" = "myfedora"`
				return nil
			}

			_, _, err := osMapper.GetOSMaps()

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to parse user OS config-map"))
		})

		It("should fail to parse user OS map values of OS Info to Common", func() {
			get = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				obj.(*corev1.ConfigMap).Data = make(map[string]string)
				obj.(*corev1.ConfigMap).Data[osmap.OsInfoToCommonKey] = `"rhel_6_9_plus_ppc64" = "myos"`
				return nil
			}

			_, _, err := osMapper.GetOSMaps()

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to parse user OS config-map"))
		})
	})
})

type mockClient struct{}

// Create implements client.Client
func (c *mockClient) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	return nil
}

// Update implements client.Client
func (c *mockClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	return nil
}

// Delete implements client.Client
func (c *mockClient) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
	return nil
}

// DeleteAllOf implements client.Client
func (c *mockClient) DeleteAllOf(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
	return nil
}

// Patch implements client.Client
func (c *mockClient) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
}

// Get implements client.Client
func (c *mockClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return get(ctx, key, obj)
}

// List implements client.Client
func (c *mockClient) List(ctx context.Context, objectList runtime.Object, opts ...client.ListOption) error {
	return nil
}

// Status implements client.StatusClient
func (c *mockClient) Status() client.StatusWriter {
	return nil
}
