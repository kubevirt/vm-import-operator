package framework

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
	v1 "kubevirt.io/client-go/api/v1"
)

// AddLabelToAllNodes adds label with given value to all schedulable nodes
func (f *Framework) AddLabelToAllNodes(name string, value string) error {
	nodeList, err := f.GetAllSchedulableNodes()
	if err != nil {
		return err
	}
	for _, node := range nodeList.Items {
		newNode := node.DeepCopy()
		newNode.Labels[name] = value
		_, err = f.K8sClient.CoreV1().Nodes().Update(context.TODO(), newNode, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveLabelFromNodes removes label from all schedulable nodes
func (f *Framework) RemoveLabelFromNodes(name string) error {
	nodeList, err := f.GetAllSchedulableNodes()
	if err != nil {
		return err
	}
	for _, node := range nodeList.Items {
		newNode := node.DeepCopy()
		delete(newNode.Labels, name)
		_, err = f.K8sClient.CoreV1().Nodes().Update(context.TODO(), newNode, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

// GetAllSchedulableNodes retrieves all schedulable nodes
func (f *Framework) GetAllSchedulableNodes() (*k8sv1.NodeList, error) {
	return f.K8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: v1.NodeSchedulable + "=" + "true"})
}

// Tests in Multus suite are expecting a Linux bridge to be available on each node, with iptables allowing
// traffic to go through. This function creates a Daemon Set on the cluster (if not exists yet), this Daemon
// Set creates a linux bridge and configures the firewall. We use iptables-compat in order to work with
// both iptables and newer nftables.
// Based on https://github.com/kubevirt/kubevirt/blob/master/tests/vmi_multus_test.go
func (f *Framework) ConfigureNodeNetwork() error {
	// Fetching the kubevirt-operator image from the pod makes this independent from the installation method / image used
	pods, err := f.K8sClient.CoreV1().Pods(f.KubeVirtInstallNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "kubevirt.io=virt-operator"})
	if err != nil {
		return err
	}

	virtOperatorImage := pods.Items[0].Spec.Containers[0].Image

	// Privileged DaemonSet configuring host networking as needed
	networkConfigDaemonSet := appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "network-config",
			Namespace: metav1.NamespaceSystem,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"name": "network-config"},
			},
			Template: k8sv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"name": "network-config"},
				},
				Spec: k8sv1.PodSpec{
					Containers: []k8sv1.Container{
						{
							Name: "network-config",
							// Reuse image which is already installed in the cluster. All we need is chroot.
							// Local OKD cluster doesn't allow us to pull from the outside.
							Image: virtOperatorImage,
							Command: []string{
								"sh",
								"-c",
								"set -x; chroot /host ip link add br10 type bridge; chroot /host iptables -I FORWARD 1 -i br10 -j ACCEPT; touch /tmp/ready; sleep INF",
							},
							SecurityContext: &k8sv1.SecurityContext{
								Privileged: pointer.BoolPtr(true),
								RunAsUser:  pointer.Int64Ptr(0),
							},
							ReadinessProbe: &k8sv1.Probe{
								Handler: k8sv1.Handler{
									Exec: &k8sv1.ExecAction{
										Command: []string{"cat", "/tmp/ready"},
									},
								},
							},
							VolumeMounts: []k8sv1.VolumeMount{
								{
									Name:      "host",
									MountPath: "/host",
								},
							},
						},
					},
					Volumes: []k8sv1.Volume{
						{
							Name: "host",
							VolumeSource: k8sv1.VolumeSource{
								HostPath: &k8sv1.HostPathVolumeSource{
									Path: "/",
								},
							},
						},
					},
					HostNetwork: true,
				},
			},
		},
	}

	// Helper function returning existing network-config DaemonSet if exists
	getNetworkConfigDaemonSet := func() (*appsv1.DaemonSet, error) {
		daemonSet, err := f.K8sClient.AppsV1().DaemonSets(metav1.NamespaceSystem).Get(context.TODO(), networkConfigDaemonSet.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return daemonSet, nil
	}

	// If the DaemonSet haven't been created yet, do so
	runningNetworkConfigDaemonSet, err := getNetworkConfigDaemonSet()
	if err != nil {
		return nil
	}
	if runningNetworkConfigDaemonSet == nil {
		_, err := f.K8sClient.AppsV1().DaemonSets(metav1.NamespaceSystem).Create(context.TODO(), &networkConfigDaemonSet, metav1.CreateOptions{})
		if err != nil {
			return nil
		}
	}

	// Make sure that all pods in the Daemon Set finished the configuration
	nodes, err := f.GetAllSchedulableNodes()
	if err != nil {
		return nil
	}

	return wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		daemonSet, err := getNetworkConfigDaemonSet()
		if err != nil {
			return false, err
		}
		if daemonSet != nil {
			available := int(daemonSet.Status.NumberAvailable)
			return available == len(nodes.Items), nil
		}
		return false, nil
	})
}
