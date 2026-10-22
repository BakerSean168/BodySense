import "../src/index.css";

import type { Preview } from "@storybook/react-vite";

const preview = {
  beforeEach: () => {
    const original = window.fetch;
    window.fetch = (input, init) => {
      const raw =
        typeof input === "string"
          ? input
          : input instanceof URL
            ? input.href
            : input.url;
      const url = new URL(raw, window.location.href);
      if (url.pathname === "/api" || url.pathname.startsWith("/api/")) {
        return Promise.reject(new Error("Storybook 预览已隔离真实 API"));
      }
      return original(input, init);
    };
    return () => {
      window.fetch = original;
    };
  },
  initialGlobals: { theme: "light" },
  globalTypes: {
    theme: {
      description: "BodySense 业务窗口主题",
      toolbar: {
        title: "窗口主题",
        icon: "circlehollow",
        items: [
          { value: "light", title: "浅色" },
          { value: "dark", title: "深色" },
        ],
      },
    },
  },
  parameters: {
    controls: {
      expanded: true,
    },
  },
} satisfies Preview;

export default preview;
