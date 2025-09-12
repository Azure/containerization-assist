/**
 * Configuration-Based Scoring Helper Functions
 *
 * Functions referenced by YAML configuration matchers
 */

/**
 * Count chained RUN commands (dockerfile optimization)
 */
export function countChainedRuns(content: string): number {
  const runLines = content.match(/^RUN\s+.*/gm) || [];
  return runLines.filter((line) => line.includes('&&')).length;
}

/**
 * Check for dependency caching patterns
 */
export function hasDependencyCaching(content: string): boolean {
  // Check for Node.js dependency caching
  const hasPackageJson = content.includes('package.json') || content.includes('package-lock.json');
  const hasRequirements = content.includes('requirements.txt');
  const hasGoMod = content.includes('go.mod');

  if (hasPackageJson) {
    const copyPackageIndex = Math.min(
      content.indexOf('COPY package.json') > -1 ? content.indexOf('COPY package.json') : Infinity,
      content.indexOf('COPY package*.json') > -1 ? content.indexOf('COPY package*.json') : Infinity,
    );
    const copyAllIndex = content.indexOf('COPY . ');
    return copyPackageIndex < copyAllIndex;
  }

  if (hasRequirements) {
    const copyReqIndex = content.indexOf('COPY requirements.txt');
    const copyAllIndex = content.indexOf('COPY . ');
    return copyReqIndex > -1 && copyReqIndex < copyAllIndex;
  }

  if (hasGoMod) {
    const copyGoModIndex = content.indexOf('COPY go.mod');
    const copyAllIndex = content.indexOf('COPY . ');
    return copyGoModIndex > -1 && copyGoModIndex < copyAllIndex;
  }

  return false;
}

/**
 * Count cleanup operations in dockerfile
 */
export function countCleanupOperations(content: string): number {
  const cleanupPatterns = [
    /rm\s+-rf\s+\/var\/lib\/apt\/lists/,
    /apt-get\s+clean/,
    /yum\s+clean\s+all/,
    /apk\s+--no-cache/,
    /pip\s+install\s+--no-cache-dir/,
    /npm\s+cache\s+clean/,
  ];
  return cleanupPatterns.filter((pattern) => content.match(pattern)).length;
}

/**
 * Count parallel operation indicators
 */
export function countParallelOperations(content: string): number {
  const parallelPatterns = [
    '--parallel',
    '-j',
    'make -j',
    'npm ci --prefer-offline',
    '--frozen-lockfile',
    '--mount=type=cache',
  ];
  return parallelPatterns.filter((pattern) => content.includes(pattern)).length;
}

/**
 * Check if all FROM statements use versioned images
 */
export function hasVersionedImages(content: string): boolean {
  const fromLines = content.match(/^FROM\s+(\S+)/gm) || [];
  if (fromLines.length === 0) return false;

  const versionedImages = fromLines.filter(
    (line) => !line.includes(':latest') && line.includes(':'),
  ).length;
  return versionedImages === fromLines.length;
}

/**
 * Check YAML indentation consistency
 */
export function hasConsistentIndentation(content: string): boolean {
  const lines = content.split('\n');
  return lines.every((line) => {
    // Check for tabs (invalid in YAML)
    if (line.includes('\t')) return false;
    // Check for proper spacing (multiples of 2)
    const leadingSpaces = line.match(/^(\s*)/)?.[1]?.length || 0;
    return leadingSpaces % 2 === 0;
  });
}

/**
 * Calculate content uniqueness ratio
 */
export function calculateContentUniqueness(content: string): number {
  const lines = content.split('\n').filter((line) => line.trim().length > 0);
  if (lines.length === 0) return 0;

  const uniqueLines = new Set(lines);
  return uniqueLines.size / lines.length;
}

/**
 * Check for documentation patterns
 */
export function hasDocumentation(content: string): boolean {
  const docPatterns = [
    /#\s+\w+/g, // Shell/Python comments
    /\/\/\s+\w+/g, // C-style comments
    /\/\*[\s\S]*?\*\//g, // Block comments
    /<!--[\s\S]*?-->/g, // HTML/XML comments
    /@\w+/g, // Annotations/decorators
  ];
  return docPatterns.some((pattern) => content.match(pattern));
}

/**
 * Check for efficient patterns in content
 */
export function countEfficientPatterns(content: string): number {
  const efficientPatterns = [
    /\|\|/g, // OR operations for fallbacks
    /&&/g, // AND operations for chaining
    /\${.*:-.*}/g, // Default values
    />/g, // Redirections
    /2>&1/g, // Error handling
  ];

  return efficientPatterns.reduce(
    (count, pattern) => count + (content.match(pattern) || []).length,
    0,
  );
}

/**
 * Check if Dockerfile uses non-root user
 */
export function hasNonRootUser(content: string): boolean {
  const userMatch = content.match(/^USER\s+(\S+)/m);
  return !!(userMatch && userMatch[1] !== 'root');
}

/**
 * Check if content has no secret patterns
 */
