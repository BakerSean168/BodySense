/**
 * AuthInkLayout: shared shell for the Penpot "01 Auth" ink proposal boards
 * (Desktop / Auth / Sign in · Proposal, Register · Proposal and their 390 mobile twins).
 *
 * Desktop (lg+): mint stage with the seated ink meridian figure on the left and a
 * rice-paper card on the right. Mobile: figure on top and a rice-paper sheet below.
 * The illustration and brush textures are exported straight from Penpot (see ../assets).
 */
import type { ReactNode } from "react";
import inkFigure from "../assets/ink-figure-meridians.webp";
import inkCornerDesktop from "../assets/ink-corner-d.webp";
import inkCornerMobile from "../assets/ink-corner-m.webp";
import logoCompact from "../assets/logo-compact.svg";

interface AuthInkLayoutProps {
  title: string;
  testId: string;
  children: ReactNode;
  footer: ReactNode;
  /**
   * Register carries one more field than the board (confirm password), so on mobile
   * its sheet trims the bottom padding (51 → 40) and the gap above the bottom link
   * (40 → 28) to keep 390×844 free of scroll. Desktop spacing is unchanged.
   */
  compactMobile?: boolean;
}

export function AuthInkLayout({
  title,
  testId,
  children,
  footer,
  compactMobile = false,
}: AuthInkLayoutProps) {
  return (
    <div
      data-testid={testId}
      className="relative min-h-screen overflow-hidden bg-[#E6EEE5] text-[#182A21]"
    >
      <div className="relative mx-auto flex min-h-screen max-w-[1440px] flex-col lg:min-h-[max(100vh,760px)] lg:flex-row lg:items-center lg:justify-between lg:gap-[35px] lg:pr-[88px] lg:pl-[clamp(40px,12.08vw,174px)]">
        {/* Brand: single mark, top-left */}
        <div className="absolute top-5 left-5 z-20 flex items-center gap-2 lg:top-9 lg:left-10 lg:gap-3">
          <img
            src={logoCompact}
            alt=""
            aria-hidden="true"
            className="h-8 w-8 rounded-full lg:h-10 lg:w-10"
          />
          <span className="text-base leading-[1.2] font-bold lg:text-xl">
            体悟
          </span>
        </div>

        {/* Stage: backdrop disc + ink figure */}
        <div
          aria-hidden="true"
          data-testid="auth-ink-stage"
          className="relative mx-auto mt-[74px] flex h-[302px] w-[323px] shrink-0 items-center justify-center rounded-[50%] bg-[#F0F6EF] lg:mx-0 lg:mt-0 lg:aspect-[691/646] lg:h-auto lg:w-auto lg:max-w-[691px] lg:min-w-0 lg:flex-1 lg:-translate-y-[19px]"
        >
          <img
            src={inkFigure}
            alt=""
            draggable={false}
            className="auth-ink-figure h-[74%] w-auto translate-y-[2%] select-none"
          />
        </div>

        <h1 className="absolute bottom-[68px] left-10 hidden text-[26px] leading-[1.2] font-bold lg:block">
          先看见身体，再决定怎么练。
        </h1>

        {/* Rice-paper card (desktop) / sheet (mobile) */}
        <main
          className={`relative z-10 mt-2 flex min-h-[460px] w-full flex-1 flex-col rounded-t-[24px] border border-b-0 border-[#E4E0D3] bg-[#FBFAF5] px-6 pt-8 ${compactMobile ? "pb-10" : "pb-[51px]"} shadow-[0_-6px_30px_rgba(29,42,34,0.08)] lg:mt-0 lg:min-h-[500px] lg:w-[452px] lg:flex-none lg:rounded-[14px] lg:border-b lg:px-10 lg:pt-12 lg:pb-[43px] lg:shadow-[0_18px_48px_rgba(29,42,34,0.08)]`}
        >
          <div
            aria-hidden="true"
            className="pointer-events-none absolute inset-0 rounded-[inherit] bg-[length:390px_460px] bg-top-right bg-no-repeat lg:hidden"
            style={{ backgroundImage: `url(${inkCornerMobile})` }}
          />
          <div
            aria-hidden="true"
            className="pointer-events-none absolute inset-0 hidden rounded-[inherit] bg-[length:452px_500px] bg-top-right bg-no-repeat lg:block"
            style={{ backgroundImage: `url(${inkCornerDesktop})` }}
          />
          <h2 className="relative flex items-start text-2xl leading-[1.2] font-bold lg:text-[28px]">
            {title}
            <span
              aria-hidden="true"
              data-testid="auth-dantian-dot"
              className="auth-dantian-dot mt-[15px] ml-2 inline-block h-2 w-2 rounded-full bg-[#F6C95A] shadow-[0_0_8px_rgba(246,201,90,0.7)] lg:mt-5"
            />
          </h2>
          <div className="relative mt-[39px] lg:mt-[50px]">{children}</div>
          <p
            className={`relative mt-auto ${compactMobile ? "pt-7" : "pt-10"} lg:pt-10 text-center text-sm leading-[1.2] text-[#718075]`}
          >
            {footer}
          </p>
        </main>
      </div>
    </div>
  );
}
