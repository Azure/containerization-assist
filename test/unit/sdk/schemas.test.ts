/**
 * JSON Schema validation tests.
 *
 * Ensures generated JSON schemas are valid and can be used for validation.
 */

import { describe, it, expect } from '@jest/globals';
import Ajv from 'ajv';
import { jsonSchemas } from '../../../src/sdk';

describe('JSON Schema generation', () => {
  // Create Ajv instance with strict mode disabled for JSON Schema draft-07 compatibility
  // zod-to-json-schema may generate schemas that use features not in strict mode
  const ajv = new Ajv({ strict: false });

  describe('schema validity', () => {
    it.each(Object.entries(jsonSchemas))(
      '%s schema is valid JSON Schema',
      (name, schema) => {
        // Compile should not throw for valid schemas
        const validate = ajv.compile(schema);
        expect(validate).toBeDefined();
        expect(typeof validate).toBe('function');
      },
    );
  });

  describe('analyzeRepo schema', () => {
    it('validates correct input', () => {
      const validate = ajv.compile(jsonSchemas.analyzeRepo);
      const valid = validate({ repositoryPath: '/path/to/repo' });
      expect(valid).toBe(true);
      expect(validate.errors).toBeNull();
    });

    it('rejects missing required field', () => {
      const validate = ajv.compile(jsonSchemas.analyzeRepo);
      const valid = validate({});
      expect(valid).toBe(false);
      expect(validate.errors).toBeDefined();
      expect(validate.errors?.some((e) => e.keyword === 'required')).toBe(true);
    });

    it('rejects wrong type for repositoryPath', () => {
      const validate = ajv.compile(jsonSchemas.analyzeRepo);
      const valid = validate({ repositoryPath: 123 });
      expect(valid).toBe(false);
      expect(validate.errors).toBeDefined();
    });
  });

  describe('buildImage schema', () => {
    it('validates input with path and imageName', () => {
      const validate = ajv.compile(jsonSchemas.buildImage);
      const valid = validate({
        path: '/path/to/app',
        imageName: 'myapp:latest',
      });
      expect(valid).toBe(true);
      expect(validate.errors).toBeNull();
    });

    it('validates input with optional fields', () => {
      const validate = ajv.compile(jsonSchemas.buildImage);
      const valid = validate({
        path: '/path/to/app',
        imageName: 'myapp:latest',
        dockerfile: 'Dockerfile.prod',
        platform: 'linux/amd64',
      });
      expect(valid).toBe(true);
    });

    it('accepts path only (imageName is optional)', () => {
      const validate = ajv.compile(jsonSchemas.buildImage);
      const valid = validate({ path: '/path/to/app' });
      expect(valid).toBe(true);
    });

    it('rejects wrong type for path', () => {
      const validate = ajv.compile(jsonSchemas.buildImage);
      const valid = validate({ path: 123 });
      expect(valid).toBe(false);
    });
  });

  describe('ops schema', () => {
    it('validates ping operation', () => {
      const validate = ajv.compile(jsonSchemas.ops);
      const valid = validate({ operation: 'ping' });
      expect(valid).toBe(true);
    });

    it('validates status operation', () => {
      const validate = ajv.compile(jsonSchemas.ops);
      const valid = validate({ operation: 'status' });
      expect(valid).toBe(true);
    });

    it('rejects invalid operation', () => {
      const validate = ajv.compile(jsonSchemas.ops);
      const valid = validate({ operation: 'invalid-operation' });
      expect(valid).toBe(false);
    });
  });

  describe('scanImage schema', () => {
    it('validates correct input', () => {
      const validate = ajv.compile(jsonSchemas.scanImage);
      const valid = validate({ imageId: 'myapp:latest' });
      expect(valid).toBe(true);
    });

    it('validates input with severity filter', () => {
      const validate = ajv.compile(jsonSchemas.scanImage);
      const valid = validate({
        imageId: 'myapp:latest',
        severity: 'HIGH',
      });
      expect(valid).toBe(true);
    });

    it('validates input with scanner option', () => {
      const validate = ajv.compile(jsonSchemas.scanImage);
      const valid = validate({
        imageId: 'myapp:latest',
        scanner: 'trivy',
        scanType: 'vulnerability',
      });
      expect(valid).toBe(true);
    });
  });

  describe('generateK8sManifests schema', () => {
    it('validates repository analysis mode input', () => {
      const validate = ajv.compile(jsonSchemas.generateK8sManifests);
      const valid = validate({
        name: 'myapp',
        modulePath: '/path/to/repo',
        manifestType: 'kubernetes',
        environment: 'production',
      });
      expect(valid).toBe(true);
    });

    it('validates ACA conversion mode input', () => {
      const validate = ajv.compile(jsonSchemas.generateK8sManifests);
      const valid = validate({
        acaManifest: 'apiVersion: containerapp.io/v1\nkind: ContainerApp',
        manifestType: 'kubernetes',
        environment: 'production',
      });
      expect(valid).toBe(true);
    });

    it('validates input with optional namespace', () => {
      const validate = ajv.compile(jsonSchemas.generateK8sManifests);
      const valid = validate({
        name: 'myapp',
        modulePath: '/path/to/repo',
        manifestType: 'kubernetes',
        environment: 'production',
        namespace: 'production',
      });
      expect(valid).toBe(true);
    });
  });

  describe('schema count', () => {
    it('exports exactly 11 schemas', () => {
      expect(Object.keys(jsonSchemas)).toHaveLength(11);
    });

    it('includes all expected schema names', () => {
      const expectedSchemas = [
        'analyzeRepo',
        'generateDockerfile',
        'fixDockerfile',
        'buildImage',
        'scanImage',
        'tagImage',
        'pushImage',
        'generateK8sManifests',
        'prepareCluster',
        'verifyDeploy',
        'ops',
      ];
      expect(Object.keys(jsonSchemas).sort()).toEqual(expectedSchemas.sort());
    });
  });
});
