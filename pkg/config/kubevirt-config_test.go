package config_test

import (
	"github.com/kubevirt/vm-import-operator/pkg/config"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Live migration in KubeVirt config ", func() {
	table.DescribeTable("should be enabled for: ", func(featureGates string) {
		cfg := config.KubeVirtConfig{
			FeatureGates: featureGates,
		}

		enabled := cfg.LiveMigrationEnabled()

		Expect(enabled).To(BeTrue())
	},
		table.Entry("only LiveMigration", "LiveMigration"),
		table.Entry("LiveMigration among others", "Foo,LiveMigration,Bar"),
	)

	table.DescribeTable("should be disabled for: ", func(featureGates string) {
		cfg := config.KubeVirtConfig{
			FeatureGates: featureGates,
		}

		enabled := cfg.LiveMigrationEnabled()

		Expect(enabled).To(BeFalse())
	},
		table.Entry("empty feature gates", ""),
		table.Entry("feature gates other than LiveMigration", "Foo,Bar"),
	)

	It("should create KubeVirt config", func() {
		featureGates := "LiveMigration"
		configMap := corev1.ConfigMap{
			Data: map[string]string{"feature-gates": featureGates},
		}
		cfg := config.NewKubeVirtConfig(configMap)

		Expect(cfg.FeatureGates).To(BeEquivalentTo(featureGates))
		Expect(cfg.LiveMigrationEnabled()).To(BeTrue())
		Expect(cfg.ConfigMap()).To(BeEquivalentTo(configMap))
	})
})

var _ = Describe("KubeVirt config creator", func() {
	featureGates := "LiveMigration"
	configMap := corev1.ConfigMap{
		Data: map[string]string{"feature-gates": featureGates},
	}
	cfg := config.NewKubeVirtConfig(configMap)

	It("should create config with given config map", func() {
		Expect(cfg.ConfigMap()).To(BeEquivalentTo(configMap))
	})

	It("should create config with feature gates", func() {
		Expect(cfg.FeatureGates).To(BeEquivalentTo(featureGates))
	})

	It("should create config with LiveMigration enabled", func() {
		Expect(cfg.LiveMigrationEnabled()).To(BeTrue())
	})
})
