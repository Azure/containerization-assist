/**
 * Docker build progress tracking utilities
 * Handles BuildKit trace decoding
 */

import type { Logger } from 'pino';

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

  constructor(options: ProgressTrackerOptions) {
    this.onProgress = options.onProgress;
    this.logger = options.logger;
  }

  /**
   * Process a stream log line and send progress notification
   */
  processStreamLog(logLine: string): void {
    if (logLine && this.onProgress) {
      this.onProgress(logLine);
    }
  }

  /**
   * Process a BuildKit trace event and extract readable status
   * @returns The extracted status message if any
   */
  processBuildKitTrace(auxData: unknown): string {
    const statusMessage = this.extractBuildKitStatus(auxData);
    if (statusMessage && this.onProgress) {
      this.onProgress(statusMessage);
    }
    return statusMessage;
  }

  /**
   * Extract readable status message from BuildKit trace data
   */
  private extractBuildKitStatus(auxData: unknown): string {
    if (!auxData || typeof auxData !== 'string') {
      return '';
    }

    try {
      const decoded = Buffer.from(auxData, 'base64').toString('utf-8');

      // Extract readable text by removing control characters and binary data
      // eslint-disable-next-line no-control-regex
      const readable = decoded.replace(/[\x00-\x1F\x7F-\xFF]/g, '').trim();

      if (readable.length > 0) {
        // Clean up multiple spaces
        const cleaned = readable.replace(/\s+/g, ' ');
        this.logger.debug({ cleaned }, 'Extracted BuildKit status (no regex)');
        return cleaned;
      }

      return '';
    } catch (error) {
      this.logger.debug({ error }, 'Failed to decode BuildKit trace');
      return '';
    }
  }
}

/**
 * Create a progress tracker for Docker build operations
 */
export function createProgressTracker(options: ProgressTrackerOptions): ProgressTracker {
  return new ProgressTracker(options);
}
