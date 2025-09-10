import type { AITemplate } from './types';

export const REPOSITORY_ANALYSIS: AITemplate = {
  id: 'repository-analysis',
  name: 'Universal Repository Analysis',
  description: 'AI-powered language and framework detection for any repository',
  version: '2.0.0',
  system:
    'You are an expert software architect with deep knowledge of ALL programming languages,\nframeworks, and build systems. Analyze repositories without bias toward any specific language.\n\nLanguages you support include but are not limited to:\n- Backend: Java, Python, Node.js/TypeScript, Go, Rust, C#, Ruby, PHP, Scala, Kotlin\n- Frontend: React, Vue, Angular, Svelte, Next.js, Nuxt.js\n- Mobile: Swift, Kotlin, React Native, Flutter\n- Data/ML: Python, R, Julia, Jupyter\n- Systems: C, C++, Rust, Zig\n\nProvide accurate, unbiased analysis focusing on the most likely language and framework.\n',
  user: 'Analyze this repository to identify the technology stack:\n\n**File listing:**\n{{fileList}}\n\n**Configuration files:**\n{{configFiles}}\n\n**Directory structure:**\n{{directoryTree}}\n\nDetermine:\n1. Primary programming language and version\n2. Framework and version (if applicable)  \n3. Build system and package manager\n4. Dependencies and dev dependencies\n5. Application entry points\n6. Default ports based on framework\n7. Recommended Docker base images (minimal, standard, secure)\n8. Containerization recommendations\n\nReturn ONLY valid JSON matching this structure:\n{\n  "language": "string",\n  "languageVersion": "string or null",\n  "framework": "string or null", \n  "frameworkVersion": "string or null",\n  "buildSystem": {\n    "type": "string",\n    "buildFile": "string",\n    "buildCommand": "string or null",\n    "testCommand": "string or null"\n  },\n  "dependencies": ["array of strings"],\n  "devDependencies": ["array of strings"],\n  "entryPoint": "string or null",\n  "suggestedPorts": [array of numbers],\n  "dockerConfig": {\n    "baseImage": "recommended base image",\n    "multistage": true/false,\n    "nonRootUser": true/false\n  }\n}\n',
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
