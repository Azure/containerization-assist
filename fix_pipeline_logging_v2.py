#!/usr/bin/env python3
"""Fix pipeline package logging from zerolog style to slog style"""

import os
import re

# Files to process
files = [
    "/home/tng/workspace/container-kit/pkg/mcp/application/orchestration/pipeline/atomic_framework.go",
    "/home/tng/workspace/container-kit/pkg/mcp/application/orchestration/pipeline/monitoring_integration.go", 
    "/home/tng/workspace/container-kit/pkg/mcp/application/orchestration/pipeline/basic_validator.go",
    "/home/tng/workspace/container-kit/pkg/mcp/application/orchestration/pipeline/cache_service.go"
]

for filepath in files:
    with open(filepath, 'r') as f:
        lines = f.readlines()
    
    new_lines = []
    i = 0
    while i < len(lines):
        line = lines[i]
        
        # Check if this line starts a logger chain
        if re.search(r'\.logger\.(Info|Debug|Warn|Error)\(\)\s*\.', line):
            # Collect the full statement
            statement_lines = [line]
            j = i + 1
            
            # Continue collecting lines until we find Msg() or Msgf()
            while j < len(lines) and not re.search(r'\.Msg(f)?\(', lines[j-1]):
                statement_lines.append(lines[j])
                j += 1
            
            # Join the statement
            full_statement = ''.join(statement_lines).strip()
            
            # Extract components
            logger_match = re.match(r'(\s*)(.+\.logger\.(Info|Debug|Warn|Error))\(\)', full_statement)
            if logger_match:
                indent = logger_match.group(1)
                logger_call = logger_match.group(2)
                level = logger_match.group(3)
                
                # Extract message
                msg_match = re.search(r'\.Msg(f)?\(([^)]+)\)', full_statement)
                if msg_match:
                    is_msgf = msg_match.group(1) == 'f'
                    message = msg_match.group(2)
                    
                    # Extract key-value pairs
                    pairs = []
                    
                    # .Str("key", value)
                    for match in re.finditer(r'\.Str\("([^"]+)",\s*([^)]+)\)', full_statement):
                        pairs.append((match.group(1), match.group(2)))
                    
                    # .Int("key", value)
                    for match in re.finditer(r'\.Int\("([^"]+)",\s*([^)]+)\)', full_statement):
                        pairs.append((match.group(1), match.group(2)))
                    
                    # .Bool("key", value)
                    for match in re.finditer(r'\.Bool\("([^"]+)",\s*([^)]+)\)', full_statement):
                        pairs.append((match.group(1), match.group(2)))
                    
                    # .Dur("key", value)
                    for match in re.finditer(r'\.Dur\("([^"]+)",\s*([^)]+)\)', full_statement):
                        pairs.append((match.group(1), match.group(2)))
                    
                    # .Err(err)
                    for match in re.finditer(r'\.Err\(([^)]+)\)', full_statement):
                        pairs.append(("error", match.group(1)))
                    
                    # Build slog-style call
                    if pairs:
                        kv_args = ', '.join([f'"{k}", {v}' for k, v in pairs])
                        new_statement = f'{indent}{logger_call}({message}, {kv_args})\n'
                    else:
                        new_statement = f'{indent}{logger_call}({message})\n'
                    
                    new_lines.append(new_statement)
                    i = j
                    continue
        
        new_lines.append(line)
        i += 1
    
    # Write back
    with open(filepath, 'w') as f:
        f.writelines(new_lines)
    
    print(f"Fixed {os.path.basename(filepath)}")

print("Done!")