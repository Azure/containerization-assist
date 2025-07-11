package clients

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/Azure/container-kit/pkg/k8s"
	"github.com/Azure/container-kit/pkg/logger"
	"github.com/Azure/container-kit/pkg/mcp/errors"
)

const APP_LABEL = "app.kubernetes.io/name"

// CheckPodStatus verifies if pods from the deployment are running correctly
func (c *Clients) CheckPodStatus(ctx context.Context, namespace string, labelSelector string, timeout time.Duration) (bool, string) {
	endTime := time.Now().Add(timeout)

	for time.Now().Before(endTime) {
		readableOutputStr, err := c.Kube.GetPods(ctx, namespace, labelSelector)
		logger.Debugf("Kubectl get pods output: \n%s", readableOutputStr)

		if err != nil {
			return false, fmt.Sprintf("Error checking pod status: %v\nOutput: %s", err, readableOutputStr)
		}

		outputStr, err := c.Kube.GetPodsJSON(ctx, namespace, labelSelector)
		if err != nil {
			return false, fmt.Sprintf("Error checking pod status: %v\nOutput: %s", err, outputStr)
		}

		// Check for problematic pod states in the output
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

// deployAndVerifySingleManifest applies a single manifest and verifies pod health
func (c *Clients) DeployAndVerifySingleManifest(ctx context.Context, manifestPath string, isDeployment bool) (bool, string, error) {
	logger.Debugf("    Source path: %s", manifestPath)
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return false, "", errors.New(errors.CodeIoError, "k8s", fmt.Sprintf("reading manifest file: %v", err), err)
	}
	o, err := k8s.ReadK8sObjects(content)
	if err != nil {
		return false, "", errors.New(errors.CodeManifestInvalid, "k8s", fmt.Sprintf("reading k8s objects from manifest file %s: %v", manifestPath, err), err)
	}

	outputStr, err := c.Kube.Apply(ctx, manifestPath)

	if err != nil {
		logger.Errorf(" ❌  failed to apply manifest %s with error: %v", manifestPath, err)
		return false, outputStr, nil
	}

	logger.Infof("    ☑️  Successfully applied manifest %s", manifestPath)

	// Only check pod status for deployment.yaml files
	baseFilename := filepath.Base(manifestPath)
	if !isDeployment {
		logger.Debugf("    Skipping pod health check for non-deployment manifest: %s", baseFilename)
		return true, outputStr, nil
	}

	logger.Debugf("    Checking pod health for deployment manifest: %s", baseFilename)

	// Extract namespace and app labels from the manifest
	// This is simplified - would need to actually take this from the manifest
	namespace := "default" // Default namespace
	k8sAppName := o.Metadata.Labels[APP_LABEL]
	if k8sAppName == "" {
		logger.Errorf("    No app label found in manifest %s", manifestPath)
		return false, "", errors.New(errors.CodeManifestInvalid, "k8s", fmt.Sprintf("no app label found in manifest %s", manifestPath), nil)
	}
	labelSelector := fmt.Sprintf("%s=%s", APP_LABEL, k8sAppName)

	// Wait for pods to become healthy
	podSuccess, podOutput := c.CheckPodStatus(ctx, namespace, labelSelector, time.Minute)
	if !podSuccess {
		logger.Debugf("    Retrieving logs for pods with label selector %s in namespace %s", labelSelector, namespace)
		podLogs, err := c.GetDeploymentLogs(ctx, labelSelector, namespace)
		if err != nil {
			logger.Errorf("Error retrieving deployment logs: %v\n", err)
			return false, outputStr + "\n" + podOutput, nil
		}
		logger.Infof("Pods are not healthy for deployment with manifest %s, cleaning up failed deployment\n", manifestPath)
		// Clean up the failed deployment
		deleteOutput, err := c.Kube.DeleteDeployment(ctx, manifestPath)
		if err != nil {
			logger.Errorf("    ⚠️ Warning: Failed to clean up deployment: %v\n", err)
		} else {
			logger.Infof("    Successfully deleted failed deployment: %s\n", deleteOutput)
		}

		// Build error response with both pod health and diagnostic info
		diagnosticOutput := fmt.Sprintf("\n=== DEPLOYMENT HEALTH CHECK RESULTS ===\n%s\n\n=== POD DIAGNOSTIC INFORMATION ===\n%s",
			podOutput, podLogs)

		return false, outputStr + "\n" + diagnosticOutput, nil
	}
	logger.Info("    Pod health check passed")

	return true, outputStr, nil
}

// GetDeploymentLogs retrieves both container logs and detailed pod descriptions
// for all pods matching the label selector in the specified namespace
func (c *Clients) GetDeploymentLogs(ctx context.Context, labelSelector string, namespace string) (string, error) {
	// Loading kubeconfig from default location
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return "", errors.New(errors.CodeConfigurationInvalid, "k8s", fmt.Sprintf("failed to load kubeconfig: %v", err), err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", errors.New(errors.CodeKubernetesApiError, "k8s", fmt.Sprintf("failed to create k8s client: %v", err), err)
	}

	logger.Debugf("Getting pod logs using label selector: %s in namespace: %s", labelSelector, namespace)
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return "", errors.New(errors.CodeKubernetesApiError, "k8s", fmt.Sprintf("failed to list pods with selector %q: %v", labelSelector, err), err)
	}

	if len(pods.Items) == 0 {
		logger.Debugf("No pods found for selector %q in namespace %q", labelSelector, namespace)
		return "", errors.New(errors.CodeResourceNotFound, "k8s", fmt.Sprintf("no pods found for selector %q in namespace %q", labelSelector, namespace), nil)
	}

	logger.Debugf("Found %d pod(s) matching selector %q", len(pods.Items), labelSelector)

	var logBuilder strings.Builder

	// Retrieve logs for each pod
	for _, pod := range pods.Items {
		podName := pod.Name
		logBuilder.WriteString(fmt.Sprintf("\n=== POD: %s ===\n", podName))

		// Get pod description first
		logger.Debugf("Fetching detailed pod description for %s", podName)
		logBuilder.WriteString("\n--- POD DETAILS ---\n")

		podDetails, describeErr := describePodStatus(client, namespace, podName)
		if describeErr != nil {
			logger.Errorf("Failed to describe pod %s: %v", podName, describeErr)
			logBuilder.WriteString(fmt.Sprintf("Error retrieving pod details: %v\n", describeErr))
		} else {
			logger.Debugf("Detailed pod description for %s:\n%s", podName, podDetails)
			logBuilder.WriteString(podDetails + "\n")
		}

		// Try to get container logs
		logBuilder.WriteString("\n--- CONTAINER LOGS ---\n")
		podLogs, err := readPodLogs(client, namespace, podName)
		if err != nil {
			logger.Debugf("Unable to read logs for pod %s: %v", podName, err)
			logBuilder.WriteString(fmt.Sprintf("Container logs not available: %v\n", err))
		} else {
			// If we got logs successfully
			if podLogs == "" {
				logBuilder.WriteString("Container logs are empty. The container may have just started.\n")
			} else {
				logger.Debugf("Retrieved logs for pod %s:\n%s", podName, podLogs)
				logBuilder.WriteString(podLogs + "\n")
			}
		}
	}

	return logBuilder.String(), nil
}

