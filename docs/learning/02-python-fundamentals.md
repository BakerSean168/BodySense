# Python 基础详解 — 从 BodySense AI 服务读懂 Python

> 学习版文档：真实源码保持整洁，这里是带逐行注释的"讲解副本"。
> 对照阅读的真实文件：
> - `apps/ai-service/src/main.py`（FastAPI 入口）
> - `apps/ai-service/src/api/routes/posture.py`（API 路由）
> - `apps/ai-service/src/models/posture.py`（Pydantic 数据模型）
> - `apps/ai-service/src/services/posture_analyzer.py`（业务逻辑 + 治理）

---

## 0. 先建立整体心智模型

AI 服务用的是 **FastAPI**（现代 Python Web 框架），核心思路和 Go 后端类似，也是分层：

```text
HTTP 请求（图片上传）
  │
  ▼
route（api/routes/posture.py）   ← 校验入参、限流、组织响应
  │
  ▼
service（services/posture_analyzer.py） ← 调用大模型 + 安全治理
  │
  ▼
AI Provider（大模型）→ 返回 JSON → 治理清洗 → 结构化输出
  │
  ▼
model（models/posture.py）       ← Pydantic 定义"输出长什么样"
```

Python 与 Go 最大的不同：
- Python 是**动态类型**，但这个项目大量用**类型注解（type hints）**+ Pydantic 来找回类型安全。
- Python 用 **async/await** 处理并发（协程），而不是 Go 的 goroutine。
- Python 用 **异常（try/except/raise）** 处理错误，而不是 Go 的返回值。

---

## 1. 模块、导入与 `__name__`

Python 里一个 `.py` 文件就是一个**模块**。目录里有 `__init__.py` 就是一个**包**。

```python
"""BodySense AI Service - FastAPI application."""
# ↑ 文件顶部的字符串叫 "docstring"（文档字符串），是这个模块的说明。

# ── 标准库导入 ──
import os                                  # 操作系统交互，os.getenv 读环境变量
from contextlib import asynccontextmanager # 从 contextlib 里只导入这一个工具
from pathlib import Path                   # 面向对象的路径操作，比字符串拼路径更安全

# ── 第三方库导入 ──
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

# ── 可选依赖的容错导入 ──
try:
    from dotenv import load_dotenv
except ModuleNotFoundError:  # 如果没装 python-dotenv，也不报错
    load_dotenv = None       # 退化为 None，后面用前先判断
```

要点：
- `import os` 导入整个模块，用时写 `os.getenv(...)`。
- `from pathlib import Path` 只导入 `Path` 这个名字，用时直接写 `Path(...)`。
- `try/except ImportError` 是"可选依赖"的常见模式：装了就用，没装就降级。

### `__file__` 与路径计算

```python
# __file__ 是当前文件的路径。Path(__file__).parent 是它所在目录。
# 连续 .parent 向上跳目录。这样定位 .env 比手写字符串更健壮。
_env_paths = [
    Path(__file__).parent.parent / ".env",         # apps/ai-service/.env
    Path(__file__).parent.parent.parent / ".env",  # 项目根 .env
]
# Path 支持用 / 运算符拼接路径（重载了运算符），比 os.path.join 更直观。

for _env_path in _env_paths:      # for ... in 遍历列表
    if load_dotenv and _env_path.exists():  # 两个条件都为真才进入
        load_dotenv(_env_path, override=True)
        break                     # 找到第一个存在的就停止
```

> 命名约定：变量名前加下划线 `_env_paths` 表示"这是模块内部私有的，别从外部引用"。这只是约定，不是强制。

---

## 2. 变量与类型注解

Python 变量不需要声明类型，直接赋值即可。但这个项目大量用**类型注解**提升可读性和工具支持：

```python
# 不带注解（也合法）
count = 0
name = "bodysense"

# 带类型注解（推荐，IDE 和类型检查器能帮你抓错）
_MAX_FILE_SIZE: int = 10 * 1024 * 1024      # 显式声明是 int
_ALLOWED_TYPES = {"image/jpeg", "image/png"} # set 集合，去重、查成员快
_VALID_VIEWS = {"front", "side", "back"}     # set 字面量用 {}

# 常量：Python 没有真正的常量，约定用全大写 + 下划线表示"别改我"。
```

