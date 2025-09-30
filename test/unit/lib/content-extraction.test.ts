import {
  extractCodeBlock,
  extractJsonContent,
  extractYamlContent,
  extractYamlDocuments,
  extractDockerfileContent,
  extractKubernetesContent,
  extractHelmContent,
  extractContent,
  type ExtractionResult,
  type ExtractionOptions,
} from '@/lib/content-extraction';

describe('Content Extraction Utilities', () => {
  describe('extractCodeBlock', () => {
    it('should extract language-specific code blocks', () => {
      const text = `
Here's a Dockerfile:
\`\`\`dockerfile
FROM node:alpine
RUN npm install
CMD ["node", "app.js"]
\`\`\`
`;
      const result = extractCodeBlock(text, { language: 'dockerfile' });
      expect(result).toBe('FROM node:alpine\nRUN npm install\nCMD ["node", "app.js"]');
    });

    it('should extract generic code blocks', () => {
      const text = `
\`\`\`
FROM alpine
RUN echo "hello"
\`\`\`
`;
      const result = extractCodeBlock(text);
      expect(result).toBe('FROM alpine\nRUN echo "hello"');
    });

    it('should extract code blocks with different languages', () => {
      const text = `
\`\`\`yaml
apiVersion: v1
kind: Pod
\`\`\`
`;
      const result = extractCodeBlock(text, { language: 'yaml' });
      expect(result).toBe('apiVersion: v1\nkind: Pod');
    });

    it('should extract inline code for short content', () => {
      const text = 'Use `FROM node:alpine` as base image';
      const result = extractCodeBlock(text);
      expect(result).toBe('FROM node:alpine');
    });

    it('should fall back to raw content when enabled', () => {
      const text = 'This is plain text content';
      const result = extractCodeBlock(text, { fallbackToRaw: true });
      expect(result).toBe('This is plain text content');
    });

    it('should return null when no code blocks found and fallback disabled', () => {
      const text = 'This is plain text content';
      const result = extractCodeBlock(text, { fallbackToRaw: false });
      expect(result).toBe(null);
    });

    it('should handle empty input', () => {
      const result = extractCodeBlock('', { fallbackToRaw: false });
      expect(result).toBe(null);
    });

    it('should handle case-insensitive language matching', () => {
      const text = `
\`\`\`DOCKERFILE
FROM node:alpine
\`\`\`
`;
      const result = extractCodeBlock(text, { language: 'dockerfile' });
      expect(result).toBe('FROM node:alpine');
    });
  });

  describe('extractJsonContent', () => {
    it('should extract valid JSON from text', () => {
      const text = '{"name": "test", "version": "1.0.0"}';
      const result = extractJsonContent(text);
      expect(result).toEqual({ name: 'test', version: '1.0.0' });
    });

    it('should extract JSON from code blocks', () => {
      const text = `
Here's the config:
\`\`\`json
{"name": "test", "value": 42}
\`\`\`
`;
      const result = extractJsonContent(text);
      expect(result).toEqual({ name: 'test', value: 42 });
    });

    it('should extract JSON from mixed content', () => {
      const text = `
Some text before
{"dockerfile": "FROM node\\nRUN npm install"}
Some text after
`;
      const result = extractJsonContent(text);
      expect(result).toEqual({ dockerfile: 'FROM node\nRUN npm install' });
    });

    it('should return null for invalid JSON', () => {
      const text = 'This is not JSON content';
      const result = extractJsonContent(text);
      expect(result).toBe(null);
    });

    it('should return null for empty input', () => {
      const result = extractJsonContent('');
      expect(result).toBe(null);
    });

    it('should handle malformed JSON gracefully', () => {
      const text = '{"name": "test", "incomplete":}';
      const result = extractJsonContent(text);
      expect(result).toBe(null);
    });
  });

  describe('extractYamlContent', () => {
    it('should extract valid YAML from text', () => {
      const yamlText = `
name: test
version: 1.0.0
dependencies:
  - express
  - lodash
`;
      const result = extractYamlContent(yamlText);
      expect(result).toEqual({
        name: 'test',
        version: '1.0.0',
        dependencies: ['express', 'lodash'],
      });
    });

    it('should extract YAML from code blocks', () => {
      const text = `
Configuration:
\`\`\`yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
\`\`\`
`;
      const result = extractYamlContent(text);
      expect(result).toEqual({
        apiVersion: 'v1',
        kind: 'Pod',
        metadata: { name: 'test-pod' },
      });
    });

    it('should return null for invalid YAML', () => {
      const text = `
invalid: yaml: content:
  - unclosed
    bracket
`;
      const result = extractYamlContent(text);
      expect(result).toBe(null);
    });

    it('should return null for empty input', () => {
      const result = extractYamlContent('');
      expect(result).toBe(null);
    });
  });

  describe('extractYamlDocuments', () => {
    it('should extract multiple YAML documents', () => {
      const text = `
apiVersion: v1
kind: Service
metadata:
  name: my-service
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment
`;
      const result = extractYamlDocuments(text);
      expect(result).toHaveLength(2);
      expect(result[0]).toEqual({
        apiVersion: 'v1',
        kind: 'Service',
        metadata: { name: 'my-service' },
      });
      expect(result[1]).toEqual({
        apiVersion: 'apps/v1',
        kind: 'Deployment',
        metadata: { name: 'my-deployment' },
      });
    });

    it('should extract documents from code blocks', () => {
      const text = `
\`\`\`yaml
apiVersion: v1
kind: ConfigMap
---
apiVersion: v1
kind: Secret
\`\`\`
`;
      const result = extractYamlDocuments(text);
      expect(result).toHaveLength(2);
      expect(result[0].kind).toBe('ConfigMap');
      expect(result[1].kind).toBe('Secret');
    });

    it('should skip invalid documents', () => {
      const text = `
apiVersion: v1
kind: Service
---
invalid: yaml: content
---
apiVersion: v1
kind: Pod
`;
      const result = extractYamlDocuments(text);
      expect(result).toHaveLength(2);
      expect(result[0].kind).toBe('Service');
      expect(result[1].kind).toBe('Pod');
    });

    it('should handle single document without separator', () => {
      const text = `
apiVersion: v1
kind: Pod
metadata:
  name: single-pod
`;
      const result = extractYamlDocuments(text);
      expect(result).toHaveLength(1);
      expect(result[0].kind).toBe('Pod');
    });

    it('should return empty array for invalid content', () => {
      const text = 'Not YAML content at all';
      const result = extractYamlDocuments(text);
      expect(result).toHaveLength(0);
    });
  });

  describe('extractDockerfileContent', () => {
    it('should extract Dockerfile from code blocks', () => {
      const text = `
Here's the Dockerfile:
\`\`\`dockerfile
FROM node:alpine
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
CMD ["npm", "start"]
\`\`\`
`;
      const result = extractDockerfileContent(text);
      expect(result.success).toBe(true);
      expect(result.source).toBe('codeblock');
      expect(result.content).toContain('FROM node:alpine');
      expect(result.content).toContain('CMD ["npm", "start"]');
    });

    it('should extract Dockerfile from JSON response', () => {
      const text = `
{
  "dockerfile": "FROM alpine\\nRUN apk add curl\\nCMD [\\"sh\\"]"
}
`;
      const result = extractDockerfileContent(text);
      expect(result.success).toBe(true);
      expect(result.source).toBe('json');
      expect(result.content).toBe('FROM alpine\nRUN apk add curl\nCMD ["sh"]');
    });

    it('should extract Dockerfile by signature detection', () => {
      const text = `
This is a Dockerfile for the application:

FROM ubuntu:20.04
RUN apt-get update && apt-get install -y python3
WORKDIR /app
COPY app.py .
CMD ["python3", "app.py"]

The above Dockerfile uses Ubuntu as base.
`;
      const result = extractDockerfileContent(text);
      expect(result.success).toBe(true);
      expect(result.source).toBe('signature');
      expect(result.content).toContain('FROM ubuntu:20.04');
    });

    it('should fall back to raw content for Dockerfile-like content', () => {
      const text = `
RUN echo "This looks like Dockerfile content"
COPY file.txt /app/
WORKDIR /app
`;
      const result = extractDockerfileContent(text);
      expect(result.success).toBe(true);
      expect(result.source).toBe('raw');
      expect(result.content).toContain('RUN echo');
    });

    it('should fail when no Dockerfile content is found', () => {
      const text = 'This is just plain text with no Docker content';
      const result = extractDockerfileContent(text);
      expect(result.success).toBe(false);
      expect(result.error).toContain('No Dockerfile content found');
    });

    it('should handle empty input', () => {
      const result = extractDockerfileContent('');
      expect(result.success).toBe(false);
      expect(result.error).toContain('No Dockerfile content found');
    });

    it('should prioritize code blocks over signature detection', () => {
      const text = `
FROM ubuntu:latest in the text, but here's the real Dockerfile:
\`\`\`dockerfile
FROM alpine:3.14
RUN apk add --no-cache curl
\`\`\`
`;
      const result = extractDockerfileContent(text);
      expect(result.success).toBe(true);
      expect(result.source).toBe('codeblock');
      expect(result.content).toContain('FROM alpine:3.14');
      expect(result.content).not.toContain('FROM ubuntu:latest');
    });
  });

  describe('extractKubernetesContent', () => {
    it('should extract Kubernetes manifests from YAML', () => {
      const text = `
\`\`\`yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
spec:
  ports:
  - port: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment
spec:
  replicas: 3
\`\`\`
`;
      const result = extractKubernetesContent(text);
      expect(result.success).toBe(true);
      expect(result.source).toBe('yaml');
      expect(result.content).toHaveLength(2);
      expect((result.content as any[])[0].kind).toBe('Service');
      expect((result.content as any[])[1].kind).toBe('Deployment');
    });

    it('should validate Kubernetes manifest structure', () => {
      const text = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  key: value
---
invalid: document
missing: required fields
`;
      const result = extractKubernetesContent(text);
      expect(result.success).toBe(true);
      expect(result.content).toHaveLength(1);
      expect((result.content as any[])[0].kind).toBe('ConfigMap');
    });

    it('should fail when no valid manifests are found', () => {
      const text = `
name: not-kubernetes
type: invalid
---
another: invalid
document: true
`;
      const result = extractKubernetesContent(text);
      expect(result.success).toBe(false);
      expect(result.error).toContain('No valid Kubernetes manifests found');
    });

    it('should handle YAML parsing errors', () => {
      const text = `
apiVersion: v1
kind: Pod
metadata:
  invalid: yaml: syntax:
    - unclosed
`;
      const result = extractKubernetesContent(text);
      expect(result.success).toBe(false);
      expect(result.error).toContain('No valid Kubernetes manifests found');
    });

    it('should handle empty input', () => {
      const result = extractKubernetesContent('');
      expect(result.success).toBe(false);
    });
  });

  describe('extractHelmContent', () => {
    it('should extract structured Helm chart from JSON', () => {
      const text = `
{
  "Chart.yaml": "apiVersion: v2\\nname: myapp\\nversion: 1.0.0",
  "values.yaml": "replicaCount: 3\\nimage:\\n  repository: myapp"
}
`;
      const result = extractHelmContent(text);
      expect(result.success).toBe(true);
      expect(result.source).toBe('json');
      expect(Object.keys(result.content!)).toContain('Chart.yaml');
      expect(Object.keys(result.content!)).toContain('values.yaml');
    });

    it('should detect and categorize Chart.yaml content', () => {
      const text = `
\`\`\`yaml
apiVersion: v2
name: myapp
version: 1.0.0
description: A Helm chart for myapp
type: application
appVersion: 1.0.0
\`\`\`
`;
      const result = extractHelmContent(text);
      expect(result.success).toBe(true);
      expect(result.source).toBe('codeblock');
      expect(Object.keys(result.content!)).toContain('Chart.yaml');
    });

    it('should detect and categorize values.yaml content', () => {
      const text = `
\`\`\`yaml
replicaCount: 3
image:
  repository: nginx
  tag: latest
service:
  type: ClusterIP
  port: 80
\`\`\`
`;
      const result = extractHelmContent(text);
      expect(result.success).toBe(true);
      expect(result.source).toBe('codeblock');
      expect(Object.keys(result.content!)).toContain('values.yaml');
    });

    it('should handle generic template content', () => {
      const text = `
\`\`\`yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Values.appName }}
spec:
  replicas: {{ .Values.replicaCount }}
\`\`\`
`;
      const result = extractHelmContent(text);
      expect(result.success).toBe(true);
      expect(result.source).toBe('codeblock');
      expect(Object.keys(result.content!)).toContain('template.yaml');
    });

    it('should extract multiple YAML documents as separate files', () => {
      const text = `
apiVersion: v2
name: myapp
version: 1.0.0
---
replicaCount: 3
image:
  repository: myapp
`;
      const result = extractHelmContent(text);
      expect(result.success).toBe(true);
      expect(result.source).toBe('yaml');
      expect(Object.keys(result.content!)).toHaveLength(2);
      expect(Object.keys(result.content!)).toContain('chart-1.yaml');
      expect(Object.keys(result.content!)).toContain('chart-2.yaml');
    });

    it('should fail when no Helm content is found', () => {
      const text = 'This is not Helm chart content';
      const result = extractHelmContent(text);
      expect(result.success).toBe(false);
      expect(result.error).toContain('No Helm chart content found');
    });

    it('should handle extraction errors gracefully', () => {
      const text = `
{
  "invalid": "json: syntax
}
`;
      const result = extractHelmContent(text);
      expect(result.success).toBe(false);
      expect(result.error).toContain('No Helm chart content found');
    });
  });

  describe('extractContent (Universal)', () => {
    it('should auto-detect and extract Dockerfile content', () => {
      const text = `
FROM node:alpine
RUN npm install
COPY . /app
CMD ["node", "server.js"]
`;
      const result = extractContent(text);
      expect(result.success).toBe(true);
      expect(typeof result.content).toBe('string');
      expect((result.content as string).includes('FROM node:alpine')).toBe(true);
    });

    it('should auto-detect and extract Kubernetes content', () => {
      const text = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
`;
      const result = extractContent(text);
      expect(result.success).toBe(true);
      expect(Array.isArray(result.content)).toBe(true);
      expect((result.content as any[])[0].kind).toBe('Deployment');
    });

    it('should auto-detect and extract Helm content', () => {
      const text = `Here's a Helm chart with templates: {{ .Values.name }}
\`\`\`yaml
apiVersion: v2
name: myapp
version: 1.0.0
description: A Helm chart
\`\`\``;
      const result = extractContent(text);
      expect(result.success).toBe(true);
      expect(typeof result.content).toBe('object');
      expect(!Array.isArray(result.content)).toBe(true);
      expect(Object.keys(result.content as Record<string, string>)).toContain('Chart.yaml');
    });

    it('should use content type hint when provided', () => {
      const text = 'name: test\nversion: 1.0';
      const result = extractContent(text, 'yaml');
      expect(result.success).toBe(true);
      expect(result.source).toBe('yaml');
      expect(typeof result.content).toBe('object');
    });

    it('should handle JSON content type hint', () => {
      const text = '{"name": "test", "version": "1.0"}';
      const result = extractContent(text, 'json');
      expect(result.success).toBe(true);
      expect(result.source).toBe('json');
      expect(result.content).toEqual({ name: 'test', version: '1.0' });
    });

    it('should handle Dockerfile content type hint', () => {
      const text = `
Some text with dockerfile:
FROM alpine
RUN echo test
`;
      const result = extractContent(text, 'dockerfile');
      expect(result.success).toBe(true);
      expect(result.source).toBe('signature');
    });

    it('should fall back to code block extraction', () => {
      const text = `
\`\`\`
Some generic code content
that doesn't match specific patterns
\`\`\`
`;
      const result = extractContent(text);
      expect(result.success).toBe(true);
      expect(result.source).toBe('codeblock');
    });

    it('should fall back to code block content for unrecognized text', () => {
      const text = 'This is just plain text content';
      const result = extractContent(text);
      expect(result.success).toBe(true);
      expect(result.source).toBe('codeblock');
      expect(result.content).toBe('This is just plain text content');
    });

    it('should handle empty input', () => {
      const result = extractContent('');
      expect(result.success).toBe(false);
      expect(result.error).toBe('Empty input text');
    });

    it('should handle invalid YAML with hint', () => {
      const text = 'invalid: yaml: content';
      const result = extractContent(text, 'yaml');
      expect(result.success).toBe(false);
      expect(result.error).toBe('Invalid YAML content');
    });

    it('should handle invalid JSON with hint', () => {
      const text = '{"invalid": json}';
      const result = extractContent(text, 'json');
      expect(result.success).toBe(false);
      expect(result.error).toBe('Invalid JSON content');
    });
  });

  describe('Edge Cases and Error Handling', () => {
    it('should handle very large input gracefully', () => {
      const largeText = 'FROM alpine\n' + 'RUN echo test\n'.repeat(10000);
      const result = extractDockerfileContent(largeText);
      expect(result.success).toBe(true);
      expect(result.content).toContain('FROM alpine');
    });

    it('should handle special characters in content', () => {
      const text = `
\`\`\`dockerfile
FROM alpine
RUN echo "Special chars: áéíóú 中文 🚀 &nbsp;"
\`\`\`
`;
      const result = extractDockerfileContent(text);
      expect(result.success).toBe(true);
      expect(result.content).toContain('Special chars');
    });

    it('should handle nested code blocks', () => {
      const text = `
\`\`\`dockerfile
FROM alpine
RUN echo "\`\`\`nested\`\`\`"
\`\`\`
`;
      const result = extractCodeBlock(text, { language: 'dockerfile' });
      expect(result).toContain('FROM alpine');
      expect(result).toContain('RUN echo "');
    });

    it('should handle mixed line endings', () => {
      const text = 'FROM alpine\r\nRUN echo test\nCOPY . /app\r\n';
      const result = extractDockerfileContent(text);
      expect(result.success).toBe(true);
      expect(result.content).toContain('FROM alpine');
    });

    it('should handle content with only whitespace', () => {
      const result = extractContent('   \n\t  \r\n   ');
      expect(result.success).toBe(false);
      expect(result.error).toBe('Empty input text');
    });
  });
});