// readPodLogs retrieves logs for a given pod and returns them as a string
func readPodLogs(clientset *kubernetes.Clientset, namespace, podName string) (string, error) {
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{})
	stream, err := req.Stream(context.Background())
	if err != nil {
		return "", fmt.Errorf("error opening log stream for pod %s: %w", podName, err)
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("error reading log stream for pod %s: %w", podName, err)
	}
	return string(data), nil
}

func describePodStatus(clientset *kubernetes.Clientset, namespace, podName string) (string, error) {
	ctx := context.Background()
	var sb strings.Builder

	// Fetch pod
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get pod: %v", err)
	}

	sb.WriteString(fmt.Sprintf("Pod: %s\nNamespace: %s\nStatus: %s\n", pod.Name, pod.Namespace, pod.Status.Phase))
	sb.WriteString("--------------------------------------------------\n")

	// Container status errors
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			sb.WriteString(fmt.Sprintf("Container: %s\n  Waiting: %s - %s\n", cs.Name, cs.State.Waiting.Reason, cs.State.Waiting.Message))
		}
		if cs.State.Terminated != nil {
			sb.WriteString(fmt.Sprintf("Container: %s\n  Terminated: %s - %s (Exit Code: %d)\n",
				cs.Name,
				cs.State.Terminated.Reason,
				cs.State.Terminated.Message,
				cs.State.Terminated.ExitCode))
		}
	}

	// Events
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.namespace=%s", podName, namespace)
	eventList, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list events: %v", err)
	}

	if len(eventList.Items) > 0 {
		sb.WriteString("\nEvents:\n")
		for _, e := range eventList.Items {
			sb.WriteString(fmt.Sprintf(
				"  %s\t%s\t%s\t%s\n",
				e.FirstTimestamp.Format("2006-01-02 15:04:05"),
				e.Type,
				e.Reason,
				e.Message,
			))
		}
	} else {
		sb.WriteString("\nNo events found for pod.\n")
	}

	return sb.String(), nil
}
