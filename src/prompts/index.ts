/**
 * Prompts Module - Manifest-based System
 *
 * New efficient prompt system with lazy loading and manifest-based discovery
 */

// Core prompt loading APIs
export {
  loadPromptText,
  getPromptMetadata as getPromptManifestMetadata,
  listPromptIds,
  isPromptEmbedded,
  clearCache,
} from './locator';

export { loadPrompt, loadPrompts } from './loader';

// Enhanced prompt registry with lazy loading (fully async)
export {
  initializeRegistry,
  initializePrompts,
  prompts,
  getPrompt, // Async with lazy loading
  listPrompts, // Async with lazy loading
  getPromptMetadata, // Async with lazy loading
  getPromptWithMessages,
  type PromptEntry,
  type PromptParameter,
  type PromptAPI,
  type VersionOptions,
  type ABTestingOptions,
  type EnhancedRenderOptions,
  type ABTestPromptResult,
} from './prompt-registry';
