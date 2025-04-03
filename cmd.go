package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	AZURE_OPENAI_KEY			= "AZURE_OPENAI_KEY"
	AZURE_OPENAI_ENDPOINT		= "AZURE_OPENAI_ENDPOINT"
	AZURE_OPENAI_DEPLOYMENT_ID	= "AZURE_OPENAI_DEPLOYMENT_ID"
)

func Execute() {
    var rootCmd = &cobra.Command{
        Use:   "container-copilot",
        Short: "An AI-Powered CLI tool to containerize your app and generate Kubernetes artifacts",
        Run: func(cmd *cobra.Command, args []string) {
           cmd.Help()
        },
    }

    var generateCmd = &cobra.Command{
        Use:   "generate",
        Short: "Generate Dockerfile and Kubernetes manifests",
		Long:  `The generate command will add Dockerfile and Kubernetes manifests to your project based on the project structure.`,
        RunE: func(cmd *cobra.Command, args []string) error{
			if len(args) < 1 {
				// set default dir to current working dir for file structure path
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("error getting current directory: %w", err)
				}
				args = append(args, cwd)
			}

			dir := args[0]
			c, err := initClient()
			if err != nil {
				return fmt.Errorf("error initializing Azure OpenAI client: %w", err)
			}
			if err := c.generate(dir); err != nil {
				return fmt.Errorf("error generating artifacts: %w", err)
			}

			return nil
        },
    }

	var testCmd = &cobra.Command{
		Use:  "test",
		Short: "Test Azure OpenAI connection",
		Long:  `The test command will test the Azure OpenAI connection based on the environment variables set and print a response.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := initClient()
			if err != nil {
				return fmt.Errorf("error initializing Azure OpenAI client: %w", err)
			}
			if err := c.testOpenAIConn(); err != nil {
				return fmt.Errorf("error testing Azure OpenAI connection: %w", err)
			}

			return nil
		},
	}

    rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(testCmd)
    rootCmd.Execute()
}

func initClient() (*AzOpenAIClient, error) {
	apiKey := os.Getenv(AZURE_OPENAI_KEY)
	endpoint := os.Getenv(AZURE_OPENAI_ENDPOINT)
	deploymentID := os.Getenv(AZURE_OPENAI_DEPLOYMENT_ID)
	
	var missingVars []string
	if apiKey == "" {
		missingVars = append(missingVars, AZURE_OPENAI_KEY)
	}
	if endpoint == "" {
		missingVars = append(missingVars, AZURE_OPENAI_ENDPOINT)
	}
	if deploymentID == "" {
		missingVars = append(missingVars, AZURE_OPENAI_DEPLOYMENT_ID)
	}

	if len(missingVars) > 0 {
		return nil, fmt.Errorf("missing environment variables: %s", strings.Join(missingVars, ", "))
	}

	client, err := NewAzOpenAIClient(endpoint, apiKey, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure OpenAI client: %w", err)
	}

	return client, nil
}