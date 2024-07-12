require('dotenv').config();
module.exports = {
  themeConfig: {
    colorMode: {
      defaultMode: 'dark',
      disableSwitch: false,
      respectPrefersColorScheme: false,
    },
    tableOfContents: {
      minHeadingLevel: 2,
      maxHeadingLevel: 5,
    },

  },
  
};


const lightCodeTheme = require('prism-react-renderer/themes/github');
const darkCodeTheme = require('prism-react-renderer/themes/dracula');


/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'KNDP',
  tagline: '',
  favicon: '/img/logo.png',

  // Set the production url of your site here
  url: 'https://kndp.io/',
  baseUrl: '/',
  organizationName: 'web7', // Usually your GitHub org/user name.
  projectName: 'kndp', // Usually your repo name.
  deploymentBranch: 'gh-pages',
  onBrokenLinks: 'warn',
  onBrokenMarkdownLinks: 'warn',
  customFields: {
    version: process.env.VERSION,
  },
  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },
  stylesheets: [
    "https://fonts.googleapis.com/css2?family=Rubik:wght@300;400;500;600;700&amp;display=swap",
    "https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@400;500;600;700&amp;display=swap",
    "https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&amp;display=swap",
  ],
  scripts: [
    "/js/jquery-3.7.0.min.js",
    "/js/bootstrap.min.js",
    "/js/modernizr.custom.js",
    "/js/jquery.easing.js",
    "/js/jquery.appear.js",
    "/js/menu.js",
    "/js/owl.carousel.min.js",
    "/js/pricing-toggle.js",
    "/js/jquery.magnific-popup.min.js",
    "/js/quick-form.js",
    "/js/jquery.validate.min.js",
    "/js/jquery.ajaxchimp.min.js",
    "/js/popper.min.js",
    "/js/lunar.js",
    "/js/wow.js",
    "/js/custom.js",
  ],

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      {
        docs: {
          sidebarPath: require.resolve('./sidebars.js'),
        },
        pages: {
          mdxPageComponent: '../src/components/MDXPage',
        },
        blog: {
          showReadingTime: true,
        },
        theme: {
          customCss: require.resolve('./src/css/custom.css'),
          customCss: [
            require.resolve('./static/css/bootstrap.min.css'),
            require.resolve('./static/css/flaticon.css'),
            require.resolve('./static/css/menu.css'),
            require.resolve('./static/css/dropdown-effects/fade-down.css'),
            require.resolve('./static/css/magnific-popup.css'),
            require.resolve('./static/css/owl.carousel.min.css'),
            require.resolve('./static/css/owl.theme.default.min.css'),
            require.resolve('./static/css/lunar.css'),
            require.resolve('./static/css/animate.css'),
            require.resolve('./static/css/blue-theme.css'),
            require.resolve('./static/css/responsive.css'),
            require.resolve('./src/css/custom.css'),
          ],
        },
      },
    ],
  ],


  themeConfig: {
    liveCodeBlock: {
      playgroundPosition: 'bottom',
    },
    docs: {
      sidebar: {
        hideable: false,
        autoCollapseCategories: false,
      },
    },
    colorMode: {
      defaultMode: 'light',
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },
    prism: {
      additionalLanguages: ['java', 'latex'],
      magicComments: [
        {
          className: 'theme-code-block-highlighted-line',
          line: 'highlight-next-line',
          block: { start: 'highlight-start', end: 'highlight-end' },
        },
      ],
      theme: lightCodeTheme,
      darkTheme: darkCodeTheme,
      
    },
    algolia: {
      appId: ' ',
      apiKey: ' ',
      indexName: 'docusaurus-2',
    }, 

    navbar: {
      hideOnScroll: true,
      logo: {
        alt: 'KNDP Logo',
        src: '/img/logo.png',
      },
      title: 'KNDP',
      items: [
        // { to: '/why-kndp', label: 'Why KNDP?', position: 'right' },
        {
          type: 'doc',
          position: 'right',
          docId: 'overview',
          label: 'Documentation',
        },         
        // { to: '/community', label: 'Community', position: 'right' },
        // { to: '/blog', label: 'Blog', position: 'right' },
        {
          href: 'https://github.com/kndpio/cli',
          position: 'right',
          className: 'header-github-link',
          'aria-label': 'GitHub repository',
        },
      ].filter(Boolean),
    },
    footer: {
      // logo: {
      //   alt: 'Web Seven Logo',
      //   src: 'img/logo_text.png',
      //   href: 'https://web7.dev',
      //   width: 70,
      // },
      copyright: `Powered by Web7. Built with Docusaurus.`,
    },
  },
};

module.exports = config;
