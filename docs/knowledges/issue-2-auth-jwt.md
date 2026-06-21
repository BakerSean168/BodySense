# Issue 2: 用户注册/登录 + JWT 鉴权

> **目标：** 实现完整的用户认证系统，包括注册、登录、Token 刷新、路由守卫

---

## 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        Frontend (React)                          │
│                                                                  │
│  ┌──────────┐  ┌──────────┐  ┌─────────────┐  ┌────────────┐  │
│  │ LoginPage │  │Register  │  │ Protected   │  │ authStore  │  │
│  │          │  │Page      │  │ Route       │  │ (Zustand)  │  │
│  └────┬─────┘  └────┬─────┘  └──────┬──────┘  └─────┬──────┘  │
│       └──────────────┼───────────────┘               │          │
│                      │                               │          │
│              ┌───────▼───────┐               ┌──────▼───────┐  │
│              │  authFetch()  │               │ localStorage │  │
│              │  (自动刷新)   │               │  (持久化)    │  │
│              └───────┬───────┘               └──────────────┘  │
└──────────────────────┼──────────────────────────────────────────┘
                       │ HTTP + JWT (Authorization: Bearer xxx)
┌──────────────────────┼──────────────────────────────────────────┐
│  Backend (Go)        │                                          │
│              ┌───────▼───────┐                                  │
│              │ AuthMiddleware │ ← 验证 JWT 签名和过期时间        │
│              └───────┬───────┘                                  │
│              ┌───────▼───────┐                                  │
│              │ AuthHandler   │ ← 解析请求，调用 Service         │
│              └───────┬───────┘                                  │
│              ┌───────▼───────┐                                  │
│              │ AuthService   │ ← 业务逻辑（加密、生成Token）     │
│              └───────┬───────┘                                  │
│         ┌────────────┼────────────┐                             │
│   ┌─────▼─────┐            ┌─────▼─────┐                       │
│   │ PostgreSQL │            │  Redis    │                       │
│   │ (用户表)  │            │(Refresh   │                       │
│   │           │            │ Token)    │                       │
│   └───────────┘            └───────────┘                       │
└─────────────────────────────────────────────────────────────────┘
```

---

## 1. 密码加密 (bcrypt)

### 为什么不能存明文密码？

```
数据库泄露风险：
┌─────────────────┐
│ users 表        │
├─────────────────┤
│ email: a@b.com  │
│ password: 123456│  ← 攻击者直接看到密码！
└─────────────────┘
```

如果用户在多个网站用相同密码，一个网站泄露会导致所有账号被盗。

### bcrypt 工作原理

```
明文密码 "mypassword123"
        │
        ▼  + 随机盐 (salt)
┌─────────────────┐
│  bcrypt(cost=12) │
└────────┬────────┘
         │
         ▼
"$2a$12$LJ3m4ys..."  ← 存入数据库
```

**关键概念：**

| 概念 | 说明 |
|------|------|
| 哈希 (Hash) | 单向加密，无法解密 |
| 盐 (Salt) | 随机字符串，相同密码产生不同哈希 |
| Cost | 加密轮数，越大越慢越安全 |

### Go 中使用 bcrypt

```go
import "golang.org/x/crypto/bcrypt"

// 加密密码（注册时）
hashedPassword, err := bcrypt.GenerateFromPassword(
    []byte("mypassword123"),  // 明文密码
    12,                        // cost = 12
)

// 验证密码（登录时）
err := bcrypt.CompareHashAndPassword(
    hashedPassword,            // 数据库中的哈希
    []byte("mypassword123"),   // 用户输入的密码
)
if err != nil {
    // 密码不匹配
}
```

### 为什么 cost 选 12？

| Cost | 时间 | 安全性 | 适用场景 |
|------|------|--------|----------|
| 4 | ~1ms | 低 | 不推荐 |
| 10 | ~100ms | 中 | 一般应用 |
| 12 | ~300ms | 高 | **推荐** |
| 14 | ~1s | 很高 | 高安全需求 |

**权衡：** cost 越高越安全，但登录越慢。12 是安全和性能的平衡点。

### 为什么 bcrypt 比 MD5/SHA 安全？

```
暴力破解 8 位密码：

