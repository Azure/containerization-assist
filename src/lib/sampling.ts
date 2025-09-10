// Sampling types - keeping only what's exported and needed
export interface ScoringCriteria {
  buildTime: number;
  imageSize: number;
  security: number;
  bestPractices: number;
  maintenance: number;
  performance: number;
  [key: string]: number;
}
