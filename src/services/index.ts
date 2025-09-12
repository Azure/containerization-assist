/**
 * Services module exports - Docker and Kubernetes clients
 */

// Docker services
export * from './docker-client';
export type { ImageMetadata as CachedImageMetadata } from './docker-metadata-cache';
export {
  getImageMetadata as getCachedImageMetadata,
  getMultipleImageMetadata,
  preloadCommonImages,
  clearMetadataCache,
  getMetadataCacheStats,
} from './docker-metadata-cache';
export * from './docker-mutex-client';
export type { ImageMetadata as RegistryImageMetadata } from './docker-registry';
export { getImageMetadata as getRegistryImageMetadata } from './docker-registry';

// Kubernetes services
export * from './kubernetes-client';
export * from './kubernetes-idempotent-apply';