MD5:      ~0.001 秒/个  → 1 秒可尝试 1000 个
SHA256:   ~0.001 秒/个  → 1 秒可尝试 1000 个
bcrypt:   ~0.3 秒/个    → 1 秒只能尝试 3 个
```

bcrypt 故计慢，让暴力破解变得不可行。

---

## 2. JWT (JSON Web Token)

### 什么是 JWT？

JWT 是一种紧凑的、URL 安全的令牌格式，用于在双方之间传递声明（claims）。

### JWT 结构

```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.
eyJ1c2VyX2lkIjoiMTIzNCIsImVtYWlsIjoiYUBiLmNvbSJ9.
SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c

├─────────────────┤ ├─────────────────┤ ├─────────────────┤
    Header              Payload             Signature
   (算法类型)          (声明数据)          (签名验证)
```

### Header
```json
{
  "alg": "HS256",    // 签名算法
  "typ": "JWT"       // 令牌类型
}
```

### Payload (Claims)
```json
{
  "user_id": "1234-5678",    // 用户 ID
  "email": "a@b.com",       // 用户邮箱
  "exp": 1719000000,         // 过期时间
  "iat": 1718913600          // 签发时间
}
```

### Signature
```
HMACSHA256(
  base64UrlEncode(header) + "." + base64UrlEncode(payload),
  secret_key
)
```

**签名的作用：** 验证令牌没有被篡改。如果有人修改了 payload，签名验证会失败。

### Go 中生成 JWT

```go
import "github.com/golang-jwt/jwt/v5"

type Claims struct {
    UserID uuid.UUID `json:"user_id"`
    Email  string    `json:"email"`
    jwt.RegisteredClaims
}

// 生成 token
claims := Claims{
    UserID: userID,
    Email:  email,
    RegisteredClaims: jwt.RegisteredClaims{
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
        IssuedAt:  jwt.NewNumericDate(time.Now()),
    },
}

token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
tokenString, err := token.SignedString([]byte("secret-key"))
```

### Go 中验证 JWT

```go
token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
    // 验证算法
    if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, fmt.Errorf("unexpected signing method")
    }
    return []byte("secret-key"), nil
})

claims, ok := token.Claims.(*Claims)
if ok && token.Valid {
    // Token 有效，claims 中包含用户信息
}
```

---

## 3. 双 Token 机制

### 为什么需要两个 Token？

| 问题 | 只用一个 Token | 双 Token |
|------|----------------|----------|
| Token 被盗 | 攻击者永久有效 | 只有 7 天窗口 |
| 用户登出 | 无法撤销 | 删除 Redis 中的 Refresh Token |
| 续期 | 需要重新登录 | 静默刷新 |

### 双 Token 工作流程

```
┌─────────────────────────────────────────────────────────────┐
│                       登录流程                               │
└─────────────────────────────────────────────────────────────┘

1. 用户提交邮箱/密码
        │
        ▼
2. 后端验证密码 (bcrypt)
        │
        ▼
3. 生成两个 Token：
   ┌─────────────────────────────────────────┐
   │ Access Token (JWT)                       │
   │ - 包含：user_id, email                   │
   │ - 有效期：7 天                            │
   │ - 存储位置：前端 localStorage            │
   └─────────────────────────────────────────┘
   ┌─────────────────────────────────────────┐
   │ Refresh Token (UUID)                     │
   │ - 随机生成的字符串                        │
   │ - 有效期：30 天                           │
   │ - 存储位置：Redis + 前端 localStorage    │
   └─────────────────────────────────────────┘
        │
        ▼
4. 返回两个 Token 给前端

┌─────────────────────────────────────────────────────────────┐
│                       请求流程                               │
└─────────────────────────────────────────────────────────────┘

前端                           后端
  │                             │
  │ Authorization: Bearer xxx   │
  │ ──────────────────────────→ │
  │                             │ 1. 验证 JWT 签名
  │                             │ 2. 检查是否过期
  │                             │ 3. 提取 user_id
  │                             │
  │ ←────────────────────────── │
  │         200 OK              │

┌─────────────────────────────────────────────────────────────┐
│                    Token 过期流程                             │
└─────────────────────────────────────────────────────────────┘

前端                           后端
  │                             │
  │ 请求 /api/data              │
  │ ──────────────────────────→ │
  │                             │
  │ ←── 401 Unauthorized ───── │  Token 过期
  │                             │
  │ POST /auth/refresh          │
  │ { refresh_token: "xxx" }    │
  │ ──────────────────────────→ │
  │                             │ 1. 检查 Redis 中是否有此 token
  │                             │ 2. 删除旧 token
  │                             │ 3. 生成新的两个 token
  │ { new_access_token, ... }   │
  │ ←────────────────────────── │
  │                             │
  │ 重新发起原始请求             │
  │ ──────────────────────────→ │
  │                             │
  │ ←────────────────────────── │
  │         200 OK              │
