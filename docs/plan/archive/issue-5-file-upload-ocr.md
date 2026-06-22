# Issue #5: 文件上传 + OCR 体检报告识别

**Issue**: [profile] 文件上传 + OCR 体检报告识别
**分支**: `feature/5-file-upload-ocr`
**创建日期**: 2026-06-22
**状态**: 进行中

---

## 📋 需求概述

实现文件上传功能，支持体态照片和体检报告上传，以及体检报告的 OCR 文字识别和关键指标提取。

**端到端行为：**
1. 用户在个人档案页选择上传体态照片（正面/侧面/背面）或体检报告（图片/PDF）
2. 文件通过 Go 后端上传，存储到本地文件系统（MVP 阶段）
3. 体检报告上传后，Go 转发给 Python AI 服务进行 OCR 识别
4. Python 使用 PaddleOCR 提取报告中的文字内容和关键指标（维生素、微量元素等）
5. OCR 结果以 JSONB 格式存入 user_uploads 表
6. 前端展示上传的文件列表和 OCR 提取结果，低置信度字段标记为"待确认"
7. 用户可删除已上传的文件

---

## 🎯 验收标准

- [x] DB: user_uploads 表创建，含 file_type/file_path/ocr_result 字段
- [x] Go: 文件上传 API 支持图片和 PDF，限制文件大小（如 10MB）
- [x] Go: 文件删除 API 同时清除磁盘文件和数据库记录
- [x] Go: 文件列表 API 返回当前用户的所有上传记录
- [x] Python: OCR 服务接口接收图片/PDF，返回结构化文字内容
- [x] Python: 体检报告关键指标提取（维生素、微量元素等数值）
- [x] Python: 低置信度识别结果标记 confidence: low
- [x] React: 上传组件支持拖拽/点击上传，显示上传进度
- [x] React: 文件列表展示缩略图和 OCR 结果预览
- [x] 端到端：上传体检报告图片 → OCR 提取 → 查看结果 → 删除文件

---

## 🏗️ 实施步骤

### 阶段 1：数据库层 (DB) ✅

**目标**: 创建 user_uploads 表存储文件元数据和 OCR 结果

#### 1.1 创建数据库迁移文件 ✅
- 文件: `apps/api/migrations/000005_create_user_uploads.up.sql`
- 内容:
  ```sql
  CREATE TABLE user_uploads (
      id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
      user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      file_type VARCHAR(50) NOT NULL,         -- 'photo_front', 'photo_side', 'photo_back', 'report'
      original_name VARCHAR(255) NOT NULL,
      file_path VARCHAR(500) NOT NULL,
      file_size BIGINT NOT NULL,
      mime_type VARCHAR(100) NOT NULL,
      ocr_result JSONB,                       -- OCR 提取的结构化结果
      ocr_status VARCHAR(20) DEFAULT 'pending', -- 'pending', 'processing', 'completed', 'failed'
      created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
      updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
  );

  CREATE INDEX idx_user_uploads_user_id ON user_uploads(user_id);
  CREATE INDEX idx_user_uploads_file_type ON user_uploads(file_type);
  CREATE INDEX idx_user_uploads_created_at ON user_uploads(created_at);
  ```

- 回滚文件: `apps/api/migrations/000005_create_user_uploads.down.sql`
  ```sql
  DROP TABLE IF EXISTS user_uploads;
  ```

#### 1.2 创建 Go 模型
- 文件: `apps/api/internal/model/user_upload.go`
- 定义 GORM 模型 struct，JSONB 字段使用 `datatypes.JSONType`

#### 1.3 创建 Go Repository
- 文件: `apps/api/internal/repository/upload_repository.go`
- 方法: `Create`, `GetByID`, `GetByUserID`, `Delete`, `UpdateOCRResult`

**验证**: 运行数据库迁移，确认表创建成功

---

### 阶段 2：Go 后端 — 文件上传 API ✅

**目标**: 实现文件上传、列表、删除的 REST API

#### 2.1 创建 Upload Service ✅
- 文件: `apps/api/internal/service/upload_service.go`
- 功能:
  - `UploadFile(userID, file, fileType)` — 保存文件到磁盘，创建数据库记录
  - `GetUploads(userID)` — 获取用户的所有上传记录
  - `DeleteUpload(userID, uploadID)` — 删除文件和数据库记录
  - `ProcessOCR(uploadID)` — 异步调用 AI 服务进行 OCR
