import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://mlorentedev.github.io',
  base: '/ts-bridge',
  integrations: [
    starlight({
      title: 'ts-bridge',
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/mlorentedev/ts-bridge',
        },
      ],
      sidebar: [
        { label: 'Home', slug: '' },
        { label: 'Getting Started', slug: 'getting-started' },
      ],
    }),
  ],
});
