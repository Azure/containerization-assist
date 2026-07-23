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
 *   basic   (default)  → LOG_LEVEL=warn  — CA warnings/errors only
 *   verbose (--verbose) → LOG_LEVEL=info  — full CA logs
 *
 * An env `LOG_LEVEL` sets the initial mode; the `--verbose` flag forces `info`.
 */

// Runtime verbosity flag. Starts true when LOG_LEVEL is already at an info-or-more
// level (info/debug/trace), and is flipped on by `enableVerboseLogging()` (the
// `--verbose` flag), which also raises the log level.
let verbose = ['info', 'debug', 'trace'].includes((process.env.LOG_LEVEL ?? '').toLowerCase());

/** True when verbose output is active (the `--verbose` flag). */
export function isVerbose(): boolean {
  return verbose;
}

// Default to quiet BEFORE any pino logger is constructed. Respect an explicit
// LOG_LEVEL if the operator set one (e.g. LOG_LEVEL=info for full CA logs).
if (!process.env.LOG_LEVEL) {
  process.env.LOG_LEVEL = 'warn';
}

/**
 * Raise the CA log level to `info` for the loggers created *after* this call
 * (the harness and per-tool-call loggers — i.e. the bulk of the volume) and
 * turn on the eval's own verbose failure formatting. The handful of
 * module-level loggers are fixed at import time, so full verbosity across
 * those also needs `LOG_LEVEL=info` set in the environment.
 */
export function enableVerboseLogging(): void {
  verbose = true;
  process.env.LOG_LEVEL = 'info';
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

/** Char caps for captured command output / error tails (keep the tail — the end is decisive). */
export const MAX_FAILURE_DETAIL_CHARS = 8000;
export const MAX_BUILD_OUTPUT_CHARS = 6000;
export const MAX_STEP_OUTPUT_CHARS = 4000;
export const MAX_PATH_LOG_CHARS = 3000;
export const MAX_INLINE_DETAIL_CHARS = 1000;
