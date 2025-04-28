package prompt

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"os"
	"strings"
)

// DecodeXML decodes XML data into the provided struct
func DecodeXML(data string, target interface{}) error {
	// Create a decoder that's more tolerant to XML issues
	decoder := xml.NewDecoder(strings.NewReader(data))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity

	if err := decoder.Decode(target); err != nil && err != io.EOF {
		return fmt.Errorf("failed to decode XML: %w", err)
	}

	return nil
}

// DecodeXMLFromFile decodes XML from a file into the provided struct
func DecodeXMLFromFile(filePath string, target interface{}) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read XML file: %w", err)
	}

	return DecodeXML(string(content), target)
}

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
