package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// checkPodStatus verifies if pods from the deployment are running correctly
func checkPodStatus(namespace string, labelSelector string, timeout time.Duration) (bool, string) {
	endTime := time.Now().Add(timeout)

	for time.Now().Before(endTime) {
		cmd := exec.Command("kubectl", "get", "pods", "-n", namespace, "-o", "json")
		output, err := cmd.CombinedOutput()

		//fmt.Println("Kubectl get pods output:", string(output))
		if err != nil {
			return false, fmt.Sprintf("Error checking pod status: %v\nOutput: %s", err, string(output))
		}

		// Check for problematic pod states in the output
		outputStr := string(output)
		if strings.Contains(outputStr, "CrashLoopBackOff") ||
			strings.Contains(outputStr, "Error") ||
			strings.Contains(outputStr, "ImagePullBackOff") {
			return false, fmt.Sprintf("Pods are in a failed state:\n%s", outputStr)
		}

		// Check if all pods are running and ready
		if strings.Contains(outputStr, "\"phase\": \"Running\"") && !strings.Contains(outputStr, "\"ready\": false") {
			return true, "All pods are running and ready"
		}

		// Wait before checking again
		time.Sleep(5 * time.Second)
	}

	return false, "Timeout waiting for pods to become ready"
}
