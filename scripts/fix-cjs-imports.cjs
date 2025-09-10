#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

function findFiles(dir, ext) {
  const files = [];
  const items = fs.readdirSync(dir, { withFileTypes: true });
  
  for (const item of items) {
    const fullPath = path.join(dir, item.name);
    if (item.isDirectory()) {
      files.push(...findFiles(fullPath, ext));
    } else if (item.name.endsWith(ext)) {
      files.push(fullPath);
    }
  }
  
  return files;
}

function fixImports(content) {
  // Fix require statements with .js extensions for local files
  content = content.replace(/require\("(\.[^"]+)\.js"\)/g, 'require("$1")');
  content = content.replace(/require\('(\.[^']+)\.js'\)/g, "require('$1')");
  
  // Fix exports from statements with .js extensions for local files
  content = content.replace(/from "(\.[^"]+)\.js"/g, 'from "$1"');
  content = content.replace(/from '(\.[^']+)\.js'/g, "from '$1'");
  
  // For MCP SDK, the exports mapping in their package.json handles the path resolution
  // We should use the bare paths without dist/cjs prefix
  content = content.replace(
    /require\("@modelcontextprotocol\/sdk\/server\/([^"]+)\.js"\)/g,
    'require("@modelcontextprotocol/sdk/server/$1.js")'
  );
  content = content.replace(
    /require\("@modelcontextprotocol\/sdk\/([^"]+)\.js"\)/g,
    (match, path) => {
      // Keep server paths as-is, they're handled above
      if (path.startsWith('server/')) {
        return match;
      }
      // For other paths, use the bare import
      return `require("@modelcontextprotocol/sdk/${path}.js")`;
    }
  );
  
  return content;
}

function processFile(filePath) {
  try {
    let content = fs.readFileSync(filePath, 'utf8');
    const originalContent = content;
    
    content = fixImports(content);
    
    if (content !== originalContent) {
      fs.writeFileSync(filePath, content);
      console.log(`Fixed: ${path.relative(process.cwd(), filePath)}`);
    }
  } catch (error) {
    console.error(`Error processing ${filePath}:`, error.message);
  }
}

function main() {
  const distCjsDir = path.join(process.cwd(), 'dist-cjs');
  
  if (!fs.existsSync(distCjsDir)) {
    console.error('dist-cjs directory not found.');
    process.exit(1);
  }
  
  // Create package.json in dist-cjs to mark it as CommonJS
  const packageJsonPath = path.join(distCjsDir, 'package.json');
  fs.writeFileSync(packageJsonPath, JSON.stringify({ type: 'commonjs' }, null, 2));
  console.log('Created dist-cjs/package.json');
  
  const jsFiles = findFiles(distCjsDir, '.js');
  
  console.log(`Processing ${jsFiles.length} JavaScript files...`);
  
  for (const file of jsFiles) {
    processFile(file);
  }
  
  console.log('CommonJS import fix complete.');
}

if (require.main === module) {
  main();
}