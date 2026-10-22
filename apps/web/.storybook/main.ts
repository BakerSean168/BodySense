import { fileURLToPath } from "node:url";
import { dirname } from "node:path";

import type { StorybookConfig } from "@storybook/react-vite";

const config: StorybookConfig = {
  stories: ["../src/**/*.@(mdx|stories.@(js|jsx|ts|tsx))"],
  addons: [],
  viteFinal: (viteConfig) => ({
    ...viteConfig,
    // Stories have independent entry/chunk graphs; application budgets remain
    // enforced by the unchanged production Vite build.
    plugins: viteConfig.plugins?.filter((plugin) =>
      !(plugin && typeof plugin === "object" && "name" in plugin && plugin.name === "bodysense-web-chunk-budgets"),
    ),
  }),
  framework: {
    name: getAbsolutePath("@storybook/react-vite"),
    options: {
      builder: {
        viteConfigPath: "vite.config.ts",
      },
    },
  },
};

function getAbsolutePath(value: string): string {
  return dirname(fileURLToPath(import.meta.resolve(`${value}/package.json`)));
}

export default config;

// To customize your Vite configuration you can use the viteFinal field.
// Check https://storybook.js.org/docs/react/builders/vite#configuration
// and https://nx.dev/recipes/storybook/custom-builder-configs
