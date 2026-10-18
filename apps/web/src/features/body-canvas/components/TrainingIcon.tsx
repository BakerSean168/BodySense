import type { SVGProps } from "react";

/** Penpot "Icon / Training": a level dumbbell (lucide's is diagonal). */
export function TrainingIcon({ size = 18, ...props }: SVGProps<SVGSVGElement> & { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      strokeLinecap="round"
      strokeLinejoin="round"
      {...props}
    >
      <path d="M3 10v4M6.5 7.5v9M17.5 7.5v9M21 10v4M6.5 12h11" />
    </svg>
  );
}
