import { MessageCircle, Pause, Play } from "lucide-react";
import type { ReferenceMotion } from "../model/bodyCanvas";

/** Reference motion only, not a patient-specific training prescription. */
export function CanvasPlaybackControls({
  motionMode,
  definition,
  phase,
  speed,
  effectivePlaying,
  safetyReview,
  onTogglePlaying,
  onSpeedChange,
  onSeek,
  onAsk,
}: {
  motionMode: boolean;
  definition: ReferenceMotion;
  phase: number;
  speed: number;
  effectivePlaying: boolean;
  safetyReview: boolean;
  onTogglePlaying: () => void;
  onSpeedChange: (speed: number) => void;
  onSeek: (phase: number) => void;
  onAsk: () => void;
}) {
  return (
    <>
      {motionMode && (
        <>
          <section className="bc-playback" aria-label="动作播放控制">
            <button
              className="bc-play-toggle"
              disabled={safetyReview}
              onClick={onTogglePlaying}
              aria-label={effectivePlaying ? "暂停动作" : "播放动作"}
            >
              {effectivePlaying ? <Pause size={20} /> : <Play size={20} />}
            </button>
            <div className="bc-playback-track">
              <div>
                <strong>{definition.label}</strong>
                <span>
                  {(phase * definition.duration).toFixed(2)}s /{" "}
                  {definition.duration.toFixed(2)}s
                </span>
              </div>
              <input
                type="range"
                min="0"
                max="1"
                step="0.001"
                value={phase}
                aria-label="动作进度"
                onChange={(event) => onSeek(Number(event.target.value))}
              />
            </div>
            <label className="bc-speed">
              <span className="sr-only">播放速度</span>
              <select
                aria-label="播放速度"
                value={speed}
                onChange={(event) => onSpeedChange(Number(event.target.value))}
              >
                <option value={0.5}>0.5×</option>
                <option value={1}>1×</option>
                <option value={1.5}>1.5×</option>
              </select>
            </label>
            <button
              className="bc-icon-button"
              onClick={onAsk}
              aria-label="询问当前动作"
            >
              <MessageCircle size={19} />
            </button>
          </section>
          <span className="bc-motion-disclaimer">
            参考动画 · 不估算真实受力，不作为医学评估
          </span>
        </>
      )}
    </>
  );
}