常见容器类型：
```python
my_list = [1, 2, 3]            # list 列表：有序、可变、可重复
my_tuple = (1, 2, 3)          # tuple 元组：有序、不可变
my_set = {1, 2, 3}            # set 集合：无序、去重
my_dict = {"key": "value"}   # dict 字典：键值对
```

### 现代类型注解写法（Python 3.10+）

```python
from __future__ import annotations  # 让所有注解"延迟求值"，
                                    # 使得 X | None、list[X] 等新语法在旧解释器也能用

from typing import Literal

# Literal 表示"只能是这几个字面量之一"，比普通 str 更精确。
Confidence = Literal["high", "medium", "low"]  # 这是一个"类型别名"
Severity = Literal["mild", "moderate", "marked"]

# X | None 表示"X 或者 None"（可空）。等价于旧写法 Optional[X]。
metric: PostureMetric | None = None
# list[PostureFinding] 表示"元素是 PostureFinding 的列表"。
findings: list[PostureFinding] = []
```

---

## 3. Pydantic 模型 — Python 版的"强类型数据契约"

Pydantic 让你用 class 定义数据结构，并**自动校验**输入。FastAPI 用它来解析请求/响应。

```python
from pydantic import BaseModel, Field

# 继承 BaseModel 就获得了自动校验、序列化、JSON 转换能力。
class PostureMetric(BaseModel):
    """一个量化的几何指标。"""  # class 的 docstring

    # 字段写法：字段名: 类型 = Field(默认值, 说明...)
    # Field(...) 里第一个参数是 ...（省略号），表示"必填，没有默认值"。
    name: str = Field(..., description="指标 id，如 'craniovertebral_angle'")
    value: float = Field(..., description="测量值")
    unit: str = Field(..., description="单位，如 'deg'")


class PostureFinding(BaseModel):
    """单个视角下的一条体态观察。"""

    key: str = Field(..., description="与知识库 problem_slug 对齐")
    label: str = Field(..., description="人类可读标签")
    severity: Severity = Field(...)      # 类型是上面定义的 Literal 别名
    confidence: Confidence = Field(...)
    evidence: str = Field("", description="照片中的可观察证据")  # 默认值 "" → 可选
    # 可空字段：类型是 PostureMetric 或 None，默认 None。
    metric: PostureMetric | None = Field(
        default=None,
        description="量化指标（仅 Phase 2；Phase 1 为 None）",
    )


class PostureAnalysis(BaseModel):
    """单视角的结构化体态分析。"""

    schema_version: int = 1              # 直接给默认值，无需 Field
    view: View
    overall_confidence: Confidence = "medium"
    # default_factory=list：每次新建实例时调用 list() 生成一个新空列表。
    # ⚠️ 为什么不写 = []？因为可变默认值会被所有实例共享（经典 Python 陷阱），
    #    default_factory 每次都造新的，避免这个坑。
    findings: list[PostureFinding] = Field(default_factory=list)
    red_flags: list[PostureRedFlag] = Field(default_factory=list)
    summary_markdown: str = Field("", description="通俗总结")
    disclaimer: str = Field(..., description="强制的医疗免责声明")
```

> 记住这个坑：**函数/字段的默认值绝不要用可变对象（`[]`、`{}`）**，要用 `default_factory=list` 或在函数里 `if x is None: x = []`。

---

## 4. async/await 与 FastAPI 路由

### 装饰器与路由定义