```

### 为什么 Refresh Token 存 Redis？

```
┌─────────────────────────────────────────────────────────────┐
│                    Token 撤销场景                             │
└─────────────────────────────────────────────────────────────┘

场景 1: 用户主动登出
   前端: 删除本地 token
   后端: 删除 Redis 中的 refresh token
   结果: 即使有人拿到旧 token 也无法刷新

场景 2: 用户改密码
   后端: 删除该用户所有 refresh token
   结果: 旧设备需要重新登录

场景 3: 疑似被盗
   后端: 删除该用户所有 refresh token
   结果: 攻击者无法续期
```

Redis 支持快速查询和删除，适合存储需要频繁验证的 token。

---

## 4. Go 分层架构

### 分层模型

```
┌─────────────────────────────────────────────────────────────┐
│                        请求流向                               │
└─────────────────────────────────────────────────────────────┘

HTTP Request
     │
     ▼
┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐
│ Handler │ ──→ │ Service │ ──→ │  Repo   │ ──→ │   DB    │
└─────────┘     └─────────┘     └─────────┘     └─────────┘
     │               │               │
     │               │               └── 只做 CRUD
     │               └────────────────── 业务逻辑
     └────────────────────────────────── 解析/返回
```

### 各层职责

#### Handler（处理层）
```go
func (h *AuthHandler) Register(c *gin.Context) {
    // 1. 解析请求
    var req dto.RegisterRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, dto.ErrorResponse{Error: err.Error()})
        return
    }
    
    // 2. 调用 Service
    resp, err := h.authService.Register(c.Request.Context(), req)
    if err != nil {
        c.JSON(500, dto.ErrorResponse{Error: err.Error()})
        return
    }
    
    // 3. 返回响应
    c.JSON(201, resp)
}
```

**职责：**
- 解析 HTTP 请求（JSON、Query 参数）
- 参数验证
- 调用 Service
- 返回 HTTP 响应

#### Service（服务层）
```go
func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error) {
    // 1. 检查邮箱是否已注册
    exists, _ := s.userRepo.EmailExists(ctx, req.Email)
    if exists {
        return nil, errors.New("email already registered")
    }
    
    // 2. 加密密码
    hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
    
    // 3. 创建用户
    user := &model.User{Email: req.Email, PasswordHash: string(hashedPassword)}
    s.userRepo.Create(ctx, user)
    
    // 4. 生成 Token
    return s.generateTokens(ctx, user)
}
```

**职责：**
- 业务逻辑
- 调用 Repository
- 不直接处理 HTTP

#### Repository（数据层）
```go
func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
    return r.db.WithContext(ctx).Create(user).Error
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
    var user model.User
    err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
    return &user, err
}
```

**职责：**
- 数据库操作（CRUD）
- 不包含业务逻辑
- 使用 GORM 操作数据库

#### Model（模型层）
```go
type User struct {
    ID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
    Email        string     `gorm:"type:varchar(255);uniqueIndex"`
    PasswordHash string     `gorm:"type:varchar(255)"`
    CreatedAt    time.Time
    LastLoginAt  *time.Time
}
```

**职责：**
- 定义数据结构
- GORM 标签定义表结构和约束

#### DTO（数据传输对象）
```go
type RegisterRequest struct {
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"password" binding:"required,min=8"`
}

type AuthResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int64  `json:"expires_in"`
}
```

**职责：**
- 定义请求和响应的数据格式
- 包含验证规则

### 为什么分层？

| 优点 | 说明 |
|------|------|
| 职责清晰 | 每层只做一件事 |
| 易于测试 | 可以单独测试 Service 层 |
| 易于维护 | 修改数据库不影响业务逻辑 |
| 可复用 | Service 可以被多个 Handler 调用 |

---

## 5. React 状态管理 (Zustand)

### 什么是 Zustand？

Zustand 是一个轻量级的 React 状态管理库，比 Redux 简单得多。

### Auth Store 实现

```typescript
import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface AuthState {
  // 状态
  user: User | null;
  accessToken: string | null;
  refreshToken: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;
  
  // 方法
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string) => Promise<void>;
  logout: () => void;
  refreshAccessToken: () => Promise<boolean>;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      accessToken: null,
      refreshToken: null,
      isAuthenticated: false,
      isLoading: false,
      error: null,
      
      login: async (email, password) => {
        set({ isLoading: true, error: null });
        
        try {
          const response = await fetch('/api/v1/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password }),
          });
          
          const data = await response.json();
          
          set({
            accessToken: data.access_token,
            refreshToken: data.refresh_token,
            isAuthenticated: true,
            isLoading: false,
          });
          
          // 获取用户信息
          await get().fetchUser();
        } catch (error) {
          set({ isLoading: false, error: error.message });
        }
      },
      
      logout: () => {
        const { refreshToken } = get();
        
        // 调用后端撤销 refresh token
        fetch('/api/v1/auth/logout', {
          method: 'POST',
          body: JSON.stringify({ refresh_token: refreshToken }),
        });
        
        // 清空本地状态
        set({
          user: null,
          accessToken: null,
          refreshToken: null,
          isAuthenticated: false,
        });
      },
    }),
    {
      name: 'auth-storage',  // localStorage key
      partialize: (state) => ({
        accessToken: state.accessToken,
        refreshToken: state.refreshToken,
        isAuthenticated: state.isAuthenticated,
        user: state.user,
      }),
    }
  )
);
```

### 在组件中使用

```typescript
import { useAuthStore } from '@/stores/authStore';

function LoginPage() {
  const { login, isLoading, error } = useAuthStore();
  
  const handleSubmit = async (e) => {
    e.preventDefault();
    await login(email, password);
    navigate('/dashboard');
  };
  
  return (
    <form onSubmit={handleSubmit}>
      {error && <p>{error}</p>}
      <button disabled={isLoading}>
        {isLoading ? 'Signing in...' : 'Sign in'}
      </button>
    </form>
  );
}
```

### 为什么用 Zustand？

| 特性 | Redux | Zustand |
|------|-------|---------|
| 代码量 | 多（action, reducer, store） | 少（一个 create） |
| 学习曲线 | 陡峭 | 平缓 |
| TypeScript 支持 | 需要额外配置 | 原生支持 |
| 持久化 | 需要 redux-persist | 内置 persist |
| 包大小 | ~11KB | ~1KB |

---

## 6. 路由守卫 (ProtectedRoute)

### 实现

```typescript
import { Navigate, useLocation } from 'react-router';
import { useAuthStore } from '@/stores/authStore';

export function ProtectedRoute({ children }) {
  const { isAuthenticated } = useAuthStore();
  const location = useLocation();
  
  if (!isAuthenticated) {
    // 保存当前路径，登录后跳回
    return <Navigate to="/login" state={{ from: location }} replace />;
  }
  
  return children;
}
```

### 路由配置

```typescript
import { BrowserRouter, Routes, Route } from 'react-router';

function App() {
  return (
    <BrowserRouter>
      <Routes>
        {/* 公开路由 */}
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        
        {/* 受保护路由 */}
        <Route path="/dashboard" element={
          <ProtectedRoute>
            <DashboardPage />
          </ProtectedRoute>
        } />
        
        {/* 默认跳转 */}
        <Route path="/" element={<Navigate to="/dashboard" />} />
      </Routes>
    </BrowserRouter>
  );
}
```

### 工作流程

```
用户访问 /dashboard
        │
        ▼
   ┌─────────────┐
   │ isAuthenticated? │
   └───────┬─────┘
           │
    ┌──────┴──────┐
    │             │
   true         false
    │             │
    ▼             ▼
 显示页面     跳转 /login
              (保存当前路径)
```

---

## 7. 自动 Token 刷新

### authFetch 封装

```typescript
const API_BASE_URL = 'http://localhost:8080';

export async function authFetch(url, options = {}) {
  const { skipAuth = false, ...fetchOptions } = options;
  const { accessToken, refreshAccessToken } = useAuthStore.getState();
  
  // 添加 Authorization header
  if (!skipAuth && accessToken) {
    fetchOptions.headers = {
      ...fetchOptions.headers,
      Authorization: `Bearer ${accessToken}`,
    };
  }
  
  // 发起请求
  let response = await fetch(`${API_BASE_URL}${url}`, fetchOptions);
  
  // 如果 401，尝试刷新 token
  if (response.status === 401 && !skipAuth) {
    const refreshed = await refreshAccessToken();
    
    if (refreshed) {
      // 用新 token 重试
      const newToken = useAuthStore.getState().accessToken;
      fetchOptions.headers = {
        ...fetchOptions.headers,
        Authorization: `Bearer ${newToken}`,
      };
      response = await fetch(`${API_BASE_URL}${url}`, fetchOptions);
    }
  }
  
  return response;
}
```

### 使用示例

```typescript
// 普通请求（自动带 token）
const response = await authFetch('/api/v1/me');
const user = await response.json();

// 跳过认证（登录、注册）
const response = await authFetch('/api/v1/auth/login', {
  method: 'POST',
  body: JSON.stringify({ email, password }),
  skipAuth: true,
});
```

### 用户体验

```
用户操作 → 请求失败 (401) → 静默刷新 token → 自动重试
                ↑
                └── 用户完全无感知，不会看到错误
```

---

## 8. CORS (跨域资源共享)

### 什么是 CORS？

CORS 是浏览器的安全机制，限制网页只能请求同源的资源。

### 同源策略

```
同源（允许）：
http://localhost:5173 → http://localhost:5173/api

跨源（默认阻止）：
http://localhost:5173 → http://localhost:8080/api
     前端                    后端
```

### 解决方案：CORS 头

```go
r.Use(func(c *gin.Context) {
    // 允许所有来源（开发环境）
    c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
    
    // 允许的 HTTP 方法
    c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
    
    // 允许的请求头
    c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
    
    // 预检请求缓存时间（秒）
    c.Writer.Header().Set("Access-Control-Max-Age", "86400")
    
    // 处理预检请求
    if c.Request.Method == "OPTIONS" {
        c.AbortWithStatus(204)
        return
    }
    
    c.Next()
})
```

### 预检请求 (Preflight)

```
浏览器                           服务器
   │                               │
   │ OPTIONS /api/v1/auth/login    │  ← 预检请求
   │ Origin: http://localhost:5173 │
   │ Access-Control-Request-Method: POST
   │ ─────────────────────────────→│
   │                               │
   │ ←─────────────────────────────│
   │ Access-Control-Allow-Origin: *│
   │ Access-Control-Allow-Methods: POST
   │                               │
   │ POST /api/v1/auth/login       │  ← 实际请求
   │ ─────────────────────────────→│
   │                               │
   │ ←─────────────────────────────│
   │         200 OK                │
```

### 生产环境配置

```go
// 不要用 *，指定允许的域名
c.Writer.Header().Set("Access-Control-Allow-Origin", "https://bodysense.com")
```

---

## 9. 数据库设计

### Users 表

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMPTZ
);

