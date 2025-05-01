package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/ai"
	"github.com/Azure/container-copilot/pkg/clients"
	"github.com/Azure/container-copilot/pkg/docker"
	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/kind"
	"github.com/Azure/container-copilot/pkg/runner"
	"github.com/spf13/cobra"
)

const (
	AZURE_OPENAI_KEY           = "AZURE_OPENAI_KEY"
	AZURE_OPENAI_ENDPOINT      = "AZURE_OPENAI_ENDPOINT"
	AZURE_OPENAI_DEPLOYMENT_ID = "AZURE_OPENAI_DEPLOYMENT_ID"
)

var (
	registry            string
	dockerfileGenerator string
	generateSnapshot    bool
	timeout             time.Duration
)

var rootCmd = &cobra.Command{
	Use: "container-copilot",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate Dockerfile and Kubernetes manifests",
	Long:  `The generate command will add Dockerfile and Kubernetes manifests to your project based on the project structure.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
		defer cancel()
		targetDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("error getting current directory: %w", err)
		}
		if len(args) > 0 {
			targetDir = args[0]
		}

		c, err := initClients()
		if err != nil {
			return fmt.Errorf("error initializing Azure OpenAI client: %w", err)
		}
		if err := generate(ctx, targetDir, registry, dockerfileGenerator == "draft", generateSnapshot, c); err != nil {
			return fmt.Errorf("error generating artifacts: %w", err)
		}

		return nil
	},
}

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test Azure OpenAI connection",
	Long:  `The test command will test the Azure OpenAI connection based on the environment variables set and print a response.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		c, err := initClients()
		if err != nil {
			return fmt.Errorf("error initializing Azure OpenAI client: %w", err)
		}
		if err := c.TestOpenAIConn(ctx); err != nil {
			return fmt.Errorf("error testing Azure OpenAI connection: %w", err)
		}

		return nil
	},
}

func Execute() {
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.ExecuteContext(context.Background())
}

func initClients() (*clients.Clients, error) {

	// read from .env

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

	azOpenAIClient, err := ai.NewAzOpenAIClient(endpoint, apiKey, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure OpenAI client: %w", err)
	}

	cmdRunner := &runner.DefaultCommandRunner{}

	clients := &clients.Clients{
		AzOpenAIClient: azOpenAIClient,
		Docker:         docker.NewDockerCmdRunner(cmdRunner),
		Kind:           kind.NewKindCmdRunner(cmdRunner),
		Kube:           k8s.NewKubeCmdRunner(cmdRunner),
	}

	return clients, nil
}

func init() {
	rootCmd.PersistentFlags().DurationVarP(&timeout, "timeout", "t", 5*time.Minute, "Timeout for the whole operation")
	generateCmd.PersistentFlags().StringVarP(&registry, "registry", "r", "localhost:5001", "Docker registry to push the image to")
	generateCmd.PersistentFlags().StringVarP(&dockerfileGenerator, "dockerfile-generator", "", "draft", "Which generator to use for the Dockerfile, options: draft, none")
	generateCmd.PersistentFlags().BoolVarP(&generateSnapshot, "snapshot", "s", false, "Generate a snapshot of the Dockerfile and Kubernetes manifests generated in each iteration")

}
