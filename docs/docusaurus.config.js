// @ts-check

const {themes: prismThemes} = require('prism-react-renderer');

function stripBadProgressPlugin() {
  return {
    name: 'strip-bad-progress-plugin',
    configureWebpack(config) {
      const plugins = (config.plugins || []).filter((p) => {
        const opts = p && p.options;
        if (!opts || typeof opts !== 'object') return true;
        return !('name' in opts || 'color' in opts || 'reporters' in opts || 'reporter' in opts);
      });
      return {
        plugins,
        mergeStrategy: { plugins: 'replace' },
      };
    },
  };
}

module.exports = async function createConfig() {
  const {default: remarkAdmonitions} = await import(
    'remark-github-admonitions-to-directives'
  );

  /** @type {import('@docusaurus/types').Config} */
  const config = {
  title: 'Overlock',
  titleDelimiter: '//',
  tagline: 'Manage Crossplane environments with ease',
  favicon: 'overlock_white_alpha.png',

  url: 'https://overlock.app',
  baseUrl: '/',

  organizationName: 'web-seven',
  projectName: 'overlock',

  onBrokenLinks: 'warn',
  onBrokenMarkdownLinks: 'warn',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  staticDirectories: ['static'],

  stylesheets: [
    {
      href: 'https://fonts.googleapis.com/css2?family=Saira:wdth,wght@75..125,400;75..125,500;75..125,600;75..125,700&family=Sora:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500;600;700&display=swap',
      rel: 'stylesheet',
      crossorigin: 'anonymous',
    },
  ],

  plugins: [stripBadProgressPlugin],

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          // Serve .md files from their current locations in ./docs
          path: '.',
          routeBasePath: 'docs',
          sidebarPath: './sidebars.js',
          // Exclude generated/build artifacts and non-doc directories
          exclude: [
            '**/node_modules/**',
            '**/build/**',
            '**/.docusaurus/**',
            '**/src/**',
            '**/static/**',
          ],
          editUrl: 'https://github.com/web-seven/overlock/edit/main/docs/',
          beforeDefaultRemarkPlugins: [remarkAdmonitions],
        },
        blog: {
          path: '../blog',
          routeBasePath: 'blog',
          blogTitle: 'Blog',
          blogDescription: 'Updates and articles from the Overlock team',
          showReadingTime: true,
          postsPerPage: 10,
          editUrl: 'https://github.com/web-seven/overlock/edit/main/',
        },
        theme: {
          customCss: './src/css/custom.css',
        },
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      image: 'overlock_galaxy.png',
      colorMode: {
        defaultMode: 'dark',
        respectPrefersColorScheme: false,
        disableSwitch: true,
      },
      navbar: {
        title: 'Overlock',
        hideOnScroll: false,
        logo: {
          alt: 'Overlock',
          src: 'overlock_white_alpha.png',
        },
        items: [
          {to: '/#dash', label: 'Features', position: 'right', activeBaseRegex: '^/__never__$'},
          {to: '/#env', label: 'Environments', position: 'right', activeBaseRegex: '^/__never__$'},
          {to: '/#nodes', label: 'Nodes', position: 'right', activeBaseRegex: '^/__never__$'},
          {to: '/#packages', label: 'Packages', position: 'right', activeBaseRegex: '^/__never__$'},
          {
            type: 'docSidebar',
            sidebarId: 'docs',
            position: 'right',
            label: 'Docs',
          },
          {to: '/#compare', label: 'Compare', position: 'right', activeBaseRegex: '^/__never__$'},
          {to: '/blog', label: 'Blog', position: 'right'},
          {
            href: 'https://github.com/web-seven/overlock',
            label: 'github',
            position: 'right',
            className: 'navbar-cta navbar-cta--ghost',
          },
          {
            to: '/#install',
            label: 'install ↓',
            position: 'right',
            className: 'navbar-cta navbar-cta--solid',
            activeBaseRegex: '^/__never__$',
          },
        ],
      },
      footer: {
        style: 'dark',
        links: [
          {
            title: 'Documentation',
            items: [
              {
                label: 'Overview',
                to: '/docs/overview',
              },
              {
                label: 'Getting Started',
                to: '/docs/guide/getting-started',
              },
              {
                label: 'Commands',
                to: '/docs/commands',
              },
            ],
          },
          {
            title: 'Guides',
            items: [
              {
                label: 'Environments',
                to: '/docs/guide/environments',
              },
              {
                label: 'Registries',
                to: '/docs/guide/registries',
              },
              {
                label: 'Plugins',
                to: '/docs/guide/plugins',
              },
            ],
          },
          {
            title: 'Community',
            items: [
              {
                label: 'GitHub',
                href: 'https://github.com/web-seven/overlock',
              },
              {
                label: 'Issues',
                href: 'https://github.com/web-seven/overlock/issues',
              },
            ],
          },
        ],
        copyright: `Copyright © ${new Date().getFullYear()} Web Seven. Built with Docusaurus.`,
      },
      prism: {
        theme: prismThemes.github,
        darkTheme: prismThemes.dracula,
        additionalLanguages: ['bash', 'yaml', 'go'],
      },
    }),
  };

  return config;
};
