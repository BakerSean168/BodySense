import { useEffect, useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";
import type { BodyRegionId } from "@/features/body-explorer/model/bodyRegionOntology";
import BodyCanvasScene, { type BodyCanvasSceneProps } from "./BodyCanvasScene";
import "../body-canvas.css";

/**
 * A real pinned-atlas preview: no workspace API, account, or session is mounted.
 * Select/rotate the reference model; it does not create health records.
 */
function InteractiveReference(props: BodyCanvasSceneProps) {
  const [selected, setSelected] = useState<BodyRegionId[]>([
    ...props.selectedRegions,
  ]);
  useEffect(
    () => setSelected([...props.selectedRegions]),
    [props.selectedRegions],
  );

  return (
    <BodyCanvasScene
      {...props}
      selectedRegions={selected}
      onSelect={(regions, append) => {
        setSelected((current) =>
          append ? [...new Set([...current, ...regions])] : regions,
        );
        props.onSelect(regions, append);
      }}
    />
  );
}

const meta = {
  title: "BodySense/Canvas/3D Reference",
  component: BodyCanvasScene,
  parameters: { layout: "fullscreen" },
  decorators: [
    (Story, context) => (
      <main
        className={`body-canvas-theme bc-workspace${context.globals.theme === "dark" ? " bc-dark" : ""}`}
        style={{ width: "100%", height: "900px" }}
      >
        <section className="bc-stage" style={{ width: "100%", height: "100%" }}>
          <div className="bc-render-area">
            <Story />
          </div>
        </section>
      </main>
    ),
  ],
  render: (args, context) => (
    <InteractiveReference {...args} dark={context.globals.theme === "dark"} />
  ),
  args: {
    tool: "orbit",
    selectedRegions: [],
    motion: "stand",
    playing: false,
    speed: 1,
    seek: { phase: 0, revision: 0 },
    resetKey: 0,
    annotationEpoch: 0,
    dark: false,
    onSelect: fn(),
    onGestureStart: fn(),
    onPhaseChange: fn(),
    onTextFallback: fn(),
  },
} satisfies Meta<typeof BodyCanvasScene>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const RightShoulder: Story = {
  args: { selectedRegions: ["shoulder.right"] },
};
export const SkeletonLayer: Story = {
  args: { layers: { skin: true, muscles: true, skeleton: true } },
};
