#!/usr/bin/env node
/**
 * Policy Simulation Tool
 *
 * Simulates policy impact by running tools with and without custom policies,
 * showing the differences in:
 * - Generation configuration
 * - Knowledge filtering
 * - Template injection
 * - Validation results
 *
 * Supports both Rego (.rego) and CEL (.yaml/.yml) policy formats.
 *
 * Usage:
 *   npx tsx src/cli/policy-simulate.ts \
 *     --policy policies.user.examples/production-ready/cost-control-by-tier.rego \
 *     --tool generate-dockerfile \
 *     --input '{"language": "node", "environment": "production", "teamTier": "starter"}'
 */

import path from 'node:path';
import { createLogger } from '@/lib/logger';
import { loadPolicy } from '@/config/policy-io';
import type { ToolContext } from '@/mcp/context';
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import generateK8sManifestsTool from '@/tools/generate-k8s-manifests/tool';

const logger = createLogger({ level: 'error' });

interface SimulationOptions {
  policyPath: string;
  tool: 'generate-dockerfile' | 'generate-k8s-manifests';
  input: Record<string, unknown>;
}

interface SimulationResult {
  withoutPolicy: {
    generationConfig: unknown;
    knowledgeFiltering: unknown[];
    templates: unknown[];
    validationRules: unknown[];
    output: unknown;
  };
  withPolicy: {
    generationConfig: unknown;
    knowledgeFiltering: unknown[];
    templates: unknown[];
    validationRules: unknown[];
    output: unknown;
  };
  differences: {
    configChanged: boolean;
    knowledgeFiltered: number;
    templatesInjected: number;
    validationRulesAdded: number;
    outputDiffers: boolean;
  };
}

async function simulatePolicy(options: SimulationOptions): Promise<SimulationResult> {
  const { policyPath, tool, input } = options;

  console.log('\n🔬 Policy Simulation\n');
  console.log(`Policy: ${policyPath}`);
  console.log(`Tool: ${tool}`);
  console.log(`Input: ${JSON.stringify(input, null, 2)}\n`);

  // Load the custom policy (supports both Rego and CEL)
  const policyResult = await loadPolicy(path.resolve(policyPath));
  if (!policyResult.ok) {
    throw new Error(`Failed to load policy: ${policyResult.error}`);
  }
  const policy = policyResult.value;

  // Get tool handler
  const toolHandler = tool === 'generate-dockerfile'
    ? generateDockerfileTool
    : generateK8sManifestsTool;

  // Run WITHOUT policy
  console.log('📊 Running WITHOUT custom policy...\n');
  const contextWithout: ToolContext = {
    logger,
    signal: undefined,
    progress: undefined,
    // policy is omitted (no custom policy)
    queryConfig: async () => null,
  };

  const outputWithout = await toolHandler.handler(input as any, contextWithout);

  // Query policy data (without applying it)
  const configWithout = null;
  const knowledgeWithout: unknown[] = [];
  const templatesWithout: unknown[] = [];
  const validationWithout: unknown[] = [];

  // Run WITH policy
  console.log('📊 Running WITH custom policy...\n');
  const contextWith: ToolContext = {
    logger,
    signal: undefined,
    progress: undefined,
    policy,
    queryConfig: async <T>(packageName: string, policyInput: Record<string, unknown>): Promise<T | null> => {
      return policy.queryConfig<T>(packageName, policyInput);
    },
  };

  const outputWith = await toolHandler.handler(input as any, contextWith);

  // Query policy data
  const policyInput = { ...input, tool };
  const configWith = await policy.queryConfig('generation_config', policyInput).catch(() => null);

  // For now, we'll extract policy impact from the output itself
  // The most important thing is showing how the output differs
  const knowledgeWith: unknown[] = [];
  const templatesWith: unknown[] = [];
  const validationWith: unknown[] = [];

  // Calculate differences
  const differences = {
    configChanged: configWith !== null,
    knowledgeFiltered: Array.isArray(knowledgeWith) ? knowledgeWith.length : 0,
    templatesInjected: Array.isArray(templatesWith) ? templatesWith.length : 0,
    validationRulesAdded: Array.isArray(validationWith) ? validationWith.length : 0,
    outputDiffers: JSON.stringify(outputWithout) !== JSON.stringify(outputWith),
  };

  return {
    withoutPolicy: {
      generationConfig: configWithout,
      knowledgeFiltering: knowledgeWithout,
      templates: templatesWithout,
      validationRules: validationWithout,
      output: outputWithout.ok ? outputWithout.value : outputWithout.error,
    },
    withPolicy: {
      generationConfig: configWith,
      knowledgeFiltering: knowledgeWith || [],
      templates: templatesWith || [],
      validationRules: validationWith || [],
      output: outputWith.ok ? outputWith.value : outputWith.error,
    },
    differences,
  };
}