export function hasNoSecretPatterns(content: string): boolean {
  const secretPatterns = [
    /PASSWORD\s*=\s*["'][^"']+["']/i,
    /SECRET\s*=\s*["'][^"']+["']/i,
    /API_KEY\s*=\s*["'][^"']+["']/i,
    /TOKEN\s*=\s*["'][^"']+["']/i,
  ];
  return !secretPatterns.some((pattern) => content.match(pattern));
}

/**
 * Check if content has minimum number of lines
 */
export function hasMinimumLines(content: string, threshold: number): boolean {
  const lines = content.split('\n').filter((line) => line.trim().length > 0);
  return lines.length >= threshold;
}

/**
 * Check if content has consistent patterns
 */
export function hasConsistentPatterns(content: string): boolean {
  const listItems = (content.match(/^\s*[-*]\s+/gm) || []).length;
  const numberedItems = (content.match(/^\s*\d+\.\s+/gm) || []).length;
  const envVars = (content.match(/^[A-Z][A-Z_]+=/gm) || []).length;

  return listItems >= 3 || numberedItems >= 3 || envVars >= 3;
}

/**
 * Check if content has proper formatting
 */
export function hasProperFormatting(content: string): boolean {
  // Check for proper newlines and no excessive whitespace
  return content.includes('\n') && !content.includes('\r\n\r\n\r\n');
}

/**
 * Check if content has no generic security issues
 */
export function hasNoGenericSecrets(content: string): boolean {
  const securityPatterns = [
    { pattern: /password\s*[:=]\s*["'][^"']+["']/gi },
    { pattern: /api[_-]?key\s*[:=]\s*["'][^"']+["']/gi },
    { pattern: /secret\s*[:=]\s*["'][^"']+["']/gi },
    { pattern: /token\s*[:=]\s*["'][^"']+["']/gi },
    { pattern: /private[_-]?key/gi },
    { pattern: /BEGIN\s+(RSA|DSA|EC)\s+PRIVATE\s+KEY/gi },
    { pattern: /aws_access_key_id/gi },
    { pattern: /mongodb:\/\/[^@]+@/gi },
  ];

  return !securityPatterns.some(({ pattern }) => content.match(pattern));
}

/**
 * Check if content has no insecure configuration flags
 */
export function hasNoInsecureFlags(content: string): boolean {
  const insecurePatterns = [/--insecure/, /--no-check-certificate/, /0\.0\.0\.0:/, /\*:/];

  return !insecurePatterns.some((pattern) => content.match(pattern));
}

/**
 * Check if content has optimal density (not too sparse, not too dense)
 */
export function hasOptimalDensity(content: string): boolean {
  const lines = content.split('\n').filter((line) => line.trim().length > 0);
  const totalChars = content.length;

  const avgCharsPerLine = totalChars / Math.max(lines.length, 1);
  return avgCharsPerLine >= 20 && avgCharsPerLine <= 100;
}

/**
 * Check if content has high uniqueness (low repetition)
 */
export function hasHighUniqueness(content: string): boolean {
  const lines = content.split('\n').filter((line) => line.trim().length > 0);
  const uniqueLines = new Set(lines);
  const uniquenessRatio = uniqueLines.size / Math.max(lines.length, 1);

  return uniquenessRatio > 0.8;
}

/**
 * Check if content has efficient patterns
 */
export function hasEfficientPatterns(content: string): boolean {
  const efficientPatterns = [
    /\|\|/g, // OR operations for fallbacks
    /&&/g, // AND operations for chaining
    /\${.*:-.*}/g, // Default values
    />/g, // Redirections
    /2>&1/g, // Error handling
  ];

  const matches = efficientPatterns.reduce(
    (count, pattern) => count + (content.match(pattern) || []).length,
    0,
  );

  return matches > 0;
}

/**
 * Check if content has descriptive naming
 */
export function hasDescriptiveNaming(content: string): boolean {
  const descriptiveNames = [
    /[a-z][a-zA-Z]{3,}_[a-z][a-zA-Z]+/g, // snake_case
    /[a-z][a-zA-Z]{3,}[A-Z][a-zA-Z]+/g, // camelCase
  ];

  return descriptiveNames.some((pattern) => {
    const matches = content.match(pattern);
    return matches && matches.length > 0;
  });
}

/**
 * Check if content has excessively long lines
 */
export function hasExcessivelyLongLines(content: string): boolean {
  const lines = content.split('\n');
  return lines.some((line) => line.length > 200);
}

/**
 * Function registry for dynamic lookup from configuration
 */
export const SCORING_FUNCTIONS = {
  countChainedRuns,
  hasDependencyCaching,
  countCleanupOperations,
  countParallelOperations,
  hasVersionedImages,
  hasConsistentIndentation,
  calculateContentUniqueness,
  hasDocumentation,
  countEfficientPatterns,
  hasNonRootUser,
  hasNoSecretPatterns,
  hasMinimumLines,
  hasConsistentPatterns,
  hasProperFormatting,
  hasNoGenericSecrets,
  hasNoInsecureFlags,
  hasOptimalDensity,
  hasHighUniqueness,
  hasEfficientPatterns,
  hasDescriptiveNaming,
  hasExcessivelyLongLines,
} as const;

export type ScoringFunctionName = keyof typeof SCORING_FUNCTIONS;
