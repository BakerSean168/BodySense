import type { SVGProps } from "react";

/**
 * Penpot V2 "Icon / Dock" glyphs (24 x 24, 1.8 stroke): a speech bubble with
 * text lines, a clipboard with a pulse, a level dumbbell and a person. They
 * inherit the Dock item colour so pressed/hover states keep working.
 */
type DockIconProps = SVGProps<SVGSVGElement> & { size?: number };

function DockGlyph({ size = 24, d, ...props }: DockIconProps & { d: string }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.8}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
      {...props}
    >
      <path d={d} />
    </svg>
  );
}

export function DockChatIcon(props: DockIconProps) {
  return (
    <DockGlyph
      {...props}
      d="M8 5H16A4 4 0 0 1 20 9V12A4 4 0 0 1 16 16H11.5L7.8 19.4V16H8A4 4 0 0 1 4 12V9A4 4 0 0 1 8 5Z M8.5 9.4H15.5 M8.5 12.3H12.8"
    />
  );
}

export function DockDiagnosisIcon(props: DockIconProps) {
  return (
    <DockGlyph
      {...props}
      d="M8.6 4.6H7.2A2.6 2.6 0 0 0 4.6 7.2V18.4A2.6 2.6 0 0 0 7.2 21H16.8A2.6 2.6 0 0 0 19.4 18.4V7.2A2.6 2.6 0 0 0 16.8 4.6H15.4 M9.8 3H14.2A1.2 1.2 0 0 1 15.4 4.2V4.8A1.2 1.2 0 0 1 14.2 6H9.8A1.2 1.2 0 0 1 8.6 4.8V4.2A1.2 1.2 0 0 1 9.8 3Z M7.6 13.6H9.7L11.1 10.6L12.9 16.2L14.3 13.6H16.4"
    />
  );
}

export function DockTrainingIcon(props: DockIconProps) {
  return (
    <DockGlyph
      {...props}
      d="M8.8 12H15.2 M6.2 8.3A1.3 1.3 0 0 1 8.8 8.3V15.7A1.3 1.3 0 0 1 6.2 15.7Z M15.2 8.3A1.3 1.3 0 0 1 17.8 8.3V15.7A1.3 1.3 0 0 1 15.2 15.7Z M3.6 10.2V13.8 M20.4 10.2V13.8"
    />
  );
}

export function DockPersonIcon(props: DockIconProps) {
  return (
    <DockGlyph
      {...props}
      d="M15.5 8A3.5 3.5 0 1 1 8.5 8A3.5 3.5 0 1 1 15.5 8Z M5 20.5C5 16.9 8.1 14.5 12 14.5C15.9 14.5 19 16.9 19 20.5"
    />
  );
}
