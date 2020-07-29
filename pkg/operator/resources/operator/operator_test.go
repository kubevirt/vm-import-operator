package operator_test

import (
	"fmt"

	v2vv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	vmioperator "github.com/kubevirt/vm-import-operator/pkg/operator/resources/operator"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/RHsyseng/operator-utils/pkg/validation"
	"github.com/ghodss/yaml"

	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type createCrd func() *extv1.CustomResourceDefinition

type crdToCreator struct {
	resource interface{}
	creator  createCrd
}

var crdTypeMap = map[string]crdToCreator{
	"vmimport-crd": {
		&v2vv1.VirtualMachineImport{},
		vmioperator.CreateVMImport,
	},
	"resource-mapping-crd": {
		&v2vv1.ResourceMapping{},
		vmioperator.CreateResourceMapping,
	},
	"vmimportconfig-crd": {
		&v2vv1.VMImportConfig{},
		vmioperator.CreateVMImportConfig,
	},
}

var _ = Describe("Operator resource test", func() {
	It("Test CRD schemas", func() {
		for crdFileName, crdCreatorObj := range crdTypeMap {
			schema := getSchema(crdCreatorObj.creator)
			missingEntries := schema.GetMissingEntries(crdCreatorObj.resource)
			for _, missing := range missingEntries {
				if strings.HasPrefix(missing.Path, "/status") {
					//Not using subresources, so status is not expected to appear in CRD
				} else {
					msg := "Discrepancy between CRD and Struct Missing or incorrect schema validation at [%v], expected type [%v] in CRD file [%v]"
					Fail(fmt.Sprintf(msg, missing.Path, missing.Type, crdFileName))
				}
			}
		}
	})

	It("Test valid VMImportConfig custom resources", func() {
		crFileName := []byte(`{
		  "apiVersion":"v2v.kubevirt.io/v1beta1",
		  "kind":"VMImportConfig",
		  "metadata": {
		    name: vm-import-operator-config
		  },
		  "spec": {
		    "imagePullPolicy":"IfNotPresent"
		  }
		}`)
		crFileName, err := yaml.JSONToYAML(crFileName)
		Expect(err).ToNot(HaveOccurred())

		schema := getSchema(vmioperator.CreateVMImportConfig)

		var input map[string]interface{}
		err = yaml.Unmarshal([]byte(crFileName), &input)
		Expect(err).ToNot(HaveOccurred())
		err = schema.Validate(input)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Test invalid ResourceMapping custom resource", func() {
		crFileName := []byte(`{
		  "apiVersion":"v2v.kubevirt.io/v1beta1",
		  "kind":"ResourceMapping",
		  "metadata": {
		    name: resource-mapping
		  },
		  "spec": {
			  "ovirt": {
				"networkMappings": [
				  { "source": {}}
				]
			  }
		  }
		}`)
		crFileName, err := yaml.JSONToYAML(crFileName)
		Expect(err).ToNot(HaveOccurred())

		schema := getSchema(vmioperator.CreateResourceMapping)

		var input map[string]interface{}
		err = yaml.Unmarshal([]byte(crFileName), &input)
		Expect(err).ToNot(HaveOccurred())
		err = schema.Validate(input)
		Expect(err).To(HaveOccurred())
	})

	It("Test invalid VMImport custom resource", func() {
		crFileName := []byte(`{
		  "apiVersion":"v2v.kubevirt.io/v1beta1",
		  "kind":"VirtualMachineImport",
		  "metadata": {
		    name: vm-import
		  },
		  "spec": {
		  }
		}`)
		crFileName, err := yaml.JSONToYAML(crFileName)
		Expect(err).ToNot(HaveOccurred())

		schema := getSchema(vmioperator.CreateVMImport)

		var input map[string]interface{}
		err = yaml.Unmarshal([]byte(crFileName), &input)
		Expect(err).ToNot(HaveOccurred())
		err = schema.Validate(input)
		Expect(err).To(HaveOccurred())
	})
})

func getSchema(crdCreator createCrd) validation.Schema {
	crdFiles, err := yaml.Marshal(crdCreator())
	Expect(err).ToNot(HaveOccurred())
	yamlString := string(crdFiles)
	Expect(err).ToNot(HaveOccurred())
	schema, err := validation.NewVersioned([]byte(yamlString), "v1beta1")
	Expect(err).ToNot(HaveOccurred())
	return schema
}
