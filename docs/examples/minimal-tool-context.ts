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
        silent: () => { },
        level: 'info',
        child: (bindings?: any) => logger,
    };

    return {
        logger,
        signal: undefined,
        progress: async (message: string, current?: number, total?: number) => {
            const progressStr = current !== undefined && total !== undefined
                ? ` (${current}/${total})`
                : '';
            console.log(`⏳ ${message}${progressStr}`);
        },
    };
}

export { createMinimalToolContext };