export const ERROR_ANALYSIS_TEMPLATE = {
    id: "error-analysis",
    name: "Universal Error Analysis",
    description: "Analyze and provide solutions for containerization errors across all languages",
    version: "2.0.0",
    system: `You are an expert in troubleshooting containerization issues across ALL programming languages and frameworks.
Analyze errors and provide actionable solutions with root cause analysis.

Focus on:
1. Docker build failures
2. Runtime errors
3. Network and port issues
4. Dependency problems
5. Security and permission issues
6. Resource constraints`,
    user: `Analyze this containerization error and provide solutions:

**Error Context:**
- Language: {{language}}
- Framework: {{framework}}
- Build System: {{buildSystem}}
- Error Type: {{errorType}}

**Error Details:**
{{errorMessage}}

**Build Context:**
{{buildContext}}

**Requirements:**
1. Identify the root cause
2. Provide step-by-step solution
3. Suggest preventive measures
4. Include {{language}}-specific best practices

Return a structured analysis with clear recommendations.`,
    outputFormat: "text",
    variables: [
        {
            name: "language",
            description: "Programming language",
            required: true
        },
        {
            name: "framework",
            description: "Application framework",
            required: false
        },
        {
            name: "buildSystem",
            description: "Build system being used",
            required: false
        },
        {
            name: "errorType",
            description: "Type of error (build, runtime, network, etc.)",
            required: false,
            default: "build"
        },
        {
            name: "errorMessage",
            description: "The actual error message",
            required: true
        },
        {
            name: "buildContext",
            description: "Additional context about the build",
            required: false,
            default: "Standard containerization process"
        }
    ],
    tags: [
        "error-analysis",
        "troubleshooting",
        "universal"
    ]
} as const;