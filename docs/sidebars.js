// @ts-check

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  docs: [
    {
      type: 'doc',
      id: 'overview',
      label: 'Overview',
    },
    {
      type: 'doc',
      id: 'commands',
      label: 'Commands',
    },
    {
      type: 'doc',
      id: 'configuration',
      label: 'Configuration',
    },
    {
      type: 'doc',
      id: 'examples',
      label: 'Examples',
    },
    {
      type: 'doc',
      id: 'troubleshoot',
      label: 'Troubleshooting',
    },
    {
      type: 'doc',
      id: 'development',
      label: 'Development',
    },
    {
      type: 'category',
      label: 'Guides',
      collapsed: false,
      items: [
        'guide/getting-started',
        'guide/environments',
        'guide/configurations',
        'guide/providers',
        'guide/functions',
        'guide/registries',
        'guide/resources',
        'guide/local-nodes',
        'guide/remote-nodes',
        'guide/plugins',
      ],
    },
    {
      type: 'category',
      label: 'Registry',
      items: [
        'registry/auth',
      ],
    },
  ],
};

module.exports = sidebars;
