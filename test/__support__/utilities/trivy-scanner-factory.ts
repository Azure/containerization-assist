/**
 * Trivy Scanner Factory with Multiple Strategy Support
 * Provides binary, container, and mock scanning strategies
 */

import type { Logger } from 'pino';
import { exec } from 'child_process';
import { promisify } from 'util';
import { Result, Success, Failure } from '../../../src/domain/types';

const execAsync = promisify(exec);

// Define DockerScanResult interface for test utilities
interface DockerScanResult {
  vulnerabilities?: Array<{
    id?: string;
    severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW';
    package?: string;
    version?: string;
    description?: string;
    fixedVersion?: string;
  }>;
  summary?: {
    critical: number;
    high: number;
    medium: number;
    low: number;
    unknown?: number;
    total: number;
  };
  scanTime?: string;
  metadata?: {
    image: string;
  };
}

export interface ScannerStrategy {
  name: string;
  available: boolean;
  scan: (imageName: string, options?: ScanOptions) => Promise<Result<DockerScanResult>>;
  getInfo: () => { available: boolean; version?: string; type: string };
}

export interface ScanOptions {
  severity?: string;
  ignoreUnfixed?: boolean;
  timeout?: number;
}

/**
 * Binary Trivy Scanner Strategy
 */
export class TrivyBinaryScanner implements ScannerStrategy {
  name = 'binary';
  available = false;
  private version?: string;

  constructor(private logger: Logger) {}

