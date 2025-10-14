/**
 * Schema definition for analyze-repo tool
 */

import { z } from 'zod';
import { sessionId, repositoryPathAbsoluteUnix, analysisOptions } from '../shared/schemas';

export const moduleInfo = z.object({
  name: z.string().describe('The name of the module'),
  modulePathAbsoluteUnix: z
    .string()
    .describe(
      'Absolute path to module root using only forward slashes path separators. UNIX path separators only.',
    ),
  dockerfilePath: z.string().optional().describe('Path where the Dockerfile should be generated'),
  language: z
    .enum(['java', 'dotnet', 'other'])
    .optional()
    .describe('Primary programming language used in the module'),
  languageVersion: z.string().optional(),
  frameworks: z
    .array(
      z.object({
        name: z
          .string()
          .describe('Frameworks used in project like Java Spring, SpringBoot, Hibernate etc.'),
        version: z.string().optional(),
      }),
    )
    .optional(),
  buildSystem: z
    .object({
      type: z.string().optional(),
      configFile: z.string().optional(),
    })
    .optional()
    .describe('Build system information like Maven or Gradle'),
  dependencies: z
    .array(z.string())
    .optional()
    .describe('List of module dependencies including database drivers and system libraries'),
  ports: z.array(z.number()).optional(),
  entryPoint: z.string().optional(),
});
export type ModuleInfo = z.infer<typeof moduleInfo>;

export const analyzeRepoSchema = z.object({
  sessionId,
  repositoryPathAbsoluteUnix,
  ...analysisOptions,
  modules: z
    .array(moduleInfo)
    .optional()
    .describe('Optional pre-analyzed modules. If not provided, AI will analyze the repository.'),
});

export type AnalyzeRepoParams = z.infer<typeof analyzeRepoSchema>;

export interface RepositoryAnalysis {
  modules?: ModuleInfo[];
  isMonorepo?: boolean;
  sessionId?: string;
  analyzedPath?: string;
  workflowHints?: {
    message?: string;
  };
  // Fields from AI response (for parsing)
  language?: string;
  languageVersion?: string;
  framework?: string;
  frameworkVersion?: string;
  buildSystem?: {
    type?: string;
    buildFile?: string;
    configFile?: string;
    buildCommand?: string;
    testCommand?: string;
  };
  dependencies?: string[];
  devDependencies?: string[];
  entryPoint?: string;
  suggestedPorts?: number[];
}
