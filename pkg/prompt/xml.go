package prompt

import (
	"encoding/xml"
	"fmt"
	"html"
	"os"
	"strings"
)

// Takes XML struct and outputs it to a file
func EncodeXMLToFile(data interface{}, filePath string) error {
	// Encode to string first to avoid escaping issues
	xmlStr, err := EncodeXMLToString(data)
	if err != nil {
		return fmt.Errorf("failed to encode XML to string: %w", err)
	}

	humanReadable := UnescapeXML(xmlStr)

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create XML file: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(humanReadable)
	if err != nil {
		return fmt.Errorf("failed to write XML to file: %w", err)
	}

	return nil
}

// EncodeXMLToString encodes the provided struct into an XML string without unwanted HTML entities.
func EncodeXMLToString(data interface{}) (string, error) {
	var sb strings.Builder
	encoder := xml.NewEncoder(&sb)
	encoder.Indent("", "  ")

	if err := encoder.Encode(data); err != nil {
		return "", fmt.Errorf("failed to encode XML to string: %w", err)
	}

	if err := encoder.Flush(); err != nil {
		return "", fmt.Errorf("failed to flush encoder: %w", err)
	}

	// Return raw string with XML header
	return sb.String(), nil
}

// HumanReadableXML converts XML with HTML entities to a human-readable format
func UnescapeXML(s string) string {
	return html.UnescapeString(s)
}

// PrintXMLContent prints the XML content in a human-readable format
func PrintXMLContent(content string) {
	fmt.Println(UnescapeXML(content))
}
