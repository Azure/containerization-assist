/* eslint-disable @typescript-eslint/no-require-imports */
/* eslint-disable no-undef */
const { platform: _platform, arch } = require('os');
const { spawn } = require('child_process');
const process = require('process');
const path = require('path');
const fs = require('fs');

class BuildBins {
    constructor(options = {}) {
        this.options = options || {};
    }

    apply(compiler) {
        compiler.hooks.afterEmit.tapAsync('BuildBins', async (compilation, callback) => {
            const lifecycle = process.env.npm_lifecycle_event;
            const platform = process.env.MCP_PACKAGE_PLATFORM || _platform() + '-' + arch();
            try {
                if (lifecycle === 'package' || lifecycle.startsWith('package:')) {
                    const packageFor = lifecycle.split(':')[1] || '';
                    console.log('Building MCP executables for platform:', platform, 'for:', packageFor);
                    await packBins(compilation.options.output.path, compilation.options.output.filename, platform, packageFor);
                }
            } catch (error) {
                console.error('Error during MCP executables processing:', error);
                return callback(error);
            }
            callback();
        });
    }
}

function packBins(distPath, entryFile, platform, packageFor = '') {
    return new Promise((resolve, reject) => {
        const binsPath = path.join(distPath, 'bins');
        if (!fs.existsSync(binsPath)) {
            fs.mkdirSync(binsPath, { recursive: true });
        }
        const pkgTargets = [];
        switch (platform) {
            case 'darwin-x64':
                pkgTargets.push('node18-macos-x64');
                break;
            case 'darwin-arm64':
                pkgTargets.push('node18-macos-arm64');
                break;
            default:
                pkgTargets.push(`node18-${platform}`);
        }
        if (pkgTargets.length === 0) {
            return reject(new Error('No MCP package targets specified'));
        }
        const serverName = packageFor ? path.join(binsPath, `mcp-server-${packageFor}`) : path.join(binsPath, 'mcp-server');
        const pkgConfigFile = packageFor ? path.join(__dirname, '../', `pkg.config.${packageFor}.json`) : path.join(__dirname, '../', 'pkg.config.json');
        console.log('Using pkg config file:', pkgConfigFile);
        const entryFilePath = path.join(distPath, entryFile);
        const child = spawn(
            'npx',
            ['pkg', '--targets', pkgTargets.join(','), '--compress', 'Gzip', '--config', pkgConfigFile, '--list', '--output', serverName, entryFilePath],
            {
                stdio: 'inherit',
                env: {
                    ...process.env,
                },
                shell: process.platform === 'win32',
            },
        );

        child.on('error', (error) => {
            console.error('Error spawning pkg command:', error);
            reject(error);
        });

        child.on('close', (code) => {
            if (code !== 0) {
                return reject(new Error(`pkg command failed with code ${code}`));
            }

            resolve();
        });
    });
}

module.exports = { BuildBins };
