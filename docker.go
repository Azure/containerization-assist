package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// buildDockerfile attempts to build the Docker image and returns any error output
func buildDockerfile(dockerfilePath string) (bool, string) {
	// Get the directory containing the Dockerfile to use as build context
	dockerfileDir := filepath.Dir(dockerfilePath)

	registryName := os.Getenv("REGISTRY")

	// Run Docker build with explicit context path
	// Use the absolute path for the dockerfile and specify the context directory
	cmd := exec.Command("docker", "build", "-f", dockerfilePath, "-t", registryName+"/tomcat-hello-world-workflow:latest", dockerfileDir)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		fmt.Println("Docker build failed with error:", err)
		return false, outputStr
	}

	return true, outputStr
}
