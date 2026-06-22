import { useState } from 'react';

interface BodyVisualizationProps {
  highlightedParts: string[];
  onPartClick?: (part: string) => void;
}

// Map of body part names to SVG group IDs
const BODY_PART_MAP: Record<string, string[]> = {
  '头部': ['head'],
  '颈椎': ['neck'],
  '肩部': ['shoulder-left', 'shoulder-right'],
  '左肩': ['shoulder-left'],
  '右肩': ['shoulder-right'],
  '胸部': ['chest'],
  '背部': ['back'],
  '腰部': ['lower-back'],
  '手臂': ['arm-left', 'arm-right'],
  '左手': ['arm-left'],
  '右手': ['arm-right'],
  '髋部': ['hip-left', 'hip-right'],
  '骨盆': ['pelvis'],
  '膝盖': ['knee-left', 'knee-right'],
  '左膝': ['knee-left'],
  '右膝': ['knee-right'],
  '腿部': ['leg-left', 'leg-right'],
  '脚踝': ['ankle-left', 'ankle-right'],
};

type ViewType = 'front' | 'side' | 'back';

export function BodyVisualization({ highlightedParts, onPartClick }: BodyVisualizationProps) {
  const [activeView, setActiveView] = useState<ViewType>('front');

  // Get highlighted SVG IDs
  const highlightedIds = new Set<string>();
  for (const part of highlightedParts) {
    const ids = BODY_PART_MAP[part];
    if (ids) {
      ids.forEach((id) => highlightedIds.add(id));
    }
  }

  const handlePartClick = (partName: string) => {
    onPartClick?.(partName);
  };

  return (
    <div className="flex flex-col items-center">
      {/* View selector */}
      <div className="flex gap-2 mb-3">
        {(['front', 'side', 'back'] as ViewType[]).map((view) => (
          <button
            key={view}
            onClick={() => setActiveView(view)}
            className={`px-3 py-1 text-xs rounded-full transition-colors ${
              activeView === view
                ? 'bg-blue-600 text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
            }`}
          >
            {view === 'front' ? '正面' : view === 'side' ? '侧面' : '背面'}
          </button>
        ))}
      </div>

      {/* SVG Body */}
      <div className="w-full max-w-[200px]">
        {activeView === 'front' && (
          <FrontViewSVG highlightedIds={highlightedIds} onPartClick={handlePartClick} />
        )}
        {activeView === 'side' && (
          <SideViewSVG highlightedIds={highlightedIds} onPartClick={handlePartClick} />
        )}
        {activeView === 'back' && (
          <BackViewSVG highlightedIds={highlightedIds} onPartClick={handlePartClick} />
        )}
      </div>

      {/* Legend */}
      {highlightedParts.length > 0 && (
        <div className="mt-3 text-xs text-gray-500 text-center">
          高亮部位：{highlightedParts.join('、')}
        </div>
      )}
    </div>
  );
}

interface ViewSVGProps {
  highlightedIds: Set<string>;
  onPartClick: (partName: string) => void;
}

function getFill(id: string, highlightedIds: Set<string>): string {
  return highlightedIds.has(id) ? '#3b82f6' : '#e5e7eb';
}

function getStroke(id: string, highlightedIds: Set<string>): string {
  return highlightedIds.has(id) ? '#2563eb' : '#9ca3af';
}

