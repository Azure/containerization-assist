/**
 * Node.js Express Basic Repository Fixture
 * Simple Express application for testing
 */

export const nodeExpressBasicRepository = {
  'package.json': {
    name: 'node-express-basic',
    version: '1.0.0',
    description: 'Basic Express application',
    main: 'index.js',
    scripts: {
      start: 'node index.js',
      dev: 'nodemon index.js',
      build: 'echo "No build step required"',
      test: 'jest'
    },
    dependencies: {
      express: '^4.18.0',
      cors: '^2.8.5'
    },
    devDependencies: {
      nodemon: '^2.0.20',
      jest: '^29.0.0'
    },
    engines: {
      node: '>=18.0.0'
    }
  },
  'index.js': `const express = require('express');
const cors = require('cors');

const app = express();
const PORT = process.env.PORT || 3000;

app.use(cors());
app.use(express.json());

app.get('/', (req, res) => {
  res.json({ message: 'Hello World!', timestamp: new Date().toISOString() });
});

app.get('/health', (req, res) => {
  res.json({ status: 'healthy', uptime: process.uptime() });
});

app.listen(PORT, () => {
  console.log(\`Server running on port \${PORT}\`);
});

module.exports = app;`,
  '.nvmrc': '18.17.0',
  'README.md': `# Node.js Express Basic

A simple Express.js application for testing containerization.

## Running the application

\`\`\`bash
npm install
npm start
\`\`\`

The server will start on port 3000.`,
  '.gitignore': `node_modules/
npm-debug.log*
yarn-debug.log*
yarn-error.log*
.DS_Store
.env.local
.env.development.local
.env.test.local
.env.production.local`,
};

export const expectedNodeExpressAnalysis = {
  projectType: 'nodejs',
  packageManager: 'npm',
  buildTool: 'npm',
  dependencies: ['express', 'cors'],
  devDependencies: ['nodemon', 'jest'],
  scripts: {
    start: 'node index.js',
    dev: 'nodemon index.js',
    build: 'echo "No build step required"',
    test: 'jest'
  },
  ports: [3000],
  nodeVersion: '18.17.0',
  hasDockerfile: false,
  hasTests: true,
  testFramework: 'jest',
};

export const expectedNodeExpressDockerfile = `FROM node:18-alpine

WORKDIR /app

COPY package*.json ./
RUN npm ci --only=production && npm cache clean --force

COPY . .

EXPOSE 3000

USER node

CMD ["npm", "start"]`;

export {};