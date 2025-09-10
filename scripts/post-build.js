#!/usr/bin/env node
import { readFile, writeFile, chmod, cp, mkdir } from 'fs/promises';
import { existsSync } from 'fs';
import { execSync } from 'child_process';
import { join } from 'path';

console.log('🔧 Running post-build tasks...');

// Generate TypeScript declarations (skip if requested or if there are compilation errors)
if (process.env.SKIP_DECLARATIONS === 'true') {
  console.log('⏩ Skipping TypeScript declaration generation (SKIP_DECLARATIONS=true)');
} else {
  try {
    console.log('📝 Generating TypeScript declarations...');
    // Generate declarations synchronously
    execSync('npx tsc --emitDeclarationOnly --outDir dist --skipLibCheck --skipDefaultLibCheck --incremental --tsBuildInfoFile .tsbuildinfo', { stdio: 'pipe' });
    console.log('✅ TypeScript declarations generated');
  } catch (error) {
    console.warn('⚠️  Warning: Could not generate TypeScript declarations:', error.message);
    console.log('💡 Set SKIP_DECLARATIONS=true to skip this step during development');
  }
}

// Add shebang to CLI file
const cliPath = join('dist', 'apps', 'cli.js');
if (existsSync(cliPath)) {
  console.log('🔧 Processing CLI executable...');
  const content = await readFile(cliPath, 'utf-8');
  if (!content.startsWith('#!/usr/bin/env node')) {
    await writeFile(cliPath, `#!/usr/bin/env node\n${content}`);
    console.log('✅ Shebang added to CLI');
  }
  // Make CLI executable
  await chmod(cliPath, 0o755)
    .then(() => console.log('✅ CLI made executable'))
    .catch((err) => console.warn('⚠️  Warning: Could not make CLI executable:', err.message));
}

// Copy AI prompt templates if they exist
const templatesSource = join('src', 'infrastructure', 'ai', 'prompts', 'templates');
const templatesDest = join('dist', 'infrastructure', 'ai', 'prompts', 'templates');

if (existsSync(templatesSource)) {
  console.log('📋 Copying AI prompt templates...');
  try {
    // Ensure destination directory exists
    await mkdir(join('dist', 'infrastructure', 'ai', 'prompts'), { recursive: true });
    await cp(templatesSource, templatesDest, { recursive: true });
    console.log('✅ AI prompt templates copied');
  } catch (err) {
    console.warn('⚠️  Warning: Could not copy templates:', err.message);
  }
}

console.log('🎉 Build complete with all post-build tasks finished!');