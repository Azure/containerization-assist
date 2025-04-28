package prompt

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
)

// DecodeXML decodes XML data into the provided struct
func DecodeXML(data string, target interface{}) error {
	// Create a decoder that's more tolerant of XML issues
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

// EncodeXML encodes the provided struct into XML and writes it to the given writer
func EncodeXML(data interface{}, w io.Writer) error {
	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode XML: %w", err)
	}

	if err := encoder.Flush(); err != nil {
		return fmt.Errorf("failed to flush encoder: %w", err)
	}

	return nil
}

// EncodeXMLToString encodes the provided struct to an XML string
func EncodeXMLStructToString(data interface{}) (string, error) {
	var sb strings.Builder
	if err := EncodeXML(data, &sb); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// EncodeXMLToFile encodes the provided struct to XML and writes it to a file
func EncodeXMLToFile(data interface{}, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create XML file: %w", err)
	}
	defer file.Close()

	if err := EncodeXML(data, file); err != nil {
		return fmt.Errorf("failed to write XML to file: %w", err)
	}

	return nil
}

// PrintXMLContent prints XML content to the console
func PrintXMLContent(content string) {
	fmt.Println(content)
}
