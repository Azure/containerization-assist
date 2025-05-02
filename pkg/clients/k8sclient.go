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

	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/logger"
)

// CheckPodStatus verifies if pods from the deployment are running correctly
func (c *Clients) CheckPodStatus(ctx context.Context, namespace string, labelSelector string, timeout time.Duration) (bool, string) {
	endTime := time.Now().Add(timeout)

	for time.Now().Before(endTime) {
		readableOutputStr, err := c.Kube.GetPods(ctx, namespace, labelSelector)
		logger.Infof("Kubectl get pods output: \n%s", readableOutputStr)

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
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return false, "", fmt.Errorf("reading manifest file: %w", err)
	}
	_, err = k8s.ReadK8sObjects(content)
	if err != nil {
		return false, "", fmt.Errorf("reading k8s objects from manifest file %s: %w", manifestPath, err)
	}

	// Apply the manifest
	outputStr, err := c.Kube.Apply(ctx, manifestPath)

	if err != nil {
		logger.Errorf("Kubernetes deployment failed for %s with error: %v", manifestPath, err)
		return false, outputStr, nil
	}

	logger.Infof("Successfully applied %s", manifestPath)

	// Only check pod status for deployment.yaml files
	baseFilename := filepath.Base(manifestPath)
	if !isDeployment {
		logger.Infof("Skipping pod health check for non-deployment manifest: %s", baseFilename)
		return true, outputStr, nil
	}

	logger.Infof("Checking pod health for deployment...")

	// Extract namespace and app labels from the manifest
	// This is simplified - would need to actually take this from the manifest
	namespace := "default" // Default namespace
	labelSelector := ""    // Default label selector #TODO: actually parse this from the manifest

	// Wait for pods to become healthy
	podSuccess, podOutput := c.CheckPodStatus(ctx, namespace, labelSelector, time.Minute)
	if !podSuccess {
		podLogs, err := GetDeploymentLogs(ctx, labelSelector, namespace)
		if err != nil {
			logger.Errorf("Error retrieving deployment logs: %v\n", err)
		}
		logger.Infof("Pods are not healthy for deployment with manifest %s, cleaning up failed deployment\n", manifestPath)
		// Clean up the failed deployment
		deleteOutput, err := c.Kube.DeleteDeployment(ctx, manifestPath)
		if err != nil {
			logger.Errorf("Warning: Failed to clean up deployment: %v\n", err)
		} else {
			logger.Infof("Successfully deleted failed deployment: %s\n", deleteOutput)
		}
		return false, outputStr + "\n" + podOutput + "\n" + podLogs, nil
	}
	logger.Info("Pod health check passed")

	return true, outputStr, nil
}

// GetDeploymentLogs retrieves logs for pods matching the label selector in the specified namespace
func GetDeploymentLogs(ctx context.Context, deploymentName string, namespace string) (string, error) {
	// Loading kubeconfig from default location
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return "", fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Create Kubernetes client
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("failed to create k8s client: %w", err)
	}

	// Get the Deployment
	// Note only please: We may want to handle the case where the deployment does not exist
	// or is not found in the specified namespace
	// This is a simplified example and may need to be adjusted based on our needs
	deployClient := client.AppsV1().Deployments(namespace)
	deployment, err := deployClient.Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get deployment: %w", err)
	}

	// Get the matching pods
	labelSelector := metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: deployment.Spec.Selector.MatchLabels})
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{

		LabelSelector: labelSelector,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found for selector %q in namespace %q", labelSelector, namespace)
	}

	var logBuilder strings.Builder
	// Retrieve logs for each pod
	for _, pod := range pods.Items {
		logBuilder.WriteString(fmt.Sprintf("Logs for Pod: %s\n", pod.Name))
		podLogs, err := readPodLogs(client, namespace, pod.Name)
		if err != nil {
			return "", fmt.Errorf("error retrieving logs for pod %s: %w", pod.Name, err)
		}
		logBuilder.WriteString(podLogs)
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
