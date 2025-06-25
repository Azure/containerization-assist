package utils

import "strings"

// IsPodInFailedState checks if the pod output indicates a failed state
func IsPodInFailedState(podOutput string) bool {
	failedStates := []string{
		"CrashLoopBackOff",
		"Error",
		"ImagePullBackOff",
		"ErrImagePull",
		"CreateContainerConfigError",
		"InvalidImageName",
		"CreateContainerError",
		"PreCreateHookError",
		"PostStartHookError",
		"PreStopHookError",
	}

	for _, state := range failedStates {
		if strings.Contains(podOutput, state) {
			return true
		}
	}
	return false
}

// IsPodReady checks if the pod is running and ready
func IsPodReady(podOutput string) bool {
	// Check if pod is in Running phase
	if !strings.Contains(podOutput, `"phase": "Running"`) &&
		!strings.Contains(podOutput, "Running") {
		return false
	}

	// Check that it's not marked as not ready
	if strings.Contains(podOutput, `"ready": false`) {
		return false
	}

	return true
}
