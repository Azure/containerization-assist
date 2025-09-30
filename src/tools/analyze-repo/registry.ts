import type { RepositoryAnalysis } from './schema';

interface StoredAnalysis extends RepositoryAnalysis {
  analyzedPath?: string;
  sessionId?: string;
}

const registry = new Map<string, StoredAnalysis>();

export function storeRepositoryAnalysis(sessionId: string, analysis: StoredAnalysis): void {
  registry.set(sessionId, analysis);
}

export function getRepositoryAnalysis(sessionId: string): StoredAnalysis | undefined {
  return registry.get(sessionId);
}

export function clearRepositoryAnalysis(sessionId: string): void {
  registry.delete(sessionId);
}
