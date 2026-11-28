import { Turnstile, type TurnstileInstance } from "@marsidev/react-turnstile";
import { useEffect, useRef, useState, type Ref } from "react";

// Supabase verifies the CAPTCHA server-side with the Turnstile secret held in
// the Supabase dashboard. The browser only receives a public site key.
export const turnstileSiteKey =
  import.meta.env.VITE_TURNSTILE_SITE_KEY?.trim() ?? "";

// If neither the widget nor an error callback shows up within this window,
// the Turnstile script never loaded (CSP, network filter, outage). That must
// surface as a visible failure with a retry — never as a silent gap that the
// form then reports as "请完成人机验证" for a widget that never appeared.
const WIDGET_LOAD_TIMEOUT_MS = 10_000;

// The rendered Turnstile box is 72px tall at the auth forms' content width
// (measured in a real browser). The widget and the load-failure message share
// this fixed-height slot so the form's total height never changes when the
// captcha state flips: a load failure must not shove the fields and the
// submit button around, and the failure copy + retry live inside the slot
// instead of adding new vertical height to the page.
export const CAPTCHA_SLOT_CLASS = "mt-5 h-[72px]";

export function AuthCaptcha({
  captchaRef,
  onLoadFailureChange,
}: {
  captchaRef: Ref<TurnstileInstance>;
  onLoadFailureChange?: (failed: boolean) => void;
}) {
  const [attempt, setAttempt] = useState(0);
  const [failed, setFailed] = useState(false);
  const loadedRef = useRef(false);
  const failureCallbackRef = useRef(onLoadFailureChange);

  useEffect(() => {
    failureCallbackRef.current = onLoadFailureChange;
  });

  useEffect(() => {
    if (!turnstileSiteKey) return undefined;
    const timer = setTimeout(() => {
      if (!loadedRef.current) {
        setFailed(true);
        failureCallbackRef.current?.(true);
      }
    }, WIDGET_LOAD_TIMEOUT_MS);
    return () => {
      clearTimeout(timer);
    };
  }, [attempt]);

  const markLoaded = () => {
    loadedRef.current = true;
    if (failed) {
      setFailed(false);
      failureCallbackRef.current?.(false);
    }
  };

  const markFailed = () => {
    loadedRef.current = true;
    setFailed(true);
    failureCallbackRef.current?.(true);
  };

  const retry = () => {
    loadedRef.current = false;
    setFailed(false);
    failureCallbackRef.current?.(false);
    setAttempt((value) => value + 1);
  };

  if (!turnstileSiteKey) {
    // A build without a site key never demands a captcha token (both forms
    // gate every captcha requirement on the key being configured), so this
    // note is a developer-facing signal, not a user blocker — and production
    // builds always carry a key. It must still not consume in-flow height:
    // the auth sheets follow a fixed ink board with zero spare vertical
    // room (see the auth-ink-pages E2E overflow assertion), so unlike the
    // widget/failure slot the note floats at the slot's position and
    // leaves the form's height untouched.
    return (
      <div className="relative h-0">
        <p
          className="absolute inset-x-0 top-0 text-[13px] leading-[1.2] text-[var(--ui-muted)]"
          data-testid="auth-turnstile-unconfigured"
        >
          人机验证未启用，本次提交不会进行人机验证。
        </p>
      </div>
    );
  }

  if (failed) {
    return (
      <div
        className={`${CAPTCHA_SLOT_CLASS} flex flex-col justify-center`}
        data-testid="auth-turnstile-failed"
      >
        <p
          role="alert"
          className="text-[13px] leading-[1.2] font-semibold text-[var(--ui-danger)]"
        >
          人机验证加载失败，可能是网络波动或浏览器拦截导致的。
        </p>
        <button
          type="button"
          onClick={retry}
          className="mt-2 self-start text-[13px] leading-[1.2] font-semibold text-[var(--ui-primary)] underline underline-offset-4"
        >
          重新加载验证
        </button>
      </div>
    );
  }

  return (
    <div className={CAPTCHA_SLOT_CLASS} data-testid="auth-turnstile">
      <Turnstile
        key={attempt}
        ref={captchaRef}
        siteKey={turnstileSiteKey}
        options={{ theme: "light", refreshExpired: "auto" }}
        onWidgetLoad={markLoaded}
        onSuccess={markLoaded}
        onError={markFailed}
        scriptOptions={{ onError: markFailed }}
      />
    </div>
  );
}
