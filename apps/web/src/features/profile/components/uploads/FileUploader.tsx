import { useState, useCallback, useRef } from 'react';
import { useUploadStore } from '@/stores/uploadStore';
import type { FileType } from '../../types/upload.types';
import { FILE_TYPE_LABELS } from '../../types/upload.types';

const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10MB
const ALLOWED_TYPES = ['image/jpeg', 'image/png', 'image/webp', 'application/pdf'];

interface FileUploaderProps {
  onUploadComplete?: () => void;
}

export function FileUploader({ onUploadComplete }: FileUploaderProps) {
  const { uploadFile, isLoading } = useUploadStore();
  const [isDragging, setIsDragging] = useState(false);
  const [selectedType, setSelectedType] = useState<FileType>('report');
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const validateFile = (file: File): string | null => {
    if (!ALLOWED_TYPES.includes(file.type)) {
      return '不支持的文件类型，请上传 JPEG、PNG、WebP 或 PDF 文件';
    }
    if (file.size > MAX_FILE_SIZE) {
      return '文件大小超过 10MB 限制';
    }
    return null;
  };

  const handleFile = useCallback(
    async (file: File) => {
      setError(null);
      const validationError = validateFile(file);
      if (validationError) {
        setError(validationError);
        return;
      }

      try {
        await uploadFile(file, selectedType);
        onUploadComplete?.();
      } catch (err) {
        setError(err instanceof Error ? err.message : '上传失败');
      }
    },
    [uploadFile, selectedType, onUploadComplete]
  );

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
  }, []);

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      setIsDragging(false);

      const files = Array.from(e.dataTransfer.files);
      if (files.length > 0) {
        handleFile(files[0]);
      }
    },
    [handleFile]
  );

  const handleFileSelect = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const files = e.target.files;
      if (files && files.length > 0) {
        handleFile(files[0]);
      }
      // Reset input so the same file can be selected again
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
    },
    [handleFile]
  );

  return (
    <div className="space-y-4">
      {/* File type selector */}
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-2">
          文件类型
        </label>
        <div className="flex flex-wrap gap-2">
          {(Object.entries(FILE_TYPE_LABELS) as [FileType, string][]).map(
            ([value, label]) => (
              <button
                key={value}
                type="button"
                onClick={() => setSelectedType(value)}
                className={`px-3 py-1.5 text-sm rounded-md border transition-colors ${
                  selectedType === value
                    ? 'bg-blue-50 border-blue-300 text-blue-700'
                    : 'bg-white border-gray-300 text-gray-700 hover:bg-gray-50'
                }`}
              >
                {label}
              </button>
            )
          )}
        </div>
      </div>

      {/* Drop zone */}
      <div
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
        onClick={() => fileInputRef.current?.click()}
        className={`relative border-2 border-dashed rounded-lg p-8 text-center cursor-pointer transition-colors ${
          isDragging
            ? 'border-blue-400 bg-blue-50'
            : 'border-gray-300 hover:border-gray-400'
        } ${isLoading ? 'opacity-50 pointer-events-none' : ''}`}
      >
        <input
          ref={fileInputRef}
          type="file"
          accept={ALLOWED_TYPES.join(',')}
          onChange={handleFileSelect}
          className="hidden"
        />

        <div className="space-y-2">
          <svg
            className="mx-auto h-12 w-12 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
            />
          </svg>

          <div className="text-sm text-gray-600">
            <span className="font-medium text-blue-600 hover:text-blue-500">
              点击选择文件
            </span>
            {' '}或拖拽文件到此处
          </div>

          <p className="text-xs text-gray-500">
            支持 JPEG、PNG、WebP、PDF，最大 10MB
          </p>
        </div>

        {isLoading && (
          <div className="absolute inset-0 flex items-center justify-center bg-white/80 rounded-lg">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
          </div>
        )}
      </div>

      {/* Error message */}
      {error && (
        <div className="rounded-md bg-red-50 p-3">
          <p className="text-sm text-red-800">{error}</p>
        </div>
      )}
    </div>
  );
}
