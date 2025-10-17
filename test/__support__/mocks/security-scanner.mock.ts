/**
 * Mock Security Scanner for Testing
 */

import { Result, Success } from "../../src/types"
import { DockerScanResult } from '../../src/types/docker';

export const mockScan = async (imageName: string): Promise<Result<DockerScanResult>> => {
  // Generate realistic mock data based on image characteristics
  const isAlpine = imageName.toLowerCase().includes('alpine');
  const isNode = imageName.toLowerCase().includes('node');
  const isOld = imageName.includes(':3.7') || imageName.includes('debian:8');

  let critical = 0,
    high = 0,
    medium = 0,
    low = 0;
  const vulnerabilities: Array<{
    id?: string;
    severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'UNKNOWN';
    package: string;
    version: string;
    fixedVersion?: string;
    description?: string;
  }> = [];

  if (isOld) {
    critical = Math.floor(Math.random() * 5) + 1;
    high = Math.floor(Math.random() * 10) + 5;
    medium = Math.floor(Math.random() * 15) + 10;
    low = Math.floor(Math.random() * 20) + 5;
  } else if (isAlpine) {
    critical = 0;
    high = Math.floor(Math.random() * 2);
    medium = Math.floor(Math.random() * 5);
    low = Math.floor(Math.random() * 3);
  } else if (isNode) {
    critical = Math.floor(Math.random() * 2);
    high = Math.floor(Math.random() * 3) + 1;
    medium = Math.floor(Math.random() * 8) + 2;
    low = Math.floor(Math.random() * 10) + 1;
  }

  const total = critical + high + medium + low;
  const severities = [
    ...Array(critical).fill('critical'),
    ...Array(high).fill('high'),
    ...Array(medium).fill('medium'),
    ...Array(low).fill('low'),
  ];

  for (let i = 0; i < Math.min(total, 10); i++) {
    const packages = ['openssl', 'curl', 'bash', 'glibc', 'zlib'];
    vulnerabilities.push({
      id: `CVE-2024-${1000 + i}`,
      package: packages[Math.floor(Math.random() * packages.length)] || 'unknown',
      version: '1.0.0',
      fixedVersion: '1.0.1',
      severity: (severities[i] || 'low').toUpperCase() as 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW',
      description: 'Mock vulnerability for testing purposes',
    });
  }

  await new Promise((resolve) => setTimeout(resolve, 100));

  return Success({
    vulnerabilities,
    summary: { critical, high, medium, low, unknown: 0, total },
    scanTime: new Date().toISOString(),
    metadata: {
      image: imageName,
      scanner: 'mock',
      version: '1.0.0-mock',
    },
  });
};