```python
from fastapi import APIRouter, File, Form, HTTPException, UploadFile

# APIRouter 是"子路由"，prefix 给这组接口统一加前缀，tags 用于文档分组。
router = APIRouter(prefix="/api/posture", tags=["posture"])

# @router.post(...) 是"装饰器"：它把下面的函数注册成一个 POST 接口。
# response_model 声明返回类型 → FastAPI 自动校验并生成 API 文档。
@router.post("/analyze", response_model=PostureAnalysisResponse)
# async def 定义"协程函数"。函数参数用 = Form(...) / = File(...) 声明
# 它们从 multipart 表单里取值。... 表示必填。
async def analyze(view: str = Form(...), file: UploadFile = File(...)):
    """分析单视角体态照片，返回结构化、已治理的结果。"""

    # ── 参数校验：不合法就 raise HTTPException ──
    # raise 抛出异常，FastAPI 捕获后转成对应状态码的 HTTP 响应。
    if view not in _VALID_VIEWS:            # in / not in 判断成员
        raise HTTPException(status_code=400, detail=f"invalid view: {view}")

    if file.content_type not in _ALLOWED_TYPES:
        raise HTTPException(
            status_code=400,
            # f"..." 是 f-string（格式化字符串），{} 里可写表达式。
            # ", ".join(...) 把可迭代对象用 ", " 连成字符串。
            detail=f"Unsupported file type: {file.content_type}. "
            f"Allowed: {', '.join(sorted(_ALLOWED_TYPES))}",
        )

    # ── await：等待一个异步操作完成 ──
    # file.read() 是异步的（读上传的文件），用 await 等它返回字节。
    file_bytes = await file.read()
    if not file_bytes:                      # 空字节 → not 判断为 True
        raise HTTPException(status_code=400, detail="Empty file")
    if len(file_bytes) > _MAX_FILE_SIZE:    # len() 取长度
        raise HTTPException(
            status_code=413,
            detail=f"File too large ({len(file_bytes)} bytes). ...",
        )
```

**为什么用 async？** 大模型调用、文件读取、数据库访问都是"等 I/O"的操作。`async/await` 让程序在等待时**去处理别的请求**，而不是干等。一个进程能并发处理很多请求。

### try/except/raise ... from — 异常处理与链

```python
    try:
        # 调用业务层，await 等待大模型分析完成。
        result = await analyze_posture(file_bytes, file.content_type, view)
    except HTTPException:
        # 已经是 HTTP 异常，原样往上抛（不要吞掉）。
        raise
    except Exception as e:
        # 捕获其他所有异常（大模型全挂了等）。
        # logger.exception 会打印异常 + 完整堆栈，方便排查。
        logger.exception("posture analysis failed")
        # raise ... from e：抛新异常但保留原始异常 e 作为"起因"（异常链）。
        raise HTTPException(status_code=502, detail=f"posture analysis failed: {e}") from e

    # 构造响应对象。PostureAnalysis(**result) 里的 ** 是"字典解包"：
    # 把 result 字典的每个键值对，当作关键字参数传给构造函数。
    # 等价于 PostureAnalysis(view=..., findings=..., ...)。
    return PostureAnalysisResponse(status="completed", result=PostureAnalysis(**result))
```

> `except Exception as e` 里的 `Exception` 是所有异常的基类。捕获顺序要**从具体到宽泛**（先 `HTTPException` 再 `Exception`）。

---

## 5. 业务逻辑层 — 函数、全局单例、字典操作

### 模块级单例（避免重复初始化）

```python
# 类型注解：这个变量是 AIService 或 None，初始为 None。
_ai_service_instance: AIService | None = None

def _get_ai_service() -> AIService:   # -> AIService 声明返回类型
    # global 声明：函数内要修改的是模块级变量，而不是新建局部变量。
    global _ai_service_instance
    if _ai_service_instance is None:  # 用 is None 判断（不是 == None）
        _ai_service_instance = AIService()  # 第一次调用才创建（懒加载）
    return _ai_service_instance
```

> `is` 比较"是不是同一个对象"，`==` 比较"值是否相等"。判空一律用 `is None`。

### 默认参数、base64、构造消息列表

