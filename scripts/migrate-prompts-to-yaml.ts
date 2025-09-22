#!/usr/bin/env node

import * as fs from 'node:fs/promises';
import * as path from 'node:path';
import yaml from 'js-yaml';

interface OldPromptFormat {
  id: string;
  name?: string;
  description: string;
  version?: string;
  system?: string;
  user?: string;
  outputFormat?: string;
  variables?: Array<{
    name: string;
    description?: string;
    required?: boolean;
    default?: unknown;
  }>;
  tags?: string[];
}

interface NewPromptFormat {
  id: string;
  version: string;
  description: string;
  category: string;
  format: 'json' | 'text' | 'yaml';
  cache?: {
    enabled: boolean;
    ttl: number;
  };
  parameters: Array<{
    name: string;
    type: string;
    required: boolean;
    description?: string;
    default?: unknown;
  }>;
  template: string;
  metadata?: Record<string, unknown>;
}

function inferCategory(id: string, tags?: string[]): string {
  // Check tags first
  if (tags) {
    if (tags.includes('analysis') || id.includes('analysis')) return 'analysis';
    if (tags.includes('dockerfile') || tags.includes('containerization')) return 'containerization';
    if (tags.includes('kubernetes') || tags.includes('k8s') || tags.includes('orchestration')) return 'orchestration';
    if (tags.includes('security')) return 'security';
    if (tags.includes('validation')) return 'validation';
    if (tags.includes('sampling')) return 'sampling';
  }

  // Infer from ID
  if (id.includes('analysis') || id.includes('analyze')) return 'analysis';
  if (id.includes('dockerfile') || id.includes('container')) return 'containerization';
  if (id.includes('k8s') || id.includes('kubernetes') || id.includes('helm') || id.includes('aca')) return 'orchestration';
  if (id.includes('security') || id.includes('scan')) return 'security';
  if (id.includes('validation') || id.includes('repair')) return 'validation';
  if (id.includes('sampling') || id.includes('optimization')) return 'sampling';

  return 'analysis'; // Default
}

function inferType(name: string, defaultValue?: unknown): string {
  if (defaultValue !== undefined) {
    if (typeof defaultValue === 'string') return 'string';
    if (typeof defaultValue === 'number') return 'number';
    if (typeof defaultValue === 'boolean') return 'boolean';
    if (Array.isArray(defaultValue)) return 'array';
    if (typeof defaultValue === 'object') return 'object';
  }

  // Infer from name
  if (name.includes('port') || name.includes('count') || name.includes('replicas')) return 'number';
  if (name.includes('enable') || name.includes('is') || name.includes('has')) return 'boolean';
  if (name.includes('list') || name.includes('array') || name.includes('items')) return 'array';

  return 'string'; // Default
}

function convertToMustacheTemplate(system?: string, user?: string): string {
  if (!system && !user) {
    return 'Generate output based on the provided parameters.';
  }

  let template = '';

  if (system) {
    template += system + '\n\n';
  }

  if (user) {
    // Convert variable placeholders from {{var}} to Mustache syntax
    // The existing format already uses Mustache, so we just need to ensure it's clean
    let userTemplate = user;

    // Ensure proper formatting
    userTemplate = userTemplate.trim();

    if (template) {
      template += '## User Request\n\n' + userTemplate;
    } else {
      template = userTemplate;
    }
  }

  return template.trim();
}

async function migratePrompt(oldPrompt: OldPromptFormat): Promise<NewPromptFormat> {
  const category = inferCategory(oldPrompt.id, oldPrompt.tags);

  const parameters = (oldPrompt.variables || []).map(v => ({
    name: v.name,
    type: inferType(v.name, v.default),
    required: v.required ?? false,
    description: v.description,
    default: v.default,
  }));

  const template = convertToMustacheTemplate(oldPrompt.system, oldPrompt.user);

  const format = oldPrompt.outputFormat === 'dockerfile' ? 'text' :
                  oldPrompt.outputFormat === 'yaml' ? 'yaml' : 'json';

  return {
    id: oldPrompt.id,
    version: oldPrompt.version || '2.0.0',
    description: oldPrompt.description,
    category,
    format,
    cache: {
      enabled: true,
      ttl: category === 'analysis' ? 600 : 300, // Longer cache for analysis
    },
    parameters,
    template,
    metadata: oldPrompt.tags ? { tags: oldPrompt.tags } : undefined,
  };
}

async function migrateYamlFile(inputPath: string, outputDir: string): Promise<void> {
  console.log(`Migrating ${inputPath}...`);

  const content = await fs.readFile(inputPath, 'utf-8');
  const oldPrompt = yaml.load(content) as OldPromptFormat;

  const newPrompt = await migratePrompt(oldPrompt);

  // Determine output path based on category
  const categoryDir = path.join(outputDir, newPrompt.category);
  await fs.mkdir(categoryDir, { recursive: true });

  const outputPath = path.join(categoryDir, `${newPrompt.id}.yaml`);

  const yamlContent = yaml.dump(newPrompt, {
    lineWidth: 120,
    noRefs: true,
    sortKeys: false,
  });

  await fs.writeFile(outputPath, yamlContent);
  console.log(`  ✓ Migrated to ${outputPath}`);
}

async function migrateJsonFile(inputPath: string, outputDir: string): Promise<void> {
  console.log(`Migrating ${inputPath}...`);

  const content = await fs.readFile(inputPath, 'utf-8');
  const oldPrompt = JSON.parse(content) as OldPromptFormat;

  const newPrompt = await migratePrompt(oldPrompt);

  // Determine output path based on category
  const categoryDir = path.join(outputDir, newPrompt.category);
  await fs.mkdir(categoryDir, { recursive: true });

  const outputPath = path.join(categoryDir, `${newPrompt.id}.yaml`);

  const yamlContent = yaml.dump(newPrompt, {
    lineWidth: 120,
    noRefs: true,
    sortKeys: false,
  });

  await fs.writeFile(outputPath, yamlContent);
  console.log(`  ✓ Migrated to ${outputPath}`);
}

async function main() {
  const projectRoot = path.resolve(process.cwd());
  const outputDir = path.join(projectRoot, 'src', 'prompts');

  // Migrate YAML templates from resources/ai-templates
  const templatesDir = path.join(projectRoot, 'resources', 'ai-templates');
  try {
    const yamlFiles = await fs.readdir(templatesDir);
    for (const file of yamlFiles) {
      if (file.endsWith('.yaml')) {
        await migrateYamlFile(path.join(templatesDir, file), outputDir);
      }
    }
  } catch (error) {
    console.error('Error migrating YAML templates:', error);
  }

  // Migrate JSON prompts from src/prompts subdirectories
  const promptsDir = path.join(projectRoot, 'src', 'prompts');
  const subdirs = ['analysis', 'containerization', 'orchestration', 'validation', 'security', 'sampling'];

  for (const subdir of subdirs) {
    const subdirPath = path.join(promptsDir, subdir);
    try {
      const files = await fs.readdir(subdirPath);
      for (const file of files) {
        if (file.endsWith('.json')) {
          await migrateJsonFile(path.join(subdirPath, file), outputDir);
        }
      }
    } catch (error) {
      // Directory might not exist yet
    }
  }

  console.log('\n✅ Migration complete!');
}

// Run the migration
main().catch(console.error);