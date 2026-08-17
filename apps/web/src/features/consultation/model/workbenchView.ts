export const workspaceViews = [
  "state",
  "diagnosis",
  "treatment",
  "progress",
] as const;

export type WorkspaceView = (typeof workspaceViews)[number];

export function parseWorkspaceView(value: string | null): WorkspaceView {
  return workspaceViews.includes(value as WorkspaceView)
    ? (value as WorkspaceView)
    : "state";
}
