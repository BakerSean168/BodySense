import { Check, ChevronDown, Play, Undo2, X } from "lucide-react";
import type { BodyRecords } from "@/features/profile/services/bodyRecordsService";
import type { BodyRegionId } from "@/features/body-explorer/model/bodyRegionOntology";
import { getBodyRegionDefinition } from "@/features/body-explorer/model/bodyRegionOntology";
import type { CanvasSelection, ReferenceMotion } from "../model/bodyCanvas";

/** Selected-body facts, region actions, and the compact reopen entry. */
export function CanvasRegionDetails({
  primary,
  selection,
  regionFacts,
  hasBodyState,
  loading,
  error,
  regionCardOpen,
  regionListOpen,
  activeWindow,
  motionMode,
  definition,
  phase,
  selectionLabel,
  onCardOpenChange,
  onRemoveRegion,
  onUndo,
  onClear,
  onAsk,
  onOpenPerson,
}: {
  primary: BodyRegionId | null;
  selection: CanvasSelection;
  regionFacts: BodyRecords["facts"];
  hasBodyState: boolean;
  loading?: boolean;
  error?: boolean;
  regionCardOpen: boolean;
  regionListOpen: boolean;
  activeWindow: boolean;
  motionMode: boolean;
  definition: ReferenceMotion;
  phase: number;
  selectionLabel: string;
  onCardOpenChange: (value: boolean) => void;
  onRemoveRegion: (region: BodyRegionId) => void;
  onUndo: () => void;
  onClear: () => void;
  onAsk: () => void;
  onOpenPerson: () => void;
}) {
  if (!primary) return null;
  const showRegionCard = regionCardOpen && !activeWindow && !regionListOpen;
  const setRegionCardOpen = onCardOpenChange;
  const removeRegion = onRemoveRegion;
  const undo = onUndo;
  const clear = onClear;
  const ask = onAsk;
  return (
    <>
      {showRegionCard && (
        <aside
          className="bc-region-card bc-floating-panel"
          aria-label="已选区域当前状况"
        >
          <div className="bc-panel-heading">
            {/* Penpot "Body / Region": region name + a mint 已选择 chip. */}
            <div className="bc-region-title">
              <h2>
                {selection.regions.length > 1
                  ? `${selection.regions.length} 个区域`
                  : getBodyRegionDefinition(primary).labels["zh-CN"]}
              </h2>
              <span className="bc-region-selected">已选择</span>
            </div>
            <button
              className="bc-icon-button"
              onClick={() => setRegionCardOpen(false)}
              aria-label="收起区域详情"
            >
              <X size={20} />
            </button>
          </div>
          <div className="bc-selection-chips">
            {selection.regions.map((id) => (
              <button
                key={id}
                onClick={() => removeRegion(id)}
                aria-label={`取消选择${getBodyRegionDefinition(id).labels["zh-CN"]}`}
              >
                {getBodyRegionDefinition(id).labels["zh-CN"]}
                <X size={12} />
              </button>
            ))}
          </div>
          <div className="bc-region-facts">
            {loading ? (
              <p role="status">正在读取这个区域的记录…</p>
            ) : error && !hasBodyState ? (
              <p>暂时无法读取记录；这不代表这个区域没有问题。</p>
            ) : regionFacts.length ? (
              <>
                <span className="bc-small-label">{regionFacts.length} 条</span>
                {regionFacts.slice(0, 3).map((fact) => (
                  <article key={fact.id}>
                    <Check
                      className="bc-region-fact-mark"
                      size={14}
                      aria-hidden="true"
                    />
                    <strong>“{fact.statement}”</strong>
                    <span>
                      用户陈述 ·{" "}
                      {new Date(fact.recorded_at).toLocaleDateString("zh-CN")}
                    </span>
                  </article>
                ))}
              </>
            ) : (
              <>
                <h3>还没有记录</h3>
                <p>选中一个位置，不代表那里一定存在问题。</p>
              </>
            )}
          </div>
          {motionMode && (
            <div className="bc-context-note">
              <Play size={15} />
              <span>
                {definition.label} · {(phase * definition.duration).toFixed(2)}s
                <br />
                <small>通用参考动作，不是你的实际动作证据</small>
              </span>
            </div>
          )}
          <div className="bc-panel-actions">
            <div className="bc-actions">
              <button
                className="bc-text-button"
                onClick={undo}
                disabled={!selection.history.length}
              >
                <Undo2 size={15} />
                撤销选择
              </button>
              <button className="bc-text-button" onClick={clear}>
                清除选区
              </button>
            </div>
          </div>
          {/* Penpot footer: 身体记录 on the left, the region question right. */}
          <div className="bc-region-footer">
            <button
              className="bc-text-button bc-region-records"
              onClick={onOpenPerson}
            >
              身体记录
            </button>
            <button className="bc-button bc-region-ask" onClick={ask}>
              {selection.regions.length > 1
                ? "围绕选区提问"
                : `围绕${getBodyRegionDefinition(primary).labels["zh-CN"]}提问`}
            </button>
          </div>
        </aside>
      )}
      {primary && !showRegionCard && !activeWindow && !regionListOpen && (
        <button
          className="bc-reopen-selection bc-button"
          onClick={() => setRegionCardOpen(true)}
        >
          {selectionLabel}
          <ChevronDown size={16} />
        </button>
      )}
    </>
  );
}
