import { useRef } from "react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useCanvasContextRestore, useResizeObserver } from "./useViewerCanvas";

function Viewer({
  ready,
  onRestore,
  onWidth,
}: {
  ready: boolean;
  onRestore: () => void;
  onWidth: (width: number) => void;
}) {
  const root = useRef<HTMLDivElement>(null);
  useCanvasContextRestore(root, onRestore, ready);
  useResizeObserver(root, onWidth, ready);
  return ready ? (
    <div ref={root}>
      <canvas data-testid="canvas" />
    </div>
  ) : (
    <p>loading</p>
  );
}

describe("viewer canvas lifecycle", () => {
  it("attaches after asynchronous loading and removes listeners on unmount", () => {
    const onRestore = vi.fn();
    const onWidth = vi.fn();
    const observe = vi.spyOn(ResizeObserver.prototype, "observe");
    const { rerender, unmount } = render(
      <Viewer ready={false} onRestore={onRestore} onWidth={onWidth} />,
    );
    rerender(<Viewer ready onRestore={onRestore} onWidth={onWidth} />);
    const canvas = screen.getByTestId("canvas");
    canvas.dispatchEvent(new Event("webglcontextrestored"));
    expect(onRestore).toHaveBeenCalledTimes(1);
    expect(observe).toHaveBeenCalled();
    unmount();
    canvas.dispatchEvent(new Event("webglcontextrestored"));
    expect(onRestore).toHaveBeenCalledTimes(1);
    observe.mockRestore();
  });
});
