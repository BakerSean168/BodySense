import type { Decorator } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import "../body-canvas.css";

// Storybook uses the same theme scope and positioned stage as the real workspace.
// None of these examples mounts the 3D scene or starts a live business session.
const previewQueryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: false, refetchOnWindowFocus: false },
  },
});

export const windowCanvasDecorator: Decorator = (Story, context) => (
  <QueryClientProvider client={previewQueryClient}>
    <div
      className={`body-canvas-theme bc-workspace${context.globals.theme === "dark" ? " bc-dark" : ""}`}
      style={{ height: "920px", minHeight: "920px" }}
    >
      <div className="bc-stage">
        <div className="bc-business-layer">
          <Story />
        </div>
      </div>
    </div>
  </QueryClientProvider>
);