  async initialize(): Promise<Result<void>> {
    try {
      const { stdout } = await execAsync('trivy version', { timeout: 5000 });
      const versionMatch = stdout.match(/Version:\s*(.+)/);
      this.version = versionMatch ? versionMatch[1].trim() : 'unknown';
      this.available = true;
      this.logger.info({ version: this.version }, 'Trivy binary scanner initialized');
      return Success(undefined);
    } catch (error) {
      this.available = false;
      return Failure(`Trivy binary not available: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  async scan(imageName: string, options: ScanOptions = {}): Promise<Result<DockerScanResult>> {
    if (!this.available) {
      return Failure('Trivy binary scanner not initialized');
    }

    try {
      const { severity = 'CRITICAL,HIGH,MEDIUM,LOW', ignoreUnfixed = false, timeout = 120000 } = options;
      
      const args = [
        'image',
        '--format', 'json',
        '--severity', severity
      ];

      if (ignoreUnfixed) {
        args.push('--ignore-unfixed');
      }

      args.push(imageName);

      const command = `trivy ${args.join(' ')}`;
      const { stdout } = await execAsync(command, { timeout });

      return this.parseTrivyOutput(stdout, imageName);
    } catch (error) {
      return Failure(`Trivy binary scan failed: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  getInfo() {
    return {
      available: this.available,
      version: this.version,
      type: 'binary'
    };
  }

  private parseTrivyOutput(output: string, imageName: string): Result<DockerScanResult> {
    try {
      const trivyResult = JSON.parse(output);
      const vulnerabilities = [];
      let total = 0;
      let critical = 0, high = 0, medium = 0, low = 0, unknown = 0;

      if (trivyResult.Results) {
        for (const result of trivyResult.Results) {
          if (result.Vulnerabilities) {
            for (const vuln of result.Vulnerabilities) {
              vulnerabilities.push({
                id: vuln.VulnerabilityID,
                package: vuln.PkgName,
                version: vuln.InstalledVersion,
                fixedVersion: vuln.FixedVersion,
                severity: vuln.Severity?.toLowerCase() || 'unknown',
                title: vuln.Title,
                description: vuln.Description
              });

              total++;
              switch (vuln.Severity?.toLowerCase()) {
                case 'critical': critical++; break;
                case 'high': high++; break;
                case 'medium': medium++; break;
                case 'low': low++; break;
                default: unknown++; break;
              }
            }
          }
        }
      }

      return Success({
        vulnerabilities,
        summary: { critical, high, medium, low, unknown, total },
        scanTime: new Date().toISOString(),
        metadata: {
          image: imageName,
          scanner: 'trivy-binary',
          version: this.version || 'unknown'
        }
      });
    } catch (error) {
      return Failure(`Failed to parse Trivy output: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }
}

/**
 * Server Trivy Scanner Strategy (uses running Trivy server)
 */
export class TrivyServerScanner implements ScannerStrategy {
  name = 'server';
  available = false;
  private version?: string;
  private serverUrl: string;

  constructor(private logger: Logger, serverUrl: string = 'http://localhost:4954') {
    this.serverUrl = serverUrl;
  }

  async initialize(): Promise<Result<void>> {
    try {
      // Test server availability and get version
      const response = await fetch(`${this.serverUrl}/version`, {
        method: 'GET',
        signal: AbortSignal.timeout(5000)
      });
      
      if (!response.ok) {
        throw new Error(`Server responded with ${response.status}: ${response.statusText}`);
      }
      
      const versionData = await response.json();
      this.version = versionData.Version || 'unknown';
      this.available = true;
      this.logger.info({ version: this.version, serverUrl: this.serverUrl }, 'Trivy server scanner initialized');
      return Success(undefined);
    } catch (error) {
      this.available = false;
      return Failure(`Trivy server scanner not available: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  async scan(imageName: string, options: ScanOptions = {}): Promise<Result<DockerScanResult>> {
    if (!this.available) {
      return Failure('Trivy server scanner not initialized');
    }

    try {
      const { severity = 'CRITICAL,HIGH,MEDIUM,LOW', ignoreUnfixed = false, timeout = 60000 } = options;
      
      // Prepare request body for Trivy server API
      const requestBody = {
        Target: imageName,
        Options: {
          Format: 'json',
          Severities: severity.split(',').map(s => s.trim()),
          IgnoreUnfixed: ignoreUnfixed
        }
      };

      const response = await fetch(`${this.serverUrl}/scan`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(requestBody),
        signal: AbortSignal.timeout(timeout)
      });

      if (!response.ok) {
        throw new Error(`Server scan failed with ${response.status}: ${response.statusText}`);
      }

      const scanResult = await response.text();
      return this.parseTrivyOutput(scanResult, imageName);
    } catch (error) {
      return Failure(`Trivy server scan failed: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  getInfo() {
    return {
      available: this.available,
      version: this.version,
      type: 'server',
      serverUrl: this.serverUrl
    };
  }

  private parseTrivyOutput(output: string, imageName: string): Result<DockerScanResult> {
    try {
      const trivyResult = JSON.parse(output);
      const vulnerabilities = [];
      let total = 0;
      let critical = 0, high = 0, medium = 0, low = 0, unknown = 0;

      if (trivyResult.Results) {
        for (const result of trivyResult.Results) {
          if (result.Vulnerabilities) {
            for (const vuln of result.Vulnerabilities) {
              const severity = vuln.Severity?.toLowerCase() || 'unknown';
              vulnerabilities.push({
                id: vuln.VulnerabilityID,
                package: vuln.PkgName,
                version: vuln.InstalledVersion,
                fixedVersion: vuln.FixedVersion || '',
                severity: severity,
                title: vuln.Title || '',
                description: vuln.Description || ''
              });
              
              // Count by severity
              switch (severity) {
                case 'critical': critical++; break;
                case 'high': high++; break;
                case 'medium': medium++; break;
                case 'low': low++; break;
                default: unknown++; break;
              }
              total++;
            }
          }
        }
      }

      return Success({
        vulnerabilities,
        summary: { critical, high, medium, low, unknown, total },
        scanTime: new Date().toISOString(),
        metadata: {
          image: imageName,
          scanner: 'trivy-server',
          serverUrl: this.serverUrl,
          lastScanned: new Date().toISOString()
        }
      });
    } catch (error) {
      return Failure(`Failed to parse Trivy server output: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }
}

/**
 * Container Trivy Scanner Strategy
 */
export class TrivyContainerScanner implements ScannerStrategy {
  name = 'container';
  available = false;
  private version?: string;

  constructor(private logger: Logger) {}

  async initialize(): Promise<Result<void>> {
    try {
      // Test Docker availability first
      await execAsync('docker info', { timeout: 5000 });
      
      // Test Trivy container
      const { stdout } = await execAsync('docker run --rm aquasec/trivy:latest version', { timeout: 30000 });
      const versionMatch = stdout.match(/Version:\s*(.+)/);
      this.version = versionMatch ? versionMatch[1].trim() : 'unknown';
      this.available = true;
      this.logger.info({ version: this.version }, 'Trivy container scanner initialized');
      return Success(undefined);
    } catch (error) {
      this.available = false;
      return Failure(`Trivy container scanner not available: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  async scan(imageName: string, options: ScanOptions = {}): Promise<Result<DockerScanResult>> {
    if (!this.available) {
      return Failure('Trivy container scanner not initialized');
    }

    try {
      const { severity = 'CRITICAL,HIGH,MEDIUM,LOW', ignoreUnfixed = false, timeout = 120000 } = options;
      
      const args = [
        'run', '--rm',
        '-v', '/var/run/docker.sock:/var/run/docker.sock',
        'aquasec/trivy:latest',
        'image',
        '--format', 'json',
        '--severity', severity
      ];

      if (ignoreUnfixed) {
        args.push('--ignore-unfixed');
      }

      args.push(imageName);

      const command = `docker ${args.join(' ')}`;
      const { stdout } = await execAsync(command, { timeout });

      return this.parseTrivyOutput(stdout, imageName);
    } catch (error) {
      return Failure(`Trivy container scan failed: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  getInfo() {
    return {
      available: this.available,
      version: this.version,
      type: 'container'
    };
  }

  private parseTrivyOutput(output: string, imageName: string): Result<DockerScanResult> {
    // Same parsing logic as binary scanner
    try {
      const trivyResult = JSON.parse(output);
      const vulnerabilities = [];
      let total = 0;
      let critical = 0, high = 0, medium = 0, low = 0, unknown = 0;

      if (trivyResult.Results) {
        for (const result of trivyResult.Results) {
          if (result.Vulnerabilities) {
            for (const vuln of result.Vulnerabilities) {
              vulnerabilities.push({
                id: vuln.VulnerabilityID,
                package: vuln.PkgName,
                version: vuln.InstalledVersion,
                fixedVersion: vuln.FixedVersion,
                severity: vuln.Severity?.toLowerCase() || 'unknown',
                title: vuln.Title,
                description: vuln.Description
              });

              total++;
              switch (vuln.Severity?.toLowerCase()) {
                case 'critical': critical++; break;
                case 'high': high++; break;
                case 'medium': medium++; break;
                case 'low': low++; break;
                default: unknown++; break;
              }
            }
          }
        }
      }

      return Success({
        vulnerabilities,
        summary: { critical, high, medium, low, unknown, total },
        scanTime: new Date().toISOString(),
        metadata: {
          image: imageName,
          scanner: 'trivy-container',
          version: this.version || 'unknown'
        }
      });
    } catch (error) {
      return Failure(`Failed to parse Trivy output: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }
}

/**
 * Mock Scanner Strategy for CI/Testing
 */
export class MockSecurityScanner implements ScannerStrategy {
  name = 'mock';
  available = true;

  constructor(private logger: Logger) {}

  async scan(imageName: string, options: ScanOptions = {}): Promise<Result<DockerScanResult>> {
    this.logger.info({ imageName }, 'Using mock security scanner');
    
    // Generate realistic mock data based on image name
    const isAlpine = imageName.toLowerCase().includes('alpine');
    const isNode = imageName.toLowerCase().includes('node');
    const isOld = imageName.includes(':3.7') || imageName.includes('debian:8');
    
    let critical = 0, high = 0, medium = 0, low = 0;
    const vulnerabilities = [];

    // Generate mock vulnerabilities based on image characteristics
    if (isOld) {
      critical = Math.floor(Math.random() * 5) + 1;
      high = Math.floor(Math.random() * 10) + 5;
      medium = Math.floor(Math.random() * 15) + 10;
      low = Math.floor(Math.random() * 20) + 5;
    } else if (isAlpine) {
      // Alpine is generally more secure
      critical = 0;
      high = Math.floor(Math.random() * 2);
      medium = Math.floor(Math.random() * 5);
      low = Math.floor(Math.random() * 3);
    } else if (isNode) {
      // Node images have moderate vulnerabilities
      critical = Math.floor(Math.random() * 2);
      high = Math.floor(Math.random() * 3) + 1;
      medium = Math.floor(Math.random() * 8) + 2;
      low = Math.floor(Math.random() * 10) + 1;
    }

    const total = critical + high + medium + low;

    // Generate sample vulnerabilities
    const severities = [
      ...Array(critical).fill('critical'),
      ...Array(high).fill('high'),
      ...Array(medium).fill('medium'),
      ...Array(low).fill('low')
    ];

    for (let i = 0; i < Math.min(total, 10); i++) { // Limit to 10 samples
      vulnerabilities.push({
        id: `CVE-2024-${1000 + i}`,
        package: ['openssl', 'curl', 'bash', 'glibc', 'zlib'][Math.floor(Math.random() * 5)],
        version: '1.0.0',
        fixedVersion: '1.0.1',
        severity: severities[i],
        title: `Mock vulnerability ${i + 1}`,
        description: `This is a mock vulnerability for testing purposes`
      });
    }

    await new Promise(resolve => setTimeout(resolve, 100)); // Simulate scan time

    return Success({
      vulnerabilities,
      summary: { critical, high, medium, low, unknown: 0, total },
      scanTime: new Date().toISOString(),
      metadata: {
        image: imageName,
        scanner: 'mock',
        version: '1.0.0-mock'
      }
    });
  }

  getInfo() {
    return {
      available: true,
      version: '1.0.0-mock',
      type: 'mock'
    };
  }
}

/**
 * Multi-Strategy Trivy Scanner Factory
 */
export class TrivyScannerFactory {
  private strategies: ScannerStrategy[] = [];
  private activeStrategy?: ScannerStrategy;

  constructor(private logger: Logger) {
    this.strategies = [
      new TrivyBinaryScanner(logger),
      new TrivyServerScanner(logger), // Try server first (fastest)
      new TrivyContainerScanner(logger),
      new MockSecurityScanner(logger)
    ];
  }

  async initialize(): Promise<Result<ScannerStrategy>> {
    // Try strategies in order of preference
    for (const strategy of this.strategies) {
      if ('initialize' in strategy && typeof strategy.initialize === 'function') {
        const result = await strategy.initialize();
        if (result.kind === 'ok') {
          this.activeStrategy = strategy;
          this.logger.info({ strategy: strategy.name }, 'Security scanner strategy selected');
          return Success(strategy);
        }
      } else if (strategy.name === 'mock') {
        // Mock strategy doesn't need initialization
        this.activeStrategy = strategy;
        this.logger.info({ strategy: strategy.name }, 'Using mock security scanner');
        return Success(strategy);
      }
    }

    return Failure('No security scanner strategy available');
  }

  async scan(imageName: string, options?: ScanOptions): Promise<Result<DockerScanResult>> {
    if (!this.activeStrategy) {
      const initResult = await this.initialize();
      if (initResult.kind === 'fail') {
        return Failure('No security scanner available');
      }
    }

    return this.activeStrategy!.scan(imageName, options);
  }

  getInfo() {
    if (this.activeStrategy) {
      return this.activeStrategy.getInfo();
    }
    return { available: false, type: 'none' };
  }

  getAvailableStrategies() {
    return this.strategies.map(strategy => ({
      name: strategy.name,
      available: strategy.available,
      info: strategy.getInfo()
    }));
  }
}