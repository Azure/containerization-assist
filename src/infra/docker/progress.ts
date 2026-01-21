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
  private readonly seenMessages: Set<string> = new Set();

  constructor(options: ProgressTrackerOptions) {
    this.onProgress = options.onProgress;
    this.logger = options.logger;
  }

  /**
   * Process a BuildKit trace event and extract a readable status message.
   *
   * @returns The extracted status message when a new, non-duplicate message is
   *          produced; otherwise an empty string (for example, when no message
   *          can be extracted, when the message is a duplicate, or when an
   *          error occurs during processing).
   */
  processBuildKitTrace(auxData: unknown): string {
    try {
      // Decode BuildKit trace synchronously
      const status = decodeBuildKitTrace(auxData, this.logger);
      if (status) {
        const message = formatBuildKitStatus(status);
        if (message && !this.seenMessages.has(message)) {
          this.seenMessages.add(message);
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
