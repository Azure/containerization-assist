package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type containerCopilotConfig struct {
	apiKey 	  string
	endpoint  string
	deploymentID string
	outputDir 	 string
	client   *AzOpenAIClient

}

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
        Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				// set default dir to current working dir for file structure path
				cwd, err := os.Getwd()
				if err != nil {
					fmt.Println("Error getting current directory:", err)
					return
				}
				args = append(args, cwd)
			}

			dir := args[0]
			c, err := initClient()
			if err != nil {
				return
			}
			if err := c.generate(dir); err != nil {
				fmt.Printf("Error generating artifacts: %v\n", err)
				return
			}
        },
    }

	var testCmd = &cobra.Command{
		Use:  "test",
		Short: "Test Azure OpenAI connection",
		Long:  `The test command will test the Azure OpenAI connection based on the environment variables set and print a response.`,
		Run: func(cmd *cobra.Command, args []string) {
			c, err := initClient()
			if err != nil {
				fmt.Println("Error initializing Azure OpenAI client:", err)
				return
			}
			if err := c.testOpenAIConn(); err != nil {
				fmt.Print("Error testing Azure OpenAI connection: %v\n", err)
				return
			}
		},
	}

    rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(testCmd)
    rootCmd.Execute()
}

func initClient() (*AzOpenAIClient, error) {
	// use flags for open api config?
	apiKey := os.Getenv("AZURE_OPENAI_KEY")
	endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	deploymentID := os.Getenv("AZURE_OPENAI_DEPLOYMENT_ID")

	if apiKey == "" || endpoint == "" || deploymentID == "" {
		fmt.Println("Error: AZURE_OPENAI_KEY, AZURE_OPENAI_ENDPOINT and AZURE_OPENAI_DEPLOYMENT_ID environment variables must be set")
		return nil, fmt.Errorf("missing required environment variables")
	}

	client, err := NewAzOpenAIClient(endpoint, apiKey, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure OpenAI client: %v", err)
	}

	return client, nil
}