CREATE INDEX idx_users_email ON users(email);
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键，自动生成 |
| email | VARCHAR(255) | 邮箱，唯一索引 |
| password_hash | VARCHAR(255) | bcrypt 哈希后的密码 |
| created_at | TIMESTAMPTZ | 创建时间，自动填充 |
| last_login_at | TIMESTAMPTZ | 最后登录时间，可空 |

### 为什么用 UUID？

| 特性 | 自增 ID | UUID |
|------|---------|------|
| 可预测 | 是（1,2,3...） | 否 |
| 安全性 | 低 | 高 |
| 分布式生成 | 需要协调 | 可本地生成 |
| 索引性能 | 高 | 略低 |

**选择 UUID 的原因：**
- 不可预测，防止遍历攻击
- 分布式系统中无需协调

---

## 10. 完整数据流

### 注册流程

```
┌─────────────────────────────────────────────────────────────┐
│ 1. 用户填写表单                                              │
└─────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. RegisterForm.handleSubmit()                              │
│    - 验证邮箱格式                                            │
│    - 验证密码长度 >= 8                                       │
│    - 验证两次密码一致                                        │
└─────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. useAuthStore.register(email, password)                   │
│    - 设置 isLoading = true                                  │
│    - 调用 POST /api/v1/auth/register                        │
└─────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. AuthHandler.Register()                                   │
│    - 解析 JSON 请求                                          │
│    - 验证 binding:"required,email"                           │
│    - 调用 authService.Register()                             │
└─────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────┐
│ 5. AuthService.Register()                                   │
│    - userRepo.EmailExists() 检查邮箱是否已注册               │
│    - bcrypt.GenerateFromPassword() 加密密码                  │
│    - userRepo.Create() 创建用户                              │
│    - generateTokens() 生成 Access + Refresh Token            │
└─────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────┐
│ 6. generateTokens()                                         │
│    - auth.GenerateAccessToken() 生成 JWT                     │
│    - auth.GenerateRefreshToken() 生成 UUID                   │
│    - redisClient.Set() 存储 Refresh Token (TTL 30天)        │
└─────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────┐
│ 7. 返回响应                                                  │
│    {                                                        │
│      "access_token": "eyJhbG...",                           │
│      "refresh_token": "a1b2c3d4...",                        │
│      "expires_in": 604800                                   │
│    }                                                        │
└─────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────┐
│ 8. 前端处理                                                  │
│    - authStore 存储 token                                    │
│    - persist 中间件保存到 localStorage                       │
│    - 跳转到 /dashboard                                       │
└─────────────────────────────────────────────────────────────┘
```

