// Package constants ...
package constants

import (
	"fmt"
	"time"
)

const (
	APIGroup = "objectdriver.csi.ibm.com"

	APIVersion           = "v1"
	CSIOperatorName      = "ibm-object-csi-driver-operator"
	CSIOperatorNamespace = "ibm-object-csi-operator"
	CSIDriverName        = "ibm-object-csi-driver"
	DriverName           = "cos.s3.csi.ibm.io"

	RbacAuthorizationAPIGroup = "rbac.authorization.k8s.io"
	SecurityOpenshiftAPIGroup = "security.openshift.io"
	StorageAPIGroup           = "storage.k8s.io"

	CSINodesResource                   = "csinodes"
	SecretsResource                    = "secrets"
	SecurityContextConstraintsResource = "securitycontextconstraints"
	StorageClassesResource             = "storageclasses"
	EventsResource                     = "events"
	NodesResource                      = "nodes"
	PersistentVolumesResource          = "persistentvolumes"
	PersistentVolumeClaimsResource     = "persistentvolumeclaims"

	VerbGet    = "get"
	VerbList   = "list"
	VerbWatch  = "watch"
	VerbCreate = "create"
	VerbUpdate = "update"
	VerbPatch  = "patch"
	VerbDelete = "delete"

	CSINodeDriverRegistrar = "csi-node-driver-registrar"
	CSIProvisioner         = "csi-provisioner"
	LivenessProbe          = "livenessprobe"

	ControllerSocketVolumeMountPath                       = "/var/lib/csi/sockets/pluginproxy/"
	NodeSocketVolumeMountPath                             = "/csi"
	ControllerLivenessProbeContainerSocketVolumeMountPath = "/csi"
	ControllerSocketPath                                  = "/var/lib/csi/sockets/pluginproxy/csi.sock"
	NodeSocketPath                                        = "/csi/csi.sock"
	NodeRegistrarSocketPath                               = "/var/lib/kubelet/plugins/cos.s3.csi.ibm.io/csi.sock"
	CSIEndpoint                                           = "unix:///var/lib/csi/sockets/pluginproxy/csi.sock"
	CSINodeEndpoint                                       = "unix:///csi/csi.sock"
	RegistrationVolumeMountPath                           = "/registration"

	NodeContainerName       = "ibm-object-csi-node"
	ControllerContainerName = "ibm-object-csi-controller"

	RegistrationVolumeName = "registration-dir"
	PluginVolumeName       = "plugin-dir"
	SocketVolumeName       = "socket-dir"

	HealthPortName   = "healthz"
	HealthPortNumber = 9808

	DriverPrefix = "ibm-object-csi"

	CSIController                         = "controller"
	CSINode                               = "node"
	CSIControllerServiceAccount           = "controller-sa"
	CSINodeServiceAccount                 = "node-sa"
	ExternalProvisionerClusterRole        = "external-provisioner-clusterrole"
	ExternalProvisionerClusterRoleBinding = "external-provisioner-clusterrolebinding"
	CSIControllerSCCClusterRole           = "controller-scc-clusterrole"
	CSIControllerSCCClusterRoleBinding    = "controller-scc-clusterrolebinding"
	CSINodeSCCClusterRole                 = "node-scc-clusterrole"
	CSINodeSCCClusterRoleBinding          = "node-scc-clusterrolebinding"

	StorageClassNamePrefix = "ibm-object-storage"
	RetainPolicyTag        = "retain"

	StorageClassPrefix = "ibm-object-storage"

	RcloneRetainStorageClass = StorageClassPrefix + "-rclone-retain"
	RcloneStorageClass       = StorageClassPrefix + "-rclone"
	S3fsRetainStorageClass   = StorageClassPrefix + "-s3fs-retain"
	S3fsStorageClass         = StorageClassPrefix + "-s3fs"

	DefaultLogTailLines    = 300
	DefaultNamespace       = "default"
	ReconcilationTime      = 5 * time.Minute
	TransportEndpointError = "transport endpoint is not connected"
)

var CommonCSIResourceLabels = map[string]string{
	"app.kubernetes.io/part-of":    CSIDriverName,
	"app.kubernetes.io/managed-by": CSIOperatorName,
}

// GetResourceName returns the name of a resource for a CSI driver
func GetResourceName(name string) string {
	return fmt.Sprintf("%s-%s", DriverPrefix, name)
}