function FrontViewSVG({ highlightedIds, onPartClick }: ViewSVGProps) {
  return (
    <svg viewBox="0 0 200 400" className="w-full h-auto">
      {/* Head */}
      <ellipse
        id="head"
        cx="100" cy="40" rx="25" ry="30"
        fill={getFill('head', highlightedIds)}
        stroke={getStroke('head', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('头部')}
      />
      {/* Neck */}
      <rect
        id="neck"
        x="90" y="68" width="20" height="15"
        fill={getFill('neck', highlightedIds)}
        stroke={getStroke('neck', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('颈椎')}
      />
      {/* Shoulders */}
      <path
        id="shoulder-left"
        d="M90,83 L55,95 L55,110 L70,105 L90,95"
        fill={getFill('shoulder-left', highlightedIds)}
        stroke={getStroke('shoulder-left', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('左肩')}
      />
      <path
        id="shoulder-right"
        d="M110,83 L145,95 L145,110 L130,105 L110,95"
        fill={getFill('shoulder-right', highlightedIds)}
        stroke={getStroke('shoulder-right', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('右肩')}
      />
      {/* Chest */}
      <path
        id="chest"
        d="M70,95 L130,95 L130,160 L70,160 Z"
        fill={getFill('chest', highlightedIds)}
        stroke={getStroke('chest', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('胸部')}
      />
      {/* Arms */}
      <path
        id="arm-left"
        d="M55,110 L40,180 L35,250 L50,250 L55,180 L65,110"
        fill={getFill('arm-left', highlightedIds)}
        stroke={getStroke('arm-left', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('左手')}
      />
      <path
        id="arm-right"
        d="M145,110 L160,180 L165,250 L150,250 L145,180 L135,110"
        fill={getFill('arm-right', highlightedIds)}
        stroke={getStroke('arm-right', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('右手')}
      />
      {/* Waist / Lower back */}
      <path
        id="lower-back"
        d="M70,160 L130,160 L125,200 L75,200 Z"
        fill={getFill('lower-back', highlightedIds)}
        stroke={getStroke('lower-back', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('腰部')}
      />
      {/* Hips */}
      <path
        id="hip-left"
        d="M75,200 L100,200 L95,230 L65,230 Z"
        fill={getFill('hip-left', highlightedIds)}
        stroke={getStroke('hip-left', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('髋部')}
      />
      <path
        id="hip-right"
        d="M100,200 L125,200 L135,230 L105,230 Z"
        fill={getFill('hip-right', highlightedIds)}
        stroke={getStroke('hip-right', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('髋部')}
      />
      {/* Legs */}
      <path
        id="leg-left"
        d="M65,230 L80,230 L78,330 L62,330 Z"
        fill={getFill('leg-left', highlightedIds)}
        stroke={getStroke('leg-left', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('腿部')}
      />
      <path
        id="leg-right"
        d="M120,230 L135,230 L138,330 L122,330 Z"
        fill={getFill('leg-right', highlightedIds)}
        stroke={getStroke('leg-right', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('腿部')}
      />
      {/* Knees */}
      <ellipse
        id="knee-left"
        cx="71" cy="280" rx="10" ry="12"
        fill={getFill('knee-left', highlightedIds)}
        stroke={getStroke('knee-left', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('左膝')}
      />
      <ellipse
        id="knee-right"
        cx="129" cy="280" rx="10" ry="12"
        fill={getFill('knee-right', highlightedIds)}
        stroke={getStroke('knee-right', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('右膝')}
      />
      {/* Feet */}
      <path
        id="ankle-left"
        d="M62,330 L55,370 L85,370 L78,330"
        fill={getFill('ankle-left', highlightedIds)}
        stroke={getStroke('ankle-left', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('脚踝')}
      />
      <path
        id="ankle-right"
        d="M122,330 L115,370 L145,370 L138,330"
        fill={getFill('ankle-right', highlightedIds)}
        stroke={getStroke('ankle-right', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('脚踝')}
      />
    </svg>
  );
}

function SideViewSVG({ highlightedIds, onPartClick }: ViewSVGProps) {
  return (
    <svg viewBox="0 0 200 400" className="w-full h-auto">
      {/* Head */}
      <ellipse
        id="head"
        cx="100" cy="40" rx="22" ry="28"
        fill={getFill('head', highlightedIds)}
        stroke={getStroke('head', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('头部')}
      />
      {/* Neck */}
      <path
        id="neck"
        d="M88,65 L105,65 L105,83 L88,83 Z"
        fill={getFill('neck', highlightedIds)}
        stroke={getStroke('neck', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('颈椎')}
      />
      {/* Upper body (chest + back) */}
      <path
        id="chest"
        d="M88,83 L120,83 L125,160 L80,160 Z"
        fill={getFill('chest', highlightedIds)}
        stroke={getStroke('chest', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('胸部')}
      />
      <path
        id="back"
        d="M80,83 L88,83 L80,160 L75,160 Z"
        fill={getFill('back', highlightedIds)}
        stroke={getStroke('back', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('背部')}
      />
      {/* Lower back */}
      <path
        id="lower-back"
        d="M80,160 L125,160 L120,200 L78,200 Z"
        fill={getFill('lower-back', highlightedIds)}
        stroke={getStroke('lower-back', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('腰部')}
      />
      {/* Arm */}
      <path
        id="arm-left"
        d="M120,90 L140,150 L145,230 L130,230 L128,150 L115,95"
        fill={getFill('arm-left', highlightedIds)}
        stroke={getStroke('arm-left', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('手臂')}
      />
      {/* Hip + leg */}
      <path
        id="hip-left"
        d="M78,200 L120,200 L115,230 L75,230 Z"
        fill={getFill('hip-left', highlightedIds)}
        stroke={getStroke('hip-left', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('骨盆')}
      />
      <path
        id="leg-left"
        d="M75,230 L115,230 L110,330 L80,330 Z"
        fill={getFill('leg-left', highlightedIds)}
        stroke={getStroke('leg-left', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('腿部')}
      />
      <ellipse
        id="knee-left"
        cx="95" cy="280" rx="12" ry="10"
        fill={getFill('knee-left', highlightedIds)}
        stroke={getStroke('knee-left', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('膝盖')}
      />
      <path
        id="ankle-left"
        d="M80,330 L75,370 L115,370 L110,330"
        fill={getFill('ankle-left', highlightedIds)}
        stroke={getStroke('ankle-left', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('脚踝')}
      />
    </svg>
  );
}

function BackViewSVG({ highlightedIds, onPartClick }: ViewSVGProps) {
  return (
    <svg viewBox="0 0 200 400" className="w-full h-auto">
      {/* Head */}
      <ellipse
        id="head"
        cx="100" cy="40" rx="25" ry="30"
        fill={getFill('head', highlightedIds)}
        stroke={getStroke('head', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('头部')}
      />
      {/* Neck */}
      <rect
        id="neck"
        x="90" y="68" width="20" height="15"
        fill={getFill('neck', highlightedIds)}
        stroke={getStroke('neck', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('颈椎')}
      />
      {/* Back */}
      <path
        id="back"
        d="M70,83 L130,83 L130,160 L70,160 Z"
        fill={getFill('back', highlightedIds)}
        stroke={getStroke('back', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('背部')}
      />
      {/* Shoulders */}
      <path
        id="shoulder-left"
        d="M90,83 L55,95 L55,110 L70,105 L90,95"
        fill={getFill('shoulder-left', highlightedIds)}
        stroke={getStroke('shoulder-left', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('左肩')}
      />
      <path
        id="shoulder-right"
        d="M110,83 L145,95 L145,110 L130,105 L110,95"
        fill={getFill('shoulder-right', highlightedIds)}
        stroke={getStroke('shoulder-right', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('右肩')}
      />
      {/* Lower back */}
      <path
        id="lower-back"
        d="M70,160 L130,160 L125,200 L75,200 Z"
        fill={getFill('lower-back', highlightedIds)}
        stroke={getStroke('lower-back', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('腰部')}
      />
      {/* Arms */}
      <path
        id="arm-left"
        d="M55,110 L40,180 L35,250 L50,250 L55,180 L65,110"
        fill={getFill('arm-left', highlightedIds)}
        stroke={getStroke('arm-left', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('左手')}
      />
      <path
        id="arm-right"
        d="M145,110 L160,180 L165,250 L150,250 L145,180 L135,110"
        fill={getFill('arm-right', highlightedIds)}
        stroke={getStroke('arm-right', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('右手')}
      />
      {/* Hips */}
      <path
        id="hip-left"
        d="M75,200 L100,200 L95,230 L65,230 Z"
        fill={getFill('hip-left', highlightedIds)}
        stroke={getStroke('hip-left', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('髋部')}
      />
      <path
        id="hip-right"
        d="M100,200 L125,200 L135,230 L105,230 Z"
        fill={getFill('hip-right', highlightedIds)}
        stroke={getStroke('hip-right', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('髋部')}
      />
      {/* Legs */}
      <path
        id="leg-left"
        d="M65,230 L80,230 L78,330 L62,330 Z"
        fill={getFill('leg-left', highlightedIds)}
        stroke={getStroke('leg-left', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('腿部')}
      />
      <path
        id="leg-right"
        d="M120,230 L135,230 L138,330 L122,330 Z"
        fill={getFill('leg-right', highlightedIds)}
        stroke={getStroke('leg-right', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('腿部')}
      />
      {/* Knees */}
      <ellipse
        id="knee-left"
        cx="71" cy="280" rx="10" ry="12"
        fill={getFill('knee-left', highlightedIds)}
        stroke={getStroke('knee-left', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('左膝')}
      />
      <ellipse
        id="knee-right"
        cx="129" cy="280" rx="10" ry="12"
        fill={getFill('knee-right', highlightedIds)}
        stroke={getStroke('knee-right', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('右膝')}
      />
      {/* Feet */}
      <path
        id="ankle-left"
        d="M62,330 L55,370 L85,370 L78,330"
        fill={getFill('ankle-left', highlightedIds)}
        stroke={getStroke('ankle-left', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('脚踝')}
      />
      <path
        id="ankle-right"
        d="M122,330 L115,370 L145,370 L138,330"
        fill={getFill('ankle-right', highlightedIds)}
        stroke={getStroke('ankle-right', highlightedIds)}
        strokeWidth="1.5"
        className="cursor-pointer"
        onClick={() => onPartClick('脚踝')}
      />
    </svg>
  );
}
