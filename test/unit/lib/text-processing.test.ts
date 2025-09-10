/**
 * Tests for text processing utilities
 */

import { describe, test, expect } from '@jest/globals';
import {
  stripFencesAndNoise,
  isValidDockerfileContent,
  isValidKubernetesContent,
  extractBaseImage,
  parseInstructions,
  cleanAIResponse,
  boundToSentences,
} from '@lib/text-processing';

describe('Text Processing Utilities', () => {
  describe('stripFencesAndNoise', () => {
    test('removes dockerfile code fences', () => {
      const input = '```dockerfile\nFROM node:18\nRUN npm install\n```';
      const expected = 'FROM node:18\nRUN npm install';
      expect(stripFencesAndNoise(input)).toBe(expected);
    });

    test('handles various fence formats', () => {
      expect(stripFencesAndNoise('```docker\nFROM alpine\n```')).toBe('FROM alpine');
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
    test('validates proper dockerfile', () => {
      expect(isValidDockerfileContent('FROM node:18\nWORKDIR /app')).toBe(true);
      expect(isValidDockerfileContent('from ubuntu:20.04\nRUN apt update')).toBe(true);
      expect(isValidDockerfileContent('  FROM alpine\n  RUN echo "hello"')).toBe(true);
    });

    test('rejects invalid dockerfile', () => {
      expect(isValidDockerfileContent('RUN npm install')).toBe(false);
      expect(isValidDockerfileContent('Just some text')).toBe(false);
      expect(isValidDockerfileContent('')).toBe(false);
    });

    test('handles FROM instruction in middle of file', () => {
      expect(isValidDockerfileContent('# Comment\nFROM node:18')).toBe(true);
      expect(isValidDockerfileContent('RUN echo "test"\nFROM node:18')).toBe(true);
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
    test('extracts base image from dockerfile', () => {
      expect(extractBaseImage('FROM node:18-alpine\nWORKDIR /app')).toBe('node:18-alpine');
      expect(extractBaseImage('FROM ubuntu:20.04')).toBe('ubuntu:20.04');
      expect(extractBaseImage('  FROM  python:3.9  \nRUN pip install')).toBe('python:3.9');
    });

    test('handles multi-stage builds', () => {
      const dockerfile = `
FROM node:18 AS builder
WORKDIR /app
FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
      `;
      expect(extractBaseImage(dockerfile)).toBe('node:18');
    });

    test('returns null for invalid dockerfile', () => {
      expect(extractBaseImage('RUN echo "no from"')).toBeNull();
      expect(extractBaseImage('')).toBeNull();
    });
  });

  describe('parseInstructions', () => {
    test('parses dockerfile instructions', () => {
      const dockerfile = `
FROM node:18
WORKDIR /app
RUN npm install
EXPOSE 3000
      `.trim();
      
      const instructions = parseInstructions(dockerfile);
      expect(instructions).toHaveLength(4);
      expect(instructions[0]).toEqual({ instruction: 'FROM', content: 'node:18' });
      expect(instructions[1]).toEqual({ instruction: 'WORKDIR', content: '/app' });
      expect(instructions[2]).toEqual({ instruction: 'RUN', content: 'npm install' });
      expect(instructions[3]).toEqual({ instruction: 'EXPOSE', content: '3000' });
    });

    test('ignores comments and empty lines', () => {
      const dockerfile = `
# This is a comment
FROM node:18

# Another comment
WORKDIR /app
      `.trim();
      
      const instructions = parseInstructions(dockerfile);
      expect(instructions).toHaveLength(2);
      expect(instructions[0]).toEqual({ instruction: 'FROM', content: 'node:18' });
      expect(instructions[1]).toEqual({ instruction: 'WORKDIR', content: '/app' });
    });

    test('handles empty dockerfile', () => {
      expect(parseInstructions('')).toHaveLength(0);
      expect(parseInstructions('# Only comments')).toHaveLength(0);
    });
  });

  describe('cleanAIResponse', () => {
    test('cleans fenced response with excessive newlines', () => {
      const response = '```dockerfile\n\n\nFROM node:18\n\n\nWORKDIR /app\n\n\n```';
      const cleaned = cleanAIResponse(response);
      expect(cleaned).toBe('FROM node:18\n\nWORKDIR /app\n');
    });

    test('ensures final newline', () => {
      const response = '```\nFROM alpine```';
      expect(cleanAIResponse(response)).toBe('FROM alpine\n');
    });

    test('removes trailing whitespace from lines', () => {
      const response = 'FROM node:18   \nWORKDIR /app  \n';
      expect(cleanAIResponse(response)).toBe('FROM node:18\nWORKDIR /app\n');
    });
  });

  describe('boundToSentences', () => {
    test('bounds text to maximum sentences', () => {
      const text = 'First sentence. Second sentence. Third sentence. Fourth sentence. Fifth sentence.';
      const bounded = boundToSentences(text, 2, 3);
      expect(bounded).toBe('First sentence. Second sentence. Third sentence.');
    });

    test('returns all sentences if under limit', () => {
      const text = 'First sentence. Second sentence.';
      const bounded = boundToSentences(text, 2, 4);
      expect(bounded).toBe('First sentence. Second sentence.');
    });

    test('handles single sentence', () => {
      const text = 'Only one sentence.';
      expect(boundToSentences(text, 2, 4)).toBe('Only one sentence.');
    });

    test('handles empty input', () => {
      expect(boundToSentences('', 2, 4)).toBe('');
    });
  });

  describe('extractTextFromContent', () => {
    test('extracts text from MCP content arrays', () => {
      const content = [
        { type: 'text', text: 'Hello ' },
        { type: 'text', text: 'World' },
        { type: 'image', data: 'base64...' }, // Should be ignored
      ];
      const text = content
        .filter((item) => item.type === 'text' && typeof item.text === 'string')
        .map((item) => item.text!)
        .join('\n')
        .trim();
      expect(text).toBe('Hello \nWorld');
    });

    test('handles empty content arrays', () => {
      const content: Array<{ type: string; text?: string }> = [];
      const text = content
        .filter((item) => item.type === 'text' && typeof item.text === 'string')
        .map((item) => item.text!)
        .join('\n')
        .trim();
      expect(text).toBe('');
    });

    test('filters out non-text content', () => {
      const content = [
        { type: 'image', data: 'base64...' },
        { type: 'audio', data: 'audio...' },
      ];
      const text = content
        .filter((item) => item.type === 'text' && typeof item.text === 'string')
        .map((item) => item.text!)
        .join('\n')
        .trim();
      expect(text).toBe('');
    });
  });
});