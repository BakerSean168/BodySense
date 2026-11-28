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
    return (
      <p
        className="mt-5 text-[13px] leading-[1.2] text-[var(--ui-muted)]"
        data-testid="auth-turnstile-unconfigured"
      >
        人机验证未启用，本次提交不会进行人机验证。
      </p>
    );
  }

  if (failed) {
    return (
      <div className="mt-5" data-testid="auth-turnstile-failed">
        <p
          role="alert"
          className="text-[13px] leading-[1.2] font-semibold text-[var(--ui-danger)]"
        >
          人机验证加载失败，可能是网络波动或浏览器拦截导致的。
        </p>
        <button
          type="button"
          onClick={retry}
          className="mt-2 text-[13px] leading-[1.2] font-semibold text-[var(--ui-primary)] underline underline-offset-4"
        >
          重新加载验证
        </button>
      </div>
    );
  }

  return (
    <div className="mt-5" data-testid="auth-turnstile">
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
