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

// Copy prompts directory for runtime use
const promptsSource = join('src', 'prompts');
const promptsDest = join('dist', 'src', 'prompts');

if (existsSync(promptsSource)) {
  console.log('📋 Copying prompts directory...');
  try {
    // Ensure destination directory exists
    await mkdir(join('dist', 'src'), { recursive: true });
    await cp(promptsSource, promptsDest, { recursive: true, filter: (source) => {
      // Copy only JSON files and directories
      return source.endsWith('.json') || !source.includes('.');
    }});
    console.log('✅ Prompts directory copied');
  } catch (err) {
    console.warn('⚠️  Warning: Could not copy prompts:', err.message);
  }
}

// Copy knowledge data directory for runtime use
const knowledgeSource = join('src', 'knowledge', 'data');
const knowledgeDest = join('dist', 'src', 'knowledge', 'data');

if (existsSync(knowledgeSource)) {
  console.log('📚 Copying knowledge data...');
  try {
    // Ensure destination directory exists
    await mkdir(join('dist', 'src', 'knowledge'), { recursive: true });
    await cp(knowledgeSource, knowledgeDest, { recursive: true });
    console.log('✅ Knowledge data copied');
  } catch (err) {
    console.warn('⚠️  Warning: Could not copy knowledge data:', err.message);
  }
}

console.log('🎉 Build complete with all post-build tasks finished!');