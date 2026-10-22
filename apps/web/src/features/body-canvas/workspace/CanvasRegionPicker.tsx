import type { RefObject } from "react";
import { ArrowRight, X } from "lucide-react";
import {
  BODY_REGION_IDS,
  getBodyRegionDefinition,
  isBodyRegionId,
  type BodyRegionId,
} from "@/features/body-explorer/model/bodyRegionOntology";
import type { CanvasSelection } from "../model/bodyCanvas";

/** Text-first region chooser; the 3D renderer is not required. */
export function CanvasRegionPicker({
  visible,
  selection,
  primary,
  selectRef,
  onSelect,
  onVisibleChange,
  onCardOpenChange,
}: {
  visible: boolean;
  selection: CanvasSelection;
  primary: BodyRegionId | null;
  selectRef: RefObject<HTMLSelectElement | null>;
  onSelect: (regions: BodyRegionId[], append?: boolean) => void;
  onVisibleChange: (visible: boolean) => void;
  onCardOpenChange: (visible: boolean) => void;
}) {
  const regionListOpen = visible;
  const select = onSelect;
  const setRegionListOpen = onVisibleChange;
  const setRegionCardOpen = onCardOpenChange;
  return (
    <>
      {regionListOpen && (
        <aside
          className="bc-region-picker bc-floating-panel"
          aria-label="区域名称选择"
        >
          <div className="bc-panel-heading">
            <div>
              <span className="bc-eyebrow">BODY REGIONS</span>
              <h2>从区域开始</h2>
            </div>
            <button
              className="bc-icon-button"
              onClick={() => setRegionListOpen(false)}
              aria-label="关闭区域列表"
            >
              <X size={20} />
            </button>
          </div>
          <label className="bc-field">
            选择身体区域
            <select
              ref={selectRef}
              aria-label="选择身体区域"
              value={primary ?? ""}
              onChange={(event) => {
                if (isBodyRegionId(event.target.value))
                  select([event.target.value]);
              }}
            >
              <option value="">请选择区域</option>
              {BODY_REGION_IDS.map((id) => (
                <option key={id} value={id}>
                  {getBodyRegionDefinition(id).labels["zh-CN"]}
                </option>
              ))}
            </select>
          </label>
          <p className="bc-muted">左右以人体自身为准。可按名称追加多个区域。</p>
          <div className="bc-region-grid">
            {BODY_REGION_IDS.map((id) => (
              <button
                key={id}
                aria-pressed={selection.regions.includes(id)}
                onClick={() => select([id], true)}
              >
                {getBodyRegionDefinition(id).labels["zh-CN"]}
              </button>
            ))}
          </div>
          <button
            className="bc-button bc-primary"
            disabled={!primary}
            onClick={() => {
              setRegionListOpen(false);
              setRegionCardOpen(true);
            }}
          >
            查看已选区域
            <ArrowRight size={16} />
          </button>
        </aside>
      )}
    </>
  );
}
