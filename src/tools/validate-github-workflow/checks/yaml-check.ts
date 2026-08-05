/**
 * Layer 1 — YAML validity.
 *
 * Parses the workflow once with the already-bundled `yaml` package, surfacing:
 *   - fatal parse errors  → `required`
 *   - duplicate mapping keys → `high`
 *   - tab indentation      → `high`
 */

import {
  parseDocument,
  isMap,
  isSeq,
  isScalar,
  LineCounter,
  type Document,
  type YAMLError,
} from 'yaml';
import { makeIssue, lineOfOffset } from './helpers';
import type { WorkflowValidationIssue } from '../schema';

export interface ParsedWorkflow {
  /** The parsed document, or null when parsing threw. */
  doc: Document.Parsed | null;
  /** Parse-level (Layer 1) findings. */
  findings: WorkflowValidationIssue[];
  /** True when parse errors make schema/semantic walking impossible. */
  fatal: boolean;
  /**
   * Offset -> line/col index, populated by the parser as a side effect of parsing.
   *
   * `linePos()` binary-searches precomputed line starts, so position lookups are O(log n)
   * instead of rescanning the source from offset 0 on every call. Layers 2 and 4 recover a
   * line for most of their findings, so a linear scan per finding would make the whole
   * validation O(content x findings) for no reason.
   */
  lineCounter: LineCounter;
}

/**
 * What the parse error is about. The line is carried separately on the finding's `line`
 * field, so this contributes only the column — repeating the line here would render as
 * "line 6, line 6" once a consumer joins the two.
 *
 * `linePos` reports 1-based line *and* column (matching yaml's own "at line L, column C"
 * message), so the column is emitted as-is. The exception is its degenerate `{ line: 0,
 * col: <raw offset> }` branch: there `col` is a byte offset, not a column, so it is
 * suppressed rather than rendered as one.
 *
 * Both components are range-checked, since emitting `column 0` would be worse than
 * emitting no column at all. The checks are written as positive comparisons (`>= 1`)
 * rather than `< 1` so a `NaN` fails them instead of slipping through.
 */
function locationOf(err: YAMLError): string | undefined {
  const pos = err.linePos?.[0];
  if (!pos || !(pos.line >= 1) || !(pos.col >= 1)) return undefined;
  return `column ${pos.col}`;
}

/** 1-based line of a parse error, when the parser reported a usable position. */
function lineOf(err: YAMLError): number | undefined {
  const pos = err.linePos?.[0];
  return pos && pos.line >= 1 ? pos.line : undefined;
}

/**
 * Parse the workflow. `uniqueKeys: false` keeps duplicate keys from becoming fatal
 * parse errors so we can report them as `high` findings instead (via checkYaml).
 */
export function parseWorkflow(content: string): ParsedWorkflow {
  const findings: WorkflowValidationIssue[] = [];
  const lineCounter = new LineCounter();
  let doc: Document.Parsed;
  try {
    doc = parseDocument(content, { prettyErrors: true, uniqueKeys: false, lineCounter });
  } catch (err) {
    findings.push(
      makeIssue({
        layer: 'yaml',
        ruleId: 'yaml/parse',
        severity: 'required',
        message: `YAML could not be parsed: ${err instanceof Error ? err.message : String(err)}`,
      }),
    );
    return { doc: null, findings, fatal: true, lineCounter };
  }

  for (const err of doc.errors) {
    findings.push(
      makeIssue({
        layer: 'yaml',
        ruleId: 'yaml/parse',
        severity: 'required',
        message: err.message,
        location: locationOf(err),
        line: lineOf(err),
      }),
    );
  }

  return { doc, findings, fatal: doc.errors.length > 0, lineCounter };
}

/** Layer-1 findings that require the parsed document + the raw text. */
export function checkYaml(
  content: string,
  doc: Document.Parsed | null,
  lineCounter: LineCounter,
): WorkflowValidationIssue[] {
  const findings: WorkflowValidationIssue[] = [];

  // Tab indentation — YAML forbids tabs for indentation.
  content.split('\n').forEach((line, i) => {
    const indent = line.match(/^[ \t]*/)?.[0] ?? '';
    if (indent.includes('\t')) {
      findings.push(
        makeIssue({
          layer: 'yaml',
          ruleId: 'yaml/tab-indent',
          severity: 'high',
          message: 'Tab character used for indentation; YAML requires spaces.',
          location: 'indentation',
          line: i + 1,
        }),
      );
    }
  });

  if (doc?.contents) {
    collectDuplicateKeys(doc.contents as unknown, lineCounter, findings);
  }

  return findings;
}

/** Recursively flag duplicate keys within every mapping in the document. */
function collectDuplicateKeys(
  node: unknown,
  lineCounter: LineCounter,
  findings: WorkflowValidationIssue[],
): void {
  if (isMap(node)) {
    const seen = new Set<string>();
    for (const item of node.items) {
      if (isScalar(item.key)) {
        const key = String(item.key.value);
        if (seen.has(key)) {
          const offset = Array.isArray(item.key.range) ? item.key.range[0] : undefined;
          const line = offset !== undefined ? lineOfOffset(lineCounter, offset) : undefined;
          findings.push(
            makeIssue({
              layer: 'yaml',
              ruleId: 'yaml/duplicate-key',
              severity: 'high',
              message: `Duplicate mapping key "${key}".`,
              location: `key "${key}"`,
              line,
            }),
          );
        } else {
          seen.add(key);
        }
      }
      collectDuplicateKeys(item.value, lineCounter, findings);
    }
  } else if (isSeq(node)) {
    for (const item of node.items) {
      collectDuplicateKeys(item, lineCounter, findings);
    }
  }
}
