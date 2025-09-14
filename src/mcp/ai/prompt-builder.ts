/**
 * AI Prompt Builder - Utility for constructing well-formatted prompts
 */

import type { AIParamRequest } from './host-ai-assist';

/**
 * Builder class for constructing AI prompts with consistent formatting
 */
export class AIPromptBuilder {
  private sections: string[] = [];

  /**
   * Add a titled section to the prompt
   */
  addSection(title: string, content: unknown): this {
    if (content !== undefined && content !== null) {
      const formatted = typeof content === 'string' ? content : JSON.stringify(content, null, 2);
      this.sections.push(`${title}: ${formatted}`);
    }
    return this;
  }

  /**
   * Add a plain instruction line to the prompt
   */
  addInstruction(instruction: string): this {
    this.sections.push(instruction);
    return this;
  }

  /**
   * Add a blank line separator
   */
  addSeparator(): this {
    this.sections.push('');
    return this;
  }

  /**
   * Build the final prompt string
   */
  build(): string {
    return this.sections.join('\n');
  }

  /**
   * Create a prompt for parameter suggestion
   */
  static forParameterSuggestion(request: AIParamRequest): string {
    return new AIPromptBuilder()
      .addSection('Tool', request.toolName)
      .addSection('Current', request.currentParams)
      .addSection('Missing', request.missingParams.join(', '))
      .addSection('Schema', request.schema)
      .addSection('Context', request.sessionContext)
      .addSeparator()
      .addInstruction('Return JSON object with suggested parameter values.')
      .addInstruction('Example: {"path": ".", "imageId": "app:latest"}')
      .build();
  }

  /**
   * Create a prompt for context analysis
   */
  static forContextAnalysis(context: Record<string, unknown>, objective: string): string {
    return new AIPromptBuilder()
      .addSection('Objective', objective)
      .addSection('Context', context)
      .addSeparator()
      .addInstruction('Analyze the context and provide insights.')
      .build();
  }
}
