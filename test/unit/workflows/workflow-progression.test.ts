/**
 * Tests for centralized workflow progression system
 */

import { 
  getSuccessProgression, 
  getFailureProgression,
  createWorkflowChainHint
} from '../../../src/workflows/workflow-progression';

describe('Workflow Progression', () => {
  describe('getSuccessProgression', () => {
    it('should suggest next tool in containerization sequence', () => {
      const progression = getSuccessProgression('build-image', { completed_steps: [] });
      
      expect(progression.nextSteps).toHaveLength(1);
      expect(progression.nextSteps[0]?.tool).toBe('scan-image');
      expect(progression.summary).toContain('build-image completed successfully');
    });

    it('should skip completed steps', () => {
      const progression = getSuccessProgression('build-image', { 
        completed_steps: ['scan-image'] 
      });
      
      expect(progression.nextSteps[0]?.tool).toBe('tag-image');
    });

    it('should indicate workflow completion', () => {
      const progression = getSuccessProgression('push-image', { 
        completed_steps: ['analyze-repo', 'generate-dockerfile', 'build-image', 'scan-image', 'tag-image'] 
      });
      
      expect(progression.nextSteps).toHaveLength(0);
      expect(progression.summary).toContain('Workflow finished');
    });
  });

  describe('getFailureProgression', () => {
    it('should suggest fix-dockerfile for build-image failures', () => {
      const progression = getFailureProgression('build-image', 'Build failed', { 
        completed_steps: [] 
      });
      
      expect(progression.nextSteps[0]?.tool).toBe('fix-dockerfile');
      expect(progression.summary).toContain('build-image failed');
    });

    it('should suggest generate-dockerfile when fix already tried', () => {
      const progression = getFailureProgression('build-image', 'Build failed', { 
        completed_steps: ['fix-dockerfile'] 
      });
      
      expect(progression.nextSteps[0]?.tool).toBe('generate-dockerfile');
    });

    it('should suggest analyze-repo for fix-dockerfile failures', () => {
      const progression = getFailureProgression('fix-dockerfile', 'Fix failed', { 
        completed_steps: [] 
      });
      
      expect(progression.nextSteps[0]?.tool).toBe('analyze-repo');
    });

    it('should fallback for unknown tools', () => {
      const progression = getFailureProgression('unknown-tool', 'Error', { 
        completed_steps: [] 
      });
      
      expect(progression.nextSteps[0]?.tool).toBe('analyze-repo');
      expect(progression.summary).toContain('Fallback to analysis');
    });
  });

  describe('createWorkflowChainHint', () => {
    it('should format progression with next steps', () => {
      const progression = {
        nextSteps: [{ tool: 'scan', description: 'Scan image', priority: 10 }],
        summary: 'build-image completed successfully. Continue with scan'
      };
      
      const hint = createWorkflowChainHint(progression);
      expect(hint).toBe('build-image completed successfully. Continue with scan. Next: scan');
    });

    it('should handle empty next steps', () => {
      const progression = {
        nextSteps: [],
        summary: 'Workflow completed'
      };
      
      const hint = createWorkflowChainHint(progression);
      expect(hint).toBe('Workflow completed');
    });
  });
});