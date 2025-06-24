package tools

// removeCopilotIncompatible recursively strips meta-schema fields that
// Copilot's AJV-Draft-7 validator cannot handle.
func removeCopilotIncompatible(node map[string]any) {
	delete(node, "$schema")        // drop any draft URI
	delete(node, "$id")            // AJV rejects nested id
	delete(node, "$dynamicRef")    // draft-2020 keyword
	delete(node, "$dynamicAnchor") // draft-2020 keyword

	// draft-2020 unevaluatedProperties is also unsupported
	delete(node, "unevaluatedProperties")

	for _, v := range node { // walk children
		switch child := v.(type) {
		case map[string]any:
			removeCopilotIncompatible(child)
		case []any:
			for _, elem := range child {
				if m, ok := elem.(map[string]any); ok {
					removeCopilotIncompatible(m)
				}
			}
		}
	}
}
