package os

import (
	"context"
	"fmt"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	yaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// GuestOsToCommonKey represents the element key in OS map of guest OS to common template mapping
	GuestOsToCommonKey = "guestos2common"

	// OsInfoToCommonKey represents the element key in OS map of OS type to common template mapping
	OsInfoToCommonKey = "osinfo2common"
)

var log = logf.Log.WithName("os-map-provider")

// OSMaps is responsible for getting the operating systems maps which contain mapping of GuestOS to common templates
// and mapping of osinfo to common templates
type OSMaps struct {
	client               client.Client
	osConfigMapName      string
	osConfigMapNamespace string
}

// OSMapProvider is responsible for getting the operating systems maps
type OSMapProvider interface {
	GetOSMaps() (map[string]string, map[string]string, error)
}

// NewOSMapProvider creates new OSMapProvider
func NewOSMapProvider(client client.Client, osConfigMapName string, osConfigMapNamespace string) *OSMaps {
	return &OSMaps{
		client:               client,
		osConfigMapName:      osConfigMapName,
		osConfigMapNamespace: osConfigMapNamespace,
	}
}

// GetOSMaps retrieve the OS mapping config map
func (o *OSMaps) GetOSMaps() (map[string]string, map[string]string, error) {
	guestOsToCommon := initGuestOsToCommon()
	osInfoToCommon := initOsInfoToCommon()
	err := o.updateOsMapsByUserMaps(guestOsToCommon, osInfoToCommon)
	return guestOsToCommon, osInfoToCommon, err
}

func (o *OSMaps) updateOsMapsByUserMaps(guestOsToCommon map[string]string, osInfoToCommon map[string]string) error {
	osConfigMap := &corev1.ConfigMap{}
	if o.osConfigMapName == "" && o.osConfigMapNamespace == "" {
		return nil
	}
	log.Info("Using user-provided OS maps", "name", o.osConfigMapName, "namespace", o.osConfigMapNamespace)
	err := o.client.Get(
		context.TODO(),
		types.NamespacedName{Name: o.osConfigMapName, Namespace: o.osConfigMapNamespace},
		osConfigMap,
	)
	if err != nil {
		return fmt.Errorf(
			"Failed to read user OS config-map [%s/%s] due to: [%v]",
			o.osConfigMapNamespace,
			o.osConfigMapName,
			err,
		)
	}

	err = yaml.Unmarshal([]byte(osConfigMap.Data[GuestOsToCommonKey]), &guestOsToCommon)
	if err != nil {
		return fmt.Errorf(
			"Failed to parse user OS config-map [%s/%s] element %s due to: [%v]",
			o.osConfigMapNamespace,
			o.osConfigMapName,
			GuestOsToCommonKey,
			err,
		)
	}

	err = yaml.Unmarshal([]byte(osConfigMap.Data[OsInfoToCommonKey]), &osInfoToCommon)
	if err != nil {
		return fmt.Errorf(
			"Failed to parse user OS config-map [%s/%s] element %s due to: [%v]",
			o.osConfigMapNamespace,
			o.osConfigMapName,
			OsInfoToCommonKey,
			err,
		)
	}

	return nil
}

func initGuestOsToCommon() map[string]string {
	return map[string]string{
		"Red Hat Enterprise Linux Server": "rhel",
		"Red Hat Enterprise Linux":        "rhel",
		"CentOS Linux":                    "centos",
		"Fedora":                          "fedora",
		"Ubuntu":                          "ubuntu",
		"openSUSE":                        "opensuse",
	}
}

func initOsInfoToCommon() map[string]string {
	return map[string]string{
		"rhel_6_9_plus_ppc64": "rhel6.9",
		"rhel_6_ppc64":        "rhel6.9",
		"rhel_6":              "rhel6.9",
		"rhel_6x64":           "rhel6.9",
		"rhel_7_ppc64":        "rhel7.7",
		"rhel_7_s390x":        "rhel7.7",
		"rhel_7x64":           "rhel7.7",
		"rhel_8x64":           "rhel8.1",
		"sles_11_ppc64":       "opensuse15.0",
		"sles_11":             "opensuse15.0",
		"sles_12_s390x":       "opensuse15.0",
		"ubuntu_12_04":        "ubuntu18.04",
		"ubuntu_12_10":        "ubuntu18.04",
		"ubuntu_13_04":        "ubuntu18.04",
		"ubuntu_13_10":        "ubuntu18.04",
		"ubuntu_14_04_ppc64":  "ubuntu18.04",
		"ubuntu_14_04":        "ubuntu18.04",
		"ubuntu_16_04_s390x":  "ubuntu18.04",
		"windows_10":          "win10",
		"windows_10x64":       "win10",
		"windows_2003":        "win10",
		"windows_2003x64":     "win10",
		"windows_2008R2x64":   "win2k8",
		"windows_2008":        "win2k8",
		"windows_2008x64":     "win2k8",
		"windows_2012R2x64":   "win2k12r2",
		"windows_2012x64":     "win2k12r2",
		"windows_2016x64":     "win2k16",
		"windows_2019x64":     "win2k19",
		"windows_7":           "win10",
		"windows_7x64":        "win10",
		"windows_8":           "win10",
		"windows_8x64":        "win10",
		"windows_xp":          "win10",
		// vmware guest identifiers
		"centos6_64Guest":       "centos6.10",
		"centos64Guest":         "centos5.11",
		"centos6Guest":          "centos6.10",
		"centos7_64Guest":       "centos7.0",
		"centos7Guest":          "centos7.0",
		"centos8_64Guest":       "centos8",
		"centos8Guest":          "centos8",
		"debian4_64Guest":       "debian4",
		"debian4Guest":          "debian4",
		"debian5_64Guest":       "debian5",
		"debian5Guest":          "debian5",
		"debian6_64Guest":       "debian6",
		"debian6Guest":          "debian6",
		"debian7_64Guest":       "debian7",
		"debian7Guest":          "debian7",
		"debian8_64Guest":       "debian8",
		"debian8Guest":          "debian8",
		"debian9_64Guest":       "debian9",
		"debian9Guest":          "debian9",
		"debian10_64Guest":      "debian10",
		"debian10Guest":         "debian10",
		"fedora64Guest":         "fedora-unknown",
		"fedoraGuest":           "fedora-unknown",
		"genericLinuxGuest":     "linux",
		"rhel2Guest":            "rhel2.1.7",
		"rhel3_64Guest":         "rhel3.9",
		"rhel3Guest":            "rhel3.9",
		"rhel4_64Guest":         "rhel4.9",
		"rhel4Guest":            "rhel4.9",
		"rhel5_64Guest":         "rhel5.11",
		"rhel5Guest":            "rhel5.11",
		"rhel6_64Guest":         "rhel6.9",
		"rhel6Guest":            "rhel6.9",
		"rhel7_64Guest":         "rhel7.7",
		"rhel7Guest":            "rhel7.7",
		"rhel8_64Guest":         "rhel8.1",
		"ubuntu64Guest":         "ubuntu18.04",
		"ubuntuGuest":           "ubuntu18.04",
		"win2000AdvServGuest":   "win2k",
		"win2000ProGuest":       "win2k",
		"win2000ServGuest":      "win2k",
		"windows7Guest":         "win7",
		"windows7Server64Guest": "win2k8r2",
		"windows8_64Guest":      "win8",
		"windows8Guest":         "win8",
		"windows8Server64Guest": "win2k12r2",
		"windows9_64Guest":      "win10",
		"windows9Guest":         "win10",
		"windows9Server64Guest": "win2k19",
	}
}