### 登录流程

```
1. 用户输入邮箱/密码
        │
        ▼
2. POST /api/v1/auth/login
        │
        ▼
3. AuthService.Login()
   ├── userRepo.FindByEmail() 查找用户
   ├── bcrypt.CompareHashAndPassword() 验证密码
   ├── userRepo.UpdateLastLoginAt() 更新登录时间
   └── generateTokens() 生成新 Token
        │
        ▼
4. 返回 Token
        │
        ▼
5. 前端存储并跳转
```

### Token 刷新流程

```
1. API 请求返回 401
        │
        ▼
2. authFetch() 检测到 401
        │
        ▼
3. useAuthStore.refreshAccessToken()
   ├── POST /api/v1/auth/refresh
   └── { refresh_token: "xxx" }
        │
        ▼
4. AuthService.RefreshToken()
   ├── redisClient.Get() 从 Redis 获取
   ├── redisClient.Del() 删除旧 token
   ├── userRepo.FindByID() 获取用户信息
   └── generateTokens() 生成新 Token
        │
        ▼
5. 返回新 Token
        │
        ▼
6. 前端更新存储
        │
        ▼
7. 用新 Token 重试原始请求
```

---

## 11. 安全最佳实践

### 密码安全

| 实践 | 说明 |
|------|------|
| 使用 bcrypt | 不要用 MD5/SHA |
| cost >= 12 | 平衡安全和性能 |
| 不存明文 | 只存哈希值 |
| 限制尝试次数 | 防止暴力破解 |

