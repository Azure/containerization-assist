/**
 * Type definitions for AI template structures
 */

export interface TemplateVariable {
  name: string;
  description: string;
  required: boolean;
  default?: string | number | boolean | unknown[] | Record<string, unknown>;
}

export interface TemplateExample {
  input: Record<string, unknown>;
  output: string;
}

export interface AITemplate {
  id: string;
  name: string;
  description: string;
  version: string;
  system: string;
  user: string;
  outputFormat: string;
  max_tokens?: number;
  temperature?: number;
  variables?: TemplateVariable[];
  examples?: TemplateExample[];
  tags?: string[];
}