- 文件存储路径: `uploads/{user_id}/{uuid}.{ext}`
- 文件大小限制: 10MB
- 允许的 MIME 类型: image/jpeg, image/png, image/webp, application/pdf

#### 2.2 创建 Upload Handler
- 文件: `apps/api/internal/handler/upload_handler.go`
- 端点:
  ```
  POST   /api/v1/uploads           # 上传文件（multipart form）
  GET    /api/v1/uploads           # 获取当前用户的上传列表
  GET    /api/v1/uploads/:id       # 获取单个上传详情
  DELETE /api/v1/uploads/:id       # 删除上传文件
  ```
- 请求参数:
  - `file`: 文件内容（multipart）
  - `file_type`: 文件类型（form field）

#### 2.3 路由注册
- 更新 `apps/api/cmd/server/main.go`
- 注入 UploadRepository → UploadService → UploadHandler
- 注册路由到 protected 分组

#### 2.4 静态文件服务
- 配置 Gin 的 `Static` 中间件，使 `/uploads/*` 可访问上传的文件
- 或通过 API 端点代理文件访问（更安全）

**验证**: 通过 curl 测试文件上传、列表、删除

---

### 阶段 3：Go 后端 — OCR 代理 ✅

**目标**: Go 后端转发文件到 Python AI 服务进行 OCR

#### 3.1 更新 Upload Service ✅
- 添加 `callOCRService(filePath)` 方法
- 使用 `multipart` 请求将文件转发到 Python 服务
- Python 服务 URL 从环境变量 `AI_SERVICE_URL` 获取

#### 3.2 异步处理
- 上传文件后，立即返回 201，OCR 在 goroutine 中异步执行
- 更新 `ocr_status` 字段跟踪处理状态
- 失败时记录错误信息到 `ocr_result` 的 `error` 字段

**验证**: 上传图片后，检查 ocr_status 从 pending 变为 completed

---

### 阶段 4：Python AI 服务 — OCR 模块 ✅

**目标**: 实现 OCR 文字识别和关键指标提取

#### 4.1 安装依赖 ✅
- 更新 `apps/ai-service/pyproject.toml`
- 添加依赖:
  ```
  paddlepaddle
  paddleocr
  Pillow
  python-multipart    # FastAPI 文件上传支持
  ```

#### 4.2 创建 OCR 服务
- 文件: `apps/ai-service/src/services/ocr.py`
- 功能:
  - `extract_text(image_path)` — 使用 PaddleOCR 提取文字
  - `extract_text_from_pdf(pdf_path)` — PDF 转图片后 OCR
  - `extract_health_indicators(text)` — 从 OCR 文本中提取关键指标
  - 返回结构化结果，低置信度字段标记 `confidence: "low"`

#### 4.3 创建指标提取器
- 文件: `apps/ai-service/src/services/indicator_extractor.py`
- 功能:
  - 使用正则表达式和 NLP 提取维生素、微量元素等数值
  - 识别常见体检指标名称（如"维生素D"、"铁蛋白"等）
  - 提取数值和单位
  - 低于阈值的置信度标记为 low

#### 4.4 创建 Pydantic 模型
- 文件: `apps/ai-service/src/models/ocr.py`
- 定义请求/响应模型:
  ```python
  class OCRResult(BaseModel):
      raw_text: str
      indicators: list[HealthIndicator]
      confidence: str  # "high", "medium", "low"

  class HealthIndicator(BaseModel):
      name: str
      value: float | str
      unit: str | None
      reference_range: str | None
      confidence: str
  ```

#### 4.5 创建 OCR 路由
- 文件: `apps/ai-service/src/api/routes/ocr.py`
- 端点:
  ```
  POST /api/ocr/extract    # 上传文件，返回 OCR 结果
  POST /api/ocr/extract-text  # 上传文件，返回纯文本
  ```
- 更新 `apps/ai-service/src/main.py` 注册新路由

**验证**: 上传体检报告图片，返回结构化的 OCR 结果

