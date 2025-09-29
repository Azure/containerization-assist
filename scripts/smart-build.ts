#!/usr/bin/env tsx
/**
 * Smart Build Script - Enhanced Current Approach
 * Enables parallel builds and optimized build processes
 */

import { exec, execSync } from 'child_process';
import { promisify } from 'util';
import * as fs from 'fs/promises';
import { watch, FSWatcher } from 'fs';
import * as path from 'path';
import * as crypto from 'crypto';

const execAsync = promisify(exec);

interface BuildOptions {
  parallel?: boolean;
  verbose?: boolean;
  skipTests?: boolean;
  watch?: boolean;
  clean?: boolean;
}

interface CacheEntry {
  filepath: string;
  hash: string;
  lastModified: number;
  dependencies: string[];
}

interface BuildStats {
  startTime: number;
  endTime?: number;
  phase: string;
  memory: number;
  errors: number;
}

class SmartBuilder {
  private startTime = Date.now();
  private options: BuildOptions;
  private buildCache: Map<string, CacheEntry> = new Map();
  private stats: BuildStats;
  private retryCount = 3;
  private retryDelay = 1000;
  private isBuilding = false;
  private watchedPaths: Set<string> = new Set();
  private watchTimeout: NodeJS.Timeout | null = null;

  constructor(options: BuildOptions = {}) {
    this.options = {
      parallel: true,
      verbose: false,
      skipTests: false,
      watch: false,
      clean: true,
      ...options
    };
    this.stats = {
      startTime: Date.now(),
      phase: 'initializing',
      memory: process.memoryUsage().heapUsed,
      errors: 0
    };
  }

  private log(message: string, type: 'info' | 'success' | 'error' | 'warning' = 'info') {
    // Only show essential messages
    if (type === 'error' || type === 'warning' || this.options.verbose) {
      const elapsed = ((Date.now() - this.startTime) / 1000).toFixed(1);
      const prefix = {
        info: '📝',
        success: '✅',
        error: '❌',
        warning: '⚠️'
      }[type];
      console.log(`[${elapsed}s] ${prefix} ${message}`);
    }
  }

  private async withRetry<T>(
    operation: () => Promise<T>,
    description: string,
    maxRetries = this.retryCount
  ): Promise<T> {
    let lastError: Error | undefined;

    for (let attempt = 1; attempt <= maxRetries; attempt++) {
      try {
        return await operation();
      } catch (error: any) {
        lastError = error;
        if (attempt < maxRetries) {
          this.log(
            `${description} failed (attempt ${attempt}/${maxRetries}). Retrying in ${this.retryDelay}ms...`,
            'warning'
          );
          await new Promise(resolve => setTimeout(resolve, this.retryDelay));
        }
      }
    }

    this.stats.errors++;
    throw lastError;
  }

  private async runCommand(cmd: string, description: string): Promise<void> {
    await this.withRetry(
      async () => {
        const { stdout, stderr } = await execAsync(cmd, {
          maxBuffer: 10 * 1024 * 1024, // 10MB buffer
          env: { ...process.env, NODE_OPTIONS: '--max-old-space-size=4096' }
        });

        if (this.options.verbose) {
          if (stdout) console.log(stdout);
          if (stderr && !stderr.includes('Warning')) console.error(stderr);
        }
      },
      description
    );
  }

  private async getFileHash(filepath: string): Promise<string> {
    try {
      const content = await fs.readFile(filepath, 'utf8');
      return crypto.createHash('md5').update(content).digest('hex');
    } catch {
      return '';
    }
  }

  private async shouldRebuild(filepath: string): Promise<boolean> {
    if (!this.buildCache.has(filepath)) return true;

    const cached = this.buildCache.get(filepath)!;
    const currentHash = await this.getFileHash(filepath);

    return currentHash !== cached.hash;
  }

