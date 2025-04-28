package prompt

import (
	"encoding/xml"
	"fmt"
)

// Prompt represents the root XML element in prompt templates
type Prompt struct {
	Role         string      `xml:"role"`
	Context      interface{} `xml:"context"`
	Instructions string      `xml:"instructions"`
	Note         string      `xml:"note"`
}

// Element represents a section with description and content
// If the content is empty, it will be omitted from the LLM Prompt
type Element struct {
	Description xml.CharData `xml:"description"`
	Content     xml.CharData `xml:"content,omitempty"`
}

// GetContent returns the content as a clean string without CDATA tags
func (e *Element) GetContent() string {
	return string(e.Content)
}

// SetContent sets the content, ensuring it will be wrapped in CDATA tags when marshaled
func (e *Element) SetContent(content string) {
	e.Content = xml.CharData(content)
}

// DockerfileContext represents the context section within the Dockerfile prompt
type DockerfileContext struct {
	RepoStructure  Element `xml:"repo_structure"`
	ApprovedImages Element `xml:"approved_container_images"`
	ManifestErrors Element `xml:"manifest_errors"`
	BuildErrors    Element `xml:"build_errors"`
	Dockerfile     Element `xml:"dockerfile"`
}

type LLMResponse struct {
	XMLName      xml.Name     `xml:"response"`
	Analysis     xml.CharData `xml:"analysis"`
	Explanation  xml.CharData `xml:"explanation"`
	FixedContent xml.CharData `xml:"fixed_content"`
}

// LoadDockerfilePromptTemplateFromBytes loads the Dockerfile prompt template from a byte slice
func LoadDockerfilePromptTemplateFromBytes(data []byte) (*Prompt, error) {
	prompt := &Prompt{
		Context: &DockerfileContext{},
	}
	if err := xml.Unmarshal(data, prompt); err != nil {
		return nil, err
	}
	return prompt, nil
}

// // LoadManifestPromptTemplate loads the Manifest prompt template from the given path
// func LoadManifestPromptTemplate(templatePath string) (*Prompt, error) {
// 	prompt := &Prompt{
// 		Context: &ManifestContext{},
// 	}

// 	err := DecodeXMLFromFile(templatePath, prompt)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return prompt, nil
// }

// Fill populates the Dockerfile prompt with the specified content
// Requires specifc function because the context can differ
func (p *Prompt) FillDockerfilePrompt(dockerfileContent, manifestErrors, approvedImages, buildErrors, repoStructure string) {
	ctx, ok := p.Context.(*DockerfileContext)
	if !ok {
		return // Context is not a DockerfileContext
	}

	ctx.Dockerfile.SetContent(dockerfileContent)
	ctx.ManifestErrors.SetContent(manifestErrors)
	ctx.ApprovedImages.SetContent(approvedImages)
	ctx.BuildErrors.SetContent(buildErrors)
	ctx.RepoStructure.SetContent(repoStructure)
}

// // FillManifestPrompt populates the manifest template with the specified content
// func FillManifestPrompt(template *Prompt, manifestContent string,
// 	deploymentErrors string, approvedServices string, repoStructure string) {

// 	ctx, ok := template.Context.(*ManifestContext)
// 	if !ok {
// 		return // Context is not a ManifestContext
// 	}

// 	// Fill in the content sections
// 	ctx.Manifest.Content = manifestContent

// 	if deploymentErrors != "" {
// 		ctx.DeploymentErrors.Content = deploymentErrors
// 	}

// 	if approvedServices != "" {
// 		ctx.ApprovedServices.Content = approvedServices
// 	}

// 	if repoStructure != "" {
// 		ctx.RepoStructure.Content = repoStructure
// 	}
// }

func ExtractResponseSections(responseText string) (string, string, string, error) {
	// Parse the response using XML unmarshalling
	response, err := unmarshalLLMResponse(responseText)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to extract response sections: %w", err)
	}

	// Verify that required sections exist
	if len(response.FixedContent) == 0 {
		return "", "", "", fmt.Errorf("response missing required fixed_content section")
	}
	if len(response.Analysis) == 0 {
		return "", "", "", fmt.Errorf("response missing required analysis section")
	}
	if len(response.Explanation) == 0 {
		return "", "", "", fmt.Errorf("response missing required explanation section")
	}

	fixedContent := string(response.FixedContent)
	analysis := string(response.Analysis)
	explanation := string(response.Explanation)

	return fixedContent, analysis, explanation, nil
}

// unmarshalLLMResponse parses the LLM response into structured data
func unmarshalLLMResponse(responseText string) (*LLMResponse, error) {
	// Check if the response is empty
	if responseText == "" {
		return nil, fmt.Errorf("response is empty")
	}
	fmt.Println("Response text:", responseText)

	var response LLMResponse

	err := xml.Unmarshal([]byte(responseText), &response)
	if err != nil {
		fmt.Println("Error unmarshalling response:", err)
	}

	if err != nil {

		return nil, fmt.Errorf("failed to unmarshal LLM response: %w", err)

	}

	return &response, nil
}

// PrintStruct prints a struct in a human-readable format
func PrintStruct(data interface{}) {
	xmlStr, err := EncodeXMLToString(data)
	if err != nil {
		fmt.Printf("Error encoding struct to XML: %v\n", err)
		return
	}

	// Print in human-readable format
	PrintXMLContent(xmlStr)
}
