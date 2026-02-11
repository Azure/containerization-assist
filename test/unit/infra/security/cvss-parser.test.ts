import { describe, it, expect } from '@jest/globals';
import { parseCVSSVector } from '@/infra/security/osv-scanner/cvss-parser';

describe('CVSS Parser', () => {
  describe('CVSS v3.1', () => {
    it('should parse CVSS v3.1 vector and calculate CRITICAL severity', () => {
      // Log4Shell CVSS vector
      const vector = 'CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H';
      const result = parseCVSSVector(vector);

      expect(result.baseScore).toBeGreaterThanOrEqual(9.0);
      expect(result.severity).toBe('CRITICAL');
    });

    it('should parse CVSS v3.1 vector and calculate HIGH severity', () => {
      const vector = 'CVSS:3.1/AV:N/AC:L/PR:L/UI:N/S:U/C:H/I:H/A:H';
      const result = parseCVSSVector(vector);

      expect(result.baseScore).toBeGreaterThanOrEqual(7.0);
      expect(result.baseScore).toBeLessThan(9.0);
      expect(result.severity).toBe('HIGH');
    });

    it('should parse CVSS v3.1 vector and calculate MEDIUM severity', () => {
      const vector = 'CVSS:3.1/AV:N/AC:L/PR:N/UI:R/S:U/C:L/I:L/A:N';
      const result = parseCVSSVector(vector);

      expect(result.baseScore).toBeGreaterThanOrEqual(4.0);
      expect(result.baseScore).toBeLessThan(7.0);
      expect(result.severity).toBe('MEDIUM');
    });

    it('should parse CVSS v3.1 vector and calculate LOW severity', () => {
      const vector = 'CVSS:3.1/AV:L/AC:H/PR:H/UI:R/S:U/C:L/I:N/A:N';
      const result = parseCVSSVector(vector);

      expect(result.baseScore).toBeGreaterThan(0.0);
      expect(result.baseScore).toBeLessThan(4.0);
      expect(result.severity).toBe('LOW');
    });

    it('should parse CVSS v3.1 vector and calculate NONE severity', () => {
      const vector = 'CVSS:3.1/AV:L/AC:H/PR:H/UI:R/S:U/C:N/I:N/A:N';
      const result = parseCVSSVector(vector);

      expect(result.baseScore).toBe(0.0);
      expect(result.severity).toBe('NONE');
    });
  });

  describe('CVSS v3.0', () => {
    it('should parse CVSS v3.0 vector and calculate severity', () => {
      const vector = 'CVSS:3.0/AV:N/AC:L/PR:L/UI:N/S:U/C:H/I:H/A:H';
      const result = parseCVSSVector(vector);

      expect(result.baseScore).toBeGreaterThanOrEqual(7.0);
      expect(result.severity).toBe('HIGH');
    });
  });

  describe('CVSS v2.0', () => {
    it('should parse CVSS v2.0 vector and calculate HIGH severity', () => {
      const vector = 'AV:N/AC:L/Au:N/C:C/I:C/A:C';
      const result = parseCVSSVector(vector);

      expect(result.baseScore).toBeGreaterThanOrEqual(7.0);
      expect(result.severity).toBe('HIGH');
    });

    it('should parse CVSS v2.0 vector and calculate MEDIUM severity', () => {
      const vector = 'AV:N/AC:M/Au:N/C:P/I:P/A:P';
      const result = parseCVSSVector(vector);

      expect(result.baseScore).toBeGreaterThanOrEqual(4.0);
      expect(result.baseScore).toBeLessThan(7.0);
      expect(result.severity).toBe('MEDIUM');
    });

    it('should parse CVSS v2.0 vector and calculate LOW severity', () => {
      const vector = 'AV:L/AC:H/Au:S/C:P/I:N/A:N';
      const result = parseCVSSVector(vector);

      expect(result.baseScore).toBeGreaterThan(0.0);
      expect(result.baseScore).toBeLessThan(4.0);
      expect(result.severity).toBe('LOW');
    });
  });

  describe('Error handling', () => {
    it('should throw error for invalid vector', () => {
      expect(() => parseCVSSVector('')).toThrow();
      expect(() => parseCVSSVector('invalid')).toThrow();
    });

    it('should throw error for CVSS v4.0 (not yet implemented)', () => {
      const vector = 'CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N';
      expect(() => parseCVSSVector(vector)).toThrow('CVSS v4.0 parsing not yet fully implemented');
    });

    it('should throw error for incomplete CVSS v3.1 vector', () => {
      const vector = 'CVSS:3.1/AV:N/AC:L';
      expect(() => parseCVSSVector(vector)).toThrow('Missing required CVSS v3 base metrics');
    });

    it('should throw error for incomplete CVSS v2.0 vector', () => {
      const vector = 'AV:N/AC:L';
      expect(() => parseCVSSVector(vector)).toThrow('Missing required CVSS v2 base metrics');
    });
  });
});