  private async updateCache(filepath: string): Promise<void> {
    const hash = await this.getFileHash(filepath);
    const fileStats = await fs.stat(filepath);

    this.buildCache.set(filepath, {
      filepath,
      hash,
      lastModified: fileStats.mtimeMs,
      dependencies: []
    });
  }

  private async clean() {
    if (this.options.clean) {
      this.stats.phase = 'cleaning';
      await this.runCommand(
        'rm -rf dist dist-cjs coverage .tsbuildinfo* .tshy .tshy-build',
        'Clean build directories'
      );
      this.buildCache.clear();
    }
  }

  private async buildESM() {
    this.stats.phase = 'building-esm';
    await this.runCommand(
      'tsc && tsc-alias -f',
      'Build ESM'
    );
    // Copy knowledge data files to ESM build
    await this.copyKnowledgeData('dist');
  }

  private async buildCJS() {
    this.stats.phase = 'building-cjs';
    await this.runCommand(
      'tsc -p tsconfig.cjs.json && tsc-alias -p tsconfig.cjs.json -f',
      'Build CJS'
    );
    // Copy knowledge data files to CJS build
    await this.copyKnowledgeData('dist-cjs');
  }

  private async copyKnowledgeData(outputDir: string) {
    const sourceDir = path.join(process.cwd(), 'knowledge', 'packs');
    const targetDir = path.join(process.cwd(), outputDir, 'knowledge', 'packs');

    try {
      // Create target directory
      await fs.mkdir(targetDir, { recursive: true });

      // Copy all JSON files
      const files = await fs.readdir(sourceDir);
      const jsonFiles = files.filter(f => f.endsWith('.json'));

      for (const file of jsonFiles) {
        const source = path.join(sourceDir, file);
        const target = path.join(targetDir, file);
        await fs.copyFile(source, target);
      }

      // Knowledge pack files copied
    } catch (error: any) {
      this.log(`Failed to copy knowledge data: ${error.message}`, 'warning');
    }
  }

  private async validateBuilds(): Promise<void> {
    this.stats.phase = 'validating';

    const requiredFiles = [
      'dist/src/index.js',
      'dist/src/index.d.ts',
      'dist-cjs/src/index.js',
      'dist/src/mcp/mcp-server.js',
      'dist-cjs/src/mcp/mcp-server.js'
    ];

    for (const file of requiredFiles) {
      try {
        await fs.access(path.join(process.cwd(), file));
      } catch {
        throw new Error(`Required build output missing: ${file}`);
      }
    }
  }

  private async generateExports() {
    this.stats.phase = 'generating-exports';

    const exportPatterns = [
      { path: 'src/index', name: '.' },
      { path: 'src/mcp/mcp-server', name: './server' },
      { path: 'src/exports/tools', name: './tools' },
      { path: 'src/types/index', name: './types' },
      { path: 'src/config/index', name: './config' },
      { path: 'src/lib/index', name: './lib' },
    ];

    const exports: any = {};
    for (const pattern of exportPatterns) {
      exports[pattern.name] = {
        types: `./dist/${pattern.path}.d.ts`,
        import: `./dist/${pattern.path}.js`,
        require: `./dist-cjs/${pattern.path}.js`,
        default: `./dist/${pattern.path}.js`
      };
    }

    // Add tool exports
    const toolsDir = path.join(process.cwd(), 'src/tools');
    try {
      const tools = await fs.readdir(toolsDir, { withFileTypes: true });

      for (const tool of tools) {
        if (tool.isDirectory() && !tool.name.startsWith('.')) {
          const toolName = `./tools/${tool.name}`;
          exports[toolName] = {
            types: `./dist/src/tools/${tool.name}/index.d.ts`,
            import: `./dist/src/tools/${tool.name}/index.js`,
            require: `./dist-cjs/src/tools/${tool.name}/index.js`,
            default: `./dist/src/tools/${tool.name}/index.js`
          };
        }
      }
    } catch {
      this.log('Could not read tools directory', 'warning');
    }

    return exports;
  }

