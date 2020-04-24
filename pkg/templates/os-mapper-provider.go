package templates

import (
	"context"

	yaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clientutil "kubevirt.io/client-go/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	osConfigMapName    = "vmimport-os-mapper"
	guestOsToCommonKey = "guestos2common"
	osInfoToCommonKey  = "osinfo2common"
)

// OSMaps is responsible for getting the operating systems maps which contain mapping of GuestOS to common templates
// and mapping of osinfo to common templates
type OSMaps struct {
	Client client.Client
}

// OSMapProvider is responsible for getting the operating systems maps
type OSMapProvider interface {
	GetOSMaps() (map[string]string, map[string]string, error)
}

// NewOSMapProvider creates new OSMapProvider
func NewOSMapProvider(client client.Client) *OSMaps {
	return &OSMaps{
		Client: client,
	}
}

// GetOSMaps retrieve the OS mapping config map
func (o *OSMaps) GetOSMaps() (map[string]string, map[string]string, error) {
	osConfigMap := &corev1.ConfigMap{}
	kubevirtNamespace, _ := clientutil.GetNamespace()
	err := o.Client.Get(
		context.TODO(),
		types.NamespacedName{Name: osConfigMapName, Namespace: kubevirtNamespace},
		osConfigMap,
	)
	if err != nil {
		return nil, nil, err
	}
	guestOsToCommon := make(map[string]string)
	err = yaml.Unmarshal([]byte(osConfigMap.Data[guestOsToCommonKey]), &guestOsToCommon)
	if err != nil {
		return nil, nil, err
	}
	osInfoToCommon := make(map[string]string)
	err = yaml.Unmarshal([]byte(osConfigMap.Data[osInfoToCommonKey]), &osInfoToCommon)
	if err != nil {
		return nil, nil, err
	}
	return guestOsToCommon, osInfoToCommon, nil
}
