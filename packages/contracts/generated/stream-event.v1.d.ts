/* eslint-disable */
/**
 * Code generated from packages/contracts/schemas/stream-event.v1.schema.json.
 * DO NOT EDIT. Run `pnpm contracts:generate` instead.
 */

export type BodySenseStreamEventV1 = (
  | {
      type: "conversation.created";
      channel: "conversation";
      payload: {
        title: string;
        title_status: string;
        status: "active";
        last_message_at: string;
        created_at: string;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "run.started";
      channel: "run";
      payload: {
        status: "running";
        source: "start_turn";
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "run.resumed";
      channel: "run";
      payload: {
        status: "running";
        interaction_id: string;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "run.interrupted";
      channel: "run";
      payload: {
        status: "waiting_user";
        interaction_id: string;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "run.completed";
      channel: "run";
      payload: {
        status: "completed";
        usage?: unknown;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "run.failed";
      channel: "run";
      payload: (
        | {
            reason: string;
            [k: string]: unknown;
          }
        | {
            error: StreamFailureError;
            [k: string]: unknown;
          }
      ) & {
        status: "failed";
        error?: StreamFailureError;
        reason?: string;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "run.cancelled";
      channel: "run";
      payload: {
        status: "cancelled";
        reason: string;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "message.persisted";
      channel: "message";
      payload: {
        client_message_id: string;
        role: string;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "message.created";
      channel: "message";
      payload: {
        role: string;
        status: string;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "message.text.delta";
      channel: "message";
      payload: {
        delta: string;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "message.completed";
      channel: "message";
      payload: {
        status: "completed";
        finish_reason: string;
        usage?: unknown;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "message.failed";
      channel: "message";
      payload: {
        status: "failed";
        error: StreamFailureError;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "tool.call";
      channel: "tool";
      payload: {
        tool: string;
        args: unknown;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "tool.result";
      channel: "tool";
      payload: {
        tool: string;
        result: unknown;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "state.extracted_info.upsert";
      channel: "state";
      payload: {
        info: ExtractedInfo;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "state.lifestyle_context.upsert";
      channel: "state";
      payload: {
        context: LifestyleContext;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "state.phase.changed";
      channel: "state";
      payload: {
        to: string;
        reason: string;
        from?: string;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "state.interaction.required";
      channel: "state";
      payload: {
        interaction_id: string;
        question: InteractionQuestion;
        created_at: string;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "state.interaction.answered";
      channel: "state";
      payload: {
        interaction_id: string;
        answer: unknown;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "state.interaction.expired";
      channel: "state";
      payload: {
        interaction_id: string;
        expired_at: string;
        reason?: string;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "source.citation.added";
      channel: "source";
      payload: {
        citation: Citation;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "source.answer_attribution.added";
      channel: "source";
      payload: {
        attribution: ConsultationAnswerAttribution;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "source.knowledge_gap";
      channel: "source";
      payload: {
        query: string;
        message: string;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "safety.red_flag.detected";
      channel: "safety";
      payload: {
        has_red_flags: boolean;
        flags: RedFlag[];
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "safety.output_reviewed";
      channel: "safety";
      payload: {
        kind: string;
        verdict: "accepted" | "degraded" | "rejected";
        reasons?: string[];
        issues?: unknown[];
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "safety.output_rejected";
      channel: "safety";
      payload: {
        kind: string;
        verdict: "rejected";
        reasons?: string[];
        safety_fallback?: string;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "usage.reported";
      channel: "usage";
      payload: {
        usage: unknown;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "title.generated";
      channel: "title";
      payload: {
        title: string;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "stream.done";
      channel: "stream";
      payload: {
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "stream.error";
      channel: "stream";
      payload: {
        message: string;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "job.created";
      channel: "job";
      payload: {
        job_type: string;
        status?: string;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "job.progress";
      channel: "job";
      payload: {
        progress?: unknown;
        stage?: string;
        percent?: number;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "job.completed";
      channel: "job";
      payload: {
        result?: unknown;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
  | {
      type: "job.failed";
      channel: "job";
      payload: {
        error: unknown;
        [k: string]: unknown;
      };
      [k: string]: unknown;
    }
) & {
  version: 1;
  seq: number;
  channel:
    "conversation" | "run" | "message" | "tool" | "state" | "source" | "safety" | "usage" | "job" | "stream" | "title";
  type: string;
  ids: {
    conversation_id?: string | null;
    run_id?: string | null;
    turn_id?: string | null;
    message_id?: string | null;
    tool_call_id?: string | null;
    interaction_id?: string | null;
    job_id?: string | null;
  };
  payload: {
    [k: string]: unknown;
  };
};

export interface StreamFailureError {
  message: string;
  code?: string;
  [k: string]: unknown;
}
export interface ExtractedInfo {
  body_part: string;
  symptom_type: string;
  severity?: string;
  duration?: string;
  trigger?: string;
  relief?: string;
  radiation?: string;
  functional_impact?: string;
  neurological_signs?: string;
  onset?: string;
  additional_notes?: string;
  capture_id?: string;
  confirmed?: boolean;
  [k: string]: unknown;
}
export interface LifestyleContext {
  section: "activity" | "sleep" | "exercise" | "nutrition" | "substances" | "recovery";
  summary: string;
  details: {
    [k: string]: unknown;
  };
  [k: string]: unknown;
}
export interface InteractionQuestion {
  question: string;
  reason?: string;
  answer_type?: "text" | "single_choice" | "multi_choice" | "number" | "date" | "scale" | "select";
  options?: string[];
  context?: string;
  allow_custom_input?: boolean;
  required?: boolean;
  /**
   * @maxItems 3
   */
  fields?:
    | []
    | [InteractionQuestionField]
    | [InteractionQuestionField, InteractionQuestionField]
    | [InteractionQuestionField, InteractionQuestionField, InteractionQuestionField];
  purpose?: string;
  state_binding?: InteractionStateBinding;
  [k: string]: unknown;
}
export interface InteractionQuestionField {
  key: string;
  label: string;
  answer_type?: "text" | "single_choice" | "multi_choice" | "number" | "date" | "scale" | "select";
  options?: string[];
  required?: boolean;
  [k: string]: unknown;
}
export interface InteractionStateBinding {
  revision: string;
  capture_id: string;
  seed_info: {
    [k: string]: unknown;
  };
  field_map: {
    [k: string]: string;
  };
  [k: string]: unknown;
}
export interface Citation {
  title: string;
  summary?: string;
  content?: string;
  url?: string;
  snippet?: string;
  body_markdown?: string;
  source_title?: string;
  source_author?: string;
  category?: string;
  problem_slug?: string;
  unit_type?: string;
  unit_key?: string;
  source_key?: string;
  source_type?: string;
  lifecycle_status?: string;
  review_status?: string;
  publication_id?: string;
  publication_key?: string;
  publication_batch_key?: string;
  quality_score?: number;
  published_version?: number;
  source_locator?: {
    [k: string]: unknown;
  };
  claim_id?: string;
  claim_kind?: string;
  claim_review_id?: string;
  tags?: string[];
  clips?: unknown[];
  [k: string]: unknown;
}
export interface ConsultationAnswerAttribution {
  attribution_id: string;
  policy_revision: "consultation-answer-attribution-v1";
  claim_text: string;
  /**
   * @minItems 1
   * @maxItems 3
   */
  evidence_refs: [string] | [string, string] | [string, string, string];
  grounding_status: "supported" | "degraded" | "rejected";
  reason_codes: string[];
  /**
   * @minItems 1
   * @maxItems 3
   */
  bindings:
    | [AnswerAttributionBinding]
    | [AnswerAttributionBinding, AnswerAttributionBinding]
    | [AnswerAttributionBinding, AnswerAttributionBinding, AnswerAttributionBinding];
  [k: string]: unknown;
}
export interface AnswerAttributionBinding {
  evidence_ref: string;
  publication_id: string;
  publication_key: string;
  publication_batch_key: string;
  published_version: number;
  unit_key: string;
  claim_id: string;
  claim_review_id: string;
  claim_kind?: string;
  grounding_status?: "supported" | "degraded" | "rejected";
  reason_codes?: string[];
  source_locator: AnswerAttributionSourceLocator;
  [k: string]: unknown;
}
export interface AnswerAttributionSourceLocator {
  locator_type: "markdown_lines";
  repository: string;
  git_commit: string;
  path: string;
  line_start: number;
  line_end: number;
  [k: string]: unknown;
}
export interface RedFlag {
  category: string;
  message: string;
  matched_text?: string;
  source?: string;
  [k: string]: unknown;
}
