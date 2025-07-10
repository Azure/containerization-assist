package logging

import (
	"log/slog"
)

// Standards is now a direct alias to slog.Logger
// This eliminates the adapter pattern and uses slog directly
type Standards = *slog.Logger
