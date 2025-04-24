package ai

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
)

// DecodeXML parses XML data into the provided struct.
// The target parameter should be a pointer to a struct that has appropriate xml tags.
func DecodeXML(data string, target interface{}) error {
	reader := strings.NewReader(data)
	decoder := xml.NewDecoder(reader)

	err := decoder.Decode(target)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to decode XML: %w", err)
	}

	return nil
}

// DecodeXMLFromReader parses XML data from an io.Reader into the provided struct.
// The target parameter should be a pointer to a struct that has appropriate xml tags.
func DecodeXMLFromReader(reader io.Reader, target interface{}) error {
	decoder := xml.NewDecoder(reader)

	err := decoder.Decode(target)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to decode XML from reader: %w", err)
	}

	return nil
}

// DecodeXMLFromFile parses XML data from a file into the provided struct.
// The target parameter should be a pointer to a struct that has appropriate xml tags.
func DecodeXMLFromFile(filePath string, target interface{}) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open XML file: %w", err)
	}
	defer file.Close()

	decoder := xml.NewDecoder(file)

	err = decoder.Decode(target)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to decode XML from file: %w", err)
	}

	return nil
}
