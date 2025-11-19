import { setupJavaMigrationMcpServer } from './server';

try {
    setupJavaMigrationMcpServer();
} catch (error) {
    process.stderr.write(`Failed to start MCP server: ${error}\n`);
    process.exit(1);
}