  private async updatePackageJson(exports: any): Promise<void> {
    const pkgPath = path.join(process.cwd(), 'package.json');
    const pkg = JSON.parse(await fs.readFile(pkgPath, 'utf8'));

    // Only update if exports have changed
    if (JSON.stringify(pkg.exports) !== JSON.stringify(exports)) {
      pkg.exports = exports;
      await fs.writeFile(pkgPath, JSON.stringify(pkg, null, 2) + '\n');
    }
  }

  private async runTests() {
    if (!this.options.skipTests) {
      this.stats.phase = 'testing';
      await this.runCommand(
        'NODE_OPTIONS="--experimental-vm-modules" npm run test:unit',
        'Run unit tests'
      );
    } else {
      this.log('Skipping tests (--skip-tests flag)', 'warning');
    }
  }

  private async measurePerformance() {
    this.stats.phase = 'measuring';

    if (!this.options.verbose) return;

    try {
      const distSize = execSync('du -sh dist/ 2>/dev/null || echo "0"').toString().trim();
      const cjsSize = execSync('du -sh dist-cjs/ 2>/dev/null || echo "0"').toString().trim();

      console.log(`\n📊 Build sizes: ESM ${distSize}, CJS ${cjsSize}`);
    } catch {
      // Silently skip metrics if not available
    }
  }

  async build() {
    console.log(`🚀 Building (${this.options.parallel ? 'parallel' : 'sequential'})...`);

    try {
      // Clean phase
      await this.clean();

      // Build phase
      this.stats.phase = 'building';
      let exports: any;

      if (this.options.parallel) {
        const [, , generatedExports] = await Promise.all([
          this.buildESM(),
          this.buildCJS(),
          this.generateExports()
        ]);
        exports = generatedExports;
      } else {
        await this.buildESM();
        await this.buildCJS();
        exports = await this.generateExports();
      }

      // Validate builds
      await this.validateBuilds();

      // Update package.json exports
      if (exports) {
        await this.updatePackageJson(exports);
      }

      // Post-build validation
      await this.runTests();
      await this.measurePerformance();

      // Final summary
      this.stats.endTime = Date.now();
      const totalTime = ((this.stats.endTime - this.startTime) / 1000).toFixed(2);

      console.log(`✅ Build completed in ${totalTime}s`);

      // Watch mode
      if (this.options.watch) {
        await this.startWatchMode();
      }

    } catch (error: any) {
      this.stats.endTime = Date.now();
      const totalTime = ((this.stats.endTime - this.startTime) / 1000).toFixed(2);

      console.error(`❌ Build failed in ${totalTime}s: ${error.message}`);
      if (!this.options.verbose) {
        console.error('Run with --verbose for more details');
      }

      process.exit(1);
    }
  }

  private async startWatchMode() {
    console.log('👀 Watch mode activated (Ctrl+C to exit)');

    const watchPaths = ['src', 'knowledge', 'package.json', 'tsconfig.json', 'tsconfig.cjs.json'];
    const watchers: FSWatcher[] = [];

    const handleFileChange = (filename: string | null) => {
      if (!filename) return;

      // Debounce file changes - wait 500ms after last change
      if (this.watchTimeout) {
        clearTimeout(this.watchTimeout);
      }

      this.watchTimeout = setTimeout(async () => {
        if (this.isBuilding) {
          this.log('Build already in progress, skipping...', 'warning');
          return;
        }

        this.log(`File changed: ${filename}`, 'info');
        await this.triggerRebuild();
      }, 500);
    };

    // Set up watchers for each path
    for (const watchPath of watchPaths) {
      try {
        const fullPath = path.join(process.cwd(), watchPath);
        const stats = await fs.stat(fullPath).catch(() => null);

        if (stats) {
          const watcher = watch(fullPath, {
            recursive: stats.isDirectory(),
            persistent: true
          }, (eventType, filename) => {
            // Filter out irrelevant files
            if (filename && this.shouldWatchFile(filename)) {
              handleFileChange(filename);
            }
          });

          watchers.push(watcher);
          this.watchedPaths.add(watchPath);
          this.log(`Watching: ${watchPath}`, 'info');
        }
      } catch (error: any) {
        this.log(`Failed to watch ${watchPath}: ${error.message}`, 'warning');
      }
    }

    // Set up cleanup
    const cleanup = () => {
      console.log('\n🛑 Shutting down watch mode...');

      if (this.watchTimeout) {
        clearTimeout(this.watchTimeout);
      }

      watchers.forEach(watcher => {
        try {
          watcher.close();
        } catch (error) {
          // Ignore cleanup errors
        }
      });

      console.log('👋 Watch mode terminated');
      process.exit(0);
    };

    process.on('SIGINT', cleanup);
    process.on('SIGTERM', cleanup);

    // Keep the process alive
    process.stdin.resume();
  }

