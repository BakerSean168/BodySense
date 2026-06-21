# BodySense 开发知识库

本目录记录项目开发过程中的技术知识点和学习笔记。

## 目录结构

```
knowledges/
├── README.md                    ← 本文件
├── issue-1-dev-infra.md         ← Issue 1: 开发环境基础设施
├── issue-2-auth-jwt.md          ← Issue 2: 用户认证 + JWT
└── glossary.md                  ← 术语表
```

## 知识点索引

### Issue 1: 开发环境基础设施
- Docker Compose 多容器编排
- PostgreSQL + pgvector 向量数据库
- Redis 内存数据库
- Go 项目结构 (cmd/internal/pkg)
- 数据库迁移 (golang-migrate)
- Dockerfile 多阶段构建
- 环境变量管理

### Issue 2: 用户认证 + JWT
- bcrypt 密码加密
- JWT 双 Token 机制
- Refresh Token + Redis 存储
- Go 分层架构 (Handler/Service/Repository)
- React 状态管理 (Zustand)
- 路由守卫 (ProtectedRoute)
- CORS 跨域配置
- 自动 Token 刷新

---

*最后更新：2026-06-21*
