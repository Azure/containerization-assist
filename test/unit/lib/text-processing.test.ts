/**
 * Tests for text processing utilities
 */

import { describe, test, expect } from '@jest/globals';
import {
  stripFencesAndNoise,
  isValidDockerfileContent,
  isValidKubernetesContent,
  extractBaseImage,
} from '@/lib/text-processing';

describe('Text Processing Utilities', () => {
  describe('stripFencesAndNoise', () => {
    test('removes dockerfile code fences', () => {
      const input = '```dockerfile\nFROM node:18\nRUN npm install\n```';
      const expected = 'FROM node:18\nRUN npm install';
      expect(stripFencesAndNoise(input, 'dockerfile')).toBe(expected);
    });

    test('handles various fence formats', () => {
      expect(stripFencesAndNoise('```docker\nFROM alpine\n```', 'dockerfile')).toBe('FROM alpine');
      expect(stripFencesAndNoise('```\nFROM alpine\n```')).toBe('FROM alpine');
      expect(stripFencesAndNoise('FROM alpine')).toBe('FROM alpine');
    });

    test('handles text without fences', () => {
      const input = 'FROM alpine\nRUN apk add --no-cache nodejs';
      expect(stripFencesAndNoise(input)).toBe(input);
    });

    test('handles empty input', () => {
      expect(stripFencesAndNoise('')).toBe('');
      expect(stripFencesAndNoise('```\n```')).toBe('');
    });
  });

  describe('isValidDockerfileContent', () => {
    test('validates proper dockerfile', async () => {
      expect(await isValidDockerfileContent('FROM node:18\nWORKDIR /app')).toBe(true);
      expect(await isValidDockerfileContent('from ubuntu:20.04\nRUN apt update')).toBe(true);
      expect(await isValidDockerfileContent('  FROM alpine\n  RUN echo "hello"')).toBe(true);
    });

    test('rejects invalid dockerfile', async () => {
      expect(await isValidDockerfileContent('RUN npm install')).toBe(false);
      expect(await isValidDockerfileContent('Just some text')).toBe(false);
      expect(await isValidDockerfileContent('')).toBe(false);
    });

    test('handles FROM instruction in middle of file', async () => {
      expect(await isValidDockerfileContent('# Comment\nFROM node:18')).toBe(true);
      expect(await isValidDockerfileContent('RUN echo "test"\nFROM node:18')).toBe(true);
    });
  });

  describe('isValidKubernetesContent', () => {
    test('validates proper kubernetes manifest', () => {
      const manifest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
      `.trim();
      expect(isValidKubernetesContent(manifest)).toBe(true);
    });

    test('validates with different field order', () => {
      const manifest = `
kind: Service
apiVersion: v1
metadata:
  name: my-service
      `.trim();
      expect(isValidKubernetesContent(manifest)).toBe(true);
    });

    test('rejects invalid kubernetes content', () => {
      expect(isValidKubernetesContent('just some yaml\nkey: value')).toBe(false);
      expect(isValidKubernetesContent('apiVersion: v1\n# missing kind')).toBe(false);
      expect(isValidKubernetesContent('')).toBe(false);
    });
  });

  describe('extractBaseImage', () => {
    test('extracts base image from dockerfile', async () => {
      expect(await extractBaseImage('FROM node:18-alpine\nWORKDIR /app')).toBe('node:18-alpine');
      expect(await extractBaseImage('FROM ubuntu:20.04')).toBe('ubuntu:20.04');
      expect(await extractBaseImage('  FROM  python:3.9  \nRUN pip install')).toBe('python:3.9');
    });

    test('handles multi-stage builds', async () => {
      const dockerfile = `
FROM node:18 AS builder
WORKDIR /app
FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
      `;
      expect(await extractBaseImage(dockerfile)).toBe('node:18');
    });

    test('returns null for invalid dockerfile', async () => {
      expect(await extractBaseImage('RUN echo "no from"')).toBeNull();
      expect(await extractBaseImage('')).toBeNull();
    });
  });
});
