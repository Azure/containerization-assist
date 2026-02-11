/**
 * CVSS Vector Parser
 *
 * Parses CVSS v2.0, v3.0, v3.1, and v4.0 vector strings and calculates base scores.
 * Based on FIRST.org CVSS specifications.
 */

interface CVSSScore {
  baseScore: number;
  severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'NONE';
}

/**
 * Parse a CVSS vector string and calculate the base score
 *
 * @param vector CVSS vector string (e.g., "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H")
 * @returns CVSSScore object with baseScore and severity rating
 * @throws Error if vector is invalid or unsupported
 */
export function parseCVSSVector(vector: string): CVSSScore {
  if (!vector || typeof vector !== 'string') {
    throw new Error('Invalid CVSS vector: must be a non-empty string');
  }

  // CVSS v3.x and v4.x start with "CVSS:"
  if (vector.startsWith('CVSS:3.0/')) {
    return parseCVSSv3(vector, '3.0');
  } else if (vector.startsWith('CVSS:3.1/')) {
    return parseCVSSv3(vector, '3.1');
  } else if (vector.startsWith('CVSS:4.0/')) {
    return parseCVSSv4(vector);
  } else {
    // CVSS v2.0 doesn't have the CVSS: prefix
    return parseCVSSv2(vector);
  }
}

/**
 * Parse CVSS v3.0/3.1 vector string
 */
function parseCVSSv3(vector: string, _version: '3.0' | '3.1'): CVSSScore {
  const metrics = parseMetrics(vector);

  // Extract Base metrics (required)
  const AV = metrics.AV; // Attack Vector
  const AC = metrics.AC; // Attack Complexity
  const PR = metrics.PR; // Privileges Required
  const UI = metrics.UI; // User Interaction
  const S = metrics.S; // Scope
  const C = metrics.C; // Confidentiality Impact
  const I = metrics.I; // Integrity Impact
  const A = metrics.A; // Availability Impact

  if (!AV || !AC || !PR || !UI || !S || !C || !I || !A) {
    throw new Error('Missing required CVSS v3 base metrics');
  }

  // Convert metrics to numerical values
  const avScore = { N: 0.85, A: 0.62, L: 0.55, P: 0.2 }[AV] ?? 0;
  const acScore = { L: 0.77, H: 0.44 }[AC] ?? 0;
  const prScore =
    S === 'U' ? ({ N: 0.85, L: 0.62, H: 0.27 }[PR] ?? 0) : ({ N: 0.85, L: 0.68, H: 0.5 }[PR] ?? 0);
  const uiScore = { N: 0.85, R: 0.62 }[UI] ?? 0;
  const cScore = { H: 0.56, L: 0.22, N: 0 }[C] ?? 0;
  const iScore = { H: 0.56, L: 0.22, N: 0 }[I] ?? 0;
  const aScore = { H: 0.56, L: 0.22, N: 0 }[A] ?? 0;

  // Calculate Impact Sub Score (ISS)
  const iss = 1 - (1 - cScore) * (1 - iScore) * (1 - aScore);

  // Calculate Impact
  const impact = S === 'U' ? 6.42 * iss : 7.52 * (iss - 0.029) - 3.25 * Math.pow(iss - 0.02, 15);

  // Calculate Exploitability
  const exploitability = 8.22 * avScore * acScore * prScore * uiScore;

  // Calculate Base Score
  let baseScore: number;
  if (impact <= 0) {
    baseScore = 0;
  } else if (S === 'U') {
    baseScore = roundUp(Math.min(impact + exploitability, 10));
  } else {
    baseScore = roundUp(Math.min(1.08 * (impact + exploitability), 10));
  }

  return {
    baseScore,
    severity: scoreToV3Severity(baseScore),
  };
}

/**
 * Parse CVSS v2.0 vector string
 */
function parseCVSSv2(vector: string): CVSSScore {
  const metrics = parseMetrics(vector);

  // Extract Base metrics (required)
  const AV = metrics.AV; // Access Vector
  const AC = metrics.AC; // Access Complexity
  const Au = metrics.Au; // Authentication
  const C = metrics.C; // Confidentiality Impact
  const I = metrics.I; // Integrity Impact
  const A = metrics.A; // Availability Impact

  if (!AV || !AC || !Au || !C || !I || !A) {
    throw new Error('Missing required CVSS v2 base metrics');
  }

  // Convert metrics to numerical values
  const avScore = { L: 0.395, A: 0.646, N: 1.0 }[AV] ?? 0;
  const acScore = { H: 0.35, M: 0.61, L: 0.71 }[AC] ?? 0;
  const auScore = { M: 0.45, S: 0.56, N: 0.704 }[Au] ?? 0;
  const cScore = { N: 0.0, P: 0.275, C: 0.66 }[C] ?? 0;
  const iScore = { N: 0.0, P: 0.275, C: 0.66 }[I] ?? 0;
  const aScore = { N: 0.0, P: 0.275, C: 0.66 }[A] ?? 0;

  // Calculate Impact
  const impact = 10.41 * (1 - (1 - cScore) * (1 - iScore) * (1 - aScore));

  // Calculate Exploitability
  const exploitability = 20 * avScore * acScore * auScore;

  // Calculate f(Impact)
  const fImpact = impact === 0 ? 0 : 1.176;

  // Calculate Base Score
  const baseScore = roundTo1Decimal((0.6 * impact + 0.4 * exploitability - 1.5) * fImpact);

  return {
    baseScore: Math.max(0, baseScore),
    severity: scoreToV2Severity(baseScore),
  };
}

/**
 * Parse CVSS v4.0 vector string
 * Note: This is a simplified implementation focusing on base score
 */
function parseCVSSv4(_vector: string): CVSSScore {
  // CVSS v4.0 has a more complex scoring algorithm
  // For now, we'll extract what we can or throw an error
  throw new Error('CVSS v4.0 parsing not yet fully implemented');
}

/**
 * Parse metrics from vector string into key-value pairs
 */
function parseMetrics(vector: string): Record<string, string> {
  const metrics: Record<string, string> = {};

  // Remove CVSS: prefix if present
  const cleanVector = vector.replace(/^CVSS:[0-9.]+\//, '');

  // Split by / and parse each metric
  const parts = cleanVector.split('/');
  for (const part of parts) {
    const [key, value] = part.split(':');
    if (key && value) {
      metrics[key] = value;
    }
  }

  return metrics;
}

/**
 * Round up to 1 decimal place (CVSS v3.x rounding)
 */
function roundUp(value: number): number {
  return Math.ceil(value * 10) / 10;
}

/**
 * Round to 1 decimal place (CVSS v2.0 rounding)
 */
function roundTo1Decimal(value: number): number {
  return Math.round(value * 10) / 10;
}

/**
 * Map CVSS v3.x score to severity rating
 */
function scoreToV3Severity(score: number): 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'NONE' {
  if (score === 0.0) return 'NONE';
  if (score >= 9.0) return 'CRITICAL';
  if (score >= 7.0) return 'HIGH';
  if (score >= 4.0) return 'MEDIUM';
  return 'LOW';
}

/**
 * Map CVSS v2.0 score to severity rating
 */
function scoreToV2Severity(score: number): 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'NONE' {
  if (score === 0.0) return 'NONE';
  if (score >= 7.0) return 'HIGH';
  if (score >= 4.0) return 'MEDIUM';
  return 'LOW';
}
