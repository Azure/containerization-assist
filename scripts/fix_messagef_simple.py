#!/usr/bin/env python3

import os
import re
import sys

def fix_messagef_in_file(filepath):
    """Fix Messagef calls with %w to use Message().Cause() pattern"""

    with open(filepath, 'r') as f:
        content = f.read()

    original_content = content

    # Pattern 1: Simple Messagef with only %w at the end
    # .Messagef("message: %w", err) -> .Message("message").Cause(err)
    content = re.sub(
        r'\.Messagef\("([^"]*?):\s*%w"\s*,\s*([a-zA-Z0-9_]+)\)',
        r'.Message("\1").Cause(\2)',
        content
    )

    content = re.sub(
        r'\.Messagef\("([^"]*?)\s+%w"\s*,\s*([a-zA-Z0-9_]+)\)',
        r'.Message("\1").Cause(\2)',
        content
    )

    content = re.sub(
        r'\.Messagef\("([^"]*?)%w"\s*,\s*([a-zA-Z0-9_]+)\)',
        r'.Message("\1").Cause(\2)',
        content
    )

    # Pattern 2: Messagef with other format parameters
    # This is more complex - we need to handle each case
    lines = content.split('\n')
    new_lines = []

    for i, line in enumerate(lines):
        if 'Messagef' in line and '%w' in line and '.Cause(' not in line:
            # Check if this line has other format verbs
            match = re.search(r'\.Messagef\("([^"]+)"\s*,(.+)\)', line)
            if match:
                format_str = match.group(1)
                args_str = match.group(2).strip()

                # Count format verbs
                format_verbs = re.findall(r'%[^%\s]', format_str)

                if '%w' in format_str and len(format_verbs) > 1:
                    # Has other format verbs besides %w
                    # Remove %w and its separator
                    new_format = format_str.replace(': %w', '')
                    new_format = new_format.replace(' %w', '')
                    new_format = new_format.replace('%w', '')

                    # Split arguments
                    # This is tricky because arguments can contain commas
                    # We'll use a simple heuristic: the last argument is the error
                    args_parts = []
                    paren_depth = 0
                    current_arg = ''

                    for char in args_str:
                        if char == '(' or char == '{' or char == '[':
                            paren_depth += 1
                        elif char == ')' or char == '}' or char == ']':
                            paren_depth -= 1
                        elif char == ',' and paren_depth == 0:
                            args_parts.append(current_arg.strip())
                            current_arg = ''
                            continue
                        current_arg += char

                    if current_arg:
                        args_parts.append(current_arg.strip())

                    if len(args_parts) >= len(format_verbs):
                        # The last argument should be the error
                        error_arg = args_parts[-1].rstrip(')')
                        other_args = args_parts[:-1]

                        # Rebuild the line
                        if other_args:
                            new_args = ', '.join(other_args)
                            new_line = line.replace(
                                f'.Messagef("{format_str}", {args_str}',
                                f'.Message(fmt.Sprintf("{new_format}", {new_args})).Cause({error_arg}'
                            )

                            # Check if we need to add fmt import
                            if 'fmt.Sprintf' in new_line and 'import' in '\n'.join(lines[:20]):
                                # This is a rough check - proper would parse imports
                                pass
                        else:
                            # No other arguments, just the error
                            new_line = line.replace(
                                f'.Messagef("{format_str}", {args_str}',
                                f'.Message("{new_format}").Cause({error_arg}'
                            )

                        line = new_line

        new_lines.append(line)

    content = '\n'.join(new_lines)

    if content != original_content:
        # Check if we need to add fmt import
        if 'fmt.Sprintf' in content and 'import "fmt"' not in content:
            # Find the import block
            import_match = re.search(r'import\s*\(([^)]+)\)', content)
            if import_match:
                imports = import_match.group(1)
                if '"fmt"' not in imports:
                    # Add fmt import
                    new_imports = imports.rstrip() + '\n\t"fmt"\n'
                    content = content.replace(import_match.group(0), f'import ({new_imports})')

        with open(filepath, 'w') as f:
            f.write(content)

        print(f"✅ Fixed {filepath}")
        return True

    return False

def main():
    fixed_count = 0

    # Walk through pkg/mcp directory
    for root, dirs, files in os.walk('pkg/mcp'):
        # Skip vendor directories
        if 'vendor' in root:
            continue

        for file in files:
            if file.endswith('.go'):
                filepath = os.path.join(root, file)

                # Quick check if file needs processing
                try:
                    with open(filepath, 'r') as f:
                        content = f.read()
                        if 'Messagef' in content and '%w' in content:
                            if fix_messagef_in_file(filepath):
                                fixed_count += 1
                except Exception as e:
                    print(f"❌ Error processing {filepath}: {e}")

    print(f"\n🎉 Fixed {fixed_count} files!")
    print("\n📋 Next steps:")
    print("1. Review changes: git diff")
    print("2. Run tests: make test")
    print("3. Commit if tests pass")

if __name__ == '__main__':
    main()
