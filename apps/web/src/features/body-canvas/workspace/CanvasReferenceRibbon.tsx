import { useEffect, useRef, useState } from "react";
import { ChevronDown, ChevronUp } from "lucide-react";
import {
  REFERENCE_MOTIONS,
  SHOW_POSE_BAR,
  type MotionId,
} from "../model/bodyCanvas";
import { MOTION_ICONS } from "../components/MotionIcons";

/** Reference motion picker owns its fold state and focus restoration. */
export function CanvasReferenceRibbon({
  motion,
  motionMode,
  onChoosePose,
}: {
  motion: MotionId;
  motionMode: boolean;
  onChoosePose: (id: MotionId) => void;
}) {
  // Penpot motion ribbon fold (Home · Motions collapsed).
  const [ribbonCollapsed, setRibbonCollapsed] = useState(false);
  const ribbonToggleRef = useRef<HTMLButtonElement>(null);
  const ribbonToggled = useRef(false);
  useEffect(() => {
    if (!ribbonToggled.current) return;
    ribbonToggled.current = false;
    ribbonToggleRef.current?.focus();
  }, [ribbonCollapsed]);
  const toggleRibbon = () => {
    ribbonToggled.current = true;
    setRibbonCollapsed((value) => !value);
  };

  const choosePose = onChoosePose;
  return (
    <>
      {!SHOW_POSE_BAR ? null : ribbonCollapsed ? (
        <button
          ref={ribbonToggleRef}
          type="button"
          className="bc-motion-ribbon-tab"
          aria-label="展开参考动作"
          title="展开参考动作"
          aria-expanded={false}
          onClick={toggleRibbon}
        >
          <ChevronDown size={18} strokeWidth={2.2} aria-hidden="true" />
        </button>
      ) : (
        <div className="bc-motion-ribbon" role="group" aria-label="参考动作">
          {REFERENCE_MOTIONS.slice(0, 4).map((item, index) => {
            const Icon = MOTION_ICONS[item.id];
            const pressed =
              item.id === "stand"
                ? !motionMode
                : motionMode && motion === item.id;
            return (
              <button
                key={item.id}
                type="button"
                className="bc-motion-pose"
                aria-label={item.label}
                title={`${item.label} · ${index + 1}`}
                aria-pressed={pressed}
                onClick={() => choosePose(item.id)}
              >
                {Icon ? <Icon /> : item.label}
                <kbd aria-hidden="true">{index + 1}</kbd>
              </button>
            );
          })}
          <button
            ref={ribbonToggleRef}
            type="button"
            className="bc-motion-ribbon-collapse"
            aria-label="收起参考动作"
            title="收起参考动作"
            aria-expanded={true}
            onClick={toggleRibbon}
          >
            <ChevronUp size={18} strokeWidth={2.2} aria-hidden="true" />
          </button>
        </div>
      )}
    </>
  );
}