---

### 阶段 5：React 前端 — 文件上传组件 ✅

**目标**: 实现文件上传 UI 和上传列表展示

#### 5.1 创建 Upload Store ✅
- 文件: `apps/web/src/stores/uploadStore.ts`
- 状态:
  ```typescript
  interface UploadStore {
    uploads: UserUpload[];
    isLoading: boolean;
    error: string | null;
    fetchUploads: () => Promise<void>;
    uploadFile: (file: File, fileType: string) => Promise<UserUpload>;
    deleteUpload: (id: string) => Promise<void>;
  }
  ```

#### 5.2 创建 Upload Service
- 文件: `apps/web/src/features/profile/services/uploadService.ts`
- 功能:
  - `uploadFile(file, fileType, token)` — multipart 上传
  - `getUploads(token)` — 获取列表
  - `deleteUpload(id, token)` — 删除文件

#### 5.3 创建文件上传组件
- 文件: `apps/web/src/features/profile/components/uploads/FileUploader.tsx`
- 功能:
  - 拖拽上传区域（dropzone）
  - 点击选择文件
  - 文件类型选择（体态照片/体检报告）
  - 上传进度显示
  - 文件大小和类型验证
  - 使用 shadcn/ui 组件

#### 5.4 创建文件列表组件
- 文件: `apps/web/src/features/profile/components/uploads/UploadList.tsx`
- 功能:
  - 展示已上传文件的缩略图/图标
  - 显示文件名、类型、上传时间
  - OCR 状态显示（pending/processing/completed/failed）
  - OCR 结果预览（关键指标列表）
  - 删除按钮
  - 低置信度字段标记为"待确认"

#### 5.5 创建 OCR 结果展示组件
- 文件: `apps/web/src/features/profile/components/uploads/OCRResultView.tsx`
- 功能:
  - 展示提取的健康指标表格
  - 指标名称、数值、单位、参考范围
  - 低置信度指标高亮显示
  - 可折叠/展开详情

#### 5.6 集成到 Profile 页面
- 更新 `apps/web/src/features/profile/pages/ProfilePage.tsx`
- 添加"文件管理"标签页或区域
- 在 ProfileView 中展示已上传文件摘要

#### 5.7 类型定义
- 文件: `apps/web/src/features/profile/types/upload.types.ts`
- 定义:
  ```typescript
  interface UserUpload {
    id: string;
    file_type: string;
    original_name: string;
    file_path: string;
    file_size: number;
    mime_type: string;
    ocr_result: OCRResult | null;
    ocr_status: 'pending' | 'processing' | 'completed' | 'failed';
    created_at: string;
  }

  interface OCRResult {
    raw_text: string;
    indicators: HealthIndicator[];
    confidence: string;
  }

  interface HealthIndicator {
    name: string;
    value: number | string;
    unit: string | null;
    reference_range: string | null;
    confidence: string;
  }
  ```

**验证**: 文件上传、列表展示、OCR 结果预览功能正常

---

### 阶段 6：集成测试与端到端验证 ✅

**目标**: 验证完整流程

#### 6.1 Go 单元测试 ✅
- 文件: `apps/api/internal/service/upload_service_test.go`
- 测试用例:
  - 文件上传成功
  - 文件大小超限拒绝
  - 文件类型验证
  - 删除文件和记录

#### 6.2 Python 单元测试
- 文件: `apps/ai-service/tests/unit/test_ocr.py`
- 测试用例:
  - 图片 OCR 提取
  - PDF OCR 提取
  - 指标提取准确性
  - 低置信度标记

#### 6.3 端到端测试
1. 登录用户
2. 上传体检报告图片
3. 等待 OCR 处理完成
4. 查看 OCR 提取结果
5. 删除上传文件
6. 确认文件和记录已清除

**验证**: 端到端流程通过

---

## 📁 文件清单

### 数据库
- `apps/api/migrations/000005_create_user_uploads.up.sql`
- `apps/api/migrations/000005_create_user_uploads.down.sql`

### Go 后端
- `apps/api/internal/model/user_upload.go`
- `apps/api/internal/repository/upload_repository.go`
- `apps/api/internal/service/upload_service.go`
- `apps/api/internal/handler/upload_handler.go`
- `apps/api/cmd/server/main.go` (更新)

