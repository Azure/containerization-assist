/**
 * Layer 1 — YAML validity.
 *
 * Parses the workflow once with the already-bundled `yaml` package, surfacing:
 *   - fatal parse errors  → `required`
 *   - duplicate mapping keys → `high`
 *   - tab indentation      → `high`
 */

import { parseDocument, isMap, isSeq, isScalar, type Document, type YAMLError } from 'yaml';
import { makeIssue, lineOfOffset } from './helpers';
import type { WorkflowValidationIssue } from '../schema';

export interface ParsedWorkflow {
  /** The parsed document, or null when parsing threw. */
  doc: Document.Parsed | null;
  /** Parse-level (Layer 1) findings. */
  findings: WorkflowValidationIssue[];
  /** True when parse errors make schema/semantic walking impossible. */
  fatal: boolean;
}

function locationOf(err: YAMLError): string | undefined {
  const lp = err.linePos?.[0];
  return lp ? `line ${lp.line}, col ${lp.col}` : undefined;
}

/**
 * Parse the workflow. `uniqueKeys: false` keeps duplicate keys from becoming fatal
 * parse errors so we can report them as `high` findings instead (via checkYaml).
 */
export function parseWorkflow(content: string): ParsedWorkflow {
  const findings: WorkflowValidationIssue[] = [];
  let doc: Document.Parsed;
  try {
    doc = parseDocument(content, { prettyErrors: true, uniqueKeys: false });
  } catch (err) {
    findings.push(
      makeIssue({
        layer: 'yaml',
        ruleId: 'yaml/parse',
        severity: 'required',
        message: `YAML could not be parsed: ${err instanceof Error ? err.message : String(err)}`,
      }),
    );
    return { doc: null, findings, fatal: true };
  }

  for (const err of doc.errors) {
    findings.push(
      makeIssue({
        layer: 'yaml',
        ruleId: 'yaml/parse',
        severity: 'required',
        message: err.message,
        location: locationOf(err),
      }),
    );
  }

  return { doc, findings, fatal: doc.errors.length > 0 };
}

/** Layer-1 findings that require the parsed document + the raw text. */
export function checkYaml(
  content: string,
  doc: Document.Parsed | null,
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
          location: `line ${i + 1}`,
        }),
      );
    }
  });

  if (doc?.contents) {
    collectDuplicateKeys(doc.contents as unknown, content, findings);
  }

  return findings;
}

/** Recursively flag duplicate keys within every mapping in the document. */
function collectDuplicateKeys(
  node: unknown,
  content: string,
  findings: WorkflowValidationIssue[],
): void {
  if (isMap(node)) {
    const seen = new Set<string>();
    for (const item of node.items) {
      if (isScalar(item.key)) {
        const key = String(item.key.value);
        if (seen.has(key)) {
          const offset = Array.isArray(item.key.range) ? item.key.range[0] : undefined;
          findings.push(
            makeIssue({
              layer: 'yaml',
              ruleId: 'yaml/duplicate-key',
              severity: 'high',
              message: `Duplicate mapping key "${key}".`,
              location: offset !== undefined ? `line ${lineOfOffset(content, offset)}` : undefined,
            }),
          );
        } else {
          seen.add(key);
        }
      }
      collectDuplicateKeys(item.value, content, findings);
    }
  } else if (isSeq(node)) {
    for (const item of node.items) {
      collectDuplicateKeys(item, content, findings);
    }
  }
}
