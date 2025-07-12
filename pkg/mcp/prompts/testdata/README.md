# Prompt Template Golden Tests

This directory contains golden test files for the Container Kit prompt templates. Golden tests ensure that templates produce consistent output and help detect unintended changes.

## Structure

```
testdata/
└── golden/         # Golden test files
    ├── *.golden    # Expected output for each test case
    └── *.actual    # Actual output when tests fail (gitignored)
```

## Running Golden Tests

### Normal Test Run (Verification Mode)
```bash
go test ./pkg/mcp/prompts -run TestGolden
```

### Update Golden Files
When templates are intentionally changed, update the golden files:
```bash
go test ./pkg/mcp/prompts -run TestGoldenTemplates -update-golden
```

## Test Coverage

The golden tests cover:

1. **Dockerfile Generation** (`containerKit.quickDockerfile`)
   - Go with Gin framework
   - Node.js with Express
   - Python with FastAPI
   - Various language/framework combinations

2. **Deployment Workflows** (`containerKit.deploy`)
   - Production environment
   - Staging environment

3. **Troubleshooting** (`containerKit.troubleshoot`)
   - Build errors
   - Deployment errors

4. **Repository Analysis** (`containerKit.analyze`)
   - Quick analysis
   - Comprehensive analysis

5. **Kubernetes Manifests** (`containerKit.k8sManifest`)
   - Simple development setup
   - Production-ready configuration

## Adding New Golden Tests

1. Add a new test case to `golden_test.go`
2. Run with `-update-golden` to create the initial golden file
3. Review the generated golden file for correctness
4. Commit both the test and golden file

## Debugging Failed Tests

When a golden test fails:

1. The test will create a `.actual` file showing the current output
2. Compare the `.golden` and `.actual` files to see the differences
3. If the change is intentional, run with `-update-golden`
4. If unintentional, fix the template or rendering logic