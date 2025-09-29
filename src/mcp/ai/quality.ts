/**
 * Unified Scoring System for AI Responses
 *
 * Provides consistent scoring across all AI services with contextual awareness.
 * Replaces individual scoring functions with a unified, configurable approach.
 */

interface ValidationResultItem {
  isValid: boolean;
  confidence: number;
  ruleId?: string;
  message?: string;
}

export interface ScoringContext {
  /** Content type for context-aware scoring */
  contentType?: 'dockerfile' | 'kubernetes' | 'security' | 'general' | 'knowledge' | 'enhancement';
  /** Focus area for targeted scoring */
  focus?: 'security' | 'performance' | 'best-practices' | 'enhancement' | 'all';
  /** Target improvement for enhancement scoring */
  targetImprovement?: string;
  /** Request details for knowledge enhancement */
  request?: {
    contentType?: string;
    focus?: string;
  };
  /** Validation options for validation scoring */
  validationOptions?: {
    contentType: string;
    focus?: string;
  };
}

export interface ScoreResult {
  /** Overall score (0-100) */
  total: number;
  /** Breakdown of scoring components */
  breakdown: Record<string, number>;
}

/**
 * Quality metrics for AI responses
 */
export interface QualityMetrics {
  score: number;
  breakdown: Record<string, number>;
  confidence: number;
}

/**
 * Unified scoring function that adapts based on content kind and context
 */
export function scoreResponse(
  kind: 'validation' | 'knowledge' | 'enhancement',
  text: string,
  context: ScoringContext = {},
): ScoreResult {
  switch (kind) {
    case 'validation':
      return scoreValidationResponse(text, context);
    case 'knowledge':
      return scoreKnowledgeResponse(text, context);
    case 'enhancement':
      return scoreEnhancementResponse(text, context);
    default:
      return { total: 0, breakdown: { error: 0 } };
  }
}

/**
 * Score validation responses with JSON structure and content quality checks
 */
function scoreValidationResponse(text: string, context: ScoringContext): ScoreResult {
  const scores = {
    format: 0,
    completeness: 0,
    specificity: 0,
    accuracy: 0,
    relevance: 0,
  };

  // Format scoring (0-25) - Check for valid JSON structure
  try {
    const parsed = JSON.parse(text);
    if (parsed.passed !== undefined) scores.format += 5;
    if (Array.isArray(parsed.results)) scores.format += 5;
    if (parsed.summary && typeof parsed.summary === 'object') scores.format += 5;
    if (
      parsed.results.every(
        (r: ValidationResultItem) => r.isValid !== undefined && r.confidence !== undefined,
      )
    )
      scores.format += 10;
  } catch {
    // Invalid JSON, low format score
    scores.format = 2;
  }

  // Completeness scoring (0-20)
  if (text.includes('"ruleId"')) scores.completeness += 5;
  if (text.includes('"severity"')) scores.completeness += 5;
  if (text.includes('"category"')) scores.completeness += 5;
  if (text.includes('"fixSuggestion"')) scores.completeness += 5;

  // Specificity scoring (0-20) - Content-type specific terms
  const contentType = context.validationOptions?.contentType || context.contentType || 'general';
  const specificTerms = getSpecificTermsForContent(contentType);
  const foundTerms = specificTerms.filter((term) => text.toLowerCase().includes(term)).length;
  scores.specificity = Math.min(foundTerms * 3, 20);

  // Accuracy scoring (0-20) - Avoid hallucination indicators
  if (!text.includes('TODO') && !text.includes('FIXME')) scores.accuracy += 5;
  if (!/lorem ipsum/i.test(text)) scores.accuracy += 5;
  if (!text.includes('placeholder')) scores.accuracy += 5;
  if (text.length > 200) scores.accuracy += 5; // Substantial analysis

  // Relevance scoring (0-15) - Focus alignment
  const focus = context.validationOptions?.focus || context.focus || 'all';
  const focusTerms = getFocusTerms(focus);
  const foundFocusTerms = focusTerms.filter((term) => text.toLowerCase().includes(term)).length;
  scores.relevance = Math.min(foundFocusTerms * 3, 15);

  return {
    total: Object.values(scores).reduce((sum, val) => sum + val, 0),
    breakdown: scores,
  };
}

/**
 * Score knowledge enhancement responses with structure and enhancement quality checks
 */
