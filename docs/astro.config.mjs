import {defineConfig} from 'astro/config';
import starlight from '@astrojs/starlight';
import {unified} from '@astrojs/markdown-remark';

const onGitHubActions = process.env.GITHUB_ACTIONS === 'true';
const site = process.env.SIGRYX_DOCS_SITE ?? (onGitHubActions ? 'https://rajabinekoo.github.io' : 'http://localhost:4321');
const base = process.env.SIGRYX_DOCS_BASE ?? (onGitHubActions ? '/sigryx' : '/');

function prefixBaseForMarkdownLinks() {
  const prefix = base === '/' ? '' : `/${base.replace(/^\/+|\/+$/g, '')}`;

  function walk(node) {
    if (!node || typeof node !== 'object') return;

    if (
      node.type === 'link' &&
      typeof node.url === 'string' &&
      node.url.startsWith('/') &&
      !node.url.startsWith('//') &&
      prefix
    ) {
      node.url = `${prefix}${node.url}`;
    }

    if (Array.isArray(node.children)) {
      for (const child of node.children) walk(child);
    }
  }

  return () => (tree) => walk(tree);
}

export default defineConfig({
  site,
  base,
  markdown: {
    processor: unified({
      remarkPlugins: [prefixBaseForMarkdownLinks()],
    }),
  },
  integrations: [
    starlight({
      title: 'Sigryx',
      description: 'Self-hosted key and signing infrastructure for application backends.',
      logo: {
        src: './src/assets/sigryx-logo.png',
        alt: 'Sigryx',
        replacesTitle: true,
      },
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/rajabinekoo/sigryx',
        },
      ],
      customCss: ['./src/styles/custom.css'],
      sidebar: [
        {
          label: 'Overview', items: [
            {label: 'Introduction', link: '/'},
            {label: '5-minute usage', link: '/usage/'},
            {label: 'Live REST API client', link: '/getting-started/api-client/'},
          ]
        },
        {
          label: 'Getting started', items: [
            {label: 'Docker install & run', link: '/getting-started/installation/'},
            {label: 'First boot', link: '/getting-started/first-run/'},
            {label: 'Authentication bootstrap', link: '/getting-started/authentication/'},
          ]
        },
        {
          label: 'Concepts', items: [
            {label: 'Architecture', link: '/concepts/architecture/'},
            {label: 'Seal & unseal', link: '/concepts/seal-unseal/'},
            {label: 'Key roots & wallets', link: '/concepts/key-roots-wallets/'},
            {label: 'Signing', link: '/concepts/signing/'},
            {label: 'Integrity signing', link: '/concepts/integrity-signing/'},
          ]
        },
        {
          label: 'Security', items: [
            {label: 'Security model', link: '/security/security-model/'},
            {label: 'Secure memory', link: '/security/secure-memory/'},
            {label: 'Auth, RBAC & network policy', link: '/security/auth-rbac-network/'},
          ]
        },
        {
          label: 'Operations', items: [
            {label: 'Configuration', link: '/operations/configuration/'},
            {label: 'Audit & retention', link: '/operations/audit-retention/'},
            {label: 'Recovery', link: '/operations/recovery/'},
            {label: 'Production deployment', link: '/operations/production/'},
            {label: 'Troubleshooting', link: '/operations/troubleshooting/'},
          ]
        },
        {
          label: 'Reference', items: [
            {label: 'HTTP API', link: '/reference/http-api/'},
            {label: 'Permissions', link: '/reference/permissions/'},
            {label: 'Error behavior', link: '/reference/errors/'},
          ]
        },
        {
          label: 'Contributing', items: [
            {label: 'Development guide', link: '/contributing/development/'},
            {label: 'Documentation guide', link: '/contributing/documentation/'},
          ]
        },
      ],
    }),
  ],
});
