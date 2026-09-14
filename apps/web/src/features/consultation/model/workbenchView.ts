export const workspaceViews = [
  "state",
  "diagnosis",
  "treatment",
  "progress",
] as const;

export type WorkspaceView = (typeof workspaceViews)[number];

export function parseWorkspaceView(value: string | null): WorkspaceView {
  switch (value) {
    case "state":
    case "diagnosis":
    case "treatment":
    case "progress":
      return value;
    default:
      return "state";
  }
}
