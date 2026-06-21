# Issue #2: 用户注册/登录 + JWT 鉴权

## 实施计划

### 1. DB Layer (Driver: B)
- [ ] 创建 users 表迁移文件
  - 字段：id (UUID), email (unique), password_hash, created_at, last_login_at
- [ ] 创建 User GORM model

### 2. Go Backend (Driver: A)
- [ ] 安装依赖：bcrypt, jwt-go, go-redis
- [ ] 实现 auth service
  - Register: 校验邮箱唯一性、密码强度，bcrypt 哈希存储
  - Login: 验证凭证，生成 JWT Access Token (7天) + Refresh Token (30天)
  - Refresh: 验证 Redis 中的 Refresh Token，生成新 Access Token
- [ ] 实现 auth handler
  - POST /api/v1/auth/register
  - POST /api/v1/auth/login
  - POST /api/v1/auth/refresh
- [ ] 实现 AuthMiddleware
  - 校验 JWT token
  - 未认证返回 401
- [ ] 配置路由

### 3. React Frontend (Driver: B)
- [ ] 安装依赖（如需要）
- [ ] 创建 auth store (Zustand)
  - 状态：user, accessToken, refreshToken, isAuthenticated
  - 方法：login, register, logout, refreshAccessToken
- [ ] 创建 API service
  - authApi: register, login, refresh
  - axios 拦截器：自动附带 token、自动刷新
- [ ] 创建 /register 页面
  - 邮箱/密码表单
  - 前端验证
  - 错误提示
- [ ] 创建 /login 页面
  - 邮箱/密码表单
  - 错误提示
- [ ] 实现路由守卫
  - ProtectedRoute 组件
  - 未登录重定向到 /login
- [ ] 配置路由

### 4. 集成测试 (Driver: A)
- [ ] Go 单元测试
- [ ] E2E 验证：注册 → 登录 → 访问受保护路由 → token 自动刷新

## 技术要点
- bcrypt cost >= 12
- JWT Access Token: 7天过期
- Refresh Token: 30天过期，存 Redis
- 前端 token 存储策略（httpOnly cookie vs localStorage）
- CORS 配置

## 文件结构

```
apps/api/
  internal/
    model/
      user.go
    repository/
      user_repository.go
    service/
      auth_service.go
    handler/
      auth_handler.go
    middleware/
      auth_middleware.go
  migrations/
    000002_create_users.up.sql
    000002_create_users.down.sql

apps/web/src/
  features/
    auth/
      components/
        LoginForm.tsx
        RegisterForm.tsx
      hooks/
        useAuth.ts
      services/
        authService.ts
      index.ts
  stores/
    authStore.ts
  pages/
    LoginPage.tsx
    RegisterPage.tsx
  components/
    ProtectedRoute.tsx
```
