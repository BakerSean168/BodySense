import {
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
  type Ref,
} from "react";
import { apiUrl } from "@/lib/api-url";

// ALTCHA replaces Cloudflare Turnstile. The widget solves a proof-of-work
// challenge issued by our own Go API; the solved payload travels to
// Supabase as signUp metadata (options.data.altcha) and the API's Before
// User Created hook verifies it server-side before any account exists.
// Nothing is configured at build time and nothing is verified in the
// browser: if the widget cannot load or the challenge cannot be fetched,
// that must surface as a visible failure with a retry — never as a silent
// gap that the form then reports as "请完成人机验证" for a widget that
// never appeared.
const WIDGET_LOAD_TIMEOUT_MS = 10_000;

// The widget box and the load-failure message share this fixed-height
// slot so the form's total height never changes when the captcha state
// flips: a load failure must not shove the fields and the submit button
// around, and the failure copy + retry live inside the slot instead of
// adding new vertical height to the page. 72px is the slot the auth ink
// board was measured against with the previous widget.
export const CAPTCHA_SLOT_CLASS = "mt-5 h-[72px]";

export interface AuthCaptchaHandle {
  /** The solved ALTCHA payload, or null while unsolved/failed. */
  getProof: () => string | null;
  /** Resolves with the payload once solved, or null on failure/timeout. */
  waitForProof: (timeoutMs: number) => Promise<string | null>;
  /** Discards the current proof and restarts the widget. */
  reset: () => void;
}

type AltchaWidgetElement = HTMLElement & {
  reset?: () => void;
};

export function AuthCaptcha({
  captchaRef,
  onLoadFailureChange,
}: {
  captchaRef: Ref<AuthCaptchaHandle>;
  onLoadFailureChange?: (failed: boolean) => void;
}) {
  const [widgetEl, setWidgetEl] = useState<AltchaWidgetElement | null>(null);
  const proofRef = useRef<string | null>(null);
  const waitersRef = useRef<Array<(proof: string | null) => void>>([]);
  const interactedRef = useRef(false);
  const [failed, setFailed] = useState(false);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const failureCallbackRef = useRef(onLoadFailureChange);

  useEffect(() => {
    failureCallbackRef.current = onLoadFailureChange;
  });

  const settleWaiters = useCallback((proof: string | null) => {
    const waiters = waitersRef.current;
    waitersRef.current = [];
    waiters.forEach((resolve) => resolve(proof));
  }, []);

  const markFailed = useCallback(() => {
    interactedRef.current = true;
    setFailed(true);
    failureCallbackRef.current?.(true);
    settleWaiters(null);
  }, [settleWaiters]);

  // The widget module is loaded on demand: at ~220KB it would push the
  // entry chunk over the production bundle budget, and only the auth
  // pages ever need it. The <altcha-widget> element below upgrades in
  // place once the module registers it; a failed load lands in the same
  // visible failure state as a failed challenge fetch.
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        await import("altcha");
        await import("altcha/i18n/zh-cn");
      } catch {
        if (!cancelled) {
          markFailed();
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [loadAttempt, markFailed]);

  // Watchdog: if the widget produces no event at all within the window,
  // the module or the challenge request never made it (CSP, network
  // filter, API down). Fail visibly instead of hanging the form.
  useEffect(() => {
    if (failed) return undefined;
    const timer = setTimeout(() => {
      if (!interactedRef.current) {
        markFailed();
      }
    }, WIDGET_LOAD_TIMEOUT_MS);
    return () => {
      clearTimeout(timer);
    };
  }, [failed, markFailed]);

  useEffect(() => {
    if (!widgetEl) return undefined;
    const onVerified = (event: Event) => {
      interactedRef.current = true;
      const payload =
        (isCustomEvent<{ payload?: string }>(event)
          ? event.detail?.payload
          : undefined) ?? readHiddenPayload(widgetEl);
      if (payload) {
        proofRef.current = payload;
        settleWaiters(payload);
      }
    };
    const onStateChange = (event: Event) => {
      const state = isCustomEvent<{ state?: string }>(event)
        ? event.detail?.state
        : undefined;
      if (state) {
        interactedRef.current = true;
      }
      if (state === "error" || state === "expired") {
        markFailed();
      }
    };
    widgetEl.addEventListener("verified", onVerified);
    widgetEl.addEventListener("statechange", onStateChange);
    return () => {
      widgetEl.removeEventListener("verified", onVerified);
      widgetEl.removeEventListener("statechange", onStateChange);
    };
  }, [widgetEl, markFailed, settleWaiters]);

  useImperativeHandle(
    captchaRef,
    () => ({
      getProof: () => proofRef.current,
      waitForProof: (timeoutMs: number) => {
        if (proofRef.current) return Promise.resolve(proofRef.current);
        if (failed) return Promise.resolve(null);
        return new Promise<string | null>((resolve) => {
          const timer = setTimeout(() => resolve(proofRef.current), timeoutMs);
          waitersRef.current.push((proof) => {
            clearTimeout(timer);
            resolve(proof);
          });
        });
      },
      reset: () => {
        proofRef.current = null;
        widgetEl?.reset?.();
      },
    }),
    [failed, widgetEl],
  );

  const retry = () => {
    interactedRef.current = false;
    proofRef.current = null;
    setFailed(false);
    failureCallbackRef.current?.(false);
    setLoadAttempt((value) => value + 1);
  };

  if (failed) {
    return (
      <div
        className={`${CAPTCHA_SLOT_CLASS} flex flex-col justify-center`}
        data-testid="auth-captcha-failed"
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
    <div className={CAPTCHA_SLOT_CLASS} data-testid="auth-captcha">
      <altcha-widget
        ref={setWidgetEl}
        challenge={apiUrl("/api/v1/captcha/challenge")}
        name="altcha"
        auto="onload"
        language="zh-cn"
      />
    </div>
  );
}

function isCustomEvent<T>(event: Event): event is CustomEvent<T> {
  return event instanceof CustomEvent;
}

// The verified event carries the payload; the hidden input is the
// form-integration copy of the same value and serves as a fallback.
function readHiddenPayload(el: HTMLElement): string | null {
  const input = el.querySelector<HTMLInputElement>('input[name="altcha"]');
  return input?.value ? input.value : null;
}
