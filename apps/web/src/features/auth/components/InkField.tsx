/**
 * Ink auth primitives from the Penpot "01 Auth" boards: label-less underline field
 * with a leading Lucide icon (mail / lock), hairline brush underline, optional eye
 * toggle, inline error; plus the wide brush-stroke primary button.
 * Inputs keep an accessible name via aria-label (no visible labels by design).
 */
import {
  forwardRef,
  useState,
  type InputHTMLAttributes,
  type ReactNode,
} from "react";
import { Eye, EyeOff, Lock, Mail } from "lucide-react";
import inkLine from "../assets/ink-line.webp";
import inkButton from "../assets/ink-btn.webp";

interface InkFieldProps extends Omit<
  InputHTMLAttributes<HTMLInputElement>,
  "type"
> {
  id: string;
  kind: "email" | "password";
  accessibleLabel: string;
  error?: string;
  errorExtra?: ReactNode;
  /** Noun used by the eye toggle's accessible name ("显示密码" / "隐藏确认密码"). */
  visibilityLabel?: string;
}

export const InkField = forwardRef<HTMLInputElement, InkFieldProps>(
  (
    {
      id,
      kind,
      accessibleLabel,
      error,
      errorExtra,
      visibilityLabel = "密码",
      className = "",
      ...props
    },
    ref,
  ) => {
    const [visible, setVisible] = useState(false);
    const Icon = kind === "email" ? Mail : Lock;
    const errorId = `${id}-error`;
    const type = kind === "email" ? "email" : visible ? "text" : "password";

    return (
      <div
        className={`auth-ink-field ${className}`}
        data-invalid={error ? "true" : undefined}
      >
        <div className="group relative flex h-6 items-center">
          <Icon
            aria-hidden="true"
            className="h-6 w-6 shrink-0 text-[#7C8C81] transition-colors group-focus-within:text-[#2C694F]"
            strokeWidth={1.8}
          />
          <input
            id={id}
            ref={ref}
            type={type}
            aria-label={accessibleLabel}
            aria-invalid={error ? true : undefined}
            aria-describedby={error ? errorId : undefined}
            className="h-6 min-w-0 flex-1 border-0 bg-transparent pl-[10px] text-[15px] leading-[1.2] text-[#182A21] outline-none placeholder:text-[#9AA89E]"
            {...props}
          />
          {kind === "password" && (
            <button
              type="button"
              onClick={() => setVisible((v) => !v)}
              aria-label={`${visible ? "隐藏" : "显示"}${visibilityLabel}`}
              aria-pressed={visible}
              aria-controls={id}
              data-testid={`${id}-visibility`}
              className="-mr-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded text-[#7C8C81] hover:text-[#2C694F] focus-visible:ring-2 focus-visible:ring-[#2C694F] focus-visible:ring-offset-2 focus-visible:ring-offset-[#FBFAF5] focus-visible:outline-none"
            >
              {visible ? (
                <Eye aria-hidden="true" className="h-6 w-6" strokeWidth={1.8} />
              ) : (
                <EyeOff
                  aria-hidden="true"
                  className="h-6 w-6"
                  strokeWidth={1.8}
                />
              )}
            </button>
          )}
        </div>
        <div aria-hidden="true" className="relative mt-[5px] h-[7px]">
          <div
            className="absolute inset-0 bg-[length:100%_100%] bg-no-repeat"
            style={{ backgroundImage: `url(${inkLine})` }}
          />
          <div
            className={`auth-ink-underline-focus absolute top-[3px] right-0 left-0 h-px ${
              error ? "bg-[#9A3F35]" : "bg-[#2C694F]"
            }`}
          />
        </div>
        {error && (
          <p
            id={errorId}
            role="alert"
            className="mt-2 text-[13px] leading-[1.2] font-semibold text-[#9A3F35]"
          >
            {error}
            {errorExtra}
          </p>
        )}
      </div>
    );
  },
);
InkField.displayName = "InkField";

interface InkButtonProps {
  children: ReactNode;
  isLoading?: boolean;
  loadingLabel: string;
  testId?: string;
}

export function InkButton({
  children,
  isLoading,
  loadingLabel,
  testId,
}: InkButtonProps) {
  return (
    <button
      type="submit"
      disabled={isLoading}
      aria-busy={isLoading || undefined}
      data-testid={testId}
      className="auth-ink-button relative -mx-[18px] flex h-[57px] w-[calc(100%+36px)] items-center justify-center bg-[length:100%_100%] bg-no-repeat text-base leading-[1.2] font-bold tracking-[4px] text-white focus-visible:outline-none disabled:cursor-wait disabled:opacity-85 lg:h-[61px]"
      style={{ backgroundImage: `url(${inkButton})` }}
    >
      <span className="pl-[4px]">{isLoading ? loadingLabel : children}</span>
    </button>
  );
}
