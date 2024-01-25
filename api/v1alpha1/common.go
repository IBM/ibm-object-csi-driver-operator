package v1alpha1

type DriverPhase string

const (
	DriverPhaseNone     DriverPhase = ""
	DriverPhaseCreating DriverPhase = "Creating"
	DriverPhaseRunning  DriverPhase = "Running"
	DriverPhaseFailed   DriverPhase = "Failed"
)
