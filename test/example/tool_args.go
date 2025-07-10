package example

//go:generate ../../bin/schemaGen -tool=example_tool_args -domain=example -output=.

// ExampleToolArgs demonstrates schema generation from struct tags
type ExampleToolArgs struct {
	// Name is the name to greet
	Name string `json:"name" description:"The name of the person to greet"`

	// Message is an optional custom message
	Message string `json:"message,omitempty" description:"Optional custom greeting message"`

	// Count is the number of times to repeat
	Count int `json:"count" description:"Number of times to repeat the greeting"`

	// Loud makes the greeting uppercase
	Loud bool `json:"loud,omitempty" description:"Whether to shout the greeting"`
}
