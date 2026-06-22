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
