/**
 * query-knowledge primitive
 *
 * Wraps the knowledge matcher library for use by skills and other primitives.
 */

import { Success, Failure, type Result } from '@types';
import type { ToolContext } from '@/core/context';
import { loadKnowledgeData } from '@/knowledge/loader';
import { findKnowledgeMatches } from '@/knowledge/matcher';
import { tool } from '@/types/tool';
import type { QueryKnowledgeInput } from './schema';
import type { QueryKnowledgeOut, KnowledgeMatchOut } from '../types';
import { queryKnowledgeToolDefinition } from './types';

async function handleQueryKnowledge(
  input: QueryKnowledgeInput,
  _ctx: ToolContext,
): Promise<Result<QueryKnowledgeOut>> {
  try {
    const knowledgeData = await loadKnowledgeData();

    const query = {
      tags: input.tags,
      ...(input.context?.language && { language: input.context.language }),
      ...(input.context?.framework && { framework: input.context.framework }),
      ...(input.context?.toolName && { tool: input.context.toolName }),
      limit: input.limit,
    };

    const allMatches = findKnowledgeMatches(knowledgeData.entries, query);

    // Filter out entries that only matched via the severity bonus (reasons=[]).
    // These are phantom hits that surface regardless of query context.
    const matches = allMatches.filter((m) => m.reasons.length > 0);

    const out: KnowledgeMatchOut[] = matches.map((m) => ({
      id: m.entry.id,
      category: m.entry.category,
      severity: m.entry.severity ?? 'low',
      title: m.entry.id,
      guidance: m.entry.recommendation,
      tags: m.entry.tags ?? [],
      score: m.score,
    }));

    return Success<QueryKnowledgeOut>({
      matches: out,
      totalMatched: out.length,
    });
  } catch (error) {
    return Failure(
      `query-knowledge failed: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

export default tool({
  ...queryKnowledgeToolDefinition,
  handler: handleQueryKnowledge,
});
