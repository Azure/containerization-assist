/**
 * Base Image Utilities
 *
 * Centralized logic for Docker base image recommendations and resolution.
 * Consolidates previously duplicated implementations across multiple files.
 */

export interface BaseImageRecommendations {
  primary: string;
  alternatives: string[];
  security?: string[];
  performance?: string[];
}

export interface BaseImageOptions {
  /** Target language/runtime */
  language: string;
  /** Optional framework context */
  framework?: string;
  /** Optimization preference */
  preference?: 'security' | 'performance' | 'size' | 'compatibility' | 'balanced';
}

/**
 * Base image mappings by language
 */
const BASE_IMAGE_MAP: Record<
  string,
  {
    primary: string;
    alternatives: string[];
    security?: string[];
    performance?: string[];
  }
> = {
  javascript: {
    primary: 'node:18-alpine',
    alternatives: ['node:18-slim', 'node:18', 'node:20-alpine'],
    security: ['node:18-alpine', 'node:20-alpine'],
    performance: ['node:18-slim', 'node:20-slim'],
  },
  typescript: {
    primary: 'node:18-alpine',
    alternatives: ['node:18-slim', 'node:18', 'node:20-alpine'],
    security: ['node:18-alpine', 'node:20-alpine'],
    performance: ['node:18-slim', 'node:20-slim'],
  },
  python: {
    primary: 'python:3.11-slim',
    alternatives: ['python:3.11-alpine', 'python:3.11', 'python:3.12-slim'],
    security: ['python:3.11-alpine', 'python:3.12-alpine'],
    performance: ['python:3.11-slim', 'python:3.12-slim'],
  },
  java: {
    primary: 'openjdk:17-alpine',
    alternatives: ['openjdk:17-slim', 'eclipse-temurin:17', 'openjdk:21-alpine'],
    security: ['openjdk:17-alpine', 'eclipse-temurin:17-alpine'],
    performance: ['openjdk:17-slim', 'eclipse-temurin:17-jre-slim'],
  },
  go: {
    primary: 'golang:1.21-alpine',
    alternatives: ['golang:1.21', 'scratch', 'alpine:latest'],
    security: ['golang:1.21-alpine', 'scratch'],
    performance: ['scratch', 'alpine:latest'],
  },
  rust: {
    primary: 'rust:alpine',
    alternatives: ['rust:slim', 'rust:latest', 'alpine:latest'],
    security: ['rust:alpine', 'alpine:latest'],
    performance: ['rust:slim', 'alpine:latest'],
  },
  ruby: {
    primary: 'ruby:3.2-alpine',
    alternatives: ['ruby:3.2-slim', 'ruby:3.2', 'ruby:3.3-alpine'],
    security: ['ruby:3.2-alpine', 'ruby:3.3-alpine'],
    performance: ['ruby:3.2-slim', 'ruby:3.3-slim'],
  },
  php: {
    primary: 'php:8.2-fpm-alpine',
    alternatives: ['php:8.2-apache', 'php:8.2-cli', 'php:8.3-fpm-alpine'],
    security: ['php:8.2-fpm-alpine', 'php:8.3-fpm-alpine'],
    performance: ['php:8.2-fpm', 'php:8.3-fpm'],
  },
  dotnet: {
    primary: 'mcr.microsoft.com/dotnet/aspnet:8.0-alpine',
    alternatives: [
      'mcr.microsoft.com/dotnet/aspnet:8.0',
      'mcr.microsoft.com/dotnet/runtime:8.0-alpine',
      'mcr.microsoft.com/dotnet/aspnet:7.0-alpine',
    ],
    security: [
      'mcr.microsoft.com/dotnet/aspnet:8.0-alpine',
      'mcr.microsoft.com/dotnet/runtime:8.0-alpine',
    ],
    performance: ['mcr.microsoft.com/dotnet/aspnet:8.0', 'mcr.microsoft.com/dotnet/runtime:8.0'],
  },
};

/**
 * Default fallback images for unknown languages
 */
const FALLBACK_IMAGES = {
  primary: 'alpine:latest',
  alternatives: ['ubuntu:22.04', 'debian:12-slim'],
  security: ['alpine:latest', 'debian:12-slim'],
  performance: ['alpine:latest'],
};