### Token 安全

| 实践 | 说明 |
|------|------|
| Access Token 短期 | 7 天过期 |
| Refresh Token 存 Redis | 可随时撤销 |
| 使用 HTTPS | 防止中间人攻击 |
| 不存敏感信息 | JWT 可被解码 |

### 前端安全

| 实践 | 说明 |
|------|------|
| 不存密码 | 只存 token |
| 使用 httpOnly Cookie | 防止 XSS（本项目用 localStorage） |
| 输入验证 | 前端验证 + 后端验证 |
| 错误信息模糊 | "邮箱或密码错误" 而不是 "密码错误" |

---

## 12. 关键概念总结

| 概念 | 作用 | 类比 |
|------|------|------|
| bcrypt | 安全地加密密码 | 像把密码放进保险箱 |
| JWT | 无状态的身份凭证 | 像一张"通行证" |
| Access Token | 短期验证身份 | 像"当日通行证" |
| Refresh Token | 安全地续期 | 像"通行证续期申请表" |
| Redis | 存储临时数据 | 像一个"快速便签本" |
| CORS | 允许跨域请求 | 像"门卫检查访客证" |
| Zustand | 前端状态管理 | 像一个"全局记事本" |
| ProtectedRoute | 保护私密页面 | 像"门禁系统" |
| authFetch | 自动刷新 token | 像"自动续费" |
| 分层架构 | 代码组织方式 | 像"流水线分工" |

---

## 13. 常见问题

### Q: JWT 被盗了怎么办？

A: JWT 有效期只有 7 天。如果怀疑被盗：
1. 删除 Redis 中所有 refresh token
2. 用户需要重新登录
3. 考虑缩短 access token 有效期

### Q: 为什么用 localStorage 而不是 httpOnly Cookie？

A: localStorage 更简单，但有 XSS 风险。生产环境建议：
- 使用 httpOnly Cookie 存储 token
- 配合 CSRF token 防护

### Q: 如何处理多个设备同时登录？

A: 每个设备生成独立的 refresh token，存入 Redis：
```
refresh_token:device1 → user_id
refresh_token:device2 → user_id
```
用户改密码时删除所有 refresh token，强制所有设备重新登录。

### Q: 为什么登录失败返回 "邮箱或密码错误" 而不是具体哪个错了？

A: 安全考虑。如果返回 "邮箱不存在"，攻击者可以枚举有效邮箱。模糊的错误信息防止信息泄露。

---

*Issue 2 知识点整理完成 | 2026-06-21*
