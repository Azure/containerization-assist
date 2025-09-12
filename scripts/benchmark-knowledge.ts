import { loadKnowledgeBase, getCompilationStats, getAllEntries } from '../src/knowledge/loader.js';
import { findKnowledgeMatches } from '../src/knowledge/matcher.js';
import type { KnowledgeQuery } from '../src/knowledge/types.js';

async function benchmark() {
  console.log('🚀 Knowledge Base Performance Benchmark\n');
  console.log('Loading knowledge base...');
  
  const loadStart = performance.now();
  await loadKnowledgeBase();
  const loadTime = performance.now() - loadStart;
  
  console.log(`\n📊 Load Performance:`);
  console.log(`  Time: ${loadTime.toFixed(2)}ms`);
  
  const stats = getCompilationStats();
  console.log(`  Entries: ${stats.totalEntries}`);
  console.log(`  Compiled: ${stats.compiledSuccessfully}`);
  console.log(`  Errors: ${stats.compilationErrors}`);
  console.log(`  Avg compilation: ${stats.avgCompilationTime.toFixed(3)}ms`);
  
  // Test queries
  const queries: KnowledgeQuery[] = [
    { category: 'dockerfile' as const, text: 'FROM node:18' },
    { category: 'kubernetes' as const, text: 'apiVersion: apps/v1' },
    { category: 'security' as const, text: 'RUN apt-get update' },
    { language: 'python', text: 'FROM python:3.9' },
    { framework: 'express', tags: ['node'] },
  ];
  
  console.log('\n📈 Query Performance:');
  const entries = getAllEntries();
  
  for (const query of queries) {
    const queryStart = performance.now();
    const matches = findKnowledgeMatches(entries, query);
    const queryTime = performance.now() - queryStart;
    
    const queryStr = JSON.stringify(query).substring(0, 50);
    console.log(`  Query: ${queryStr}${queryStr.length >= 50 ? '...' : ''}`);
    console.log(`    Time: ${queryTime.toFixed(2)}ms`);
    console.log(`    Matches: ${matches.length}`);
  }
  
  // Run 1000 queries for average
  console.log('\n⚡ Bulk Performance (1000 queries):');
  const bulkStart = performance.now();
  for (let i = 0; i < 1000; i++) {
    const query = queries[i % queries.length];
    findKnowledgeMatches(entries, query);
  }
  const bulkTime = performance.now() - bulkStart;
  console.log(`  Total: ${bulkTime.toFixed(2)}ms`);
  console.log(`  Average: ${(bulkTime / 1000).toFixed(3)}ms per query`);
  
  // Memory usage
  if (process.memoryUsage) {
    const memUsage = process.memoryUsage();
    console.log('\n💾 Memory Usage:');
    console.log(`  RSS: ${(memUsage.rss / 1024 / 1024).toFixed(2)} MB`);
    console.log(`  Heap Used: ${(memUsage.heapUsed / 1024 / 1024).toFixed(2)} MB`);
    console.log(`  Heap Total: ${(memUsage.heapTotal / 1024 / 1024).toFixed(2)} MB`);
  }
  
  console.log('\n✅ Benchmark complete!');
}

benchmark().catch(console.error);