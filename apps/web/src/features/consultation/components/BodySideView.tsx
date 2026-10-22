import {
  getFill,
  getStroke,
  getFilter,
  type ViewSVGProps,
} from "./bodyVisualizationSvg";

export function SideViewSVG({ highlightedIds, onPartClick }: ViewSVGProps) {
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
            <feFuncA type="linear" slope="0.4" />
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
        fill={getFill("head", highlightedIds)}
        stroke={getStroke("head", highlightedIds)}
        strokeWidth="1"
        filter={getFilter("head", highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick("头部")}
      />
      {/* Neck */}
      <path
        id="neck"
        d="M90,62 C90,68 87,74 85,77 L113,77 C111,74 108,68 108,62 Z"
        fill={getFill("neck", highlightedIds)}
        stroke={getStroke("neck", highlightedIds)}
        strokeWidth="1"
        filter={getFilter("neck", highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick("颈椎")}
      />
      {/* Upper body (chest + back) */}
      <path
        id="chest"
        d="M88,77 C106,77 120,85 120,105 C120,125 113,140 110,155 L80,155 C78,140 80,115 84,90 Z"
        fill={getFill("chest", highlightedIds)}
        stroke={getStroke("chest", highlightedIds)}
        strokeWidth="1"
        filter={getFilter("chest", highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick("胸部")}
      />
      <path
        id="back"
        d="M84,77 C80,90 78,115 80,155 L74,155 C72,130 76,95 84,77 Z"
        fill={getFill("back", highlightedIds)}
        stroke={getStroke("back", highlightedIds)}
        strokeWidth="1"
        filter={getFilter("back", highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick("背部")}
      />
      {/* Lower back */}
      <path
        id="lower-back"
        d="M74,155 L110,155 C108,170 104,185 100,193 L72,193 C74,183 76,167 74,155 Z"
        fill={getFill("lower-back", highlightedIds)}
        stroke={getStroke("lower-back", highlightedIds)}
        strokeWidth="1"
        filter={getFilter("lower-back", highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick("腰部")}
      />
      {/* Arm */}
      <path
        id="arm-left"
        d="M118,85 L138,145 L143,225 L128,225 L126,145 L113,90 Z"
        fill={getFill("arm-left", highlightedIds)}
        stroke={getStroke("arm-left", highlightedIds)}
        strokeWidth="1"
        filter={getFilter("arm-left", highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick("手臂")}
      />
      {/* Hip + leg */}
      <path
        id="hip-left"
        d="M72,193 L100,193 C103,205 106,215 103,220 L70,220 C68,210 70,200 72,193 Z"
        fill={getFill("hip-left", highlightedIds)}
        stroke={getStroke("hip-left", highlightedIds)}
        strokeWidth="1"
        filter={getFilter("hip-left", highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick("骨盆")}
      />
      <path
        id="leg-left"
        d="M70,220 L103,220 C100,240 97,260 94,275 C91,290 88,310 86,330 C84,335 80,335 78,333 L70,220 Z"
        fill={getFill("leg-left", highlightedIds)}
        stroke={getStroke("leg-left", highlightedIds)}
        strokeWidth="1"
        filter={getFilter("leg-left", highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick("腿部")}
      />
      <ellipse
        id="knee-left"
        cx="94"
        cy="275"
        rx="10"
        ry="9"
        fill={getFill("knee-left", highlightedIds)}
        stroke={getStroke("knee-left", highlightedIds)}
        strokeWidth="1"
        filter={getFilter("knee-left", highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick("膝盖")}
      />
      <path
        id="ankle-left"
        d="M78,333 L74,367 C72,369 70,373 66,373 L93,373 C95,373 96,369 94,367 L86,333 Z"
        fill={getFill("ankle-left", highlightedIds)}
        stroke={getStroke("ankle-left", highlightedIds)}
        strokeWidth="1"
        filter={getFilter("ankle-left", highlightedIds)}
        className="cursor-pointer transition-all duration-300 hover:opacity-90 hover:stroke-[#2a4b3a]"
        onClick={() => onPartClick("脚踝")}
      />
    </svg>
  );
}
