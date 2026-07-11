export interface HealthIndicator {
  name: string;
  value: string;
  unit: string | null;
  reference_range: string | null;
  confidence: 'high' | 'medium' | 'low';
}

export interface OCRResult {
  raw_text: string;
  indicators: HealthIndicator[];
  confidence: 'high' | 'medium' | 'low';
}

export type Confidence = 'high' | 'medium' | 'low';
export type Severity = 'mild' | 'moderate' | 'marked';
export type PostureView = 'front' | 'side' | 'back';
export type AnalysisStatus = 'none' | 'pending' | 'processing' | 'completed' | 'failed';

export interface PostureMetric {
  name: string;
  value: number;
  unit: string;
}

export interface PostureFinding {
  key: string;
  label: string;
  severity: Severity;
  confidence: Confidence;
  evidence: string;
  metric: PostureMetric | null;
}

export interface PostureRedFlag {
  category: string;
  message: string;
}

export interface PostureAnalysis {
  schema_version: number;
  view: PostureView;
  overall_confidence: Confidence;
  findings: PostureFinding[];
  red_flags: PostureRedFlag[];
  summary_markdown: string;
  disclaimer: string;
}

export interface UserUpload {
  id: string;
  user_id: string;
  file_type: 'photo_front' | 'photo_side' | 'photo_back' | 'report';
  original_name: string;
  file_path: string;
  file_size: number;
  mime_type: string;
  ocr_result: OCRResult | null;
  ocr_status: 'pending' | 'processing' | 'completed' | 'failed';
  analysis_status?: AnalysisStatus;
  analysis_result?: PostureAnalysis | null;
  created_at: string;
  updated_at: string;
}

export type FileType = UserUpload['file_type'];

export const FILE_TYPE_LABELS: Record<FileType, string> = {
  photo_front: '正面照片',
  photo_side: '侧面照片',
  photo_back: '背面照片',
  report: '体检报告',
};

export const OCR_STATUS_LABELS: Record<UserUpload['ocr_status'], string> = {
  pending: '等待处理',
  processing: '识别中...',
  completed: '已完成',
  failed: '识别失败',
};

export const ANALYSIS_STATUS_LABELS: Record<AnalysisStatus, string> = {
  none: '未分析',
  pending: '等待分析',
  processing: 'AI 分析中...',
  completed: '分析完成',
  failed: '分析失败',
};

export const SEVERITY_LABELS: Record<Severity, string> = {
  mild: '轻度',
  moderate: '中度',
  marked: '明显',
};

export const CONFIDENCE_LABELS: Record<Confidence, string> = {
  high: '高',
  medium: '中',
  low: '低',
};
