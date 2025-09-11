import type { ProgressReporter } from '@mcp/context';
import {
  STANDARD_STAGES,
  createStandardProgress,
  reportProgress,
} from '@mcp/progress-helper';

describe('progress-helper', () => {
  describe('STANDARD_STAGES', () => {
    it('should define all required stages with correct percentages', () => {
      expect(STANDARD_STAGES.VALIDATING).toEqual({
        message: 'Validating',
        percentage: 10,
      });
      expect(STANDARD_STAGES.EXECUTING).toEqual({
        message: 'Executing',
        percentage: 50,
      });
      expect(STANDARD_STAGES.FINALIZING).toEqual({
        message: 'Finalizing',
        percentage: 90,
      });
      expect(STANDARD_STAGES.COMPLETE).toEqual({
        message: 'Complete',
        percentage: 100,
      });
    });

    it('should be immutable', () => {
      // The object is frozen with `as const`, so properties can't be reassigned
      expect(STANDARD_STAGES).toEqual(expect.objectContaining({
        VALIDATING: { message: 'Validating', percentage: 10 },
        EXECUTING: { message: 'Executing', percentage: 50 },
        FINALIZING: { message: 'Finalizing', percentage: 90 },
        COMPLETE: { message: 'Complete', percentage: 100 },
      }));
    });
  });

  describe('reportProgress', () => {
    it('should call reporter when provided', async () => {
      const mockReporter = jest.fn();
      await reportProgress(mockReporter, 'Test message', 50);
      
      expect(mockReporter).toHaveBeenCalledWith('Test message', 50);
      expect(mockReporter).toHaveBeenCalledTimes(1);
    });

    it('should handle undefined reporter safely', async () => {
      // Should not throw when reporter is undefined
      await expect(
        reportProgress(undefined, 'Test message', 50)
      ).resolves.toBeUndefined();
    });

    it('should handle null reporter safely', async () => {
      // Should not throw when reporter is null
      await expect(
        reportProgress(null as any, 'Test message', 50)
      ).resolves.toBeUndefined();
    });
  });

  describe('createStandardProgress', () => {
    let mockReporter: ReturnType<typeof jest.fn>;

    beforeEach(() => {
      jest.clearAllMocks();
      mockReporter = jest.fn();
    });

    it('should create a progress handler that reports correct stages', async () => {
      const progress = createStandardProgress(mockReporter);

      await progress('VALIDATING');
      expect(mockReporter).toHaveBeenCalledWith('Validating', 10);

      await progress('EXECUTING');
      expect(mockReporter).toHaveBeenCalledWith('Executing', 50);

      await progress('FINALIZING');
      expect(mockReporter).toHaveBeenCalledWith('Finalizing', 90);

      await progress('COMPLETE');
      expect(mockReporter).toHaveBeenCalledWith('Complete', 100);

      expect(mockReporter).toHaveBeenCalledTimes(4);
    });

    it('should work without a reporter', async () => {
      const progress = createStandardProgress();

      // Should not throw
      await expect(progress('VALIDATING')).resolves.toBeUndefined();
      await expect(progress('EXECUTING')).resolves.toBeUndefined();
      await expect(progress('FINALIZING')).resolves.toBeUndefined();
      await expect(progress('COMPLETE')).resolves.toBeUndefined();
    });

    it('should work with undefined reporter', async () => {
      const progress = createStandardProgress(undefined);

      // Should not throw
      await expect(progress('VALIDATING')).resolves.toBeUndefined();
    });

    it('should enforce type-safe stage names', () => {
      const progress = createStandardProgress(mockReporter);

      // TypeScript should catch this at compile time
      // @ts-expect-error - Testing type safety
      expect(() => progress('INVALID_STAGE')).rejects.toThrow();
    });
  });

});