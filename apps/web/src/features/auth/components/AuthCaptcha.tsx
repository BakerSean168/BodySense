import { Turnstile, type TurnstileInstance } from "@marsidev/react-turnstile";
import type { Ref } from "react";

// Supabase verifies the CAPTCHA server-side with the Turnstile secret held in
// the Supabase dashboard. The browser only receives a public site key.
export const turnstileSiteKey =
  import.meta.env.VITE_TURNSTILE_SITE_KEY?.trim() ?? "";

export function AuthCaptcha({
  captchaRef,
}: {
  captchaRef: Ref<TurnstileInstance>;
}) {
  if (!turnstileSiteKey) return null;
  return (
    <div className="mt-5" data-testid="auth-turnstile">
      <Turnstile
        ref={captchaRef}
        siteKey={turnstileSiteKey}
        options={{ theme: "light", refreshExpired: "auto" }}
      />
    </div>
  );
}
