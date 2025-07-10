#!/usr/bin/env python3
"""Fix conversation package imports to use domaintypes instead of internal"""

import os
import re

# Directory to process
conversation_dir = "/home/tng/workspace/container-kit/pkg/mcp/application/internal/conversation"

# Process all Go files in the directory
for filename in os.listdir(conversation_dir):
    if not filename.endswith(".go"):
        continue
    
    filepath = os.path.join(conversation_dir, filename)
    with open(filepath, 'r') as f:
        content = f.read()
    
    original_content = content
    
    # Add import if needed
    if 'internal.' in content and 'domaintypes "github.com/Azure/container-kit/pkg/mcp/domain/types"' not in content:
        # Find the import section
        import_match = re.search(r'import \((.*?)\)', content, re.DOTALL)
        if import_match:
            imports = import_match.group(1)
            # Add the domaintypes import
            new_imports = imports.rstrip() + '\n\tdomaintypes "github.com/Azure/container-kit/pkg/mcp/domain/types"\n'
            content = content.replace(import_match.group(1), new_imports)
    
    # Replace internal.BaseToolArgs with domaintypes.BaseToolArgs
    content = re.sub(r'\binternal\.BaseToolArgs\b', 'domaintypes.BaseToolArgs', content)
    content = re.sub(r'\binternal\.BaseToolResponse\b', 'domaintypes.BaseToolResponse', content)
    content = re.sub(r'\binternal\.NewBaseResponse\b', 'domaintypes.NewBaseResponse', content)
    
    # Replace internal stages with domain types
    content = re.sub(r'\binternal\.ConversationStage\b', 'domaintypes.ConversationStage', content)
    content = re.sub(r'\binternal\.Stage(\w+)\b', r'domaintypes.Stage\1', content)
    
    # Replace other internal types
    content = re.sub(r'\binternal\.SessionManagerStats\b', 'domaintypes.SessionManagerStats', content)
    content = re.sub(r'\binternal\.UserPreferences\b', 'domaintypes.UserPreferences', content)
    content = re.sub(r'\binternal\.K8sManifest\b', 'domaintypes.K8sManifest', content)
    content = re.sub(r'\binternal\.ToolError\b', 'domaintypes.ToolError', content)
    content = re.sub(r'\binternal\.PreferenceStore\b', 'domaintypes.PreferenceStore', content)
    
    # Write back if changed
    if content != original_content:
        with open(filepath, 'w') as f:
            f.write(content)
        print(f"Updated {filename}")
    else:
        print(f"No changes needed for {filename}")

print("Done!")