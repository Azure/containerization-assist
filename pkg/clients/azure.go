package clients

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/Azure/container-copilot/pkg/logger"
)

// AzureClient wraps Azure SDK clients for cognitive services operations
type AzureClient struct {
	usagesClient        *armcognitiveservices.UsagesClient
	subscriptionsClient *armsubscriptions.Client
	credential          *azidentity.DefaultAzureCredential
}

// NewAzureClient creates a new Azure client using DefaultAzureCredential
func NewAzureClient() (*AzureClient, error) {
	// Use DefaultAzureCredential which automatically tries multiple authentication methods:
	// 1. Environment variables (AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_TENANT_ID)
	// 2. Managed Identity (when running on Azure)
	// 3. Azure CLI (when az login is active)
	// 4. Azure PowerShell (when Connect-AzAccount is active)
	// 5. Interactive browser (as fallback)
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credentials: %w", err)
	}

	// Create subscriptions client to get current subscription
	subscriptionsClient, err := armsubscriptions.NewClient(cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscriptions client: %w", err)
	}

	return &AzureClient{
		credential:          cred,
		subscriptionsClient: subscriptionsClient,
	}, nil
}

// SetSubscription sets the subscription ID for the client
func (c *AzureClient) SetSubscription(subscriptionID string) error {
	usagesClient, err := armcognitiveservices.NewUsagesClient(subscriptionID, c.credential, nil)
	if err != nil {
		return fmt.Errorf("failed to create usages client: %w", err)
	}
	c.usagesClient = usagesClient
	return nil
}

// GetCurrentSubscriptionID retrieves the current subscription ID from Azure
func (c *AzureClient) GetCurrentSubscriptionID(ctx context.Context) (string, error) {
	// List subscriptions and find the default one
	pager := c.subscriptionsClient.NewListPager(nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get subscriptions: %w", err)
		}

		// For now, return the first subscription
		// In a more sophisticated implementation, we could check for a default subscription
		for _, subscription := range page.Value {
			if subscription.SubscriptionID != nil {
				logger.Debugf("Using subscription: %s", *subscription.SubscriptionID)
				return *subscription.SubscriptionID, nil
			}
		}
	}

	return "", fmt.Errorf("no subscriptions found")
}

// QuotaInfo represents quota information for a model in a region
type QuotaInfo struct {
	Current float64
	Limit   float64
	Name    string
}

// ListUsages retrieves quota usage information for a specific model in a region
func (c *AzureClient) ListUsages(ctx context.Context, subscriptionID, location, modelID string) ([]QuotaInfo, error) {
	logger.Debugf("Checking quota for model %s in region %s using Azure SDK", modelID, location)

	// Set the subscription for this request
	if err := c.SetSubscription(subscriptionID); err != nil {
		return nil, fmt.Errorf("failed to set subscription: %w", err)
	}

	// List usages for the specified location
	pager := c.usagesClient.NewListPager(location, nil)

	var quotaInfos []QuotaInfo

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get usage page: %w", err)
		}

		// Filter usages that contain the model ID
		for _, usage := range page.Value {
			if usage.Name != nil && usage.Name.Value != nil {
				usageName := *usage.Name.Value

				// Check if this usage entry relates to our model
				if strings.Contains(usageName, modelID) {
					quotaInfo := QuotaInfo{
						Name: usageName,
					}

					// Safely extract current value
					if usage.CurrentValue != nil {
						quotaInfo.Current = float64(*usage.CurrentValue)
					}

					// Safely extract limit value
					if usage.Limit != nil {
						quotaInfo.Limit = float64(*usage.Limit)
					}

					quotaInfos = append(quotaInfos, quotaInfo)
					logger.Debugf("Found quota: %s (current: %.0f, limit: %.0f)", usageName, quotaInfo.Current, quotaInfo.Limit)
				}
			}
		}
	}

	if len(quotaInfos) == 0 {
		logger.Debugf("No quota data found for model %s in region %s", modelID, location)
	}

	return quotaInfos, nil
}

// LocationInfo represents an Azure location/region
type LocationInfo struct {
	Name        string
	DisplayName string
}

// ListLocations retrieves available Azure locations/regions
func (c *AzureClient) ListLocations(ctx context.Context) ([]LocationInfo, error) {
	// First get the subscription ID to list locations for
	subscriptionID, err := c.GetCurrentSubscriptionID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription ID: %w", err)
	}

	// List locations for the subscription
	pager := c.subscriptionsClient.NewListLocationsPager(subscriptionID, nil)

	var locations []LocationInfo

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get locations page: %w", err)
		}

		for _, location := range page.Value {
			if location.Name != nil {
				locationInfo := LocationInfo{
					Name: *location.Name,
				}

				if location.DisplayName != nil {
					locationInfo.DisplayName = *location.DisplayName
				}

				locations = append(locations, locationInfo)
			}
		}
	}

	return locations, nil
}

// GetAzureClient safely returns the Azure SDK client from a Clients instance, creating a new one if needed
func (c *Clients) GetAzureClient() (*AzureClient, error) {
	if c.AzureClient != nil {
		return c.AzureClient, nil
	}

	// Create a new client if not initialized
	azureClient, err := NewAzureClient()
	if err != nil {
		return nil, err
	}

	c.AzureClient = azureClient
	return azureClient, nil
}
