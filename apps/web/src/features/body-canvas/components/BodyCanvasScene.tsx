import { useState } from "react";
import { Canvas } from "@react-three/fiber";
import { Color, SRGBColorSpace, ACESFilmicToneMapping } from "three";
import {
  getBodyRegionDefinition,
  type BodyRegionId,
} from "@/features/body-explorer/model/bodyRegionOntology";
import { ReferenceCanvasContents } from "../scene/ReferenceCanvasContents";
import { SceneBoundary } from "../scene/SceneBoundary";
import { useReferenceSceneAssets } from "../scene/useReferenceSceneAssets";
import type { BodyCanvasSceneProps } from "../scene/types";
export type { BodyCanvasSceneProps, SkeletonLayerStatus } from "../scene/types";

export default function BodyCanvasScene(props: BodyCanvasSceneProps) {
  const {
    rig,
    error,
    setError,
    muscles,
    skeleton,
    skeletonStatus,
    attempt,
    setAttempt,
  } = useReferenceSceneAssets(
    props.layers?.skeleton ?? false,
    props.onSkeletonStatus,
  );
  const [hover, setHover] = useState<BodyRegionId | null>(null);
  const [cursor, setCursor] = useState<HTMLDivElement | null>(null);
  const [box, setBox] = useState<HTMLDivElement | null>(null);
  const fallback = (
    <div className="bc-scene-status" role="status">
      <h2>3D 暂时无法显示</h2>
      <div className="bc-actions">
        <button
          className="bc-button"
          onClick={() => setAttempt((value) => value + 1)}
        >
          重试 3D
        </button>
        <button className="bc-button bc-primary" onClick={props.onTextFallback}>
          按名称选择区域
        </button>
      </div>
      {error && (
        <details>
          <summary>查看加载信息</summary>
          <p>{error}</p>
        </details>
      )}
    </div>
  );
  if (error) return fallback;
  if (!rig)
    return (
      <div className="bc-scene-status" role="status">
        <span className="bc-loading-orbit" aria-hidden="true" />
        <h2>正在准备身体视图</h2>
        <button className="bc-button" onClick={props.onTextFallback}>
          先使用文本区域
        </button>
      </div>
    );
  return (
    <div
      className="bc-scene"
      data-tool={props.tool}
      data-viewer-state="ready"
      data-muscles={muscles ? "ready" : "loading"}
      data-skeleton={skeletonStatus}
      aria-label="交互式参考人体：拖动旋转，点击选择部位"
    >
      <SceneBoundary key={attempt} fallback={fallback}>
        <Canvas
          frameloop="demand"
          dpr={[1, 1.5]}
          camera={{
            position: [0.36, 0.94, 3.12],
            fov: 33,
            near: 0.02,
            far: 20,
          }}
          gl={{
            alpha: true,
            antialias: true,
            powerPreference: "high-performance",
          }}
          onCreated={({ gl }) => {
            gl.setClearColor(new Color(0, 0, 0), 0);
            gl.outputColorSpace = SRGBColorSpace;
            gl.toneMapping = ACESFilmicToneMapping;
            gl.toneMappingExposure = 1.22;
          }}
        >
          <ReferenceCanvasContents
            {...props}
            rig={rig}
            muscles={muscles}
            skeleton={skeleton}
            cursor={cursor}
            box={box}
            onHover={setHover}
            onContextLost={() =>
              setError("图形上下文已中断，可重试或切换文本模式。")
            }
          />
        </Canvas>
      </SceneBoundary>
      <div ref={setCursor} className="bc-brush-cursor" aria-hidden="true" />
      <div ref={setBox} className="bc-selection-box" aria-hidden="true" />
      {hover && (
        <span className="bc-hover-label">
          {getBodyRegionDefinition(hover).labels["zh-CN"]}
        </span>
      )}
    </div>
  );
}
