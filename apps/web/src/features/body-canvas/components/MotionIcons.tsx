import type { ComponentType, SVGProps } from "react";
import type { MotionId } from "../model/bodyCanvas";

/**
 * Penpot Home motion ribbon glyphs: four 24px stick-figure poses drawn in the
 * board's 1.7 stroke. Decorative only; each ribbon button carries its label.
 */
function Glyph({ children, ...props }: SVGProps<SVGSVGElement>) {
  return (
    <svg
      width={22}
      height={22}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.7}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
      {...props}
    >
      {children}
    </svg>
  );
}

export function MotionStandIcon(props: SVGProps<SVGSVGElement>) {
  return (
    <Glyph {...props}>
      <circle cx="12" cy="4" r="1.7" fill="currentColor" stroke="none" />
      <path d="M12 7v7M12 9l-3 4M12 9l3 4M12 14l-2.5 7M12 14l2.5 7" />
    </Glyph>
  );
}

export function MotionRunIcon(props: SVGProps<SVGSVGElement>) {
  return (
    <Glyph {...props}>
      <circle cx="15" cy="4" r="1.7" fill="currentColor" stroke="none" />
      <path d="M14 7.5 11.5 13M14 7.5l3 3 2.5-.5M14 7.5 10 9l-1.5 2.5M11.5 13l3.5 3-1 5M11.5 13 9 17l-4 .5" />
    </Glyph>
  );
}

export function MotionJumpIcon(props: SVGProps<SVGSVGElement>) {
  return (
    <Glyph {...props}>
      <circle cx="12" cy="4.5" r="1.7" fill="currentColor" stroke="none" />
      <path d="M8.5 3.5 12 8l3.5-4.5M12 8v6M12 14l-2 4.5M12 14l2 4.5M9 21h6" />
    </Glyph>
  );
}

export function MotionSitIcon(props: SVGProps<SVGSVGElement>) {
  return (
    <Glyph {...props}>
      <circle cx="10" cy="4" r="1.7" fill="currentColor" stroke="none" />
      <path d="M10 7v6.5h5.5V20M10 9.5l4 2M7 13v8M7 16h3" />
    </Glyph>
  );
}

export const MOTION_ICONS: Partial<
  Record<MotionId, ComponentType<SVGProps<SVGSVGElement>>>
> = {
  stand: MotionStandIcon,
  run: MotionRunIcon,
  jump: MotionJumpIcon,
  sit: MotionSitIcon,
};
