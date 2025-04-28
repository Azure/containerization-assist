package prompt

import (
	"encoding/xml"
	"io"
	"strings"
)

// RawString is a custom type that treats XML content as raw strings
// without requiring CDATA tags or escaping special characters
type RawString string

// MarshalXML implements xml.Marshaler interface to marshal the content as raw XML
func (r RawString) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.CharData(r)); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.EndElement{Name: start.Name}); err != nil {
		return err
	}
	return nil
}

// UnmarshalXML implements xml.Unmarshaler interface to unmarshal content as a raw string
func (r *RawString) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var buf strings.Builder

	for {
		token, err := d.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		switch t := token.(type) {
		case xml.CharData:
			buf.Write(t)
		case xml.EndElement:
			if t.Name == start.Name {
				*r = RawString(buf.String())
				return nil
			}
		}
	}

	return nil
}
