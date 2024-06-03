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

	StorageClassPrefix = "ibm-object-storage-"
	StorageClassSuffix = "-sc"

	// RcloneRetainStorageClass ...
	RcloneRetainStorageClass ResourceName = StorageClassPrefix + "rclone-retain" + StorageClassSuffix
	// RcloneStorageClass ...
	RcloneStorageClass ResourceName = StorageClassPrefix + "rclone" + StorageClassSuffix
	// S3fsRetainStorageClass ...
	S3fsRetainStorageClass ResourceName = StorageClassPrefix + "s3fs-retain" + StorageClassSuffix
	// S3fsStorageClass ...
	S3fsStorageClass ResourceName = StorageClassPrefix + "s3fs" + StorageClassSuffix
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
