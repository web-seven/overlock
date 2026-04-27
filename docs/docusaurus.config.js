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
  tagline: 'Manage Crossplane environments with ease',
  favicon: 'overlock_galaxy_icon.png',

  url: 'https://docs.overlock.app',
  baseUrl: '/',

  organizationName: 'web-seven',
  projectName: 'overlock',

  onBrokenLinks: 'warn',
  onBrokenMarkdownLinks: 'warn',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  // Serve existing PNG assets from the docs root alongside the standard static/ dir
  staticDirectories: ['.', 'static'],

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
      navbar: {
        title: 'Overlock',
        logo: {
          alt: 'Overlock Logo',
          src: 'overlock_galaxy_icon.png',
        },
        items: [
          {
            type: 'docSidebar',
            sidebarId: 'docs',
            position: 'left',
            label: 'Documentation',
          },
          {to: '/blog', label: 'Blog', position: 'left'},
          {
            href: 'https://github.com/web-seven/overlock',
            label: 'GitHub',
            position: 'right',
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
