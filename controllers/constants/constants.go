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
	DeploymentName       = "ibm-object-csi-operator-controller-manager"

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
	ConfigMapResource                  = "configmaps"

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
	COSCSIMounterSocketPath                               = "/var/lib/coscsi-sock/coscsi.sock"
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
	CSINodePriorityClassName              = "system-node-critical"
	CSIControllerPriorityClassName        = "system-cluster-critical"

	ParamsConfigMap          = "managed-addon-ibm-object-csi-driver"
	ParamsConfigMapNamespace = "kube-system"
	ObjectCSIDriver          = "ibm-object-csi"

	RetainPolicyTag = "retain"

	StorageClassPrefix = "ibm-object-storage"

	RcloneRetainStorageClass = StorageClassPrefix + "-rclone-retain"
	RcloneStorageClass       = StorageClassPrefix + "-rclone"
	S3fsRetainStorageClass   = StorageClassPrefix + "-s3fs-retain"
	S3fsStorageClass         = StorageClassPrefix + "-s3fs"

	S3ProviderIBM    = "ibm-cos"
	S3ProviderAWS    = "aws"
	S3ProviderWasabi = "wasabi"
	IaasIBMClassic   = "ibm-classic"
	IaasIBMVPC       = "ibm-vpc"

	IBMEP    = "https://s3.%s.%s.cloud-object-storage.appdomain.cloud"
	AWSEP    = "https://s3.%s.amazonaws.com"
	WasabiEP = "https://s3.%s.wasabisys.com"

	DefaultLogTailLines    = 300
	DefaultNamespace       = "default"
	ReconcilationTime      = 5 * time.Minute
	TransportEndpointError = "transport endpoint is not connected"

	InfraProviderPlatformIBM = "IBMCloud"
	InfraProviderType        = "VPC"

	MaxVolumesPerNodeEnv = "MAX_VOLUMES_PER_NODE"
	//ConfigMap keys
	MaxVolumesPerNodeCMKey       = "maxVolumesPerNode"
	NodeServerCPURequestCMKey    = "nodeServerCPURequest"
	NodeServerMemoryRequestCMKey = "nodeServerMemoryRequest"
	NodeServerCPULimitCMKey      = "nodeServerCPULimit"
	NodeServerMemoryLimitCMKey   = "nodeServerMemoryLimit"
)

type FinalizerOps int

const (
	AddFinalizer FinalizerOps = iota + 1
	RemoveFinalizer
)

var CommonCSIResourceLabels = map[string]string{
	"app.kubernetes.io/part-of":    CSIDriverName,
	"app.kubernetes.io/managed-by": CSIOperatorName,
}

var CommonCSIResourceLabelForCaching = map[string]string{
	"app.kubernetes.io/part-of": CSIDriverName,
}

// GetResourceName returns the name of a resource for a CSI driver
func GetResourceName(name string) string {
	return fmt.Sprintf("%s-%s", ObjectCSIDriver, name)
}
