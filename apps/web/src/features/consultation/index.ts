export { ConsultationPage } from './pages/ConsultationPage';
export { ChatPanel } from './components/ChatPanel';
export { AssistantChatPanel } from './components/AssistantChatPanel';
export { ChatMessage } from './components/ChatMessage';
export { ChatInput } from './components/ChatInput';
export { InfoPanel } from './components/InfoPanel';
export { BodyVisualization } from './components/BodyVisualization';
export { useChatSSE } from './hooks/useChatSSE';
export { useAssistantChatRuntime } from './hooks/useAssistantChatRuntime';
export { consultationApi } from './services/consultationService';
export type {
  ConsultationSession,
  ChatMessage as ChatMessageType,
  ExtractedInfo,
  SessionListResponse,
} from './services/consultationService';
