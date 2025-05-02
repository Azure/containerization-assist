package clients

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/Azure/container-copilot/pkg/k8s"
)

// CheckPodStatus verifies if pods from the deployment are running correctly
func (c *Clients) CheckPodStatus(ctx context.Context, namespace string, labelSelector string, timeout time.Duration) (bool, string) {
	endTime := time.Now().Add(timeout)

	for time.Now().Before(endTime) {
		readableOutputStr, err := c.Kube.GetPods(ctx, namespace, labelSelector)
		fmt.Println("Kubectl get pods output:\n", readableOutputStr)
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
		fmt.Printf("Kubernetes deployment failed for %s with error: %v\n", manifestPath, err)
		return false, outputStr, nil
	}

	fmt.Printf("Successfully applied %s\n", manifestPath)

	// Only check pod status for deployment.yaml files
	baseFilename := filepath.Base(manifestPath)
	if !isDeployment {
		fmt.Printf("Skipping pod health check for non-deployment manifest: %s\n", baseFilename)
		return true, outputStr, nil
	}

	fmt.Printf("Checking pod health for deployment...\n")

	// Extract namespace and app labels from the manifest
	// This is simplified - would need to actually take this from the manifest
	namespace := "default" // Default namespace
	labelSelector := ""    // Default label selector #TODO: actually parse this from the manifest

	// Wait for pods to become healthy
	podSuccess, podOutput := c.CheckPodStatus(ctx, namespace, labelSelector, time.Minute)
	if !podSuccess {
		fmt.Printf("Pods are not healthy for deployment with manifest %s, cleaning up failed deployment\n", manifestPath)
		// Clean up the failed deployment
		deleteOutput, err := c.Kube.DeleteDeployment(ctx, manifestPath)
		if err != nil {
			fmt.Printf("Warning: Failed to clean up deployment: %v\n", err)
		} else {
			fmt.Printf("Successfully deleted failed deployment: %s\n", deleteOutput)
		}
		return false, outputStr + "\n" + podOutput, nil
	}
	fmt.Println("Pod health check passed")

	return true, outputStr, nil
}

func GetDeploymentLogs(ctx context.Context, deploymentName string, namespace string) error {
	// Loading kubeconfig from default location
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Create Kubernetes client
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	// Get the Deployment
	// Note only please: We may want to handle the case where the deployment does not exist
	// or is not found in the specified namespace
	// This is a simplified example and may need to be adjusted based on our needs
	deployClient := client.AppsV1().Deployments(namespace)
	deployment, err := deployClient.Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	// Get the matching pods
	labelSelector := metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: deployment.Spec.Selector.MatchLabels})
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for %s", deploymentName)
	}

	// Print logs for the first pod (or loop through all if desired)
	for _, pod := range pods.Items {
		fmt.Printf("Logs for Pod: %s\n", pod.Name)
		err := streamPodLogs(client, namespace, pod.Name)
		if err != nil {
			log.Printf("Error getting logs for pod %s: %v\n", pod.Name, err)
		}
	}

	return nil
}

// streamPodLogs prints logs to stdout
func streamPodLogs(clientset *kubernetes.Clientset, namespace, podName string) error {
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{})
	stream, err := req.Stream(context.Background())
	if err != nil {
		return fmt.Errorf("error opening log stream: %w", err)
	}
	defer stream.Close()

	_, err = io.Copy(os.Stdout, stream)
	return err
}