  private shouldWatchFile(filename: string): boolean {
    // Only watch relevant file types
    const relevantExtensions = ['.ts', '.tsx', '.js', '.jsx', '.json'];
    const ext = path.extname(filename);

    // Ignore build outputs, node_modules, and temp files
    if (filename.includes('node_modules') ||
        filename.includes('dist') ||
        filename.includes('.git') ||
        filename.includes('.tsbuildinfo') ||
        filename.startsWith('.')) {
      return false;
    }

    return relevantExtensions.includes(ext) || filename === 'package.json';
  }

  private async triggerRebuild() {
    this.isBuilding = true;
    this.log('🔄 Rebuilding...', 'info');

    try {
      // Reset stats for rebuild
      this.stats = {
        startTime: Date.now(),
        phase: 'rebuilding',
        memory: process.memoryUsage().heapUsed,
        errors: 0
      };

      // Run a faster rebuild without cleaning
      const originalClean = this.options.clean;
      this.options.clean = false;

      // Build phase
      this.stats.phase = 'building';
      let exports: any;

      if (this.options.parallel) {
        const [, , generatedExports] = await Promise.all([
          this.buildESM(),
          this.buildCJS(),
          this.generateExports()
        ]);
        exports = generatedExports;
      } else {
        await this.buildESM();
        await this.buildCJS();
        exports = await this.generateExports();
      }

      // Validate builds
      await this.validateBuilds();

      // Update package.json exports
      if (exports) {
        await this.updatePackageJson(exports);
      }

      // Skip tests in watch mode for faster feedback
      if (!this.options.skipTests) {
        this.log('⏩ Skipping tests in watch mode for faster rebuilds', 'info');
      }

      // Restore original clean setting
      this.options.clean = originalClean;

      const rebuildTime = ((Date.now() - this.stats.startTime) / 1000).toFixed(2);
      console.log(`✅ Rebuild completed in ${rebuildTime}s`);

    } catch (error: any) {
      const rebuildTime = ((Date.now() - this.stats.startTime) / 1000).toFixed(2);
      console.error(`❌ Rebuild failed in ${rebuildTime}s: ${error.message}`);
    } finally {
      this.isBuilding = false;
    }
  }
}

// Parse CLI arguments
const args = process.argv.slice(2);

if (args.includes('--help') || args.includes('-h')) {
  console.log(`
Smart Build System - Usage:

  Options:
    --verbose, -v    Show detailed output
    --skip-tests     Skip running tests (default for npm run build)
    --watch          Enable watch mode
    --help, -h       Show this help message

  Examples:
    npm run build          # Fast parallel build
    npm run build:watch    # Build and watch for changes
`);
  process.exit(0);
}

const options: BuildOptions = {
  parallel: args.includes('--parallel') || !args.includes('--sequential'),
  verbose: args.includes('--verbose') || args.includes('-v'),
  skipTests: args.includes('--skip-tests'),
  watch: args.includes('--watch'),
  clean: !args.includes('--no-clean')
};

// Run build
const builder = new SmartBuilder(options);
builder.build().catch(console.error);