/**
 * Returns single recommended base image for language.
 * Maintains backward compatibility with legacy single-image API.
 */
export function getRecommendedBaseImage(language: string): string {
  const langKey = language.toLowerCase();
  const imageConfig = BASE_IMAGE_MAP[langKey];

  if (!imageConfig) {
    return FALLBACK_IMAGES.primary;
  }

  return imageConfig.primary;
}

/**
 * Returns array of base image alternatives for user selection.
 * Provides flexibility when primary recommendation isn't suitable.
 */
export function getSuggestedBaseImages(language: string): string[] {
  const langKey = language.toLowerCase();
  const imageConfig = BASE_IMAGE_MAP[langKey];

  if (!imageConfig) {
    return [FALLBACK_IMAGES.primary, ...FALLBACK_IMAGES.alternatives];
  }

  return [imageConfig.primary, ...imageConfig.alternatives];
}

/**
 * Applies optimization preference to base image selection.
 *
 * Trade-off: Security vs performance vs size optimization strategies
 */
function applyPreference(
  config: { primary: string; alternatives: string[]; security?: string[]; performance?: string[] },
  preference?: 'security' | 'performance' | 'size' | 'compatibility' | 'balanced',
): BaseImageRecommendations {
  let primaryImages: string[];

  switch (preference) {
    case 'security':
      primaryImages = config.security || [config.primary];
      break;
    case 'performance':
      primaryImages = config.performance || [config.primary];
      break;
    default:
      primaryImages = [config.primary];
  }

  const result: BaseImageRecommendations = {
    primary: primaryImages[0] || config.primary,
    alternatives: config.alternatives,
  };

  if (config.security) {
    result.security = config.security;
  }

  if (config.performance) {
    result.performance = config.performance;
  }

  return result;
}

/**
 * Detects the appropriate platform for a given base image
 *
 * @param baseImage - The Docker base image name
 * @returns Platform string (e.g., 'linux/amd64', 'windows/amd64')
 */
export function getPlatformForBaseImage(baseImage: string): string {
  // .NET Framework requires Windows
  if (baseImage.includes('dotnet/framework')) {
    return 'windows/amd64';
  }

  // Everything else supports Linux
  return 'linux/amd64';
}

/**
 * Get comprehensive base image recommendations with context-aware selection
 *
 * This function implements a multi-tiered selection strategy:
 * 1. Primary recommendation: Most widely compatible and supported
 * 2. Alternative options: Different trade-offs (size vs compatibility)
 * 3. Security-focused: Minimal attack surface, regularly updated
 * 4. Performance-focused: Optimized for build time and runtime efficiency
 *
 * Design rationale:
 * - Alpine images prioritized for size and security (smaller attack surface)
 * - Slim variants used when Alpine compatibility is problematic
 * - Full images available for complex dependency requirements
 * - Framework-specific optimizations applied when context available
 *
 * @param options - Selection criteria including language, framework, and optimization preference
 * @returns Comprehensive recommendations with multiple options for different scenarios
 */
