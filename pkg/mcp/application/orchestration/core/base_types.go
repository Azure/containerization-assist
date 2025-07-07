package core

// BaseToolArgs provides common fields for all tool arguments
type BaseToolArgs struct {
	SessionID string `json:"session_id,omitempty" description:"Session ID for tracking operations"`
	NoSession bool   `json:"no_session,omitempty" description:"Skip session tracking"`
}
