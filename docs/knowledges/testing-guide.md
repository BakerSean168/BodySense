# 测试指南

本指南介绍 BodySense 项目的测试策略和最佳实践。

---

## 1. 测试类型

### 单元测试 (Unit Tests)
测试单个函数或方法，不依赖外部服务。

```bash
# 运行所有测试
cd apps/api && go test ./... -v

# 运行特定包的测试
go test ./internal/auth/... -v

# 运行特定测试
go test ./internal/auth/... -run TestGenerateAccessToken -v
```

### 集成测试 (Integration Tests)
测试多个组件协作，需要数据库等外部服务。

```bash
# 需要先启动 Docker 环境
docker compose -f docker/docker-compose.yml --profile dev up -d

# 运行集成测试
go test ./internal/repository/... -v
```

---

## 2. 测试文件组织

```
apps/api/internal/
├── auth/
│   ├── jwt.go           ← 被测代码
│   └── jwt_test.go      ← 测试代码（同目录）
├── service/
│   ├── auth_service.go
│   └── auth_service_test.go
└── repository/
    ├── user_repository.go
    └── user_repository_test.go
```

**Go 测试规范：**
- 测试文件以 `_test.go` 结尾
- 测试函数以 `Test` 开头
- 与被测文件同目录

---

## 3. 已实现的测试

### JWT 测试 (auth/jwt_test.go)

| 测试 | 说明 |
|------|------|
| TestGenerateAccessToken | 测试 token 生成 |
| TestValidateAccessToken | 测试 token 验证 |
| TestValidateAccessToken_InvalidToken | 测试无效 token |
| TestValidateAccessToken_WrongSecret | 测试错误密钥 |
| TestGenerateRefreshToken | 测试 refresh token 生成 |

**运行：**
```bash
go test ./internal/auth/... -v
```

### Service 测试 (service/auth_service_test.go)

| 测试 | 说明 |
|------|------|
| TestPasswordValidation | 测试密码长度验证 |
| TestEmailValidation | 测试邮箱格式验证 |

**运行：**
```bash
go test ./internal/service/... -v
```

### Repository 测试 (repository/user_repository_test.go)

| 测试 | 说明 |
|------|------|
| TestUserRepository_Create | 测试创建用户（需要数据库） |
| TestUserRepository_FindByEmail | 测试查找用户（需要数据库） |
| TestUserRepository_EmailExists | 测试邮箱检查（需要数据库） |

**运行（需要数据库）：**
```bash
go test ./internal/repository/... -v
```

---

## 4. 测试最佳实践

### 表驱动测试 (Table-Driven Tests)

```go
func TestPasswordValidation(t *testing.T) {
    tests := []struct {
        name     string
        password string
        wantErr  bool
    }{
        {"valid password", "password123", false},
        {"too short", "1234567", true},
        {"minimum length", "12345678", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 测试逻辑
        })
    }
}
```

**优点：**
- 易于添加新测试用例
- 测试逻辑清晰
- 失败时能快速定位问题

### 测试辅助函数

```go
func setupTestDB(t *testing.T) {
    t.Helper()  // 标记为辅助函数
    // 设置逻辑
    t.Skip("Skipping database tests")  // 条件跳过
}
```

### 测试命名规范

```
Test<函数名>_<场景>
Test<函数名>_<场景>_<预期结果>

示例：
TestValidateAccessToken_InvalidToken
TestValidateAccessToken_WrongSecret
```

---

## 5. Mock 和 Stub

### 什么是 Mock？

Mock 是模拟对象，用于替代真实的依赖（如数据库、Redis）。

### 为什么需要 Mock？

```
不使用 Mock：
Test → 真实数据库 → 需要启动 Docker → 慢

使用 Mock：
Test → Mock 数据库 → 不需要外部服务 → 快
```

### 简单 Mock 示例

```go
// 定义接口
type UserRepository interface {
    Create(user *User) error
    FindByEmail(email string) (*User, error)
}

// Mock 实现
type MockUserRepository struct {
    users map[string]*User
}

func (m *MockUserRepository) Create(user *User) error {
    m.users[user.Email] = user
    return nil
}

func (m *MockUserRepository) FindByEmail(email string) (*User, error) {
    user, ok := m.users[email]
    if !ok {
        return nil, errors.New("not found")
    }
    return user, nil
}
```

---

## 6. 测试覆盖率

### 查看覆盖率

```bash
# 生成覆盖率报告
go test ./... -coverprofile=coverage.out

# 查看覆盖率
go tool cover -func=coverage.out

# 生成 HTML 报告
go tool cover -html=coverage.out -o coverage.html
```

### 覆盖率目标

| 级别 | 覆盖率 | 说明 |
|------|--------|------|
| 低 | < 50% | 需要补充测试 |
| 中 | 50-80% | 基本达标 |
| 高 | > 80% | 理想状态 |

---

## 7. 持续集成 (CI)

### GitHub Actions 示例

```yaml
name: Go Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - run: go test ./... -v
```

---

## 8. 常见问题

### Q: 测试失败怎么办？

```bash
# 查看详细输出
go test ./... -v

# 只运行失败的测试
go test ./... -run TestName -v

# 清除缓存
go clean -testcache
```

### Q: 如何跳过需要数据库的测试？

```go
func TestWithDB(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping database test in short mode")
    }
    // 测试逻辑
}

# 运行时加 -short 参数
go test -short ./...
```

### Q: 测试文件需要提交到 Git 吗？

**是的！** 测试文件是代码的一部分，应该提交到 Git。

```gitignore
# 不要忽略测试文件
# *_test.go  ← 不要加这个
```

---

## 9. 下一步

- [ ] 为 Handler 层添加测试
- [ ] 为 Middleware 添加测试
- [ ] 添加集成测试（需要数据库）
- [ ] 设置 CI/CD 自动运行测试

---

*测试指南 | 2026-06-21*
