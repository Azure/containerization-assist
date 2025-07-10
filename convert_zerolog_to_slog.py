#!/usr/bin/env python3
import re
import sys
import os

def convert_zerolog_to_slog(content):
    """Convert zerolog-style logging to slog-style"""
    
    # Pattern to match zerolog style logging chains
    # This matches logger.Level()...Msg("message")
    pattern = r'(\s*)(\w+\.)?logger\.(Debug|Info|Warn|Error)\(\)((?:\s*\.\s*\w+\([^)]*\))*)\s*\.\s*Msg\("([^"]*)"\)'
    
    def replace_match(match):
        indent = match.group(1)
        prefix = match.group(2) or ''
        level = match.group(3)
        chain = match.group(4)
        message = match.group(5)
        
        # Extract key-value pairs from the chain
        kv_pattern = r'\.\s*(\w+)\(([^)]*)\)'
        kv_matches = re.findall(kv_pattern, chain)
        
        # Build the new slog-style call
        args = []
        for method, value in kv_matches:
            if method == 'Str':
                # Extract the key and value from Str("key", value)
                str_match = re.match(r'"([^"]+)",\s*(.+)', value)
                if str_match:
                    key = str_match.group(1)
                    val = str_match.group(2).strip()
                    args.append(f'"{key}", {val}')
            elif method == 'Int':
                # Extract the key and value from Int("key", value)
                int_match = re.match(r'"([^"]+)",\s*(.+)', value)
                if int_match:
                    key = int_match.group(1)
                    val = int_match.group(2).strip()
                    args.append(f'"{key}", {val}')
            elif method == 'Err':
                # Error is typically just Err(err)
                args.append(f'"error", {value}')
            elif method == 'Bool':
                bool_match = re.match(r'"([^"]+)",\s*(.+)', value)
                if bool_match:
                    key = bool_match.group(1)
                    val = bool_match.group(2).strip()
                    args.append(f'"{key}", {val}')
            elif method == 'Interface' or method == 'Any':
                interface_match = re.match(r'"([^"]+)",\s*(.+)', value)
                if interface_match:
                    key = interface_match.group(1)
                    val = interface_match.group(2).strip()
                    args.append(f'"{key}", {val}')
        
        # Build the new logging call
        if args:
            args_str = ',\n' + indent + '\t' + (',\n' + indent + '\t').join(args)
            return f'{indent}{prefix}logger.{level}("{message}"{args_str})'
        else:
            return f'{indent}{prefix}logger.{level}("{message}")'
    
    # Replace all matches
    content = re.sub(pattern, replace_match, content, flags=re.MULTILINE | re.DOTALL)
    
    # Also handle simple .Msg() calls without any fields
    simple_pattern = r'(\s*)(\w+\.)?logger\.(Debug|Info|Warn|Error)\(\)\s*\.\s*Msg\("([^"]*)"\)'
    content = re.sub(simple_pattern, r'\1\2logger.\3("\4")', content)
    
    return content

def process_file(filepath):
    """Process a single file"""
    try:
        with open(filepath, 'r') as f:
            content = f.read()
        
        # Check if file needs conversion
        if 'logger.' not in content or '.Msg(' not in content:
            return False
            
        # Convert the content
        new_content = convert_zerolog_to_slog(content)
        
        # Only write if content changed
        if new_content != content:
            with open(filepath, 'w') as f:
                f.write(new_content)
            print(f"Converted: {filepath}")
            return True
        return False
    except Exception as e:
        print(f"Error processing {filepath}: {e}")
        return False

def main():
    if len(sys.argv) > 1:
        # Process specific files
        for filepath in sys.argv[1:]:
            process_file(filepath)
    else:
        # Process all files from zerolog_files.txt
        if os.path.exists('zerolog_files.txt'):
            with open('zerolog_files.txt', 'r') as f:
                files = f.read().strip().split('\n')
            
            converted = 0
            for filepath in files:
                if filepath and process_file(filepath):
                    converted += 1
            
            print(f"\nConverted {converted} files")
        else:
            print("No zerolog_files.txt found. Run find_zerolog_usage.sh first.")

if __name__ == '__main__':
    main()