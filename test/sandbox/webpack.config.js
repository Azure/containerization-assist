/* eslint-disable no-undef */
/* eslint-disable @typescript-eslint/no-require-imports */
const path = require('path');
const BuildBins = require('./scripts/buildBins.js').BuildBins;
module.exports = {
  target: 'node',
  entry: './src/index.ts', // Updated to correct server file path
  output: {
    path: path.resolve(__dirname, 'dist'),
    filename: 'index.js', // Changed to match package.json main
    clean: true,
  },
  devtool: 'source-map',
  resolve: {
    extensions: ['.ts', '.js'], // Added .ts extension
    alias: {
      sharp: false, // Disable sharp for webpack bundling
      'containerization-assist-mcp$': path.resolve(__dirname, '../../dist-cjs/src/index.js'),
      'containerization-assist-mcp/tools$': path.resolve(
        __dirname,
        '../../dist-cjs/src/tools/index.js',
      ),
      'containerization-assist-mcp/server$': path.resolve(
        __dirname,
        '../../dist-cjs/src/mcp/mcp-server.js',
      ),
      'containerization-assist-mcp/types$': path.resolve(
        __dirname,
        '../../dist-cjs/src/types/index.js',
      ),
      'containerization-assist-mcp/config$': path.resolve(
        __dirname,
        '../../dist-cjs/src/config/index.js',
      ),
    },
  },
  externals: {
    'onnxruntime-node': 'commonjs onnxruntime-node', // Exclude onnxruntime-node from bundling
    applicationinsights: 'commonjs applicationinsights', // Exclude applicationinsights from bundling
    '@vscode/deviceid': 'commonjs @vscode/deviceid', // Exclude @vscode/deviceid from bundling
    playwright: 'commonjs playwright', // Exclude playwright from bundling
    'playwright-core': 'commonjs playwright-core', // Exclude playwright-core from bundling
  },
  plugins: [new BuildBins()],
  module: {
    rules: [
      {
        test: /\.ts$/,
        use: 'ts-loader',
        exclude: /node_modules/,
      },
      {
        test: /\.node$/,
        loader: 'node-loader',
      },
    ],
  },
};
