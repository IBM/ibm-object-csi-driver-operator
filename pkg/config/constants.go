package config

// Add a field here if it never changes, if it changes over time, put it to settings.go
const (
	APIGroup    = "objectdriver.csi.ibm.com"
	APIVersion  = "v1"
	Name        = "ibm-object-csi-operator"
	DriverName  = "cos.s3.csi.ibm.io"
	ProductName = "ibm-object-csi-driver"

	RbacAuthorizationApiGroup = "rbac.authorization.k8s.io"
	SecurityOpenshiftApiGroup = "security.openshift.io"
	StorageApiGroup           = "storage.k8s.io"

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
)
