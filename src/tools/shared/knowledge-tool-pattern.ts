/**
 * Knowledge-Based Tool Pattern
 *
 * Reusable pattern for tools that query the knowledge base and return
 * structured plans instead of making AI calls. This pattern enables:
 * - Fast, deterministic tool execution (<100ms)
 * - Zero AI calls within tools (client AI generates content using plans)
 * - Testable, consistent behavior
 * - Reduced code duplication (30-50% code reduction per tool)
 *
 * @module tools/shared/knowledge-tool-pattern
 */

import { type Result, Success } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Topic } from '@/types/topics';
import type { KnowledgeCategory, KnowledgeQuery } from '@/knowledge/types';
import type { KnowledgeSnippet } from '@/knowledge/schemas';
import { getKnowledgeSnippets } from '@/knowledge/matcher';
import { getPolicyAwareKnowledgeSnippets } from '@/knowledge/policy-aware-matcher';
import { loadKnowledgeData } from '@/knowledge/loader';

/**
 * Configuration for knowledge query construction.
 * Defines how to translate tool input into knowledge base queries.
 */
export interface KnowledgeQueryConfig<TInput> {
  /** Knowledge topic to query */
  topic: Topic | ((input: TInput) => Topic);

  /** Knowledge category to filter by */
  category: KnowledgeCategory;

  /** Maximum characters to return */
  maxChars?: number;

  /** Maximum number of snippets to return */
  maxSnippets?: number;

  /** Extract filters from input for knowledge query */
  extractFilters: (input: TInput) => {
    environment?: string | undefined;
    language?: string | undefined;
    framework?: string | undefined;
    [key: string]: unknown;
  };
}

/**
 * Categorized knowledge snippets.
 * Tools typically categorize knowledge into 3-4 meaningful buckets
 * (e.g., security, optimization, best practices).
 */
export interface CategorizedKnowledge<TCategories extends string = string> {
  /** All knowledge snippets */
  all: KnowledgeSnippet[];

  /** Categorized by type */
  categories: Record<TCategories, KnowledgeSnippet[]>;
}

/**
 * Configuration for categorizing knowledge snippets.
 * Defines rules for grouping snippets into meaningful categories.
 */
export interface CategorizationConfig<TInput, TCategories extends string = string> {
  /** Category names */
  categoryNames: readonly TCategories[];

  /**
   * Categorize a snippet into one or more categories.
   * Returns array of category names that this snippet belongs to.
   */
  categorize: (snippet: KnowledgeSnippet, input?: TInput) => TCategories[];
}

/**
 * Configuration for rule-based logic.
 * Defines deterministic rules that tools apply to inputs.
 */
export interface RulesConfig<TInput, TRuleResults> {
  /**
   * Apply deterministic rules to input and knowledge.
   * This replaces AI decision-making with explicit, testable logic.
   */
  applyRules: (
    input: TInput,
    knowledge: CategorizedKnowledge,
    ctx: ToolContext,
  ) => TRuleResults | Promise<TRuleResults>;
}

/**
 * Configuration for confidence calculation.
 * Confidence indicates how well the knowledge base can address this request.
 */
export interface ConfidenceConfig {
  /**
   * Calculate confidence score (0.0 to 1.0) based on knowledge matches.
   * Higher match counts typically indicate higher confidence.
   *
   * Default: Math.min(0.95, 0.5 + matchCount * 0.05)
   */
  calculateConfidence?: (matchCount: number) => number;
}

/**
 * Configuration for building the final plan output.
 * Defines how to structure the tool's response.
 */
export interface PlanBuilderConfig<TInput, TRuleResults, TPlan> {
  /**
   * Build the final plan from all gathered information.
   * This is the structured output that the MCP client AI will use.
   */
  buildPlan: (
    input: TInput,
    knowledge: CategorizedKnowledge,
    rules: TRuleResults,
    confidence: number,
    ctx: ToolContext,
  ) => TPlan | Promise<TPlan>;
}

/**
 * Complete configuration for a knowledge-based tool.
 * Combines all aspects of the pattern into a single config object.
 */
export interface KnowledgeToolConfig<
  TInput,
  TPlan,
  TCategories extends string = string,
  TRuleResults = Record<string, unknown>,
