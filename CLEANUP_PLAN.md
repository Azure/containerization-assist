# Cleanup Plan: Removing Over-Engineered Code

## Current State (Terrible)
- **3 over-engineered utility modules** (700+ lines of wrapper code)
- **15+ files importing these utilities** instead of Node.js built-ins
- **Massive complexity** for zero additional functionality

## Files That Need Cleanup

### Config Files (2 files)
- `src/config/app-config.ts` - Remove imports of `getTempDir`, `getDockerSocketPath`, `getKubeConfigPath`
- `src/config/config.ts` - Remove imports of `getTempDir`, `getDockerSocketPath`, `getKubeConfigPath`

### Tool Files (9 files)  
- `src/tools/generate-k8s-manifests/tool.ts` - Replace `joinPaths` with `path.join`
- `src/tools/generate-aca-manifests/tool.ts` - Replace `joinPaths` with `path.join`
- `src/tools/analyze-repo/tool.ts` - Replace `joinPaths`, `getExtension`, `safeNormalizePath` with `path.*`
- `src/tools/generate-helm-charts/tool.ts` - Replace `joinPaths` with `path.join`
- `src/tools/build-image/tool.ts` - Replace multiple path utilities with `path.*`
- `src/tools/prepare-cluster/tool.ts` - Replace command utilities with direct spawn/API calls
- `src/tools/convert-aca-to-k8s/tool.ts` - Replace `joinPaths` with `path.join`
- `src/tools/generate-dockerfile/tool.ts` - Replace `safeNormalizePath` with `path.normalize`

### Knowledge Files (2 files)
- `src/knowledge/enhanced-loader.ts` - Replace `resolvePath` with `path.resolve`
- `src/knowledge/loader.ts` - Replace `resolvePath` with `path.resolve`

### Parsing File (1 file)
- `src/lib/parsing-package-json.ts` - Replace `joinPaths` with `path.join`

## Files to Delete Completely
- `src/lib/system-utils.ts` (150+ lines of garbage)
- `src/lib/path-utils.ts` (150+ lines of wrapper hell) 
- `src/lib/command-utils.ts` (300+ lines of abstraction hell)
- `test/unit/cross-platform.test.ts` (over-engineered test file)
- `docs/CROSS_PLATFORM_COMPATIBILITY.md` (documentation for the bad approach)

## What Stays (The Good Stuff)
- `src/config/index.ts` - Simple targeted Docker socket fix ✅
- `src/cli/cli.ts` - Simple targeted Docker socket fix ✅  
- `test/unit/simple-cross-platform.test.ts` - Simple test that validates the actual fix ✅
- `CROSS_PLATFORM_ANALYSIS_SIMPLE.md` - Good analysis of the simple approach ✅
- `ROASTING_MY_TERRIBLE_CODE.md` - Honest self-assessment ✅

## Replacement Strategy

### Instead of my wrappers:
```typescript
// BAD (my over-engineered code):
import { joinPaths, resolvePath, getTempDir } from '@/lib/path-utils';
import { getSystemInfo } from '@/lib/system-utils';
import { executeCommand } from '@/lib/command-utils';

// GOOD (Node.js built-ins):
import path from 'path';
import os from 'os';
import { spawn } from 'child_process';
```

### Simple replacements:
- `joinPaths(...)` → `path.join(...)`
- `resolvePath(...)` → `path.resolve(...)`  
- `getTempDir()` → `os.tmpdir()`
- `getExtension(...)` → `path.extname(...)`
- `safeNormalizePath(...)` → `path.normalize(...)`
- `getSystemInfo()` → `os.platform() === 'win32'`
- Complex command utilities → Use Docker/K8s APIs directly

## Expected Outcome
- **Remove 700+ lines of wrapper code**
- **Replace with ~20 lines of Node.js built-in imports**
- **Keep the 3 lines that actually fixed the cross-platform issues**
- **Dramatically simplify the codebase**
- **Remove maintenance burden**

## Risk Assessment
- **Low risk**: These are just wrappers around Node.js built-ins
- **Same functionality**: Node.js built-ins do exactly what my wrappers do
- **Better maintainability**: Standard APIs instead of custom abstractions
- **Reduced complexity**: Fewer dependencies, simpler code

This cleanup will remove my engineering mistake and return the codebase to a clean, simple state.
