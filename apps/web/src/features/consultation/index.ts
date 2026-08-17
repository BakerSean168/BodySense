export { ConsultationPage } from "./pages/ConsultationPage";
export { AssistantChatPanel } from "./components/AssistantChatPanel";
export { BodyVisualization } from "./components/BodyVisualization";
export { useAssistantChatRuntime } from "./hooks/useAssistantChatRuntime";
export { consumeSSEStream } from "./hooks/useSSEProcessor";
export { consultationApi } from "./services/consultationService";
export type {
  ConsultationSession,
  ConsultationPhase,
  ExtractedInfo,
  DiagnosisCandidate,
  DiagnosisAnalysis,
  Citation,
  Conversation,
  ConversationListResponse,
  Message,
  ConversationShare,
  SharedConversation,
  RedFlag,
  RedFlagEvent,
} from "./types/consultation";