```python
# 参数 ai 有默认值 None → 调用方可以不传。
async def analyze_posture(
    image_bytes: bytes,
    mime_type: str,
    view: str,
    ai: AIService | None = None,
) -> dict:                                # 返回一个 dict
    # "a or b"：a 为真取 a，否则取 b。这里实现"没传就用默认单例"。
    ai = ai or _get_ai_service()
    # 图片转 base64 字符串（大模型的 vision 接口要求 base64）。
    # .encode 前的 b64encode 返回 bytes，.decode() 转成 str。
    b64 = base64.b64encode(image_bytes).decode()
    # dict.get(key, default)：取值，键不存在时返回 default（不报错）。
    view_label = VIEW_LABEL.get(view, view)

    # 构造发给大模型的消息列表（system + user 两条）。
    messages = [
        ChatMessage(role="system", content=build_posture_system_prompt(view)),
        ChatMessage(
            role="user",
            # content 是一个列表，混合了文字块和图片块（多模态输入格式）。
            content=[
                {"type": "text", "text": f"这是用户的{view_label}站姿照片，..."},
                {
                    "type": "image_url",
                    # data URL 格式：把 base64 图片内联进请求。
                    "image_url": {"url": f"data:{mime_type};base64,{b64}"},
                },
            ],
        ),
    ]

    # await 大模型返回。AiRequest(...) 是关键字参数构造。
    resp = await ai.generate(
        AiRequest(use_case="posture.analyze", messages=messages, response_format="json_object")
    )

    # 大模型返回的是文本，尝试解析成 JSON。
    try:
        data = json.loads(resp.text)         # str → dict
    except (json.JSONDecodeError, TypeError): # 一次捕获多种异常，用元组
        logger.warning("posture analysis returned non-JSON output; degrading")
        data = {}                            # 解析失败就降级成空字典

    return govern_posture_result(data, view) # 交给治理函数清洗
```

### 治理函数 — 遍历、字典改写、集合、推导式

这段是"确定性安全治理"，不调大模型，纯逻辑，最适合练 Python 数据处理：

```python
def govern_posture_result(data: dict, view: str) -> dict:
    """对大模型原始结果施加所有 Phase-1 安全约束（确定性、可单测）。"""
    # isinstance 判断类型。不是 dict 就重置成空 dict（防御性）。
    if not isinstance(data, dict):
        data = {}

    # 字典赋值：键不存在则新增，存在则覆盖。
    data["schema_version"] = 1
    data["view"] = view

    # set(...) 把列表转成集合，成员判断 O(1)。
    allowed = set(VIEW_ALLOWED_KEYS.get(view, []))
    cleaned_findings: list[dict] = []

    # data.get("findings", []) or []：取不到或取到 None 都退化成空列表，
    # 保证 for 循环不会因为 None 报错。
    for f in data.get("findings", []) or []:
        if not isinstance(f, dict):
            continue                         # continue 跳过本次循环
        key = f.get("key")
        if key not in allowed:               # 丢弃跨视角的乱猜
            continue
        f["metric"] = None                   # 抗幻觉：Phase 1 不保留数值
        # "a or b" 兜底：label 为空就用映射表里的，再不行用 key 本身。
        f["label"] = f.get("label") or KEY_LABELS.get(key, key)
        if f.get("severity") not in _VALID_SEVERITY:
            f["severity"] = "mild"           # 非法值归一化
        if f.get("confidence") not in _VALID_CONFIDENCE:
            f["confidence"] = "low"
        f.setdefault("evidence", "")         # 键不存在才设默认值（存在则不动）
        cleaned_findings.append(f)           # 加入结果列表
    data["findings"] = cleaned_findings

    # 免责声明必须存在
    if not data.get("disclaimer"):
        data["disclaimer"] = DEFAULT_DISCLAIMER

    # setdefault：如果键不存在就设默认，避免后续 KeyError
    data.setdefault("summary_markdown", "")
    data.setdefault("overall_confidence", "medium")
    data.setdefault("red_flags", [])

    # 拼接扫描文本：summary + 所有 evidence。
    # 里面的 (f.get("evidence", "") for f in cleaned_findings) 是"生成器表达式"，
    # " ".join(...) 逐个消费它，把所有 evidence 用空格连起来。
    scan_text = data.get("summary_markdown", "") + " " + " ".join(
        f.get("evidence", "") for f in cleaned_findings
    )

    rf = get_red_flag_detector().detect([], scan_text)
    if rf.has_red_flags:
        # 集合推导式 {expr for item in iterable}：造一个已有红旗的 (类别,消息) 集合，
        # 用于去重。
        existing = {
            (r.get("category"), r.get("message"))
            for r in data["red_flags"]
            if isinstance(r, dict)
        }
        for flag in rf.flags:
            item = {"category": flag.category, "message": flag.message}
            # 元组 (a, b) 作为集合成员判断是否已存在，避免重复添加。
            if (item["category"], item["message"]) not in existing:
                data["red_flags"].append(item)

    # 结构化输出治理：缺必填字段就降级 confidence，而不是直接失败。
    result = AIOutputGuard().validate_structured_output(data, required_fields=_REQUIRED_FIELDS)
    # .value 取枚举的字符串值。
    if result.status.value != "accepted":
        data["overall_confidence"] = "low"

    return data
```