function scoreKnowledgeResponse(text: string, context: ScoringContext): ScoreResult {
  const scores = {
    structure: 0,
    enhancement: 0,
    knowledge: 0,
    completeness: 0,
    relevance: 0,
  };

  // Structure scoring (0-20)
  if (text.includes('## Enhanced Content')) scores.structure += 5;
  if (text.includes('## Knowledge Applied')) scores.structure += 5;
  if (text.includes('## Improvements Summary')) scores.structure += 3;
  if (text.includes('## Enhancement Areas')) scores.structure += 4;
  if (text.includes('## Additional Suggestions')) scores.structure += 3;

  // Enhancement quality scoring (0-25)
  if (text.includes('```') && text.split('```').length >= 3) scores.enhancement += 8; // Has enhanced content block
  if (text.length > 500) scores.enhancement += 5;
  if (!text.includes('TODO') && !text.includes('FIXME')) scores.enhancement += 5;
  if (text.includes('security') || text.includes('optimization')) scores.enhancement += 4;
  if (text.includes('best practice')) scores.enhancement += 3;

  // Knowledge application scoring (0-20)
  const knowledgeIndicators = [
    'knowledge',
    'best practice',
    'guideline',
    'standard',
    'recommendation',
  ];
  const foundKnowledge = knowledgeIndicators.filter((term) =>
    text.toLowerCase().includes(term),
  ).length;
  scores.knowledge = Math.min(foundKnowledge * 3, 20);

  // Completeness scoring (0-20)
  if (text.includes('Knowledge Sources')) scores.completeness += 5;
  if (text.includes('Best Practices Applied')) scores.completeness += 5;
  const suggestionCount = (text.match(/\d+\./g) || []).length;
  scores.completeness += Math.min(suggestionCount * 2, 10);

  // Context relevance scoring (0-15)
  const contentType = context.request?.contentType || context.contentType || 'general';
  const specificTerms = getSpecificTermsForContent(contentType);
  const foundTerms = specificTerms.filter((term) => text.toLowerCase().includes(term)).length;
  scores.relevance = Math.min(foundTerms * 2, 15);

  return {
    total: Object.values(scores).reduce((sum, val) => sum + val, 0),
    breakdown: scores,
  };
}

/**
 * Score enhancement responses with structure and content quality checks
 */
function scoreEnhancementResponse(text: string, context: ScoringContext): ScoreResult {
  const scores = {
    structure: 0,
    content: 0,
    specificity: 0,
    completeness: 0,
    relevance: 0,
  };

  // Structure scoring (0-20)
  if (text.includes('## Assessment')) scores.structure += 5;
  if (text.includes('## Risk Level')) scores.structure += 5;
  if (text.includes('## Suggestions')) scores.structure += 5;
  if (text.includes('## Priorities')) scores.structure += 5;

  // Content quality scoring (0-25)
  if (text.length > 200) scores.content += 5;
  if (text.split('\n').length > 10) scores.content += 5;
  if (!text.includes('TODO') && !text.includes('FIXME')) scores.content += 5;
  if (!/lorem ipsum/i.test(text)) scores.content += 5;
  if (text.includes('security') || text.includes('performance')) scores.content += 5;

  // Specificity scoring (0-20)
  const contentType = context.contentType || 'general';
  const specificTerms = getSpecificTermsForContent(contentType);
  const foundTerms = specificTerms.filter((term) => text.toLowerCase().includes(term)).length;
  scores.specificity = Math.min(foundTerms * 3, 20);

  // Completeness scoring (0-20)
  const actionableWords = ['should', 'must', 'recommend', 'suggest', 'improve', 'fix'];
  const foundActions = actionableWords.filter((word) => text.toLowerCase().includes(word)).length;
  scores.completeness = Math.min(foundActions * 3, 15);
  if (text.includes('Priority:') || text.includes('Risk:')) scores.completeness += 5;

  // Relevance scoring (0-15)
  const targetTerms = context.targetImprovement
    ? context.targetImprovement.toLowerCase().split(/\s+/)
    : ['improvement', 'enhancement', 'optimization'];
  const foundTargetTerms = targetTerms.filter((term) => text.toLowerCase().includes(term)).length;
  scores.relevance = Math.min(foundTargetTerms * 3, 15);

  return {
    total: Object.values(scores).reduce((sum, val) => sum + val, 0),
    breakdown: scores,
  };
}

/**
 * Get content-specific terms for targeted scoring
 */
function getSpecificTermsForContent(contentType: string): string[] {
  const termMap: Record<string, string[]> = {
    dockerfile: ['dockerfile', 'docker', 'container', 'image', 'layer', 'build'],
    kubernetes: ['kubernetes', 'k8s', 'pod', 'deployment', 'service', 'manifest'],
    security: ['vulnerability', 'security', 'attack', 'secret', 'permission'],
    knowledge: ['knowledge', 'best practice', 'guideline', 'standard', 'recommendation'],
    enhancement: ['enhancement', 'improvement', 'optimization', 'suggestion'],
    general: ['best practice', 'optimization', 'performance', 'maintainability'],
  };

  return termMap[contentType] ?? termMap.general ?? [];
}

/**
 * Get focus-specific terms for relevance scoring
 */
function getFocusTerms(focus: string): string[] {
  const focusMap: Record<string, string[]> = {
    security: ['security', 'vulnerability', 'attack', 'hardening'],
    performance: ['performance', 'optimization', 'resource', 'efficiency'],
    'best-practices': ['best practice', 'standard', 'maintainability'],
    optimization: ['optimization', 'performance', 'efficiency', 'resource'],
    all: ['security', 'performance', 'optimization', 'best practice'],
  };

  return focusMap[focus] ?? focusMap.all ?? [];
}
