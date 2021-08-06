package kubevirt_test

import (
	"github.com/kubevirt/vm-import-operator/pkg/config/kubevirt"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Import without templates in KubeVirt config ", func() {
	table.DescribeTable("should be enabled for: ", func(featureGates string) {
		cfg := kubevirt.KubeVirtConfig{
			FeatureGates: featureGates,
		}

		enabled := cfg.ImportWithoutTemplateEnabled()

		Expect(enabled).To(BeTrue())
	},
		table.Entry("only ImportWithoutTemplate", "ImportWithoutTemplate"),
		table.Entry("ImportWithoutTemplate among others", "ImportWithoutTemplate,LiveMigration,Bar"),
	)

	table.DescribeTable("should be disabled for: ", func(featureGates string) {
		cfg := kubevirt.KubeVirtConfig{
			FeatureGates: featureGates,
		}

		enabled := cfg.ImportWithoutTemplateEnabled()

		Expect(enabled).To(BeFalse())
	},
		table.Entry("empty feature gates", ""),
		table.Entry("feature gates other than ImportWithoutTemplate", "Foo,Bar"),
	)
})

var _ = Describe("KubeVirt config creator", func() {
	featureGates := "ImportWithoutTemplate"
	configMap := corev1.ConfigMap{
		Data: map[string]string{"feature-gates": featureGates},
	}
	cfg := kubevirt.NewKubeVirtConfig(configMap)

	It("should create config with given config map", func() {
		Expect(cfg.ConfigMap).To(BeEquivalentTo(configMap))
	})

	It("should create config with feature gates", func() {
		Expect(cfg.FeatureGates).To(BeEquivalentTo(featureGates))
	})
})