---

## 6. 应用生命周期与中间件（main.py）

```python
# @asynccontextmanager 把一个 async 生成器函数变成"异步上下文管理器"。
# yield 之前的代码在应用启动时跑，yield 之后的在应用关闭时跑。
@asynccontextmanager
async def app_lifespan(_: FastAPI):        # 参数名 _ 表示"我不用这个参数"
    async with runtime_checkpointer_lifespan():  # async with 管理异步资源
        yield                              # 应用运行期间停在这里

# 创建 FastAPI 应用实例，传入元信息和生命周期钩子。
app = FastAPI(title="BodySense AI Service", version="0.1.0", lifespan=app_lifespan)

# 读环境变量并按逗号切分成列表。
_cors_origins = os.getenv("CORS_ORIGINS", "http://localhost:5173").split(",")
# 注册 CORS 中间件。列表推导式 [o.strip() for o in _cors_origins] 去掉每项空白。
app.add_middleware(
    CORSMiddleware,
    allow_origins=[o.strip() for o in _cors_origins],  # ← 列表推导式
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# 把各个子路由挂到主应用上。
app.include_router(posture.router)
# ...

# 一个最简单的健康检查接口，返回一个字典（FastAPI 自动转成 JSON）。
@app.get("/health")
async def health():
    return {"status": "ok", "service": "bodysense-ai"}
```

---

## 7. 常用库/语法总览

| 语法/库 | 作用 | 例子 |
|---|---|---|
| `f"{x}"` | f-string 格式化 | `f"size {len(b)}"` |
| `list/dict/set/tuple` | 四种核心容器 | `[]` `{}` `set()` `()` |
| 列表/生成器/集合推导式 | 一行做映射过滤 | `[x for x in xs if x>0]` |
| `dict.get(k, default)` | 安全取值不报错 | `d.get("view", "front")` |
| `dict.setdefault(k, v)` | 键不存在才设 | `d.setdefault("red_flags", [])` |
| `is None` / `is not None` | 判空 | `if x is None:` |
| `a or b` | 兜底默认 | `ai = ai or default()` |
| `try/except/raise/from` | 异常处理 | 见 §4 |
| `async/await` | 协程并发 | `await ai.generate(...)` |
| `@decorator` | 装饰器 | `@router.post(...)` |
| `**dict` / `*list` | 解包 | `Model(**data)` |
| `pydantic.BaseModel` | 数据校验模型 | 见 §3 |
| `fastapi` | Web 框架 | `APIRouter`、`HTTPException` |
| `pathlib.Path` | 路径操作 | `Path(__file__).parent` |
| `json` | JSON 编解码 | `json.loads(s)` |
| `base64` | 二进制转文本 | `base64.b64encode(b)` |
| `logging` | 日志 | `logger.exception(...)` |

---

## 8. 小结：Python 基础清单（自测）

- [ ] 模块 / 包 / `__init__.py` 的关系？`import x` 和 `from x import y` 的区别？
- [ ] 类型注解 `x: int`、`X | None`、`list[X]`、`Literal[...]` 各表示什么？
- [ ] 为什么默认值不能写 `[]`，要用 `default_factory=list`？
- [ ] `async def` / `await` 解决什么问题？什么时候需要 `await`？
- [ ] `try/except/raise ... from e` 里 `from e` 有什么用？捕获顺序为什么从具体到宽泛？
- [ ] `dict.get` / `dict.setdefault` / `a or b` 这些"兜底"写法怎么用？
- [ ] `**dict` 解包传参、`{... for ...}` 推导式怎么读？
- [ ] Pydantic `BaseModel` + `Field(...)` 帮你做了什么（校验/序列化/文档）？
- [ ] `@decorator`（如 `@router.post`）本质是什么？

> 下一步（Next rep）：打开 `apps/ai-service/src/api/routes/ocr.py`（另一个结构类似的路由），对照本文**自己写一遍注释**，重点标出 async、异常处理和 Pydantic 模型三处。
