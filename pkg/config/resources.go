// Package config ...
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
	// CSIController ...
	CSIController ResourceName = "object-csi-controller"
	// CSINode ...
	CSINode ResourceName = "object-csi-node"
	// CSIControllerServiceAccount ...
	CSIControllerServiceAccount ResourceName = "object-csi-controller-sa"
	// CSINodeServiceAccount ...
	CSINodeServiceAccount ResourceName = "object-csi-node-sa"
	// ExternalProvisionerClusterRole ...
	ExternalProvisionerClusterRole ResourceName = "external-provisioner-clusterrole"
	// ExternalProvisionerClusterRoleBinding ...
	ExternalProvisionerClusterRoleBinding ResourceName = "external-provisioner-clusterrolebinding"
	// CSIControllerSCCClusterRole ...
	CSIControllerSCCClusterRole ResourceName = "object-csi-controller-scc-clusterrole"
	// CSIControllerSCCClusterRoleBinding ...
	CSIControllerSCCClusterRoleBinding ResourceName = "object-csi-controller-scc-clusterrolebinding"
	// CSINodeSCCClusterRole ...
	CSINodeSCCClusterRole ResourceName = "object-csi-node-scc-clusterrole"
	// CSINodeSCCClusterRoleBinding ...
	CSINodeSCCClusterRoleBinding ResourceName = "object-csi-node-scc-clusterrolebinding"
	// RcloneStorageClass ...
	RcloneStorageClass ResourceName = "cos-s3-csi-rclone-sc"
	// S3fsStorageClass ...
	S3fsStorageClass ResourceName = "cos-s3-csi-s3fs-sc"
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
