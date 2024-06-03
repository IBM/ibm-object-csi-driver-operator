// Package config ...
package config

import "time"

// Add a field here if it never changes, if it changes over time, put it to settings.go
const (
	APIGroup        = "objectdriver.csi.ibm.com"
	APIVersion      = "v1"
	CSIOperatorName = "ibm-object-csi-driver-operator"
	CSIDriverName   = "ibm-object-csi-driver"
	DriverName      = "cos.s3.csi.ibm.io"
	ProductName     = "ibm-object-csi-driver"

	RbacAuthorizationAPIGroup = "rbac.authorization.k8s.io"
	SecurityOpenshiftAPIGroup = "security.openshift.io"
	StorageAPIGroup           = "storage.k8s.io"

	CsiNodesResource                   = "csinodes"
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

	DefaultLogTailLines    = 300
	DefaultNamespace       = "default"
	ReconcilationTime      = 5 * time.Minute
	TransportEndpointError = "transport endpoint is not connected"
)

var CommonCSIResourceLabels = map[string]string{
	"app.kubernetes.io/part-of":    CSIDriverName,
	"app.kubernetes.io/managed-by": CSIOperatorName,
}