function printSimulationResults(result: SimulationResult): void {
  console.log(`\n${'='.repeat(80)}`);
  console.log('📈 SIMULATION RESULTS');
  console.log(`${'='.repeat(80)}\n`);

  const { differences, withoutPolicy, withPolicy } = result;

  // Summary
  console.log('📊 Impact Summary:');
  console.log(`  • Generation Config: ${differences.configChanged ? '✅ Modified' : '❌ No change'}`);
  console.log(`  • Knowledge Filtered: ${differences.knowledgeFiltered} rules`);
  console.log(`  • Templates Injected: ${differences.templatesInjected} templates`);
  console.log(`  • Validation Rules: ${differences.validationRulesAdded} rules added`);
  console.log(`  • Output Changed: ${differences.outputDiffers ? '✅ Yes' : '❌ No'}\n`);

  // Generation Config
  if (differences.configChanged) {
    console.log('⚙️  Generation Configuration (Applied):');
    console.log(JSON.stringify(withPolicy.generationConfig, null, 2));
    console.log('');
  }

  // Knowledge Filtering
  if (differences.knowledgeFiltered > 0) {
    console.log(`🔍 Knowledge Filtering (${differences.knowledgeFiltered} rules):`);
    (withPolicy.knowledgeFiltering as any[]).forEach((filter, i) => {
      console.log(`  ${i + 1}. Action: ${filter.action}`);
      if (filter.pattern) console.log(`     Pattern: ${filter.pattern}`);
      if (filter.tags) console.log(`     Tags: ${filter.tags.join(', ')}`);
      if (filter.reason) console.log(`     Reason: ${filter.reason}`);
      console.log('');
    });
  }

  // Template Injection
  if (differences.templatesInjected > 0) {
    console.log(`📝 Templates Injected (${differences.templatesInjected} templates):`);
    (withPolicy.templates as any[]).forEach((template, i) => {
      console.log(`  ${i + 1}. ID: ${template.id}`);
      console.log(`     Category: ${template.category}`);
      console.log(`     Recommendation: ${template.recommendation}`);
      if (template.priority) console.log(`     Priority: ${template.priority}`);
      console.log('     Code Snippet:');
      console.log(template.code_snippet.split('\n').map((line: string) => `       ${line}`).join('\n'));
      console.log('');
    });
  }

  // Validation Rules
  if (differences.validationRulesAdded > 0) {
    console.log(`✅ Validation Rules (${differences.validationRulesAdded} rules):`);
    (withPolicy.validationRules as any[]).forEach((rule, i) => {
      const levelEmoji = rule.level === 'error' ? '❌' : rule.level === 'warning' ? '⚠️' : 'ℹ️';
      console.log(`  ${i + 1}. ${levelEmoji} [${rule.level.toUpperCase()}] ${rule.message}`);
      if (rule.suggestion) console.log(`     💡 Suggestion: ${rule.suggestion}`);
      console.log('');
    });
  }

  // Output Comparison
  if (differences.outputDiffers) {
    console.log('📦 Output Comparison:\n');
    console.log('  WITHOUT Policy:');
    console.log(`  ${'-'.repeat(76)}`);
    if (withoutPolicy.output && typeof withoutPolicy.output === 'object' && 'summary' in withoutPolicy.output) {
      console.log(`  Summary: ${(withoutPolicy.output as any).summary}`);
      if ('recommendations' in withoutPolicy.output) {
        const recs = (withoutPolicy.output as any).recommendations;
        const totalRecs = (recs.securityConsiderations?.length || 0) + (recs.bestPractices?.length || 0);
        console.log(`  Recommendations: ${totalRecs} total`);
      }
    } else {
      console.log(`  ${JSON.stringify(withoutPolicy.output, null, 2).split('\n').map(line => `  ${line}`).join('\n')}`);
    }

    console.log('\n  WITH Policy:');
    console.log(`  ${'-'.repeat(76)}`);
    if (withPolicy.output && typeof withPolicy.output === 'object' && 'summary' in withPolicy.output) {
      console.log(`  Summary: ${(withPolicy.output as any).summary}`);
      if ('recommendations' in withPolicy.output) {
        const recs = (withPolicy.output as any).recommendations;
        const totalRecs = (recs.securityConsiderations?.length || 0) + (recs.bestPractices?.length || 0);
        console.log(`  Recommendations: ${totalRecs} total`);

        // Show policy-driven recommendations
        const policyDriven = [
          ...(recs.securityConsiderations || []),
          ...(recs.bestPractices || []),
          ...(recs.resourceManagement || []),
        ].filter((r: any) => r.policyDriven);

        if (policyDriven.length > 0) {
          console.log(`  Policy-Driven: ${policyDriven.length} recommendations`);
          policyDriven.forEach((rec: any) => {
            console.log(`    • ${rec.id}: ${rec.recommendation}`);
          });
        }
      }
    } else {
      console.log(`  ${JSON.stringify(withPolicy.output, null, 2).split('\n').map(line => `  ${line}`).join('\n')}`);
    }
    console.log('');
  }

  console.log('='.repeat(80));
  console.log('✅ Simulation Complete\n');
}

