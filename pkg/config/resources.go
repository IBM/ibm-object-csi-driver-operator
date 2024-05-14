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
	// DriverPrefix ...
	DriverPrefix = "ibm-object-csi"
	// CSIController ...
	CSIController ResourceName = "controller"
	// CSINode ...
	CSINode ResourceName = "node"
	// CSIControllerServiceAccount ...
	CSIControllerServiceAccount ResourceName = "controller-sa"
	// CSINodeServiceAccount ...
	CSINodeServiceAccount ResourceName = "node-sa"
	// ExternalProvisionerClusterRole ...
	ExternalProvisionerClusterRole ResourceName = "external-provisioner-clusterrole"
	// ExternalProvisionerClusterRoleBinding ...
	ExternalProvisionerClusterRoleBinding ResourceName = "external-provisioner-clusterrolebinding"
	// CSIControllerSCCClusterRole ...
	CSIControllerSCCClusterRole ResourceName = "controller-scc-clusterrole"
	// CSIControllerSCCClusterRoleBinding ...
	CSIControllerSCCClusterRoleBinding ResourceName = "controller-scc-clusterrolebinding"
	// CSINodeSCCClusterRole ...
	CSINodeSCCClusterRole ResourceName = "node-scc-clusterrole"
	// CSINodeSCCClusterRoleBinding ...
	CSINodeSCCClusterRoleBinding ResourceName = "node-scc-clusterrolebinding"
	// RcloneRetainStorageClass ...
	RcloneRetainStorageClass ResourceName = "ibm-object-storage-rclone-retain-sc"
	// RcloneStorageClass ...
	RcloneStorageClass ResourceName = "ibm-object-storage-rclone-sc"
	// S3fsRetainStorageClass ...
	S3fsRetainStorageClass ResourceName = "ibm-object-storage-s3fs-retain-sc"
	// S3fsStorageClass ...
	S3fsStorageClass ResourceName = "ibm-object-storage-s3fs-sc"
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
