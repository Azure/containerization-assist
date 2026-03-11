import { defineConfig } from 'vitepress';
import type { Plugin } from 'vite';

/**
 * Vite plugin that escapes angle brackets used as TypeScript generics in
 * markdown prose (outside of fenced code blocks) so the Vue SFC compiler
 * does not try to parse them as HTML elements.
 */
function escapeAngleBracketsPlugin(): Plugin {
  return {
    name: 'escape-angle-brackets',
    enforce: 'pre',
    transform(code: string, id: string) {
      if (!id.endsWith('.md')) return;
      const lines = code.split('\n');
      let inCodeBlock = false;
      const result = lines.map((line) => {
        if (line.trimStart().startsWith('```')) {
          inCodeBlock = !inCodeBlock;
          return line;
        }
        if (inCodeBlock) return line;
        // Escape angle brackets that look like TypeScript generics:
        //   Result<T>, Promise<Result<BuildContext>>, Array<TTool>, etc.
        // Match < followed by a word character (not HTML tags like <br>, <div>)
        // and not already inside backtick code spans.
        return line.replace(/(?<!`)(<)(\w+)(>)(?!`)/g, '&lt;$2&gt;');
      });
      return result.join('\n');
    }
  };
}

export default defineConfig({
  title: 'Containerization Assist',
  description: 'AI-powered containerization assistant MCP server',
  base: '/containerization-assist/',
  // Existing docs reference files outside the docs/ directory (e.g. ../../README,
  // ../../CLAUDE, ../sprints/) that are not part of the VitePress site.
  ignoreDeadLinks: true,
  vite: {
    plugins: [escapeAngleBracketsPlugin()]
  },
  themeConfig: {
    nav: [
      { text: 'Docs', link: '/' },
      { text: 'GitHub', link: 'https://github.com/Azure/containerization-assist' }
    ],
    sidebar: [
      {
        text: 'Getting Started',
        items: [
          { text: 'Overview', link: '/' }
        ]
      },
      {
        text: 'Guides',
        items: [
          { text: 'Policy Getting Started', link: '/guides/policy-getting-started' },
          { text: 'Policy Authoring', link: '/guides/policy-authoring' },
          { text: 'Writing Rego Policies', link: '/guides/writing-rego-policies' },
          { text: 'VS Code Integration', link: '/guides/vscode-extension-integration' },
          { text: 'Policy Example', link: '/guides/policy-example/README' },
          { text: 'Platform and Tag Policies', link: '/guides/policy-example/PLATFORM_AND_TAG_POLICY_USAGE' }
        ]
      },
      {
        text: 'Examples',
        items: [
          { text: 'SDK Integration', link: '/examples/README' },
          { text: 'Template Injection', link: '/examples/template-injection-example' },
          { text: 'Dynamic Defaults', link: '/examples/dynamic-defaults-example' }
        ]
      }
    ],
    search: {
      provider: 'local'
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/Azure/containerization-assist' }
    ],
    editLink: {
      pattern: 'https://github.com/Azure/containerization-assist/edit/main/docs/:path',
      text: 'Edit this page on GitHub'
    },
    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2026 Microsoft'
    }
  }
});
