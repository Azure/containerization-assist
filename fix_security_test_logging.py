#!/usr/bin/env python3
"""Fix security test logging from zerolog to slog"""

import re

filepath = "/home/tng/workspace/container-kit/pkg/core/security/secret_discovery_test.go"

with open(filepath, 'r') as f:
    content = f.read()

# Replace zerolog.Nop() with slog.New(slog.NewTextHandler(os.Stdout, nil))
content = re.sub(r'logger := zerolog\.Nop\(\)', 
                 'logger := slog.New(slog.NewTextHandler(os.Stdout, nil))', 
                 content)

# Replace other zerolog references
content = re.sub(r'zerolog\.New\(os\.Stdout\)', 
                 'slog.New(slog.NewTextHandler(os.Stdout, nil))', 
                 content)

# Write back
with open(filepath, 'w') as f:
    f.write(content)

print("Fixed security test logging")