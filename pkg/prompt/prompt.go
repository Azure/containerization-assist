package prompt

import (
	"encoding/xml"
	"fmt"
)

// Prompt represents the root XML element in prompt templates
type Prompt struct {
	Role         RawString   `xml:"role"`
	Context      interface{} `xml:"context"`
	Instructions RawString   `xml:"instructions"`
	Note         RawString   `xml:"note"`
}

// Element represents a section with description and content
// If the content is empty, it will be omitted from the LLM Prompt
type Element struct {
	Description RawString `xml:"description"`
	Content     RawString `xml:"content,omitempty"`
}

func (e *Element) GetContent() string {
	return string(e.Content)
}

func (e *Element) SetContent(content string) {
	e.Content = RawString(content)
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
	XMLName      xml.Name  `xml:"response"`
	Analysis     RawString `xml:"analysis"`
	Explanation  RawString `xml:"explanation"`
	FixedContent RawString `xml:"fixed_content"`
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

	var response LLMResponse

	// Use our custom decoder that's more tolerant of XML issues
	err := DecodeXML(responseText, &response)

	// If that fails, try wrapping in a response tag
	if err != nil {
		wrappedResponse := "<response>" + responseText + "</response>"
		err = DecodeXML(wrappedResponse, &response)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal LLM response: %w", err)
		}
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
