import { readFile } from 'node:fs/promises';
import { Success, Failure, type Result } from '@types';
import { extractErrorMessage } from './error-utils';

/**
 * Safely parses JSON with proper error handling.
 * Consolidates JSON.parse patterns scattered across the codebase.
 */
export function safeJsonParse<T = unknown>(jsonString: string): Result<T> {
  try {
    const parsed = JSON.parse(jsonString) as T;
    return Success(parsed);
  } catch (error) {
    return Failure(`Failed to parse JSON: ${extractErrorMessage(error)}`);
  }
}

/**
 * Parses JSON from file content with validation.
 */
export async function parseJsonFile<T = unknown>(
  filePath: string,
  validator?: (data: unknown) => data is T,
): Promise<Result<T>> {
  try {
    const content = await readFile(filePath, 'utf-8');
    const parseResult = safeJsonParse<T>(content);

    if (!parseResult.ok) return parseResult;

    if (validator && !validator(parseResult.value)) {
      return Failure(`Invalid JSON structure in ${filePath}`);
    }

    return parseResult;
  } catch (error) {
    return Failure(`Failed to read file ${filePath}: ${extractErrorMessage(error)}`);
  }
}