### Python AI 服务
- `apps/ai-service/src/services/ocr.py`
- `apps/ai-service/src/services/indicator_extractor.py`
- `apps/ai-service/src/models/ocr.py`
- `apps/ai-service/src/api/routes/ocr.py`
- `apps/ai-service/src/main.py` (更新)
- `apps/ai-service/pyproject.toml` (更新)

### React 前端
- `apps/web/src/stores/uploadStore.ts`
- `apps/web/src/features/profile/services/uploadService.ts`
- `apps/web/src/features/profile/components/uploads/FileUploader.tsx`
- `apps/web/src/features/profile/components/uploads/UploadList.tsx`
- `apps/web/src/features/profile/components/uploads/OCRResultView.tsx`
- `apps/web/src/features/profile/components/uploads/index.ts`
- `apps/web/src/features/profile/types/upload.types.ts`
- `apps/web/src/features/profile/pages/ProfilePage.tsx` (更新)

### 测试
- `apps/api/internal/service/upload_service_test.go`
- `apps/ai-service/tests/unit/test_ocr.py`

---

## 🔧 技术选型

### OCR 方案
- **首选**: PaddleOCR
  - 优点: 中文识别准确率高、开源免费、支持多语言
  - 缺点: 模型较大、首次加载慢
- **备选**: Tesseract OCR
  - 优点: 轻量、广泛使用
  - 缺点: 中文识别准确率不如 PaddleOCR

**决策**: 使用 PaddleOCR，中文体检报告识别效果更好

### 文件存储
- **MVP 阶段**: 本地文件系统
  - 路径: `uploads/{user_id}/{uuid}.{ext}`
  - 优点: 简单、无外部依赖
  - 缺点: 不适合多实例部署
- **后续**: 迁移到对象存储（S3/MinIO）

**决策**: MVP 用本地存储，架构预留接口方便后续迁移

### PDF 处理
- 使用 `pdf2image` 或 `PyMuPDF` 将 PDF 转为图片
- 逐页 OCR，合并结果

---

## ⚠️ 风险与应对

| 风险 | 影响 | 应对措施 |
|------|------|----------|
| PaddleOCR 模型下载慢 | 首次启动慢 | Docker 构建时预下载模型 |
| 中文识别准确率不足 | OCR 结果不可用 | 提供手动修正功能、标记低置信度 |
| 大文件上传超时 | 用户体验差 | 前端分片上传、后端流式处理 |
| 并发 OCR 处理 | 服务压力大 | 限制并发数、队列化处理 |
| 磁盘空间不足 | 上传失败 | 定期清理、配额限制 |

---

## 📅 时间估算

| 阶段 | 预计时间 | 依赖 |
|------|----------|------|
| 阶段 1: 数据库层 | 1 小时 | 无 |
| 阶段 2: Go 文件上传 API | 3 小时 | 阶段 1 |
| 阶段 3: Go OCR 代理 | 2 小时 | 阶段 2 |
| 阶段 4: Python OCR 模块 | 4 小时 | 无 |
| 阶段 5: React 前端 | 4 小时 | 阶段 2 |
| 阶段 6: 集成测试 | 2 小时 | 阶段 3, 4, 5 |
| **总计** | **16 小时** | |

---

## ✅ 完成检查清单

- [x] 数据库迁移成功执行
- [x] 文件上传 API 支持图片和 PDF
- [x] 文件大小限制（10MB）生效
- [x] 文件删除同时清除磁盘和数据库
- [x] OCR 服务能识别中文体检报告
- [x] 关键指标提取准确
- [x] 低置信度结果正确标记
- [x] 前端拖拽上传功能正常
- [x] 文件列表展示正确
- [x] OCR 结果预览可用
- [x] 端到端测试通过
- [x] 代码通过 lint 和 typecheck
- [x] Docker 环境验证通过

---

## 📝 备注

- 本计划基于 issue #5 的验收标准制定
- 实施过程中可能根据实际情况调整
- 遇到问题及时记录和沟通
- 完成后将本文件移至 `docs/plan/archive/`

---

**最后更新**: 2026-06-22
**作者**: AI Assistant
