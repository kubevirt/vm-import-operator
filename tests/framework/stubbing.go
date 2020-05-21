package framework

import (
	"io/ioutil"
	"strings"

	"github.com/onsi/ginkgo"
)

func (f *Framework) LoadTemplate(fileName string, replacements map[string]string) string {
	xml := f.LoadFile(fileName)
	for key := range replacements {
		xml = strings.ReplaceAll(xml, key, replacements[key])
	}
	return xml
}

func (f *Framework) LoadFile(fileName string) string {
	content, err := ioutil.ReadFile("stubbing/" + fileName)
	if err != nil {
		ginkgo.Fail(err.Error())
	}
	return string(content)
}
