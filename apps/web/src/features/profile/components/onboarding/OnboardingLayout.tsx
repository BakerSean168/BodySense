/**
 * OnboardingLayout: shell for the Penpot "02 Onboarding" boards (Onboarding / Desktop /
 * Step 01 … 16, 1440×900). White page, 64px header (back ‹, BodySense, section, n/16),
 * 6px forest-green progress bar, one 620px question column and an optional pill footer.
 * Penpot has no 390 boards; mobile keeps the same parts with 16/20px insets and a footer
 * pinned to the bottom of the screen.
 */
import { useEffect, useRef, type ReactNode } from "react";

interface OnboardingLayoutProps {
  stepIndex: number;
  totalSteps: number;
  section: string;
  title: string;
  subtitle: string;
  onBack: () => void;
  children: ReactNode;
  /** Footer contents (继续 / 完成 button or a quiet 跳过); none on choose-to-advance steps. */
  footer?: ReactNode;
}

export function OnboardingLayout({
  stepIndex,
  totalSteps,
  section,
  title,
  subtitle,
  onBack,
  children,
  footer,
}: OnboardingLayoutProps) {
  const headingRef = useRef<HTMLHeadingElement>(null);
  const step = stepIndex + 1;
  const isFirst = stepIndex === 0;

  // Move focus to the new question so screen readers announce each step.
  useEffect(() => {
    if (stepIndex > 0) headingRef.current?.focus({ preventScroll: true });
  }, [stepIndex]);

  return (
    <div
      data-testid="onboarding-page"
      data-step={step}
      className="flex min-h-[100dvh] flex-col bg-white text-[#1A1A1A]"
    >
      <header className="mx-4 flex h-14 shrink-0 items-center lg:mx-10 lg:h-16">
        <div className="flex min-w-0 flex-1 items-center gap-2 lg:gap-0">
          {!isFirst && (
            <button
              type="button"
              onClick={onBack}
              aria-label="上一步"
              className="-ml-2 flex h-9 w-9 items-center justify-center rounded-full text-[#1A1A1A] hover:bg-[#F7F7F7] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#2C694F] lg:-ml-[10px] lg:h-8 lg:w-8"
            >
              <svg
                aria-hidden="true"
                viewBox="0 0 16 16"
                className="h-4 w-4"
                fill="none"
                stroke="currentColor"
                strokeWidth={1.8}
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <path d="M10 3.5 5.5 8l4.5 4.5" />
              </svg>
            </button>
          )}
          <span
            className={`text-[15px] leading-5 font-bold ${isFirst ? "lg:ml-6" : ""}`}
          >
            BodySense
          </span>
        </div>
        <span className="text-[13px] leading-[18px] text-[#6B7280]">
          {section}
        </span>
        <span className="flex flex-1 justify-end text-[13px] leading-[18px] text-[#6B7280] tabular-nums">
          {step}/{totalSteps}
        </span>
      </header>

      <div
        role="progressbar"
        aria-label="资料填写进度"
        aria-valuemin={1}
        aria-valuemax={totalSteps}
        aria-valuenow={step}
        aria-valuetext={`第 ${step} 步，共 ${totalSteps} 步`}
        className="mx-4 mt-1 h-1.5 shrink-0 overflow-hidden rounded-[3px] bg-[#E5E7EB] lg:mx-10 lg:mt-2"
      >
        <div
          data-testid="onboarding-progress-fill"
          className="h-full rounded-[3px] bg-[#2C694F] motion-safe:transition-[width] motion-safe:duration-300"
          style={{ width: `${(step / totalSteps) * 100}%` }}
        />
      </div>

      <main className="mx-auto flex w-full max-w-[660px] flex-1 flex-col px-5 pt-8 lg:px-5 lg:pt-9">
        <h1
          ref={headingRef}
          tabIndex={-1}
          id="onboarding-question"
          className="text-[22px] leading-[30px] font-bold outline-none lg:text-2xl lg:leading-[33px]"
        >
          {title}
        </h1>
        <p
          id="onboarding-question-hint"
          className="mt-2 text-sm leading-[19px] text-[#6B7280]"
        >
          {subtitle}
        </p>
        <div className="mt-6 lg:mt-[26px]">{children}</div>

        {footer && (
          <div
            data-testid="onboarding-footer"
            className="sticky bottom-0 mt-auto bg-white pt-6 pb-[max(24px,env(safe-area-inset-bottom))] lg:static lg:pt-0 lg:pb-[min(192px,21.33vh)]"
          >
            <div className="mt-6 border-t border-[#E5E7EB] pt-3 lg:mt-10">
              {footer}
            </div>
          </div>
        )}
      </main>
    </div>
  );
}

interface FooterButtonProps {
  children: ReactNode;
  onClick: () => void;
  disabled?: boolean;
}

export function OnboardingPrimaryButton({
  children,
  onClick,
  disabled,
}: FooterButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="h-14 w-full rounded-[28px] bg-[#2C694F] text-base font-bold text-white hover:bg-[#245A43] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#2C694F] disabled:cursor-not-allowed disabled:opacity-40"
    >
      {children}
    </button>
  );
}

export function OnboardingSkipButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="h-14 w-full rounded-[28px] text-[15px] text-[#6B7280] hover:bg-[#F7F7F7] hover:text-[#1A1A1A] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#2C694F]"
    >
      跳过
    </button>
  );
}
