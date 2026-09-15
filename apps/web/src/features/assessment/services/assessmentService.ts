import {
  generateAssessment,
  getAssessment,
  listAssessments,
} from "@/generated/api/bodysense";
import type {
  AssessmentReportOutput as GeneratedAssessmentReport,
  AssessmentReportV2Output as GeneratedAssessmentReportV2,
} from "@/generated/api/model";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

export type AssessmentEvidenceSource =
  "body_state" | "report" | "posture_analysis";

export type AssessmentEvidenceDomain =
  | "posture"
  | "exercise"
  | "lifestyle"
  | "anthropometry"
  | "health_report"
  | "injury_symptoms";

export interface AssessmentDomainCoverage {
  status: "available" | "missing";
  evidence_refs: string[];
}

export interface AssessmentEvidenceCoverage {
  status: "complete" | "partial" | "insufficient";
  available_sources: AssessmentEvidenceSource[];
  domains: Record<AssessmentEvidenceDomain, AssessmentDomainCoverage>;
}

export interface AssessmentEvidenceGap {
  dimension: AssessmentEvidenceDomain;
  /** Coverage gap only; not a clinical requirement. */
  required: false;
  description: string;
  needed_sources: AssessmentEvidenceSource[];
}

export type AssessmentObservationKind =
  | "posture_alignment"
  | "posture_asymmetry"
  | "lifestyle_pattern"
  | "exercise_pattern"
  | "report_indicator"
  | "anthropometry";

/** Evidence-grounded observation rendered by application code, not by the model. */
export interface AssessmentObservation {
  observation_id: string;
  review_state: string;
  kind: AssessmentObservationKind;
  body_region: string;
  label: string;
  description: string;
  method: "assessment_evidence" | string;
  evidence_refs: [string];
}

interface AssessmentReportBase {
  id: string;
  user_id: string;
  status: "completed" | "insufficient_information";
  summary: string;
  safety_notes: string[];
  body_state_revision?: number;
  created_at: string;
}

export interface EvidenceAssessmentReport extends AssessmentReportBase {
  contract_revision: "assessment-output-v2";
  evidence_coverage: AssessmentEvidenceCoverage;
  evidence_gaps: AssessmentEvidenceGap[];
  observations: AssessmentObservation[];
  /** New reports do not carry pseudo health grades/scores. */
  health_grade?: never;
  dimension_scores?: never;
  information_gaps?: never;
}

export type AssessmentReport = EvidenceAssessmentReport;

export interface AssessmentListResponse {
  reports: AssessmentReport[];
  total: number;
}

function projectCoverage(
  coverage: GeneratedAssessmentReportV2["evidence_coverage"],
): AssessmentEvidenceCoverage {
  return {
    status: coverage.status,
    available_sources: [...coverage.available_sources],
    domains: {
      posture: {
        ...coverage.domains.posture,
        evidence_refs: [...coverage.domains.posture.evidence_refs],
      },
      exercise: {
        ...coverage.domains.exercise,
        evidence_refs: [...coverage.domains.exercise.evidence_refs],
      },
      lifestyle: {
        ...coverage.domains.lifestyle,
        evidence_refs: [...coverage.domains.lifestyle.evidence_refs],
      },
      anthropometry: {
        ...coverage.domains.anthropometry,
        evidence_refs: [...coverage.domains.anthropometry.evidence_refs],
      },
      health_report: {
        ...coverage.domains.health_report,
        evidence_refs: [...coverage.domains.health_report.evidence_refs],
      },
      injury_symptoms: {
        ...coverage.domains.injury_symptoms,
        evidence_refs: [...coverage.domains.injury_symptoms.evidence_refs],
      },
    },
  };
}

function projectEvidenceReport(
  report: GeneratedAssessmentReportV2,
): EvidenceAssessmentReport {
  return {
    id: report.id,
    user_id: report.user_id,
    status: report.status,
    contract_revision: "assessment-output-v2",
    evidence_coverage: projectCoverage(report.evidence_coverage),
    evidence_gaps: report.evidence_gaps.map((gap) => ({
      dimension: gap.dimension,
      required: false,
      description: gap.description,
      needed_sources: [...gap.needed_sources],
    })),
    observations: report.observations.map((observation) => {
      const [evidenceRef] = observation.evidence_refs;
      if (observation.evidence_refs.length !== 1 || !evidenceRef) {
        throw new Error(
          "validated assessment observation lost its single evidence reference",
        );
      }
      return {
        observation_id: observation.observation_id,
        review_state: observation.review_state,
        kind: observation.kind,
        body_region: observation.body_region,
        label: observation.label,
        description: observation.description,
        method: observation.method,
        evidence_refs: [evidenceRef],
      };
    }),
    summary: report.summary,
    safety_notes: [...report.safety_notes],
    body_state_revision: report.body_state_revision,
    created_at: report.created_at,
  };
}

function projectAssessmentReport(
  report: GeneratedAssessmentReport,
): AssessmentReport {
  return projectEvidenceReport(report);
}

export const assessmentApi = {
  async generate(): Promise<EvidenceAssessmentReport> {
    return withOpenApiError(async () =>
      projectEvidenceReport(
        await generateAssessment(undefined, openApiAuthFetch),
      ),
    );
  },

  async getReport(id: string): Promise<AssessmentReport> {
    return withOpenApiError(async () =>
      projectAssessmentReport(
        await getAssessment(id, undefined, openApiAuthFetch),
      ),
    );
  },

  async listReports(params?: {
    limit?: number;
    offset?: number;
  }): Promise<AssessmentListResponse> {
    return withOpenApiError(async () => {
      const response = await listAssessments(
        params,
        undefined,
        openApiAuthFetch,
      );
      return {
        reports: response.reports.map(projectAssessmentReport),
        total: response.total,
      };
    });
  },
};
