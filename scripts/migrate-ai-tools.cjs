#!/usr/bin/env node

/**
 * Script to migrate AI tools to use the new prompt engine
 * This script finds all remaining AI tools using the old pattern
 * and updates them to use buildMessages from prompt-engine.ts
 */

const fs = require('fs');
const path = require('path');

// Tools to migrate
const toolsToMigrate = [
  'generate-helm-charts',
  'generate-aca-manifests',
  'convert-aca-to-k8s',
  'analyze-repo',
  'resolve-base-images'
];

// Topic mapping for each tool
const topicMap = {
  'generate-helm-charts': 'generate_helm_charts',
  'generate-aca-manifests': 'generate_aca_manifests',
  'convert-aca-to-k8s': 'convert_aca_to_k8s',
  'analyze-repo': 'analyze_repository',
  'resolve-base-images': 'resolve_base_images'
};

// Contract names for each tool
const contractMap = {
  'generate-helm-charts': 'helm_chart_v1',
  'generate-aca-manifests': 'aca_manifests_v1',
  'convert-aca-to-k8s': 'aca_to_k8s_v1',
  'analyze-repo': 'repository_analysis_v1',
  'resolve-base-images': 'base_images_v1'
};

// Knowledge budget for each tool
const knowledgeBudgetMap = {
  'generate-helm-charts': 4000,
  'generate-aca-manifests': 3500,
  'convert-aca-to-k8s': 3000,
  'analyze-repo': 2500,
  'resolve-base-images': 2000
};

function migrateToolFile(toolName) {
  const toolPath = path.join(__dirname, '..', 'src', 'tools', toolName, 'tool.ts');

  if (!fs.existsSync(toolPath)) {
    console.log(`❌ Tool file not found: ${toolPath}`);
    return false;
  }

  let content = fs.readFileSync(toolPath, 'utf8');
  const originalContent = content;

  // Check if already migrated
  if (content.includes('buildMessages')) {
    console.log(`✅ ${toolName} already migrated`);
    return true;
  }

  // Replace imports
  content = content.replace(
    /import \{ applyPolicyConstraints \} from ['"]@\/config\/policy-prompt['"];?\n/g,
    ''
  );

  content = content.replace(
    /import \{ enhancePrompt \} from ['"]\.\.\/knowledge-helper['"];?\n/g,
    ''
  );

  // Add buildMessages import after other imports
  if (!content.includes('buildMessages')) {
    const lastImportMatch = content.match(/import[^;]+from[^;]+;/g);
    if (lastImportMatch) {
      const lastImport = lastImportMatch[lastImportMatch.length - 1];
      const insertPosition = content.indexOf(lastImport) + lastImport.length;
      content = content.slice(0, insertPosition) +
        "\nimport { buildMessages } from '@/ai/prompt-engine';" +
        content.slice(insertPosition);
    }
  }

  // Find and replace the 3-step pattern
  const pattern = /\/\/ Enhance with knowledge base[\s\S]*?\/\/ Apply policy constraints[\s\S]*?constrained\);/g;

  if (pattern.test(content)) {
    content = content.replace(pattern, (match) => {
      const topic = topicMap[toolName];
      const contract = contractMap[toolName];
      const budget = knowledgeBudgetMap[toolName];

      return `  // Build messages using the new prompt engine
  const messages = await buildMessages({
    basePrompt,
    topic: '${topic}',
    tool: '${toolName}',
    environment: validatedParams.environment || 'production',
    contract: {
      name: '${contract}',
      description: 'Generate ${toolName.replace(/-/g, ' ')}'
    },
    knowledgeBudget: ${budget}
  });`;
    });

    // Update the AI execution call
    content = content.replace(
      /messages:\s*\[\s*\{\s*role:\s*['"]user['"],\s*content:\s*\[\s*\{\s*type:\s*['"]text['"],\s*text:\s*constrained\s*\}\s*\]\s*\}\s*\]/g,
      '...messages'
    );

    // Save the migrated file using atomic write to avoid race condition
    const tmpPath = toolPath + '.tmp';
    fs.writeFileSync(tmpPath, content, 'utf8');
    fs.renameSync(tmpPath, toolPath);

    // Check if migration was successful
    if (content !== originalContent) {
      console.log(`✅ Migrated ${toolName}`);
      return true;
    } else {
      console.log(`⚠️  No changes made to ${toolName}`);
      return false;
    }
  } else {
    console.log(`⚠️  Pattern not found in ${toolName}, needs manual migration`);
    return false;
  }
}

// Main execution
console.log('🔄 Starting AI tools migration...\n');

let successCount = 0;
let failCount = 0;

for (const tool of toolsToMigrate) {
  console.log(`Processing ${tool}...`);
  if (migrateToolFile(tool)) {
    successCount++;
  } else {
    failCount++;
  }
  console.log('');
}

console.log(`\n📊 Migration Summary:`);
console.log(`   ✅ Successfully migrated: ${successCount} tools`);
console.log(`   ❌ Failed or needs manual work: ${failCount} tools`);

if (failCount > 0) {
  console.log('\n⚠️  Some tools need manual migration. Please check them individually.');
  process.exit(1);
} else {
  console.log('\n🎉 All tools successfully migrated!');
  process.exit(0);
}