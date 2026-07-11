import { useEffect, useRef, useState } from 'react';
import { useUploadStore } from '@/stores/uploadStore';
import type { AnalysisStatus, FileType, PostureAnalysis } from '../../../types/upload.types';
import { FILE_TYPE_LABELS, SEVERITY_LABELS } from '../../../types/upload.types';
import { PostureAnalysisView } from '../../uploads/PostureAnalysisView';

const ANALYSIS_BADGE: Record<AnalysisStatus, { text: string; className: string }> = {
  none: { text: '未分析', className: 'bg-slate-100 text-slate-600' },
  pending: { text: '等待分析', className: 'bg-slate-100 text-slate-600' },
  processing: { text: 'AI 分析中', className: 'bg-amber-50 text-amber-700 border border-amber-150 animate-pulse' },
  completed: { text: '分析完成', className: 'bg-emerald-50 text-emerald-700 border border-emerald-150' },
  failed: { text: '分析失败', className: 'bg-rose-50 text-rose-700 border border-rose-150' },
};

export function UploadStep() {
  const { uploads, fetchUploads, uploadFile, deleteUpload } = useUploadStore();
  const [uploadingTypes, setUploadingTypes] = useState<Record<string, boolean>>({});
  const [error, setError] = useState<string | null>(null);

  const fileInputRefs = {
    photo_front: useRef<HTMLInputElement>(null),
    photo_side: useRef<HTMLInputElement>(null),
    photo_back: useRef<HTMLInputElement>(null),
    report: useRef<HTMLInputElement>(null),
  };

  useEffect(() => {
    fetchUploads();
  }, [fetchUploads]);

  // Posture analysis runs as an async job; poll while any photo is still
  // pending/processing, and stop once everything settles.
  const hasInFlightAnalysis = uploads.some(
    (u) =>
      u.file_type.startsWith('photo_') &&
      (u.analysis_status === 'pending' || u.analysis_status === 'processing'),
  );

  useEffect(() => {
    if (!hasInFlightAnalysis) return;
    const timer = setInterval(() => {
      fetchUploads();
    }, 4000);
    return () => clearInterval(timer);
  }, [hasInFlightAnalysis, fetchUploads]);

  // Find files by type
  const getPhotoByType = (type: FileType) => {
    return uploads.find((u) => u.file_type === type);
  };

  const getReports = () => {
    return uploads.filter((u) => u.file_type === 'report');
  };

  const handleFileChange = async (type: FileType, file: File) => {
    setError(null);
    setUploadingTypes((prev) => ({ ...prev, [type]: true }));

    try {
      await uploadFile(file, type);
    } catch (err) {
      setError(err instanceof Error ? err.message : '文件上传失败');
    } finally {
      setUploadingTypes((prev) => ({ ...prev, [type]: false }));
    }
  };

  const handleSelectClick = (type: FileType) => {
    fileInputRefs[type].current?.click();
  };

  const handleDelete = async (id: string) => {
    try {
      await deleteUpload(id);
    } catch (err) {
      setError(err instanceof Error ? err.message : '文件删除失败');
    }
  };

  // Render a single photo upload slot
  const renderPhotoSlot = (type: Extract<FileType, 'photo_front' | 'photo_side' | 'photo_back'>, label: string, desc: string) => {
    const photo = getPhotoByType(type);
    const isUploading = uploadingTypes[type];

    return (
      <div className="border rounded-xl p-4 bg-slate-50/50 relative overflow-hidden flex flex-col justify-between h-56 transition-all hover:border-slate-300">
        <input
          type="file"
          ref={fileInputRefs[type]}
          onChange={(e) => {
            const files = e.target.files;
            if (files && files.length > 0) {
              handleFileChange(type, files[0]);
            }
          }}
          accept="image/*"
          className="hidden"
        />

        <div>
          <h3 className="font-semibold text-slate-800 text-sm">{label}</h3>
          <p className="text-xs text-slate-500 mt-1 leading-relaxed">{desc}</p>
        </div>

        {photo ? (
          <div className="mt-4 flex-1 flex flex-col justify-end space-y-2">
            <div className="bg-white border border-slate-200 text-slate-700 rounded-lg p-2.5 flex items-center justify-between text-xs font-medium">
              <div className="flex items-center space-x-2 truncate">
                <svg className="w-4.5 h-4.5 text-emerald-500 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
                <span className="truncate">{photo.original_name}</span>
              </div>
              <button
                onClick={() => handleDelete(photo.id)}
                className="text-slate-400 hover:text-red-500 p-1 rounded-full hover:bg-slate-200/50 transition-colors"
                title="删除图片"
              >
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </div>

            {(() => {
              const status = (photo.analysis_status ?? 'none') as AnalysisStatus;
              const badge = ANALYSIS_BADGE[status];
              const findings = photo.analysis_result?.findings ?? [];
              return (
                <div className="flex flex-wrap items-center gap-1.5">
                  <span
                    className={`px-2 py-0.5 rounded text-[10px] font-semibold tracking-wider uppercase ${badge.className}`}
                  >
                    {badge.text}
                  </span>
                  {status === 'completed' &&
                    findings.slice(0, 2).map((f) => (
                      <span
                        key={f.key}
                        className="px-1.5 py-0.5 rounded bg-slate-100 text-slate-600 text-[10px] font-medium"
                      >
                        {f.label}·{SEVERITY_LABELS[f.severity] ?? f.severity}
                      </span>
                    ))}
                  {status === 'completed' && findings.length === 0 && (
                    <span className="text-[10px] text-slate-400">未见明显偏差</span>
                  )}
                </div>
              );
            })()}
          </div>
        ) : (
          <div className="mt-4 flex-1 flex flex-col justify-end">
            <button
              type="button"
              disabled={isUploading}
              onClick={() => handleSelectClick(type)}
              className="w-full bg-white hover:bg-slate-100/70 border border-slate-200 text-slate-700 py-2.5 rounded-lg text-xs font-semibold shadow-sm transition-all flex items-center justify-center space-x-1"
            >
              {isUploading ? (
                <>
                  <div className="animate-spin rounded-full h-3.5 w-3.5 border-b-2 border-slate-600 mr-1.5" />
                  <span>上传中...</span>
                </>
              ) : (
                <>
                  <svg className="w-4 h-4 mr-1 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                  </svg>
                  <span>选择照片</span>
                </>
              )}
            </button>
          </div>
        )}
      </div>
    );
  };

  const reports = getReports();
  const isUploadingReport = uploadingTypes['report'];

  // Completed per-view posture analyses, ordered front → side → back.
  const postureAnalyses: PostureAnalysis[] = (['photo_front', 'photo_side', 'photo_back'] as const)
    .map((t) => getPhotoByType(t))
    .filter(
      (u): u is NonNullable<typeof u> =>
        !!u && u.analysis_status === 'completed' && !!u.analysis_result,
    )
    .map((u) => u.analysis_result as PostureAnalysis);

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-xl font-bold text-slate-900">材料上传 (可选)</h2>
        <p className="text-sm text-slate-500 mt-1 leading-relaxed">
          上传您的体态照片，AI 会通过多模态视觉进行体态分析（如头前移、高低肩等倾向）；上传体检报告，我们将自动识别微量元素等影响体态康复的指标。
        </p>
        <p className="text-[11px] text-slate-400 mt-1">
          注：AI 分析基于照片视觉判断，仅供参考，不构成医疗诊断。
        </p>
      </div>

      {error && (
        <div className="rounded-lg bg-rose-50 border border-rose-100 p-3 flex items-center space-x-2 text-rose-800 text-xs font-medium">
          <svg className="w-4.5 h-4.5 text-rose-500 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          <span>{error}</span>
        </div>
      )}

      {/* Posture Photos Row */}
      <div>
        <h3 className="text-xs font-bold text-slate-400 uppercase tracking-widest mb-3">体态三视图</h3>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {renderPhotoSlot('photo_front', FILE_TYPE_LABELS.photo_front, '请自然站立拍摄正面全身照，确保双肩双脚清晰可见。')}
          {renderPhotoSlot('photo_side', FILE_TYPE_LABELS.photo_side, '请双手自然下垂拍摄侧面全身照，展示头部和耳垂重心。')}
          {renderPhotoSlot('photo_back', FILE_TYPE_LABELS.photo_back, '请放松站立拍摄背面全身照，观察两肩和肩胛骨对称度。')}
        </div>

        {postureAnalyses.length > 0 && (
          <div className="mt-4">
            <PostureAnalysisView analyses={postureAnalyses} />
          </div>
        )}
      </div>

      {/* Health Reports Section */}
      <div className="border-t border-slate-100 pt-6">
        <h3 className="text-xs font-bold text-slate-400 uppercase tracking-widest mb-3">体检报告</h3>

        <input
          type="file"
          ref={fileInputRefs['report']}
          onChange={(e) => {
            const files = e.target.files;
            if (files && files.length > 0) {
              handleFileChange('report', files[0]);
            }
          }}
          accept="image/*,application/pdf"
          className="hidden"
        />

        <div className="flex flex-col space-y-3">
          {reports.map((report) => (
            <div
              key={report.id}
              className="border rounded-lg p-3 bg-slate-50/50 flex items-center justify-between text-xs transition-all hover:bg-slate-50"
            >
              <div className="flex items-center space-x-3 truncate">
                <svg className="w-5 h-5 text-slate-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                </svg>
                <div className="truncate pr-4">
                  <p className="font-semibold text-slate-800 truncate">{report.original_name}</p>
                  <p className="text-[10px] text-slate-500 mt-0.5">
                    大小: {(report.file_size / (1024 * 1024)).toFixed(2)} MB • {
                      report.ocr_status === 'completed' && report.ocr_result
                        ? `AI 成功识别 ${report.ocr_result.indicators?.length || 0} 项健康指标`
                        : report.ocr_status === 'processing'
                        ? 'AI 正在智能读取中...'
                        : report.ocr_status === 'failed'
                        ? '指标读取失败'
                        : '等待 AI 读取'
                    }
                  </p>
                </div>
              </div>

              <div className="flex items-center space-x-2">
                <span
                  className={`px-2 py-0.5 rounded text-[10px] font-semibold tracking-wider uppercase ${
                    report.ocr_status === 'completed'
                      ? 'bg-emerald-50 text-emerald-700 border border-emerald-150'
                      : report.ocr_status === 'processing'
                      ? 'bg-amber-50 text-amber-700 border border-amber-150 animate-pulse'
                      : report.ocr_status === 'failed'
                      ? 'bg-rose-50 text-rose-700 border border-rose-150'
                      : 'bg-slate-100 text-slate-600'
                  }`}
                >
                  {report.ocr_status === 'completed'
                    ? '识别成功'
                    : report.ocr_status === 'processing'
                    ? '处理中'
                    : report.ocr_status === 'failed'
                    ? '失败'
                    : '等待中'}
                </span>
                <button
                  onClick={() => handleDelete(report.id)}
                  className="text-slate-400 hover:text-red-500 p-1.5 rounded-full hover:bg-slate-200/50 transition-colors"
                >
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                </button>
              </div>
            </div>
          ))}

          <button
            type="button"
            disabled={isUploadingReport}
            onClick={() => handleSelectClick('report')}
            className="border-2 border-dashed border-slate-200 hover:border-slate-300 rounded-xl p-4 bg-slate-50/20 text-center cursor-pointer transition-colors flex flex-col items-center justify-center space-y-1.5 text-slate-500 hover:text-slate-700"
          >
            {isUploadingReport ? (
              <>
                <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-slate-600 mb-1" />
                <span className="text-xs font-semibold">正在上传报告并进行 OCR 分析...</span>
              </>
            ) : (
              <>
                <svg className="w-6 h-6 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                </svg>
                <span className="text-xs font-semibold">添加体检报告单</span>
                <span className="text-[10px] text-slate-400">支持上传单张/多张体检单照片或 PDF 文件</span>
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
