#!/usr/bin/env python3
"""Fix pipeline package logging from zerolog style to slog style"""

import os
import re

# Directory to process
pipeline_dir = "/home/tng/workspace/container-kit/pkg/mcp/application/orchestration/pipeline"

def fix_logging_style(content):
    """Convert zerolog style logging to slog style"""
    
    # Pattern for logger.Info().Str().Msg() style
    pattern = r'(\w+\.logger\.(Info|Debug|Warn|Error))\(\)\s*\.((?:[\s\n]*\.?(?:Str|Int|Bool|Err|Dur)\([^)]+\))+)\s*\.(Msg|Msgf)\(([^)]+)\)'
    
    def replace_logging(match):
        logger_prefix = match.group(1)
        method_chain = match.group(3)
        msg_type = match.group(4)
        message = match.group(5)
        
        # Extract key-value pairs from method chain
        pairs = []
        # Find all .Str("key", value) patterns
        str_matches = re.findall(r'\.Str\("([^"]+)",\s*([^)]+)\)', method_chain)
        for key, value in str_matches:
            pairs.append(f'"{key}", {value}')
            
        # Find all .Int("key", value) patterns  
        int_matches = re.findall(r'\.Int\("([^"]+)",\s*([^)]+)\)', method_chain)
        for key, value in int_matches:
            pairs.append(f'"{key}", {value}')
            
        # Find all .Bool("key", value) patterns
        bool_matches = re.findall(r'\.Bool\("([^"]+)",\s*([^)]+)\)', method_chain)
        for key, value in bool_matches:
            pairs.append(f'"{key}", {value}')
            
        # Find all .Dur("key", value) patterns
        dur_matches = re.findall(r'\.Dur\("([^"]+)",\s*([^)]+)\)', method_chain)
        for key, value in dur_matches:
            pairs.append(f'"{key}", {value}')
            
        # Find all .Err(err) patterns
        err_matches = re.findall(r'\.Err\(([^)]+)\)', method_chain)
        for err in err_matches:
            pairs.append(f'"error", {err}')
        
        # Build the new slog-style call
        if pairs:
            args = ', '.join(pairs)
            return f'{logger_prefix}({message}, {args})'
        else:
            return f'{logger_prefix}({message})'
    
    # Apply the replacement
    content = re.sub(pattern, replace_logging, content, flags=re.MULTILINE | re.DOTALL)
    
    # Also fix simple logger calls without chaining
    content = re.sub(r'(\w+\.logger\.(Info|Debug|Warn|Error))\(\)\s*\.(Msg|Msgf)\(([^)]+)\)',
                     r'\1(\4)', content)
    
    return content

# Process all Go files in the directory
for filename in os.listdir(pipeline_dir):
    if not filename.endswith(".go") or filename.endswith("_test.go"):
        continue
    
    filepath = os.path.join(pipeline_dir, filename)
    with open(filepath, 'r') as f:
        content = f.read()
    
    original_content = content
    content = fix_logging_style(content)
    
    if content != original_content:
        with open(filepath, 'w') as f:
            f.write(content)
        print(f"Fixed logging in {filename}")

print("Done fixing pipeline logging!")