> {
  /** Tool name for logging */
  name: string;

  /** Knowledge query configuration */
  query: KnowledgeQueryConfig<TInput>;

  /** Categorization configuration */
  categorization: CategorizationConfig<TInput, TCategories>;

  /** Rule-based logic configuration */
  rules: RulesConfig<TInput, TRuleResults>;

  /** Confidence calculation configuration */
  confidence?: ConfidenceConfig;

  /** Plan building configuration */
  plan: PlanBuilderConfig<TInput, TRuleResults, TPlan>;
}

/**
 * Default confidence calculation.
 * Starts at 0.5 and increases by 0.05 per match, capped at 0.95.
 */
export const defaultConfidenceCalculation = (matchCount: number): number => {
  return matchCount > 0 ? Math.min(0.95, 0.5 + matchCount * 0.05) : 0.5;
};

/**
 * Create a knowledge-based tool runner from configuration.
 *
 * This factory function creates a tool `run` function that:
 * 1. Queries the knowledge base (fast, deterministic)
 * 2. Categorizes knowledge snippets
 * 3. Applies rule-based logic
 * 4. Calculates confidence
 * 5. Builds and returns a structured plan
 *
 * No AI calls are made - the tool returns data for the MCP client AI to use.
 *
 * @param config - Complete tool configuration
 * @returns Tool run function
 *
 * @example
 * ```typescript
 * const run = createKnowledgeTool({
 *   name: 'generate-dockerfile-plan',
 *   query: {
 *     topic: TOPICS.DOCKERFILE,
 *     category: CATEGORY.DOCKERFILE,
 *     maxChars: 8000,
 *     maxSnippets: 20,
 *     extractFilters: (input) => ({
 *       environment: input.environment || 'production',
 *       language: input.language,
 *       framework: input.framework,
 *     }),
 *   },
 *   categorization: {
 *     categoryNames: ['security', 'optimization', 'bestPractices'] as const,
 *     categorize: (snippet) => {
 *       const categories = [];
 *       if (snippet.category === 'security' || snippet.tags?.includes('security')) {
 *         categories.push('security');
 *       }
 *       // ... more categorization logic
 *       return categories;
 *     },
 *   },
 *   rules: {
 *     applyRules: (input, knowledge) => ({
 *       multistage: ['java', 'go', 'rust'].includes(input.language),
 *     }),
 *   },
 *   plan: {
 *     buildPlan: (input, knowledge, rules, confidence) => ({
 *       repositoryInfo: { ... },
 *       recommendations: { ... },
 *       knowledgeMatches: knowledge.all,
 *       confidence,
 *     }),
 *   },
 * });
 * ```
 */
export function createKnowledgeTool<
  TInput,
  TPlan,
  TCategories extends string = string,
  TRuleResults = Record<string, unknown>,
