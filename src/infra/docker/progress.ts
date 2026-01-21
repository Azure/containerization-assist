/**
 * Docker build progress tracking utilities
 * Handles BuildKit trace decoding
 */

import type { Logger } from 'pino';
import { decodeBuildKitTrace, formatBuildKitStatus } from './buildkit-decoder';

export type ProgressCallback = (message: string) => void;

/**
 * Options for progress tracking
 */
export interface ProgressTrackerOptions {
  /** Callback to invoke with progress messages */
  onProgress?: ProgressCallback;
  /** Logger for debug output */
  logger: Logger;
}

/**
 * Progress tracker for Docker builds
 * Handles BuildKit trace decoding
 */
export class ProgressTracker {
  private readonly onProgress: ProgressCallback | undefined;
  private readonly logger: Logger;
  private readonly collectedLogs: string[] = [];

  constructor(options: ProgressTrackerOptions) {
    this.onProgress = options.onProgress;
    this.logger = options.logger;
  }

  /**
   * Process a BuildKit trace event and extract readable status
   * @returns The extracted status message if any
   */
  processBuildKitTrace(auxData: unknown): string {
    try {
      // Decode BuildKit trace synchronously
      const status = decodeBuildKitTrace(auxData, this.logger);
      if (status) {
        const message = formatBuildKitStatus(status);
        if (message && !this.collectedLogs.includes(message)) {
          this.collectedLogs.push(message);
          if (this.onProgress) {
            this.onProgress(message);
          }
          return message;
        }
      }
    } catch (error) {
      this.logger.error(
        {
          error,
          errorMessage: error instanceof Error ? error.message : String(error),
        },
        'Error in processBuildKitTrace',
      );
    }

    return '';
  }
}

/**
 * Create a progress tracker for Docker build operations
 */
export function createProgressTracker(options: ProgressTrackerOptions): ProgressTracker {
  return new ProgressTracker(options);
}
