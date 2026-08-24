import { authFetch } from "@/features/auth/services/authService";
import { extractErrorMessage, safeJson } from "@/lib/api-url";

export interface PrivacyDataCount {
  name: string;
  count: number;
}

export interface PrivacyErasurePlan {
  destructive: true;
  confirmation_phrase: string;
  counts: PrivacyDataCount[];
  retained_audit: string[];
}

export interface PrivacyErasureRequestResult {
  request_id: string;
  status: "pending" | "running" | "retryable" | "completed";
  message: string;
}

export const privacyApi = {
  async getErasurePlan(): Promise<PrivacyErasurePlan> {
    const response = await authFetch("/api/v1/privacy/erasure-plan", {
      cache: "no-store",
    });
    if (!response.ok) {
      throw new Error(await extractErrorMessage(response));
    }
    return safeJson<PrivacyErasurePlan>(response);
  },

  async requestErasure(
    confirmation: string,
  ): Promise<PrivacyErasureRequestResult> {
    const response = await authFetch("/api/v1/privacy/erasure", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ confirmation }),
      cache: "no-store",
    });
    if (!response.ok) {
      throw new Error(await extractErrorMessage(response));
    }
    return safeJson<PrivacyErasureRequestResult>(response);
  },
};
