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
      { text: 'Guides', link: '/guides/' },
      { text: 'Examples', link: '/examples/' },
      { text: 'ADRs', link: '/adr/' },
      { text: 'GitHub', link: 'https://github.com/Azure/containerization-assist' }
    ],
    sidebar: {
      '/guides/': [
        {
          text: 'Guides',
          items: [
            { text: 'Index', link: '/guides/' },
            { text: 'Policy Getting Started', link: '/guides/policy-getting-started' },
            { text: 'Policy Authoring', link: '/guides/policy-authoring' },
            { text: 'Writing Rego Policies', link: '/guides/writing-rego-policies' },
            { text: 'VS Code Integration', link: '/guides/vscode-extension-integration' },
            { text: 'Policy Example README', link: '/guides/policy-example/README' },
            { text: 'Platform and Tag Policies', link: '/guides/policy-example/PLATFORM_AND_TAG_POLICY_USAGE' }
          ]
        }
      ],
      '/examples/': [
        {
          text: 'Examples',
          items: [
            { text: 'Index', link: '/examples/' },
            { text: 'README', link: '/examples/README' },
            { text: 'Template Injection', link: '/examples/template-injection-example' },
            { text: 'Dynamic Defaults', link: '/examples/dynamic-defaults-example' }
          ]
        }
      ],
      '/adr/': [
        {
          text: 'Architecture Decisions',
          items: [
            { text: 'Index', link: '/adr/000-index' },
            { text: '001: Result<T> Pattern', link: '/adr/001-result-pattern' },
            { text: '002: Unified Tool Interface', link: '/adr/002-tool-interface' },
            { text: '003: Knowledge Enhancement', link: '/adr/003-knowledge-enhancement' },
            { text: '004: Policy System', link: '/adr/004-policy-system' },
            { text: '005: MCP Integration', link: '/adr/005-mcp-integration' },
            { text: '006: Infrastructure Organization', link: '/adr/006-infrastructure-organization' },
            { text: '007: SDK Decoupling', link: '/adr/007-sdk-decoupling' }
          ]
        }
      ]
    },
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
