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
      <div className="flex gap-2.5 mb-5 bg-[#F7F5F0] p-1 rounded-full border border-[#E5E3DF]">
        {(['front', 'side', 'back'] as ViewType[]).map((view) => (
          <button
            key={view}
            onClick={() => setActiveView(view)}
            className={`px-4.5 py-1.5 text-xs font-bold rounded-full transition-all duration-300 cursor-pointer ${
              activeView === view
                ? 'bg-primary-700 text-[#FBFBFA] shadow-sm shadow-[#2a4b3a]/10'
                : 'text-[#4A554E] hover:text-[#1A221E]'
            }`}
          >
            {view === 'front' ? '正面' : view === 'side' ? '侧面' : '背面'}
          </button>
        ))}
      </div>

      {/* SVG Body */}
      <div className="w-full max-w-[210px] py-2">
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
        <div className="mt-5 px-4 py-2 bg-primary-50/50 border border-primary-200/30 rounded-2xl text-xs font-bold text-primary-800 flex items-center gap-1.5 shadow-sm">
          <span className="w-1.5 h-1.5 rounded-full bg-accent-terracotta animate-pulse" />
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
  return highlightedIds.has(id) ? 'url(#pain-gradient)' : 'url(#clay-gradient)';
}

function getStroke(id: string, highlightedIds: Set<string>): string {
  return highlightedIds.has(id) ? '#B65E49' : '#C4BFAF';
}

function getFilter(id: string, highlightedIds: Set<string>): string | undefined {
  return highlightedIds.has(id) ? 'url(#glow-filter)' : undefined;
}

