export { ConsultationPage } from './pages/ConsultationPage';
export { ChatPanel } from './components/ChatPanel';
export { ChatMessage } from './components/ChatMessage';
export { ChatInput } from './components/ChatInput';
export { useChatSSE } from './hooks/useChatSSE';
export { consultationApi } from './services/consultationService';
export type {
  ConsultationSession,
  ChatMessage as ChatMessageType,
  ExtractedInfo,
  SessionListResponse,
} from './services/consultationService';
