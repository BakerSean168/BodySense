import { useEffect, useRef, useState } from "react";
import {
  Brush,
  ChevronLeft,
  ChevronRight,
  Layers3,
  LoaderCircle,
  MousePointer2,
  RotateCcw,
  Search,
  X,
} from "lucide-react";
import type {
  BodyCanvasLayers,
  BodyLayerId,
  CanvasTool,
} from "../model/bodyCanvas";
import type { SkeletonLayerStatus } from "../components/BodyCanvasScene";
import { regionPaletteShortcut } from "../model/bodyCanvas";

/**
 * Penpot Home tools rail: 视角 R, 图层 L, 标注 B, 选择 V (top to bottom), then
 * the region-by-name palette. Box select keeps its S shortcut.
 */
const SELECT_TOOLS = [
  ["brush", "标注 · 涂抹选区", "标注", "B", Brush],
  ["orbit", "选择 · 旋转与点选", "选择", "V", MousePointer2],
] as const;

/** 精细解剖 layers offered on the Home body (L). */
const BODY_LAYER_OPTIONS: ReadonlyArray<[BodyLayerId, string, string]> = [
  ["skin", "皮肤", "#ebe6d8"],
  ["muscles", "肌肉", "#c99285"],
  ["skeleton", "骨骼", "#d9cfb4"],
];

/** Home rail, anatomy layer toggles and tool hint share one visual owner. */
export function CanvasToolControls({
  tool,
  onSelectTool,
  onExitTool,
  onReset,
  onOpenRegionPalette,
  anatomyMode,
  onToggleAnatomy,
  bodyLayers,
  skeletonStatus,
  onToggleLayer,
}: {
  tool: CanvasTool;
  onSelectTool: (tool: CanvasTool) => void;
  onExitTool: () => void;
  onReset: () => void;
  onOpenRegionPalette: () => void;
  anatomyMode: boolean;
  onToggleAnatomy: () => void;
  bodyLayers: BodyCanvasLayers;
  skeletonStatus: SkeletonLayerStatus;
  onToggleLayer: (layer: BodyLayerId) => void;
}) {
  // Penpot "Desktop / Home · Tools collapsed": the rail folds to a side tab.
  // Tool shortcuts stay active; focus follows the toggle.
  const [toolsCollapsed, setToolsCollapsed] = useState(false);
  const toolsToggleRef = useRef<HTMLButtonElement>(null);
  const toolsToggled = useRef(false);
  useEffect(() => {
    if (!toolsToggled.current) return;
    toolsToggled.current = false;
    toolsToggleRef.current?.focus();
  }, [toolsCollapsed]);
  const toggleTools = () => {
    toolsToggled.current = true;
    setToolsCollapsed((value) => !value);
  };

  return (
    <>
      {toolsCollapsed ? (
        <button
          ref={toolsToggleRef}
          type="button"
          className="bc-tool-rail-tab"
          aria-label="展开工具栏"
          title="展开工具栏"
          aria-expanded={false}
          onClick={toggleTools}
        >
          <ChevronRight size={18} strokeWidth={2} aria-hidden="true" />
        </button>
      ) : (
        <div className="bc-tool-rail" role="toolbar" aria-label="身体画布工具">
          <button
            className="bc-icon-button"
            aria-label="视角 · 重置视角"
            title="重置视角 · R"
            onClick={onReset}
          >
            <RotateCcw size={20} strokeWidth={1.7} />
            <span className="bc-tool-caption">视角</span>
            <kbd aria-hidden="true">R</kbd>
          </button>
          <button
            className="bc-icon-button"
            aria-label="图层 · 精细解剖"
            title="精细解剖图层 · L"
            aria-pressed={anatomyMode}
            aria-expanded={anatomyMode}
            onClick={onToggleAnatomy}
          >
            <Layers3 size={20} strokeWidth={1.7} />
            <span className="bc-tool-caption">图层</span>
            <kbd aria-hidden="true">L</kbd>
          </button>
          {SELECT_TOOLS.map(([value, label, caption, key, Icon]) => (
            <button
              key={value}
              className="bc-icon-button"
              aria-label={label}
              title={`${label} · ${key}`}
              aria-pressed={tool === value}
              onClick={() => onSelectTool(value)}
            >
              <Icon size={20} strokeWidth={1.7} />
              <span className="bc-tool-caption">{caption}</span>
              <kbd aria-hidden="true">{key}</kbd>
            </button>
          ))}
          <button
            className="bc-icon-button"
            aria-label="按名称选择区域"
            title={`按名称选择区域 · ${regionPaletteShortcut()}`}
            onClick={onOpenRegionPalette}
          >
            <Search size={20} strokeWidth={1.7} />
            <span className="bc-tool-caption">区域</span>
            <kbd aria-hidden="true">{regionPaletteShortcut()}</kbd>
          </button>
          <button
            ref={toolsToggleRef}
            type="button"
            className="bc-icon-button bc-tool-rail-collapse"
            aria-label="收起工具栏"
            title="收起工具栏"
            aria-expanded={true}
            onClick={toggleTools}
          >
            <ChevronLeft size={18} strokeWidth={2} aria-hidden="true" />
          </button>
        </div>
      )}
      {anatomyMode && (
        <div
          className="bc-layer-panel"
          role="group"
          aria-label="精细解剖图层"
          data-testid="body-layer-panel"
        >
          <span className="bc-layer-panel-title" aria-hidden="true">
            图层
          </span>
          {BODY_LAYER_OPTIONS.map(([id, label, swatch]) => (
            <button
              key={id}
              type="button"
              className="bc-layer-toggle"
              aria-pressed={bodyLayers[id]}
              aria-busy={
                id === "skeleton" && skeletonStatus === "loading"
                  ? true
                  : undefined
              }
              title={
                id === "skeleton" && skeletonStatus === "error"
                  ? "骨骼层暂时无法加载，点按重试"
                  : label
              }
              onClick={() => onToggleLayer(id)}
            >
              <span
                className="bc-layer-swatch"
                style={{ background: swatch }}
                aria-hidden="true"
              />
              <span className="bc-layer-label">{label}</span>
              {id === "skeleton" && skeletonStatus === "loading" ? (
                <LoaderCircle
                  size={12}
                  className="animate-spin bc-layer-status"
                  aria-hidden="true"
                />
              ) : null}
            </button>
          ))}
          <button
            type="button"
            className="bc-layer-close"
            aria-label="关闭图层"
            title="关闭图层 · Esc"
            onClick={onToggleAnatomy}
          >
            <X size={14} aria-hidden="true" />
          </button>
        </div>
      )}
      {tool !== "orbit" && (
        <div className="bc-tool-hint" role="status">
          {tool === "brush"
            ? "涂抹可见表面，选择需要询问的区域"
            : "拖出选框，只选择可见身体区域"}
          <span>Shift 追加 · Esc 返回</span>
          <button aria-label="退出选区模式" onClick={onExitTool}>
            <X size={15} />
          </button>
        </div>
      )}
    </>
  );
}
