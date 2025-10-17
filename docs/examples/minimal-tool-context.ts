/**
 * Minimal ToolContext Example
 * 
 * This shows the absolute minimum required to implement ToolContext
 * for use with Container Assist tools.
 */

import type { ToolContext } from 'containerization-assist-mcp';

/**
 * Create a minimal ToolContext implementation
 * Copy this function to get started quickly
 */
function createMinimalToolContext(): ToolContext {
    // Simple console logger that meets Pino interface requirements
    const logger: any = {
        debug: (msg: any, ...args: any[]) => console.debug('🔍', msg, ...args),
        info: (msg: any, ...args: any[]) => console.log('ℹ️', msg, ...args),
        warn: (msg: any, ...args: any[]) => console.warn('⚠️', msg, ...args),
        error: (msg: any, ...args: any[]) => console.error('❌', msg, ...args),
        fatal: (msg: any, ...args: any[]) => console.error('💀', msg, ...args),
        trace: (msg: any, ...args: any[]) => console.trace('🔎', msg, ...args),
        silent: () => { }, // No-op for silent logging
        level: 'info',
        child: (bindings?: any) => logger, // Return self for simplicity
    };

    return {
        // Required: Logger for debugging and error tracking
        logger,

        // Required: AI sampling capabilities (can be mock for simple cases)
        sampling: {
            createMessage: async (request) => {
                // Mock implementation - replace with actual MCP client
                return {
                    role: 'assistant' as const,
                    content: [{ type: 'text' as const, text: 'Mock response' }],
                };
            },
        },

        // Required: Prompt registry access (can be mock for simple cases)
        getPrompt: async (name: string, args?: Record<string, unknown>) => {
            // Mock implementation - replace with actual prompt registry
            return {
                description: `Prompt: ${name}`,
                messages: [
                    {
                        role: 'user' as const,
                        content: [{ type: 'text' as const, text: `Prompt for ${name}` }],
                    },
                ],
            };
        },

        // Optional: Cancellation signal
        signal: undefined,

        // Optional: Progress reporting
        progress: async (message: string, current?: number, total?: number) => {
            const progressStr = current !== undefined && total !== undefined
                ? ` (${current}/${total})`
                : '';
            console.log(`⏳ ${message}${progressStr}`);
        },
    };
}

export { createMinimalToolContext };