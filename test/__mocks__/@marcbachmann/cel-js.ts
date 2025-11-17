/**
 * Mock for @marcbachmann/cel-js
 *
 * Provides simplified CEL functionality for testing without ESM import issues.
 * Enhanced to support complex boolean expressions, AND/OR operators, and multi-line conditions.
 */

export interface ParsedExpression {
  (context: any): any;
}

/**
 * Evaluate a simple CEL expression
 * Supports contains(), boolean operators, and basic string matching
 */
function evaluateSimpleExpression(expr: string, context: any): boolean {
  const trimmedExpr = expr.trim();

  // Handle literal booleans
  if (trimmedExpr === 'true') return true;
  if (trimmedExpr === 'false') return false;

  // Handle negated contains: !input.content.contains("text")
  const negatedContainsMatch = trimmedExpr.match(/^!\s*input\.content\.contains\("([^"]+)"\)$/);
  if (negatedContainsMatch) {
    const needle = negatedContainsMatch[1];
    const hasContent = context.input?.content?.includes(needle) ?? false;
    return !hasContent;
  }

  // Handle positive contains: input.content.contains("text")
  const containsMatch = trimmedExpr.match(/^input\.content\.contains\("([^"]+)"\)$/);
  if (containsMatch) {
    const needle = containsMatch[1];
    return context.input?.content?.includes(needle) ?? false;
  }

  // Handle size() calls: input.content.size() > 0
  const sizeMatch = trimmedExpr.match(/^input\.content\.size\(\)\s*([><=!]+)\s*(\d+)$/);
  if (sizeMatch) {
    const operator = sizeMatch[1];
    const value = parseInt(sizeMatch[2], 10);
    const size = context.input?.content?.length ?? 0;

    switch (operator) {
      case '>':
        return size > value;
      case '<':
        return size < value;
      case '>=':
        return size >= value;
      case '<=':
        return size <= value;
      case '==':
        return size === value;
      case '!=':
        return size !== value;
      default:
        return false;
    }
  }

  // Handle kind checks: input.content.contains("kind: Deployment")
  // Already handled by contains above

  // Default to false for unknown simple expressions
  return false;
}

/**
 * Mock parse function
 * Parses CEL expressions and returns a function for evaluation
 */
export function parse(expression: string): ParsedExpression {
  // Check for obviously invalid syntax during parse (compilation)
  if (expression.includes('@#$') || expression.includes('invalid syntax here')) {
    throw new Error(`Parse error: Invalid CEL syntax in expression: ${expression.substring(0, 50)}`);
  }

  // Check for invalid CEL patterns
  if (expression.includes('this is not valid CEL')) {
    throw new Error('Parse error: Unexpected token');
  }

  // Check for unclosed strings (incomplete condition)
  const singleQuotes = (expression.match(/'/g) || []).length;
  const doubleQuotes = (expression.match(/"/g) || []).length;
  if (singleQuotes % 2 !== 0 || doubleQuotes % 2 !== 0) {
    // Note: Real CEL library would give a more specific error
    // For now, we silently accept (some tests expect this to pass compilation)
    // The error will be caught at evaluation time
  }

  // Return evaluation function
  return (context: any) => {
    try {
      // Normalize expression - remove extra whitespace and newlines
      const normalized = expression.replace(/\s+/g, ' ').trim();

      // Handle input.nonexistent.method() for error tests
      if (normalized.includes('input.nonexistent')) {
        throw new Error('Cannot read property of undefined');
      }

      // Handle OR expressions: expr1 || expr2
      if (normalized.includes('||')) {
        const parts = normalized.split('||').map((p) => p.trim());
        return parts.some((part) => evaluateSimpleExpression(part, context));
      }

      // Handle AND expressions: expr1 && expr2
      if (normalized.includes('&&')) {
        const parts = normalized.split('&&').map((p) => p.trim());
        return parts.every((part) => evaluateSimpleExpression(part, context));
      }

      // Handle simple expression
      return evaluateSimpleExpression(normalized, context);
    } catch (error) {
      throw error;
    }
  };
}

/**
 * Mock evaluate function
 * Direct evaluation without pre-parsing
 */
export function evaluate(expression: string, context: any = {}): any {
  const program = parse(expression);
  return program(context);
}
