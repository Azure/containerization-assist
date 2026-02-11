/**
 * JSON Schema Exports for VS Code Extension Integration
 *
 * Provides JSON Schema versions of all tool input schemas, suitable for
 * use in VS Code package.json languageModelTools definitions.
 *
 * @example
 * ```typescript
 * import { jsonSchemas } from 'containerization-assist-mcp/sdk';
 *
 * // Use in package.json generation or validation
 * console.log(jsonSchemas.analyzeRepo);
 * ```
 */

import { convertZodToJsonSchema } from './json-schema-converter.js';

// Re-export for consumers who need to convert custom schemas
export { convertZodToJsonSchema } from './json-schema-converter.js';

// Import all Zod schemas
import { analyzeRepoSchema } from '@/tools/analyze-repo/schema';
import { generateDockerfileSchema } from '@/tools/generate-dockerfile/schema';
import { fixDockerfileSchema } from '@/tools/fix-dockerfile/schema';
import { buildImageSchema } from '@/tools/build-image/schema';
import { scanImageSchema } from '@/tools/scan-image/schema';
import { tagImageSchema } from '@/tools/tag-image/schema';
import { pushImageSchema } from '@/tools/push-image/schema';
import { generateK8sManifestsSchema } from '@/tools/generate-k8s-manifests/schema';
import { prepareClusterSchema } from '@/tools/prepare-cluster/schema';
import { verifyDeploySchema } from '@/tools/verify-deploy/schema';
import { opsToolSchema } from '@/tools/ops/schema';

// ===== INDIVIDUAL SCHEMA EXPORTS =====

/**
 * JSON Schema for analyze-repo tool input.
 */
export const analyzeRepoJsonSchema = convertZodToJsonSchema(analyzeRepoSchema);

/**
 * JSON Schema for generate-dockerfile tool input.
 */
export const generateDockerfileJsonSchema = convertZodToJsonSchema(generateDockerfileSchema);

/**
 * JSON Schema for fix-dockerfile tool input.
 */
export const fixDockerfileJsonSchema = convertZodToJsonSchema(fixDockerfileSchema);

/**
 * JSON Schema for build-image tool input.
 */
export const buildImageJsonSchema = convertZodToJsonSchema(buildImageSchema);

/**
 * JSON Schema for scan-image tool input.
 */
export const scanImageJsonSchema = convertZodToJsonSchema(scanImageSchema);

/**
 * JSON Schema for tag-image tool input.
 */
export const tagImageJsonSchema = convertZodToJsonSchema(tagImageSchema);

/**
 * JSON Schema for push-image tool input.
 */
export const pushImageJsonSchema = convertZodToJsonSchema(pushImageSchema);

/**
 * JSON Schema for generate-k8s-manifests tool input.
 */
export const generateK8sManifestsJsonSchema = convertZodToJsonSchema(generateK8sManifestsSchema);

/**
 * JSON Schema for prepare-cluster tool input.
 */
export const prepareClusterJsonSchema = convertZodToJsonSchema(prepareClusterSchema);

/**
 * JSON Schema for verify-deploy tool input.
 */
export const verifyDeployJsonSchema = convertZodToJsonSchema(verifyDeploySchema);

/**
 * JSON Schema for ops tool input.
 */
export const opsJsonSchema = convertZodToJsonSchema(opsToolSchema);

// ===== AGGREGATED EXPORT =====

/**
 * All JSON Schemas for VS Code languageModelTools definitions.
 *
 * Use these schemas in your VS Code extension's package.json:
 *
 * @example
 * ```json
 * {
 *   "contributes": {
 *     "languageModelTools": [{
 *       "name": "analyze_repo",
 *       "inputSchema": { ... } // Use jsonSchemas.analyzeRepo
 *     }]
 *   }
 * }
 * ```
 */
export const jsonSchemas = {
  analyzeRepo: analyzeRepoJsonSchema,
  generateDockerfile: generateDockerfileJsonSchema,
  fixDockerfile: fixDockerfileJsonSchema,
  buildImage: buildImageJsonSchema,
  scanImage: scanImageJsonSchema,
  tagImage: tagImageJsonSchema,
  pushImage: pushImageJsonSchema,
  generateK8sManifests: generateK8sManifestsJsonSchema,
  prepareCluster: prepareClusterJsonSchema,
  verifyDeploy: verifyDeployJsonSchema,
  ops: opsJsonSchema,
} as const;

/**
 * Type for the jsonSchemas object keys.
 */
export type ToolSchemaName = keyof typeof jsonSchemas;
