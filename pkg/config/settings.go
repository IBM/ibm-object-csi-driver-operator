package config

import (
	"fmt"
	"os"

	v1alpha1 "github.ibm.com/alchemy-containers/ibm-object-csi-driver-operator/api/v1alpha1"
	"sigs.k8s.io/yaml"
)

const (
	EnvNameIBMObjectCSICrYaml = "IBMObjectCSI_CR_YAML"
	DefaultLogLevel           = "DEBUG"
	ControllerUserID          = int64(9999)

	NodeAgentPort = "10086"
)

var DefaultIBMObjectCSICr v1alpha1.IBMObjectCSI

var DefaultSidecarsByName map[string]v1alpha1.CSISidecar

func LoadDefaultsOfIBMObjectCSI() error {
	yamlFile, err := getCrYamlFile(EnvNameIBMObjectCSICrYaml)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(yamlFile, &DefaultIBMObjectCSICr)
	if err != nil {
		return fmt.Errorf("error unmarshaling yaml: %v", err)
	}

	DefaultSidecarsByName = make(map[string]v1alpha1.CSISidecar)

	for _, sidecar := range DefaultIBMObjectCSICr.Spec.Sidecars {
		DefaultSidecarsByName[sidecar.Name] = sidecar
	}

	return nil
}

func getCrYamlFile(crPathEnvVariable string) ([]byte, error) {
	crYamlPath, err := getCrYamlPath(crPathEnvVariable)
	if err != nil {
		return []byte{}, err
	}

	yamlFile, err := os.ReadFile(crYamlPath)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to read file %q: %v", yamlFile, err)
	}
	return yamlFile, nil
}

func getCrYamlPath(crPathEnvVariable string) (string, error) {
	crYamlPath := os.Getenv(crPathEnvVariable)

	if crYamlPath == "" {
		return "", fmt.Errorf("environment variable %q was not set", crPathEnvVariable)
	}
	return crYamlPath, nil
}