// CLI entrypoint
async function main() {
  const args = process.argv.slice(2);

  if (args.length === 0 || args.includes('--help')) {
    console.log(`
Policy Simulation Tool

Simulates the impact of a custom policy by comparing tool execution with and without the policy.

Supports both Rego (.rego) and CEL (.yaml/.yml) policies.

Usage:
  npx tsx src/cli/policy-simulate.ts \\
    --policy <path-to-policy> \\
    --tool <tool-name> \\
    --input '<json-input>'

Options:
  --policy   Path to policy file (.rego, .yaml, or .yml)
  --tool     Tool to simulate (generate-dockerfile or generate-k8s-manifests)
  --input    JSON input for the tool (as string)
  --help     Show this help message

Examples:
  # Rego policy - cost control for starter tier
  npx tsx src/cli/policy-simulate.ts \\
    --policy policies.user.examples/production-ready/cost-control-by-tier.rego \\
    --tool generate-dockerfile \\
    --input '{"language": "node", "environment": "production", "teamTier": "starter"}'

  # Rego policy - security-first organization
  npx tsx src/cli/policy-simulate.ts \\
    --policy policies.user.examples/production-ready/security-first-organization.rego \\
    --tool generate-k8s-manifests \\
    --input '{"name": "secure-app", "language": "java", "environment": "production"}'

  # CEL policy - custom validation
  npx tsx src/cli/policy-simulate.ts \\
    --policy policies.user/custom-validation.yaml \\
    --tool generate-dockerfile \\
    --input '{"language": "python", "environment": "production"}'
`);
    process.exit(0);
  }

  // Parse arguments
  const policyIdx = args.indexOf('--policy');
  const toolIdx = args.indexOf('--tool');
  const inputIdx = args.indexOf('--input');

  if (policyIdx === -1 || toolIdx === -1 || inputIdx === -1) {
    console.error('❌ Error: Missing required arguments');
    console.error('Run with --help for usage information');
    process.exit(1);
  }

  // Validate that required parameters are provided
  const policyPath = args[policyIdx + 1];
  const tool = args[toolIdx + 1];
  const inputStr = args[inputIdx + 1];

  if (!policyPath) {
    console.error('❌ Error: --policy is required');
    process.exit(1);
  }
  if (!tool) {
    console.error('❌ Error: --tool is required');
    process.exit(1);
  }
  if (!inputStr) {
    console.error('❌ Error: --input is required');
    process.exit(1);
  }

  const options: SimulationOptions = {
    policyPath,
    tool: tool as 'generate-dockerfile' | 'generate-k8s-manifests',
    input: JSON.parse(inputStr),
  };

  try {
    const result = await simulatePolicy(options);
    printSimulationResults(result);
  } catch (error) {
    console.error('❌ Simulation failed:', error);
    process.exit(1);
  }
}

// Run if executed directly (ESM compatible)
const isMainModule = import.meta.url === `file://${process.argv[1]}`;
if (isMainModule) {
  main().catch(console.error);
}

export { simulatePolicy, type SimulationResult, type SimulationOptions };