function FrontViewSVG({ highlightedIds, onPartClick }: ViewSVGProps) {
  return (
    <svg viewBox="0 0 200 400" className="w-full h-auto">
      <defs>
        <linearGradient id="clay-gradient" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#FAF9F6" />
          <stop offset="100%" stopColor="#E5E1D7" />
        </linearGradient>
        <linearGradient id="pain-gradient" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#FDA48F" />
          <stop offset="100%" stopColor="#CD7B67" />
        </linearGradient>
        <filter id="glow-filter" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="2.5" result="blur" />
          <feComponentTransfer in="blur" result="glow1">
            <feFuncA type="linear" slope="0.4"/>
          </feComponentTransfer>
          <feMerge>
            <feMergeNode in="glow1" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>
      {/* Head */}
      <path
        id="head"
        d="M100,14 C111,14 121,23 121,35 C121,48 111,62 100,62 C89,62 79,48 79,35 C79,23 89,14 100,14 Z"
        fill={getFill('head', highlightedIds)}
        stroke={getStroke('head', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('head', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('头部')}
      />
      {/* Neck */}
      <path
        id="neck"
        d="M92,62 C92,68 89,74 86,77 L114,77 C111,74 108,68 108,62 Z"
        fill={getFill('neck', highlightedIds)}
        stroke={getStroke('neck', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('neck', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('颈椎')}
      />
      {/* Shoulders */}
      <path
        id="shoulder-left"
        d="M86,77 C74,78 62,83 56,91 C54,94 54,99 56,102 C59,105 65,103 70,100 L86,89 Z"
        fill={getFill('shoulder-left', highlightedIds)}
        stroke={getStroke('shoulder-left', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('shoulder-left', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('左肩')}
      />
      <path
        id="shoulder-right"
        d="M114,77 C126,78 138,83 144,91 C146,94 146,99 144,102 C141,105 135,103 130,100 L114,89 Z"
        fill={getFill('shoulder-right', highlightedIds)}
        stroke={getStroke('shoulder-right', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('shoulder-right', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('右肩')}
      />
      {/* Chest */}
      <path
        id="chest"
        d="M86,89 L114,89 C122,95 128,105 128,120 C128,140 123,155 120,155 L80,155 C77,155 72,140 72,120 C72,105 78,95 86,89 Z"
        fill={getFill('chest', highlightedIds)}
        stroke={getStroke('chest', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('chest', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('胸部')}
      />
      {/* Arms */}
      <path
        id="arm-left"
        d="M56,102 C50,130 42,175 38,233 C37,239 40,243 46,243 C50,243 53,235 55,215 C58,175 64,135 70,100 Z"
        fill={getFill('arm-left', highlightedIds)}
        stroke={getStroke('arm-left', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('arm-left', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('左手')}
      />
      <path
        id="arm-right"
        d="M144,102 C150,130 158,175 162,233 C163,239 160,243 154,243 C150,243 147,235 145,215 C142,175 136,135 130,100 Z"
        fill={getFill('arm-right', highlightedIds)}
        stroke={getStroke('arm-right', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('arm-right', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('右手')}
      />
      {/* Waist */}
      <path
        id="lower-back"
        d="M80,155 L120,155 C118,170 121,183 123,193 L77,193 C79,183 82,170 80,155 Z"
        fill={getFill('lower-back', highlightedIds)}
        stroke={getStroke('lower-back', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('lower-back', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('腰部')}
      />
      {/* Hips */}
      <path
        id="hip-left"
        d="M77,193 L100,193 L100,220 L70,220 C68,210 72,200 77,193 Z"
        fill={getFill('hip-left', highlightedIds)}
        stroke={getStroke('hip-left', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('hip-left', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('髋部')}
      />
      <path
        id="hip-right"
        d="M100,193 L123,193 C128,200 132,210 130,220 L100,220 Z"
        fill={getFill('hip-right', highlightedIds)}
        stroke={getStroke('hip-right', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('hip-right', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('髋部')}
      />
      {/* Legs */}
      <path
        id="leg-left"
        d="M70,220 L85,220 C84,240 83,260 82,275 C81,290 80,310 78,330 C76,337 72,337 68,333 C64,310 64,290 66,275 C68,260 69,240 70,220 Z"
        fill={getFill('leg-left', highlightedIds)}
        stroke={getStroke('leg-left', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('leg-left', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('腿部')}
      />
      <path
        id="leg-right"
        d="M115,220 L130,220 C131,240 132,260 134,275 C136,290 136,310 132,333 C128,337 124,337 122,330 C120,310 119,290 118,275 C117,260 116,240 115,220 Z"
        fill={getFill('leg-right', highlightedIds)}
        stroke={getStroke('leg-right', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('leg-right', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('腿部')}
      />
      {/* Knees */}
      <ellipse
        id="knee-left"
        cx="73" cy="275" rx="8" ry="9"
        fill={getFill('knee-left', highlightedIds)}
        stroke={getStroke('knee-left', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('knee-left', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('左膝')}
      />
      <ellipse
        id="knee-right"
        cx="127" cy="275" rx="8" ry="9"
        fill={getFill('knee-right', highlightedIds)}
        stroke={getStroke('knee-right', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('knee-right', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('右膝')}
      />
      {/* Feet */}
      <path
        id="ankle-left"
        d="M68,333 L67,367 C67,371 62,373 56,373 L80,373 C82,373 84,369 82,367 L78,333 Z"
        fill={getFill('ankle-left', highlightedIds)}
        stroke={getStroke('ankle-left', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('ankle-left', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('脚踝')}
      />
      <path
        id="ankle-right"
        d="M122,330 L118,367 C116,369 118,373 120,373 L144,373 C138,373 133,371 133,367 L132,333 Z"
        fill={getFill('ankle-right', highlightedIds)}
        stroke={getStroke('ankle-right', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('ankle-right', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('脚踝')}
      />
    </svg>
  );
}

function SideViewSVG({ highlightedIds, onPartClick }: ViewSVGProps) {
  return (
    <svg viewBox="0 0 200 400" className="w-full h-auto">
      <defs>
        <linearGradient id="clay-gradient" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#FAF9F6" />
          <stop offset="100%" stopColor="#E5E1D7" />
        </linearGradient>
        <linearGradient id="pain-gradient" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#FDA48F" />
          <stop offset="100%" stopColor="#CD7B67" />
        </linearGradient>
        <filter id="glow-filter" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="2.5" result="blur" />
          <feComponentTransfer in="blur" result="glow1">
            <feFuncA type="linear" slope="0.4"/>
          </feComponentTransfer>
          <feMerge>
            <feMergeNode in="glow1" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>
      {/* Head */}
      <path
        id="head"
        d="M100,14 C111,14 119,23 119,35 C119,48 111,62 100,62 C89,62 81,48 81,35 C81,23 89,14 100,14 Z"
        fill={getFill('head', highlightedIds)}
        stroke={getStroke('head', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('head', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('头部')}
      />
      {/* Neck */}
      <path
        id="neck"
        d="M90,62 C90,68 87,74 85,77 L113,77 C111,74 108,68 108,62 Z"
        fill={getFill('neck', highlightedIds)}
        stroke={getStroke('neck', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('neck', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('颈椎')}
      />
      {/* Upper body (chest + back) */}
      <path
        id="chest"
        d="M88,77 C106,77 120,85 120,105 C120,125 113,140 110,155 L80,155 C78,140 80,115 84,90 Z"
        fill={getFill('chest', highlightedIds)}
        stroke={getStroke('chest', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('chest', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('胸部')}
      />
      <path
        id="back"
        d="M84,77 C80,90 78,115 80,155 L74,155 C72,130 76,95 84,77 Z"
        fill={getFill('back', highlightedIds)}
        stroke={getStroke('back', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('back', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('背部')}
      />
      {/* Lower back */}
      <path
        id="lower-back"
        d="M74,155 L110,155 C108,170 104,185 100,193 L72,193 C74,183 76,167 74,155 Z"
        fill={getFill('lower-back', highlightedIds)}
        stroke={getStroke('lower-back', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('lower-back', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('腰部')}
      />
      {/* Arm */}
      <path
        id="arm-left"
        d="M118,85 L138,145 L143,225 L128,225 L126,145 L113,90 Z"
        fill={getFill('arm-left', highlightedIds)}
        stroke={getStroke('arm-left', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('arm-left', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('手臂')}
      />
      {/* Hip + leg */}
      <path
        id="hip-left"
        d="M72,193 L100,193 C103,205 106,215 103,220 L70,220 C68,210 70,200 72,193 Z"
        fill={getFill('hip-left', highlightedIds)}
        stroke={getStroke('hip-left', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('hip-left', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('骨盆')}
      />
      <path
        id="leg-left"
        d="M70,220 L103,220 C100,240 97,260 94,275 C91,290 88,310 86,330 C84,335 80,335 78,333 L70,220 Z"
        fill={getFill('leg-left', highlightedIds)}
        stroke={getStroke('leg-left', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('leg-left', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('腿部')}
      />
      <ellipse
        id="knee-left"
        cx="94" cy="275" rx="10" ry="9"
        fill={getFill('knee-left', highlightedIds)}
        stroke={getStroke('knee-left', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('knee-left', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('膝盖')}
      />
      <path
        id="ankle-left"
        d="M78,333 L74,367 C72,369 70,373 66,373 L93,373 C95,373 96,369 94,367 L86,333 Z"
        fill={getFill('ankle-left', highlightedIds)}
        stroke={getStroke('ankle-left', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('ankle-left', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('脚踝')}
      />
    </svg>
  );
}

function BackViewSVG({ highlightedIds, onPartClick }: ViewSVGProps) {
  return (
    <svg viewBox="0 0 200 400" className="w-full h-auto">
      <defs>
        <linearGradient id="clay-gradient" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#FAF9F6" />
          <stop offset="100%" stopColor="#E5E1D7" />
        </linearGradient>
        <linearGradient id="pain-gradient" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#FDA48F" />
          <stop offset="100%" stopColor="#CD7B67" />
        </linearGradient>
        <filter id="glow-filter" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="2.5" result="blur" />
          <feComponentTransfer in="blur" result="glow1">
            <feFuncA type="linear" slope="0.4"/>
          </feComponentTransfer>
          <feMerge>
            <feMergeNode in="glow1" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>
      {/* Head */}
      <path
        id="head"
        d="M100,14 C111,14 121,23 121,35 C121,48 111,62 100,62 C89,62 79,48 79,35 C79,23 89,14 100,14 Z"
        fill={getFill('head', highlightedIds)}
        stroke={getStroke('head', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('head', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('头部')}
      />
      {/* Neck */}
      <path
        id="neck"
        d="M92,62 C92,68 89,74 86,77 L114,77 C111,74 108,68 108,62 Z"
        fill={getFill('neck', highlightedIds)}
        stroke={getStroke('neck', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('neck', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('颈椎')}
      />
      {/* Back */}
      <path
        id="back"
        d="M86,89 L114,89 C122,95 128,105 128,120 C128,140 123,155 120,155 L80,155 C77,155 72,140 72,120 C72,105 78,95 86,89 Z"
        fill={getFill('back', highlightedIds)}
        stroke={getStroke('back', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('back', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('背部')}
      />
      {/* Shoulders */}
      <path
        id="shoulder-left"
        d="M86,77 C74,78 62,83 56,91 C54,94 54,99 56,102 C59,105 65,103 70,100 L86,89 Z"
        fill={getFill('shoulder-left', highlightedIds)}
        stroke={getStroke('shoulder-left', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('shoulder-left', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('左肩')}
      />
      <path
        id="shoulder-right"
        d="M114,77 C126,78 138,83 144,91 C146,94 146,99 144,102 C141,105 135,103 130,100 L114,89 Z"
        fill={getFill('shoulder-right', highlightedIds)}
        stroke={getStroke('shoulder-right', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('shoulder-right', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('右肩')}
      />
      {/* Lower back */}
      <path
        id="lower-back"
        d="M80,155 L120,155 C118,170 121,183 123,193 L77,193 C79,183 82,170 80,155 Z"
        fill={getFill('lower-back', highlightedIds)}
        stroke={getStroke('lower-back', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('lower-back', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('腰部')}
      />
      {/* Arms */}
      <path
        id="arm-left"
        d="M56,102 C50,130 42,175 38,233 C37,239 40,243 46,243 C50,243 53,235 55,215 C58,175 64,135 70,100 Z"
        fill={getFill('arm-left', highlightedIds)}
        stroke={getStroke('arm-left', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('arm-left', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('左手')}
      />
      <path
        id="arm-right"
        d="M144,102 C150,130 158,175 162,233 C163,239 160,243 154,243 C150,243 147,235 145,215 C142,175 136,135 130,100 Z"
        fill={getFill('arm-right', highlightedIds)}
        stroke={getStroke('arm-right', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('arm-right', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('右手')}
      />
      {/* Hips */}
      <path
        id="hip-left"
        d="M77,193 L100,193 L100,220 L70,220 C68,210 72,200 77,193 Z"
        fill={getFill('hip-left', highlightedIds)}
        stroke={getStroke('hip-left', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('hip-left', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('髋部')}
      />
      <path
        id="hip-right"
        d="M100,193 L123,193 C128,200 132,210 130,220 L100,220 Z"
        fill={getFill('hip-right', highlightedIds)}
        stroke={getStroke('hip-right', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('hip-right', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('髋部')}
      />
      {/* Legs */}
      <path
        id="leg-left"
        d="M70,220 L85,220 C84,240 83,260 82,275 C81,290 80,310 78,330 C76,337 72,337 68,333 C64,310 64,290 66,275 C68,260 69,240 70,220 Z"
        fill={getFill('leg-left', highlightedIds)}
        stroke={getStroke('leg-left', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('leg-left', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('腿部')}
      />
      <path
        id="leg-right"
        d="M115,220 L130,220 C131,240 132,260 134,275 C136,290 136,310 132,333 C128,337 124,337 122,330 C120,310 119,290 118,275 C117,260 116,240 115,220 Z"
        fill={getFill('leg-right', highlightedIds)}
        stroke={getStroke('leg-right', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('leg-right', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('腿部')}
      />
      {/* Knees */}
      <ellipse
        id="knee-left"
        cx="73" cy="275" rx="8" ry="9"
        fill={getFill('knee-left', highlightedIds)}
        stroke={getStroke('knee-left', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('knee-left', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('左膝')}
      />
      <ellipse
        id="knee-right"
        cx="127" cy="275" rx="8" ry="9"
        fill={getFill('knee-right', highlightedIds)}
        stroke={getStroke('knee-right', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('knee-right', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('右膝')}
      />
      {/* Feet */}
      <path
        id="ankle-left"
        d="M68,333 L67,367 C67,371 62,373 56,373 L80,373 C82,373 84,369 82,367 L78,333 Z"
        fill={getFill('ankle-left', highlightedIds)}
        stroke={getStroke('ankle-left', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('ankle-left', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('脚踝')}
      />
      <path
        id="ankle-right"
        d="M122,330 L118,367 C116,369 118,373 120,373 L144,373 C138,373 133,371 133,367 L132,333 Z"
        fill={getFill('ankle-right', highlightedIds)}
        stroke={getStroke('ankle-right', highlightedIds)}
        strokeWidth="1"
        filter={getFilter('ankle-right', highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick('脚踝')}
      />
    </svg>
  );
}
