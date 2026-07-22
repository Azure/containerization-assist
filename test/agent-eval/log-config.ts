/**
 * Eval log configuration — MUST be imported before anything that pulls in the
 * CA server / knowledge / validation modules, because several of those create
 * their pino logger at module-load time and read `LOG_LEVEL` right then.
 *
 * The CA tooling logs at `info` for every tool call, knowledge query, and timer.
 * Across an 89-cell sweep that buries the eval's own progress lines under
 * thousands of JSON blobs. So the eval defaults the CA log level to `warn`
 * ("basic") and only opens it back up to `info` ("verbose") on request:
 *
 *   basic   (default)         → LOG_LEVEL=warn  — CA warnings/errors only
 *   verbose (AGENT_EVAL_VERBOSE / --verbose) → LOG_LEVEL=info — full CA logs
 *
 * An explicit `LOG_LEVEL` in the environment always wins over both.
 */

/** True when the caller asked for full CA logs via env (import-time signal). */
export const AGENT_EVAL_VERBOSE = /^(1|true|yes|on)$/i.test(process.env.AGENT_EVAL_VERBOSE ?? '');

// Runtime verbosity flag. Seeded from the env signal, but also flipped on by
// `enableVerboseLogging()` so the `--verbose` CLI flag turns on the verbose
// failure formatting too (not just the CA log level).
let verbose = AGENT_EVAL_VERBOSE;

/** True when verbose output is active (env `AGENT_EVAL_VERBOSE` OR the `--verbose` flag). */
export function isVerbose(): boolean {
  return verbose;
}

// Set the default BEFORE any pino logger is constructed. Respect an explicit
// LOG_LEVEL if the operator set one.
if (!process.env.LOG_LEVEL) {
  process.env.LOG_LEVEL = AGENT_EVAL_VERBOSE ? 'info' : 'warn';
}

/**
 * Raise the CA log level to `info` for the loggers created *after* this call
 * (the harness and per-tool-call loggers — i.e. the bulk of the volume) and
 * turn on the eval's own verbose failure formatting. The handful of
 * module-level loggers are fixed at import time, so full verbosity across
 * those also needs `AGENT_EVAL_VERBOSE=1` in the environment.
 */
export function enableVerboseLogging(): void {
  verbose = true;
  process.env.LOG_LEVEL = 'info';
}

/** Last `n` non-empty, trimmed lines of `text` — the part of an error worth reading. */
export function lastLines(text: string | undefined, n: number): string {
  if (!text) return '';
  const lines = text
    .split('\n')
    .map((l) => l.replace(/\s+$/, ''))
    .filter((l) => l.trim().length > 0);
  return lines.slice(-n).join('\n');
}

/** Collapse `text` to a single trimmed line capped at `max` chars for a one-line log. */
export function oneLine(text: string | undefined, max = 200): string {
  if (!text) return '';
  const flat = text.replace(/\s+/g, ' ').trim();
  return flat.length > max ? `${flat.slice(0, max - 1)}…` : flat;
}

/**
 * The single most useful line of a build/tool error for the "basic" view:
 * the last line that looks like the decisive failure (`ERROR`, `failed to
 * solve`, `did not complete`, …), falling back to the last non-empty line.
 */
export function decisiveLine(text: string | undefined, max = 240): string {
  if (!text) return '';
  const lines = text
    .split('\n')
    .map((l) => l.trim())
    .filter((l) => l.length > 0);
  if (lines.length === 0) return '';
  const marker =
    /(^|\s)(error|failed to solve|did not complete|not found|access denied|cannot|unable)\b/i;
  const hit = [...lines].reverse().find((l) => marker.test(l));
  return oneLine(hit ?? lines[lines.length - 1], max);
}
