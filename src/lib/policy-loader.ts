import fs from 'node:fs';
import path from 'node:path';
import yaml from 'js-yaml';

export interface PolicyRule {
  id: string; // e.g., "no-latest-tags"
  text: string; // human-readable "must" line
  priority?: number; // lower = earlier
  tools?: string[]; // if present, subset of tools this applies to
  when?: Record<string, unknown>; // optional param-based filter
}

export interface PolicyConfig {
  rules: PolicyRule[];
}

interface UnifiedPolicyRule {
  name: string;
  description: string;
  points?: number;
  weight?: number;
  matcher?: {
    type: string;
    function?: string;
    pattern?: string;
    flags?: string;
  };
}

interface UnifiedPolicy {
  rules: {
    [contentType: string]: {
      [category: string]: UnifiedPolicyRule[] | any;
    };
  };
}

function loadUnifiedPolicy(filePath: string): UnifiedPolicy {
  const raw = fs.readFileSync(filePath, 'utf8');
  return yaml.load(raw) as UnifiedPolicy;
}

function convertUnifiedPolicyToRules(
  unifiedPolicy: UnifiedPolicy,
  tool: string,
  params: Record<string, unknown> = {},
): PolicyRule[] {
  const rules: PolicyRule[] = [];

  // Determine content type based on tool name
  let contentType = 'generic';
  if (tool.includes('dockerfile')) {
    contentType = 'dockerfile';
  } else if (tool.includes('k8s') || tool.includes('kubernetes') || tool.includes('manifest')) {
    contentType = 'kubernetes';
  }

  const contentRules = unifiedPolicy.rules[contentType];
  if (!contentRules) return rules;

  // Convert rules from each category
  let priority = 1;
  for (const [category, categoryRules] of Object.entries(contentRules)) {
    if (Array.isArray(categoryRules)) {
      for (const rule of categoryRules) {
        if (rule.name && rule.description) {
          rules.push({
            id: `${contentType}_${category}_${rule.name}`,
            text: rule.description,
            priority: priority++,
            tools: [tool], // Apply to the specific tool
            when: params, // Use provided parameters for filtering
          });
        }
      }
    }
  }

  return rules;
}

/**
 * Load and synthesize deterministic policy bullets for a given tool.
 * - Uses unified policy.yaml configuration
 * - Filters by tool + 'when' matcher (shallow equality on provided keys)
 * - Sorts by (priority asc, id asc)
 * - Returns top N (default 4) rule.text values
 */
export function getPolicyRules(
  tool: string,
  params: Record<string, unknown> = {},
  options: { max?: number; policyFile?: string } = {},
): string[] {
  const { max = 4, policyFile = 'config/policy.yaml' } = options;
  const policyPath = path.resolve(process.cwd(), policyFile);

  if (!fs.existsSync(policyPath)) {
    console.warn(`Unified policy file not found at ${policyPath}`);
    return [];
  }

  try {
    const unifiedPolicy = loadUnifiedPolicy(policyPath);
    const rules = convertUnifiedPolicyToRules(unifiedPolicy, tool, params);

    // Sort rules by priority, then id
    rules.sort((a, b) => {
      const pa = a.priority ?? 1000;
      const pb = b.priority ?? 1000;
      if (pa !== pb) return pa - pb;
      return a.id.localeCompare(b.id);
    });

    return rules.slice(0, max).map((r) => r.text);
  } catch (error) {
    console.error(`Failed to load unified policy from ${policyPath}:`, error);
    return [];
  }
}
