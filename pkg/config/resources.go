package config

import (
	"fmt"
)

// ResourceName is the type for aliasing resources that will be created.
type ResourceName string

func (rn ResourceName) String() string {
	return string(rn)
}

const (
	CSIController                         ResourceName = "object-csi-controller"
	CSINode                               ResourceName = "object-csi-node"
	CSIControllerServiceAccount           ResourceName = "object-csi-controller-sa"
	CSINodeServiceAccount                 ResourceName = "object-csi-node-sa"
	ExternalProvisionerClusterRole        ResourceName = "external-provisioner-clusterrole"
	ExternalProvisionerClusterRoleBinding ResourceName = "external-provisioner-clusterrolebinding"
	CSIControllerSCCClusterRole           ResourceName = "object-csi-controller-scc-clusterrole"
	CSIControllerSCCClusterRoleBinding    ResourceName = "object-csi-controller-scc-clusterrolebinding"
	CSINodeSCCClusterRole                 ResourceName = "object-csi-node-scc-clusterrole"
	CSINodeSCCClusterRoleBinding          ResourceName = "object-csi-node-scc-clusterrolebinding"
	RcloneStorageClass                    ResourceName = "cos-s3-csi-rclone-sc"
	S3fsStorageClass                      ResourceName = "cos-s3-csi-s3fs-sc"
)

// GetNameForResource returns the name of a resource for a CSI driver
func GetNameForResource(name ResourceName, driverName string) string {
	switch name {
	case CSIController:
		return fmt.Sprintf("%s-controller", driverName)
	case CSINode:
		return fmt.Sprintf("%s-node", driverName)
	case CSIControllerServiceAccount:
		return fmt.Sprintf("%s-controller-sa", driverName)
	case CSINodeServiceAccount:
		return fmt.Sprintf("%s-node-sa", driverName)
	default:
		return fmt.Sprintf("%s-%s", driverName, name)
	}
}
