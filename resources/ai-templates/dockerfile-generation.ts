export const DOCKERFILE_GENERATION_TEMPLATE = {
    id: "dockerfile-generation",
    name: "Universal Dockerfile Generation",
    description: "Generate optimized Dockerfiles for any technology stack",
    version: "2.0.0",
    system: `You are a Docker expert specializing in containerizing applications in ANY programming language.
Generate production-ready, secure, and optimized Dockerfiles following these principles:

1. Use official base images with specific version tags (never 'latest')
2. Implement multi-stage builds when beneficial
3. Run as non-root user for security
4. Optimize layer caching for the specific build system
5. Minimize final image size
6. Include health checks where supported
7. Handle signals properly for graceful shutdown`,
    user: `Generate a production-ready Dockerfile for:

**Technology Stack:**
- Language: {{language}} {{languageVersion}}
- Framework: {{framework}} {{frameworkVersion}}
- Build System: {{buildSystemType}}
- Entry Point: {{entryPoint}}
- Port: {{port}}

**Dependencies:**
- Production: {{dependencies}}
- Development: {{devDependencies}}

**Requirements:**
1. Optimize for {{language}} best practices
2. Use multi-stage build if it reduces image size
3. Configure for port {{port}}
4. Add health check if supported by {{framework}}
5. Include security scanning labels

Generate ONLY the Dockerfile content without explanation.`,
    outputFormat: "dockerfile",
    variables: [
        {
            name: "language",
            description: "Primary programming language",
            required: true
        },
        {
            name: "languageVersion",
            description: "Language version",
            required: false
        },
        {
            name: "framework",
            description: "Application framework",
            required: false
        },
        {
            name: "frameworkVersion",
            description: "Framework version",
            required: false
        },
        {
            name: "buildSystemType",
            description: "Build system type (npm, maven, go, etc.)",
            required: true
        },
        {
            name: "entryPoint",
            description: "Application entry point file",
            required: true
        },
        {
            name: "port",
            description: "Application port",
            required: true,
            default: "8080"
        },
        {
            name: "dependencies",
            description: "Production dependencies",
            required: false,
            default: "[]"
        },
        {
            name: "devDependencies",
            description: "Development dependencies",
            required: false,
            default: "[]"
        }
    ],
    examples: [
        {
            input: {
                language: "javascript",
                languageVersion: "18",
                framework: "express",
                frameworkVersion: "4.18.0",
                buildSystemType: "npm",
                entryPoint: "server.js",
                port: "3000",
                dependencies: "[\"express\", \"cors\"]",
                devDependencies: "[\"nodemon\", \"jest\"]"
            },
            output: "FROM node:18-slim AS builder\nWORKDIR /app\nCOPY package*.json ./\nRUN npm ci --only=production && npm cache clean --force\n\nFROM node:18-slim\nWORKDIR /app\nRUN groupadd -r appuser && useradd -r -g appuser appuser\nCOPY --from=builder --chown=appuser:appuser /app/node_modules ./node_modules\nCOPY --chown=appuser:appuser . .\nEXPOSE 3000\nUSER appuser\nHEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\\n  CMD curl -f http://localhost:3000/health || exit 1\nENTRYPOINT [\"node\", \"server.js\"]\n"
        },
        {
            input: {
                language: "python",
                languageVersion: "3.11",
                framework: "fastapi",
                frameworkVersion: "0.104.0",
                buildSystemType: "pip",
                entryPoint: "main.py",
                port: "8000",
                dependencies: "[\"fastapi\", \"uvicorn\"]",
                devDependencies: "[\"pytest\", \"black\"]"
            },
            output: "FROM python:3.11-slim AS builder\nWORKDIR /app\nCOPY requirements.txt .\nRUN pip install --user --no-cache-dir -r requirements.txt\n\nFROM python:3.11-slim\nWORKDIR /app\nRUN groupadd -r appuser && useradd -r -g appuser appuser\nCOPY --from=builder /root/.local /home/appuser/.local\nCOPY --chown=appuser:appuser . .\nENV PATH=/home/appuser/.local/bin:$PATH\nEXPOSE 8000\nUSER appuser\nHEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\\n  CMD curl -f http://localhost:8000/health || exit 1\nENTRYPOINT [\"python\", \"-m\", \"uvicorn\", \"main:app\", \"--host\", \"0.0.0.0\", \"--port\", \"8000\"]\n"
        }
    ],
    tags: [
        "dockerfile",
        "containerization",
        "universal",
        "multi-language"
    ]
} as const;