export function getBaseImageRecommendations(options: BaseImageOptions): BaseImageRecommendations {
  const langKey = options.language.toLowerCase();

  // Handle .NET Framework specific recommendations
  if (langKey === 'dotnet' && options.framework) {
    const framework = options.framework.toLowerCase();

    if (
      framework.includes('framework') ||
      framework.includes('aspnet-webapi') ||
      framework.includes('aspnet-mvc') ||
      framework.includes('wcf') ||
      framework.includes('webforms')
    ) {
      const frameworkConfig = {
        primary: 'mcr.microsoft.com/dotnet/framework/aspnet:4.8-windowsservercore-ltsc2022',
        alternatives: [
          'mcr.microsoft.com/dotnet/framework/aspnet:4.8-nanoserver-ltsc2022',
          'mcr.microsoft.com/dotnet/framework/runtime:4.8-windowsservercore-ltsc2022',
          'mcr.microsoft.com/dotnet/framework/aspnet:4.7.2-windowsservercore-ltsc2022',
        ],
        security: [
          'mcr.microsoft.com/dotnet/framework/aspnet:4.8-nanoserver-ltsc2022',
          'mcr.microsoft.com/dotnet/framework/runtime:4.8-nanoserver-ltsc2022',
        ],
        performance: [
          'mcr.microsoft.com/dotnet/framework/aspnet:4.8-windowsservercore-ltsc2022',
          'mcr.microsoft.com/dotnet/framework/runtime:4.8-windowsservercore-ltsc2022',
        ],
      };
      return applyPreference(frameworkConfig, options.preference);
    }

    if (framework.includes('blazor-webassembly')) {
      const blazorWasmConfig = {
        primary: 'nginx:alpine',
        alternatives: [
          'mcr.microsoft.com/dotnet/aspnet:8.0-alpine',
          'httpd:alpine',
          'caddy:alpine',
        ],
        security: ['nginx:alpine', 'caddy:alpine'],
        performance: ['nginx:alpine', 'httpd:alpine'],
      };
      return applyPreference(blazorWasmConfig, options.preference);
    }

    if (framework.includes('worker-service') || framework.includes('windows-service')) {
      const workerConfig = {
        primary: 'mcr.microsoft.com/dotnet/runtime:8.0-alpine',
        alternatives: [
          'mcr.microsoft.com/dotnet/runtime:8.0',
          'mcr.microsoft.com/dotnet/runtime:7.0-alpine',
          'mcr.microsoft.com/dotnet/aspnet:8.0-alpine', // If adding HTTP later
        ],
        security: [
          'mcr.microsoft.com/dotnet/runtime:8.0-alpine',
          'gcr.io/distroless/dotnet/runtime:8.0',
        ],
        performance: [
          'mcr.microsoft.com/dotnet/runtime:8.0',
          'mcr.microsoft.com/dotnet/runtime:8.0-alpine',
        ],
      };
      return applyPreference(workerConfig, options.preference);
    }

    if (framework.includes('grpc')) {
      const grpcConfig = {
        primary: 'mcr.microsoft.com/dotnet/aspnet:8.0-alpine',
        alternatives: [
          'mcr.microsoft.com/dotnet/aspnet:8.0',
          'mcr.microsoft.com/dotnet/aspnet:7.0-alpine',
        ],
        security: [
          'mcr.microsoft.com/dotnet/aspnet:8.0-alpine',
          'gcr.io/distroless/dotnet/aspnet:8.0',
        ],
        performance: [
          'mcr.microsoft.com/dotnet/aspnet:8.0',
          'mcr.microsoft.com/dotnet/aspnet:8.0-alpine',
        ],
      };
      return applyPreference(grpcConfig, options.preference);
    }

    if (framework.includes('blazor')) {
      const blazorConfig = {
        primary: 'mcr.microsoft.com/dotnet/aspnet:8.0-alpine',
        alternatives: [
          'mcr.microsoft.com/dotnet/aspnet:8.0',
          'mcr.microsoft.com/dotnet/aspnet:7.0-alpine',
        ],
        security: [
          'mcr.microsoft.com/dotnet/aspnet:8.0-alpine',
          'gcr.io/distroless/dotnet/aspnet:8.0',
        ],
        performance: [
          'mcr.microsoft.com/dotnet/aspnet:8.0',
          'mcr.microsoft.com/dotnet/aspnet:8.0-alpine',
        ],
      };
      return applyPreference(blazorConfig, options.preference);
    }
  }

  const imageConfig = BASE_IMAGE_MAP[langKey] || FALLBACK_IMAGES;

  let primaryImages: string[];

  switch (options.preference) {
    case 'security':
      primaryImages = imageConfig.security || [imageConfig.primary];
      break;
    case 'performance':
      primaryImages = imageConfig.performance || imageConfig.alternatives.slice(0, 2);
      break;
    case 'size':
      primaryImages = imageConfig.alternatives.filter((img) => img.includes('alpine')) || [
        imageConfig.primary,
      ];
      break;
    case 'compatibility':
      primaryImages = imageConfig.alternatives.filter((img) => !img.includes('alpine')) || [
        imageConfig.primary,
      ];
      break;
    default:
      primaryImages = [imageConfig.primary];
  }

  const result: BaseImageRecommendations = {
    primary: primaryImages[0] || imageConfig.primary,
    alternatives: imageConfig.alternatives,
  };

  if (imageConfig.security) {
    result.security = imageConfig.security;
  }

  if (imageConfig.performance) {
    result.performance = imageConfig.performance;
  }

  return result;
}
