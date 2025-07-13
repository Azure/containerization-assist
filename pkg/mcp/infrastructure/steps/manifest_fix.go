package steps

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/sampling"
)

// FixManifestWithAI uses MCP sampling to fix a Kubernetes manifest that failed to deploy
func FixManifestWithAI(ctx context.Context, manifestPath string, deploymentError error, dockerfileContent string, analyzeResult *AnalyzeResult, logger *slog.Logger) error {
	logger.Info("Requesting AI assistance to fix Kubernetes manifest",
		"manifest_path", manifestPath,
		"error", deploymentError)

	// Read current manifest
	manifestContent, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	// Create sampling client
	samplingClient := sampling.NewClient(logger)

	// Prepare repository analysis summary
	repoAnalysis := fmt.Sprintf("Language: %s, Framework: %s, Port: %d",
		analyzeResult.Language, analyzeResult.Framework, analyzeResult.Port)

	// Get AI fix for the manifest
	fixedManifest, err := samplingClient.AnalyzeKubernetesManifest(
		ctx,
		string(manifestContent),
		deploymentError,
		dockerfileContent,
		repoAnalysis,
	)
	if err != nil {
		return fmt.Errorf("failed to get AI fix for manifest: %w", err)
	}

	// Write the fixed manifest back
	if err := os.WriteFile(manifestPath, []byte(fixedManifest.FixedManifest), 0644); err != nil {
		return fmt.Errorf("failed to write fixed manifest: %w", err)
	}

	logger.Info("Applied AI fix to Kubernetes manifest", "path", manifestPath)
	return nil
}

// AnalyzePodFailure uses AI to diagnose why a pod is failing and suggest fixes
func AnalyzePodFailure(ctx context.Context, namespace, podName string, k8sResult *K8sResult, dockerfileContent string, logger *slog.Logger) (*sampling.ErrorAnalysis, error) {
	logger.Info("Analyzing pod failure", "namespace", namespace, "pod", podName)

	// Get pod logs
	podLogs, err := GetPodLogs(ctx, namespace, podName, logger)
	if err != nil {
		logger.Warn("Failed to get pod logs", "error", err)
		podLogs = "Unable to retrieve pod logs: " + err.Error()
	}

	// Get pod events
	podEvents, err := GetPodEvents(ctx, namespace, podName, logger)
	if err != nil {
		logger.Warn("Failed to get pod events", "error", err)
		podEvents = "Unable to retrieve pod events: " + err.Error()
	}

	// Read the deployment manifest
	manifestPath := filepath.Join(k8sResult.Manifests["deployment_path"].(string))
	manifestContent, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	// Create error details
	errorDetails := fmt.Sprintf("Pod Events:\n%s\n\nPod Status: CrashLoopBackOff or Failed", podEvents)

	// Use AI to analyze the crash
	samplingClient := sampling.NewClient(logger)
	analysis, err := samplingClient.AnalyzePodCrashLoop(
		ctx,
		podLogs,
		string(manifestContent),
		dockerfileContent,
		errorDetails,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze pod crash: %w", err)
	}

	return analysis, nil
}

// GetPodLogs retrieves logs from a Kubernetes pod
func GetPodLogs(ctx context.Context, namespace, podName string, logger *slog.Logger) (string, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "logs", "-n", namespace, podName, "--tail=100")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs: %v - %s", err, string(output))
	}
	return string(output), nil
}

// GetPodEvents retrieves events for a Kubernetes pod
func GetPodEvents(ctx context.Context, namespace, podName string, logger *slog.Logger) (string, error) {
	// Get events related to the pod
	cmd := exec.CommandContext(ctx, "kubectl", "get", "events",
		"-n", namespace,
		"--field-selector", fmt.Sprintf("involvedObject.name=%s", podName),
		"--sort-by='.lastTimestamp'")

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try alternate approach
		cmd = exec.CommandContext(ctx, "kubectl", "describe", "pod", "-n", namespace, podName)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to get pod events: %v - %s", err, string(output))
		}
		// Extract events section from describe output
		lines := strings.Split(string(output), "\n")
		eventsStart := false
		var events []string
		for _, line := range lines {
			if strings.Contains(line, "Events:") {
				eventsStart = true
				continue
			}
			if eventsStart {
				events = append(events, line)
			}
		}
		return strings.Join(events, "\n"), nil
	}
	return string(output), nil
}
