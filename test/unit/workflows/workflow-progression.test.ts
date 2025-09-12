/**
 * Tests for centralized workflow progression system
 */

import {
  getSuccessProgression,
  getFailureProgression,
  createWorkflowChainHint,
} from '../../../src/workflows/workflow-progression';

describe('Workflow Progression', () => {
  describe('getSuccessProgression', () => {
    it('should suggest next tool in containerization sequence', () => {
      const progression = getSuccessProgression('build_image', { completed_steps: [] });

      expect(progression.nextSteps).toHaveLength(1);
      expect(progression.nextSteps[0]?.tool).toBe('scan_image');
      expect(progression.summary).toContain('build_image tool execution completed successfully');
    });

    it('should skip completed steps', () => {
      const progression = getSuccessProgression('build_image', {
        completed_steps: ['scan_image'],
      });

      expect(progression.nextSteps[0]?.tool).toBe('tag_image');
    });

    it('should indicate workflow completion', () => {
      const progression = getSuccessProgression('deploy_application', {
        completed_steps: [
          'analyze_repo',
          'generate_dockerfile',
          'build_image',
          'scan_image',
          'tag_image',
          'push_image',
          'generate_k8s_manifests',
          'prepare_cluster',
        ],
      });

      expect(progression.nextSteps).toHaveLength(0);
      expect(progression.summary).toContain('Workflow finished');
    });
  });

  describe('getFailureProgression', () => {
    it('should suggest fix_dockerfile for build_image failures', () => {
      const progression = getFailureProgression('build_image', 'Build failed', {
        completed_steps: [],
      });

      expect(progression.nextSteps[0]?.tool).toBe('fix_dockerfile');
      expect(progression.summary).toContain('build_image tool failed');
    });

    it('should suggest generate_dockerfile when fix already tried', () => {
      const progression = getFailureProgression('build_image', 'Build failed', {
        completed_steps: ['fix_dockerfile'],
      });

      expect(progression.nextSteps[0]?.tool).toBe('generate_dockerfile');
    });

    it('should suggest analyze_repo for fix_dockerfile failures', () => {
      const progression = getFailureProgression('fix_dockerfile', 'Fix failed', {
        completed_steps: [],
      });

      expect(progression.nextSteps[0]?.tool).toBe('analyze_repo');
    });

    it('should fallback for unknown tools', () => {
      const progression = getFailureProgression('unknown-tool', 'Error', {
        completed_steps: [],
      });

      expect(progression.nextSteps[0]?.tool).toBe('analyze_repo');
      expect(progression.summary).toContain('Fallback to analysis');
    });
  });

  describe('createWorkflowChainHint', () => {
    it('should format progression with next steps', () => {
      const progression = {
        nextSteps: [{ tool: 'scan', description: 'Scan image', priority: 10 }],
        summary: 'build_image completed successfully. Continue with scan',
      };

      const hint = createWorkflowChainHint(progression);
      expect(hint).toBe(
        'build_image completed successfully. Continue with scan. Next Step: Call scan tool',
      );
    });

    it('should handle empty next steps', () => {
      const progression = {
        nextSteps: [],
        summary: 'Workflow completed',
      };

      const hint = createWorkflowChainHint(progression);
      expect(hint).toBe('Workflow completed');
    });
  });
});
