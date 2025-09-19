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
  
  // Copy prompts directory for CommonJS build
  const promptsSource = path.join(process.cwd(), 'src', 'prompts');
  const promptsDest = path.join(distCjsDir, 'src', 'prompts');
  
  if (fs.existsSync(promptsSource)) {
    console.log('Copying prompts directory for CommonJS build...');
    try {
      fs.mkdirSync(path.join(distCjsDir, 'src'), { recursive: true });
      copyDirRecursive(promptsSource, promptsDest, (file) => file.endsWith('.json') || file.endsWith('.yaml') || file.endsWith('.yml') || file.endsWith('.ts'));
      console.log('Prompts directory copied to dist-cjs');
    } catch (err) {
      console.warn('Warning: Could not copy prompts:', err.message);
    }
  }
  
  // Copy knowledge data for CommonJS build
  const knowledgeSource = path.join(process.cwd(), 'src', 'knowledge', 'data');
  const knowledgeDest = path.join(distCjsDir, 'src', 'knowledge', 'data');
  
  if (fs.existsSync(knowledgeSource)) {
    console.log('Copying knowledge data for CommonJS build...');
    try {
      fs.mkdirSync(path.join(distCjsDir, 'src', 'knowledge'), { recursive: true });
      copyDirRecursive(knowledgeSource, knowledgeDest, (file) => file.endsWith('.json'));
      console.log('Knowledge data copied to dist-cjs');
    } catch (err) {
      console.warn('Warning: Could not copy knowledge data:', err.message);
    }
  }
  
  const jsFiles = findFiles(distCjsDir, '.js');
  
  console.log(`Processing ${jsFiles.length} JavaScript files...`);
  
  for (const file of jsFiles) {
    processFile(file);
  }
  
  console.log('CommonJS import fix complete.');
}

function copyDirRecursive(src, dest, filter = null) {
  if (!fs.existsSync(dest)) {
    fs.mkdirSync(dest, { recursive: true });
  }
  
  const items = fs.readdirSync(src, { withFileTypes: true });
  
  for (const item of items) {
    const srcPath = path.join(src, item.name);
    const destPath = path.join(dest, item.name);
    
    if (item.isDirectory()) {
      copyDirRecursive(srcPath, destPath, filter);
    } else if (!filter || filter(item.name)) {
      fs.copyFileSync(srcPath, destPath);
    }
  }
}

if (require.main === module) {
  main();
}