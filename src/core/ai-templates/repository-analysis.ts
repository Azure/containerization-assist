import type { AITemplate } from './types';

export const REPOSITORY_ANALYSIS: AITemplate = {
  id: 'repository-analysis',
  name: 'Universal Repository Analysis',
  description: 'AI-powered language and framework detection for any repository',
  version: '2.0.0',
  system: `You are an expert software architect with deep knowledge of ALL programming languages,
frameworks, and build systems. Analyze repositories without bias toward any specific language.

Languages you support include but are not limited to:
- Backend: Java, Python, Node.js/TypeScript, Go, Rust, C#, Ruby, PHP, Scala, Kotlin
- Frontend: React, Vue, Angular, Svelte, Next.js, Nuxt.js
- Mobile: Swift, Kotlin, React Native, Flutter
- Data/ML: Python, R, Julia, Jupyter
- Systems: C, C++, Rust, Zig

Provide accurate, unbiased analysis focusing on the most likely language and framework.`,
  user: `Analyze this repository to identify the technology stack:

**File listing:**
{{fileList}}

**Configuration files:**
{{configFiles}}

**Directory structure:**
{{directoryTree}}

Determine:
1. Primary programming language and version
2. Framework and version (if applicable)  
3. Build system and package manager
4. Dependencies and dev dependencies
5. Application entry points
6. Default ports based on framework
7. Recommended Docker base images (minimal, standard, secure)
8. Containerization recommendations

Return ONLY valid JSON matching this structure:
{
  "language": "string",
  "languageVersion": "string or null",
  "framework": "string or null", 
  "frameworkVersion": "string or null",
  "buildSystem": {
    "type": "string",
    "buildFile": "string",
    "buildCommand": "string or null",
    "testCommand": "string or null"
  },
  "dependencies": ["array of strings"],
  "devDependencies": ["array of strings"],
  "entryPoint": "string or null",
  "suggestedPorts": [array of numbers],
  "dockerConfig": {
    "baseImage": "recommended base image",
    "multistage": true/false,
    "nonRootUser": true/false
  }
}`,
  outputFormat: 'json',
  variables: [
    {
      name: 'fileList',
      description: 'List of files in the repository',
      required: true,
    },
    {
      name: 'configFiles',
      description: 'Content of configuration files',
      required: true,
    },
    {
      name: 'directoryTree',
      description: 'Directory structure of the repository',
      required: true,
    },
  ],
  examples: [
    {
      input: {
        fileList: 'package.json\nserver.js\nroutes/index.js\npublic/index.html\n',
        configFiles:
          '=== package.json ===\n{\n  "name": "my-app",\n  "version": "1.0.0",\n  "dependencies": {\n    "express": "^4.18.0"\n  }\n}\n',
        directoryTree: 'package.json\nserver.js\nroutes/\n  index.js\npublic/\n  index.html\n',
      },
      output:
        '{\n  "language": "javascript",\n  "languageVersion": "18.0.0",\n  "framework": "express",\n  "frameworkVersion": "4.18.0",\n  "buildSystem": {\n    "type": "npm",\n    "buildFile": "package.json",\n    "buildCommand": "npm run build",\n    "testCommand": "npm test"\n  },\n  "dependencies": ["express"],\n  "devDependencies": [],\n  "entryPoint": "server.js",\n  "suggestedPorts": [3000],\n  "dockerConfig": {\n    "baseImage": "node:18-slim",\n    "multistage": true,\n    "nonRootUser": true\n  }\n}\n',
    },
  ],
  tags: ['repository', 'analysis', 'language-detection', 'universal'],
} as const;
