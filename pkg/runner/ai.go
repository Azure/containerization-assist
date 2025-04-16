package runner

import "fmt"

func (c *Clients) testOpenAIConn() error {
	testResponse, err := c.AzOpenAIClient.GetChatCompletion("Hello Azure OpenAI! Tell me this is working in one short sentence.")
	if err != nil {
		return fmt.Errorf("failed to get chat completion: %w", err)
	}

	fmt.Println("Azure OpenAI Test")
	fmt.Printf("Response: %s\n", testResponse)
	return nil
}
