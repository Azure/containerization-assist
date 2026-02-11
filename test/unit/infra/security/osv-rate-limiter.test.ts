/**
 * Test RateLimiter queue cleanup and retry logic
 */

import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import { osvRateLimiter } from '@/infra/security/osv-scanner/osv-api';

describe('OSV RateLimiter', () => {
  beforeEach(() => {
    osvRateLimiter.reset();
  });

  afterEach(() => {
    osvRateLimiter.reset();
  });

  it('should reset queue and resolve pending promises', async () => {
    // Exhaust tokens
    for (let i = 0; i < 10; i++) {
      await osvRateLimiter.acquire();
    }

    // Create pending promises that will wait in queue
    const pendingPromises: Promise<void>[] = [];
    for (let i = 0; i < 5; i++) {
      pendingPromises.push(osvRateLimiter.acquire());
    }

    // Reset should resolve all pending promises
    osvRateLimiter.reset();

    // All promises should resolve immediately
    const results = await Promise.all(pendingPromises);
    expect(results).toHaveLength(5);
  });

  it('should not leave hanging promises after reset', async () => {
    // Exhaust tokens
    for (let i = 0; i < 10; i++) {
      await osvRateLimiter.acquire();
    }

    // Create a pending promise
    const pendingPromise = osvRateLimiter.acquire();

    // Reset
    osvRateLimiter.reset();

    // Promise should resolve within 100ms (not hang)
    await expect(
      Promise.race([
        pendingPromise,
        new Promise((_, reject) => setTimeout(() => reject(new Error('Timeout')), 100)),
      ]),
    ).resolves.toBeUndefined();
  });

  it('should allow normal operation after reset with pending queue', async () => {
    // Exhaust tokens
    for (let i = 0; i < 10; i++) {
      await osvRateLimiter.acquire();
    }

    // Create pending promises
    const pending1 = osvRateLimiter.acquire();
    const pending2 = osvRateLimiter.acquire();

    // Reset
    osvRateLimiter.reset();

    // Should resolve pending
    await Promise.all([pending1, pending2]);

    // Should work normally after reset
    await expect(osvRateLimiter.acquire()).resolves.toBeUndefined();
  });
});