>(
  config: KnowledgeToolConfig<TInput, TPlan, TCategories, TRuleResults>,
): (input: TInput, ctx: ToolContext) => Promise<Result<TPlan>> {
  return async (input: TInput, ctx: ToolContext): Promise<Result<TPlan>> => {
    // 1. Query knowledge base
    const filters = config.query.extractFilters(input);
    const topic =
      typeof config.query.topic === 'function' ? config.query.topic(input) : config.query.topic;

    ctx.logger.info({ topic, filters }, `${config.name}: Querying knowledge base`);

    const detectedDeps = filters.detectedDependencies;
    const hasDetectedDeps = Array.isArray(detectedDeps) && detectedDeps.length > 0;

    const knowledgeOptions = {
      environment: filters.environment || 'production',
      tool: config.name,
      maxChars: config.query.maxChars || 8000,
      maxSnippets: config.query.maxSnippets || 20,
      category: config.query.category,
      ...(filters.language && { language: filters.language }),
      ...(filters.framework && { framework: filters.framework }),
      ...(hasDetectedDeps && { detectedDependencies: detectedDeps }),
    };

    // Use policy-aware knowledge matching if policy is available
    let knowledgeSnippets: KnowledgeSnippet[];
    if (ctx.policy) {
      // Load knowledge data for policy-aware matching
      const knowledgeData = await loadKnowledgeData();

      // Build query for policy-aware matcher
      const query: KnowledgeQuery = {
        text: topic,
        category: knowledgeOptions.category,
        environment: knowledgeOptions.environment,
        tool: knowledgeOptions.tool,
        ...(knowledgeOptions.language && { language: knowledgeOptions.language }),
        ...(knowledgeOptions.framework && { framework: knowledgeOptions.framework }),
        tags: [
          knowledgeOptions.tool,
          ...(knowledgeOptions.language ? [knowledgeOptions.language] : []),
          ...(knowledgeOptions.framework ? [knowledgeOptions.framework] : []),
          ...(hasDetectedDeps ? detectedDeps : []),
        ],
        limit: knowledgeOptions.maxSnippets,
      };

      const { snippets, filterResult } = await getPolicyAwareKnowledgeSnippets(
        knowledgeData.entries,
        query,
        ctx.policy,
      );

      // Log policy filtering results
      if (filterResult.policyApplied) {
        ctx.logger.info(
          {
            excluded: filterResult.excluded.length,
            boosted: filterResult.boosted.length,
            reduced: filterResult.reduced.length,
            returned: filterResult.totalReturned,
          },
          `${config.name}: Policy-aware knowledge filtering applied`,
        );
      }

      knowledgeSnippets = snippets;
    } else {
      // Fall back to standard knowledge matching
      knowledgeSnippets = await getKnowledgeSnippets(topic, knowledgeOptions);
    }

    // 2. Categorize knowledge snippets
    // Initialize empty arrays for each category
    const categoriesRecord: Record<string, KnowledgeSnippet[]> = {};
    for (const categoryName of config.categorization.categoryNames) {
      categoriesRecord[categoryName] = [];
    }

    // Categorize each snippet (snippets can belong to multiple categories)
    for (const snippet of knowledgeSnippets) {
      const categories = config.categorization.categorize(snippet, input);
      for (const category of categories) {
        const categoryArray = categoriesRecord[category];
        if (categoryArray) {
          categoryArray.push(snippet);
        } else {
          throw new Error(
            `categorize() returned undeclared category "${category}" for tool "${config.name}". ` +
              `Snippet ID: ${snippet.id}, Source: ${snippet.source}. ` +
              `This likely indicates a configuration error.`,
          );
        }
      }
    }

    const categorized: CategorizedKnowledge<TCategories> = {
      all: knowledgeSnippets,
      categories: categoriesRecord as Record<TCategories, KnowledgeSnippet[]>,
    };

    // 3. Apply rule-based logic
    const rules = await config.rules.applyRules(input, categorized, ctx);

    // 4. Calculate confidence
    const calculateConfidence =
      config.confidence?.calculateConfidence || defaultConfidenceCalculation;
    const confidence = calculateConfidence(knowledgeSnippets.length);

    // 5. Build plan
    const plan = await config.plan.buildPlan(input, categorized, rules, confidence, ctx);

    // Log completion
    const categoryCounts = Object.entries(categorized.categories).map(
      ([name, snippets]) => `${name}: ${(snippets as KnowledgeSnippet[]).length}`,
    );

    ctx.logger.info(
      {
        knowledgeMatchCount: knowledgeSnippets.length,
        confidence,
        categories: categoryCounts,
      },
      `${config.name}: Planning completed`,
    );

    return Success(plan);
  };
}

/**
 * Helper: Create a simple categorizer based on tags and category fields.
 *
 * This is a common pattern where snippets are categorized by checking
 * if they have specific tags or category values.
 *
 * @param rules - Mapping of category names to filter predicates
 * @returns Categorization function
 *
 * @example
 * ```typescript
 * const categorize = createSimpleCategorizer({
 *   security: (s) => s.category === 'security' || s.tags?.includes('security'),
 *   optimization: (s) => s.tags?.includes('optimization') || s.tags?.includes('caching'),
 *   bestPractices: (s) => true, // catch-all
 * });
 * ```
 */
export function createSimpleCategorizer<TCategories extends string>(
  rules: Record<TCategories, (snippet: KnowledgeSnippet) => boolean>,
): (snippet: KnowledgeSnippet) => TCategories[] {
  return (snippet: KnowledgeSnippet): TCategories[] => {
    const categories: TCategories[] = [];

    for (const [category, predicate] of Object.entries(rules) as Array<
      [TCategories, (snippet: KnowledgeSnippet) => boolean]
    >) {
      if (predicate(snippet)) {
        categories.push(category);
      }
    }

    return categories;
  };
}
