# Go 基础详解 — 从 BodySense 后端读懂 Go

> 学习版文档：真实源码保持整洁，这里是带逐行注释的"讲解副本"。
> 对照阅读的真实文件：
> - `apps/api/cmd/server/main.go`（启动入口）
> - `apps/api/internal/model/user.go`（数据模型）
> - `apps/api/internal/repository/user_repository.go`（数据访问层）
> - `apps/api/internal/service/auth_service.go`（业务逻辑层）
> - `apps/api/internal/handler/auth_handler.go`（HTTP 层）
> - `apps/api/internal/auth/jwt.go`（JWT 工具）
> - `apps/api/internal/middleware/auth.go`（鉴权中间件）

---

## 0. 先建立整体心智模型

Go 后端是一个**分层架构**，一个 HTTP 请求从上往下穿过这几层：

```text
HTTP 请求
  │
  ▼
handler（HTTP 层）   ← 解析请求、校验参数、组织响应，不写业务逻辑
  │
  ▼
service（业务层）    ← 真正的业务规则：密码加密、生成 token、校验归属
  │
  ▼
repository（数据层） ← 只负责跟数据库打交道（GORM），不含业务判断
  │
  ▼
model（数据模型）    ← 定义表结构（struct + GORM tag）
  │
  ▼
PostgreSQL
```

**为什么要分层？** 每层只做一件事，改动隔离：换数据库只动 repository，改业务规则只动 service，改接口格式只动 handler。这是 Go 社区的标准做法（Standard Layout）。

---

## 1. `package main` 与程序入口

每个 Go 文件第一行必须声明它属于哪个**包（package）**。其中有一个特殊包叫 `main`：

```go
// package main 是一个特殊的包名。
// 只有 package main 且包含 func main() 的包，才能被编译成"可执行程序"。
// 其他包（如 service、repository）是"库包"，只能被别的包 import，不能独立运行。
package main
```

### import：引入依赖

```go
import (
	// ── 标准库（Go 自带，无需下载）──
	"context" // 上下文：贯穿请求生命周期，携带取消信号、超时、请求级数据
	"fmt"     // 格式化：类似 C 的 printf / sprintf
	"log"     // 日志：log.Printf 打印，log.Fatal 打印后直接退出程序
	"net/http" // HTTP 协议常量与类型，如 http.StatusOK = 200
	"os"      // 操作系统交互：os.Getenv 读环境变量
	"strings" // 字符串工具：Split、TrimSpace、HasPrefix
	"time"    // 时间与时长：time.Now()、10*time.Second

	// ── 项目内部包（用 module 路径引用）──
	// module 名在 go.mod 里定义为 github.com/bodysense/api
	"github.com/bodysense/api/internal/auth"
	// 给 import 起别名：因为包名 consultation 太常见，显式改名避免歧义
	consultationruntime "github.com/bodysense/api/internal/consultation"
	"github.com/bodysense/api/internal/database"
	// ...

	// ── 第三方库（go get 下载，记录在 go.mod / go.sum）──
	"github.com/gin-gonic/gin"   // Gin：最流行的 Go Web 框架
	"github.com/joho/godotenv"   // 读取 .env 文件到环境变量
)
```

要点：
- `internal/` 是 Go 的**特殊目录**：`internal` 下的包只能被同一个 module 内的代码 import，外部项目无法引用。天然形成"私有包"边界。
- import 别名语法：`别名 "完整路径"`。

### func main()：一切从这里开始

```go
// func 关键字定义函数。main 函数没有参数、没有返回值。
// 程序启动时，运行时会自动调用 main()。
func main() {
	// godotenv.Load 把 .env 文件里的键值对加载进环境变量。
	// _ = 表示"我故意忽略这个返回值（error）"。
	// 生产环境没有 .env（env 由容器注入），加载失败也无所谓，所以忽略。
	_ = godotenv.Load("../../.env")
```

---

## 2. 变量的各种写法（Go 最容易混的点）

Go 有多种声明变量的方式，理解它们的区别是入门关键：

```go
// ① var 显式声明 + 类型（可不赋初值，会得到"零值"）
var port string          // string 的零值是 ""
var count int            // int 的零值是 0
var ready bool           // bool 的零值是 false
var user *model.User     // 指针的零值是 nil

// ② var + 初值（类型可省略，编译器自动推断）
var name = "bodysense"   // 自动推断为 string

// ③ := 短变量声明（最常用，只能在函数内部用）
//    左边是新变量，右边是值，类型自动推断。
db, err := database.Connect(dbCfg)   // 一次声明两个变量
port := os.Getenv("API_PORT")        // 推断为 string

// ④ 常量用 const
const maxRetries = 3
```

**`=` vs `:=` 的区别**（新手最容易错）：
- `:=` 声明**新**变量并赋值（至少有一个是新的）。
- `=` 给**已存在**的变量赋值。

```go
db, err := database.Connect(dbCfg) // err 第一次出现，用 :=
_, err = database.ConnectRedis(...) // err 已存在，只是重新赋值，用 =
```

### 多返回值 + 错误处理（Go 的招牌写法）

Go 没有异常（try/catch），而是**把错误当普通返回值**返回。约定：error 永远是最后一个返回值。

```go
// database.Connect 返回两个值：*gorm.DB 和 error
db, err := database.Connect(dbCfg)
// 立刻检查 err —— 这是 Go 代码里最高频的 5 行模式
if err != nil {
	// log.Fatalf = 打印日志 + os.Exit(1) 退出程序。
	// %v 是"默认格式"占位符，能打印任何类型。
	log.Fatalf("Database connection failed: %v", err)
}
_ = db // 显式忽略：这里用的是包级全局 database.DB，局部 db 不再需要
```

> 你会看到 `if err != nil { return ... }` 反复出现。这不是啰嗦，而是 Go 强迫你在每一步都正视错误，换来的是**显式、可追踪**的错误流。

---

## 3. struct 结构体、方法与"构造函数"

Go 没有 class，用 **struct（结构体）+ 方法** 来组织"数据 + 行为"。

### 数据模型（model 层）

```go
// User 是一个 struct，描述数据库 users 表的一行。
type User struct {
	// 字段名首字母大写 = 公开（可被其他包访问）；小写 = 私有。
	// 反引号里是"结构体标签（struct tag）"，供库读取元信息：
	//   gorm:"..."  告诉 GORM 如何映射到数据库列
	//   json:"..."  告诉 JSON 库序列化时用什么字段名
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Email        string     `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	// json:"-" 表示：序列化成 JSON 时**永远不输出**这个字段。
	// 密码哈希绝不能返回给前端，这行是安全关键。
	PasswordHash string     `gorm:"type:varchar(255);not null" json:"-"`
	CreatedAt    time.Time  `gorm:"not null;default:now()" json:"created_at"`
	// *time.Time 是指针类型：可以为 nil，表示"从未登录过"。
	// omitempty：值为空时 JSON 里省略这个字段。
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}

// 给 User 类型绑定一个方法。(User) 叫"接收者（receiver）"，
// 表示这个方法属于 User 类型。GORM 会调用它来确定表名。
func (User) TableName() string {
	return "users"
}
```

### 数据访问层（repository）+ 依赖注入

```go
// UserRepository 持有一个数据库连接。字段 db 小写 = 私有。
type UserRepository struct {
	db *gorm.DB
}

// Go 没有构造函数语法，社区约定用 NewXxx 函数来"造对象"。
// 它接收依赖（*gorm.DB）并返回一个初始化好的指针。
// 这就是"依赖注入"：db 从外部传进来，而不是内部自己 new。
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db} // &Struct{} 取地址，返回指针
}

// (r *UserRepository) 是指针接收者：方法能修改接收者，且避免拷贝整个 struct。
// 参数 ctx context.Context 几乎所有数据库/网络调用都带它（传递取消/超时）。
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User // 声明一个零值 User 变量
	// GORM 链式调用：WithContext 绑定上下文 → Where 加条件 → First 取第一条。
	// &user 传地址，GORM 会把查询结果填进这个变量。
	// "email = ?" 里的 ? 是占位符，email 作为参数传入 → 防 SQL 注入。
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err // 出错返回 nil 指针 + 错误
	}
	return &user, nil // 成功返回 user 的地址 + nil 错误
}

// 检查邮箱是否存在：Count 把匹配行数写入 count。
func (r *UserRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.User{}).
		Where("email = ?", email).Count(&count).Error
	return count > 0, err // 一行返回两个值：布尔判断 + 错误
}
```

> 常用库速记：**GORM** 是 ORM（对象关系映射），把 struct 和数据库表对应起来，`.Where().First().Create().Update()` 是它的链式 API。

---

## 4. 业务逻辑层（service）— 看懂真实业务流

```go
// AuthService 依赖三样东西，全部通过 New 注入。
type AuthService struct {
	userRepo     *repository.UserRepository
	jwtConfig    auth.JWTConfig
	sessionCache *cache.UserSessionCache
}

func NewAuthService(userRepo *repository.UserRepository, jwtConfig auth.JWTConfig, sessionCache *cache.UserSessionCache) *AuthService {
	return &AuthService{userRepo: userRepo, jwtConfig: jwtConfig, sessionCache: sessionCache}
}

// Register 注册：一段完整的业务规则。
func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error) {
	// 1) 查邮箱是否已存在
	exists, err := s.userRepo.EmailExists(ctx, req.Email)
	if err != nil {
		// fmt.Errorf + %w：包装错误。%w 保留原始错误，上层能用 errors.Is 判断根因。
		return nil, fmt.Errorf("failed to check email existence: %w", err)
	}
	if exists {
		// errors.New 造一个简单错误。注意这里故意说"registration failed"
		// 而不是"邮箱已存在"，避免泄露哪些邮箱注册过（安全）。
		return nil, errors.New("registration failed")
	}

	// 2) 用 bcrypt 加密密码（cost=12，越大越慢越安全）。
	//    []byte(req.Password) 把字符串转成字节切片，bcrypt 要求 []byte 输入。
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 3) 组装 model.User。&model.User{...} 造一个 User 并取地址。
	//    字段用"字段名: 值"的形式赋值，未列出的字段取零值。
	user := &model.User{
		ID:           uuid.New(),                 // 生成随机 UUID
		Email:        req.Email,
		PasswordHash: string(hashedPassword),     // []byte 转回 string 存库
	}

	// 4) 落库
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// 5) 生成 access/refresh token 并返回
	return s.generateTokens(ctx, user)
}
```

登录里有一个典型的**错误判定**写法：

```go
user, err := s.userRepo.FindByEmail(ctx, req.Email)
if err != nil {
	// errors.Is 检查 err 链里是否包含某个特定错误。
	// gorm.ErrRecordNotFound 表示"没查到记录"。
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 用户不存在。同样返回模糊信息，不告诉攻击者是"邮箱错"还是"密码错"。
		return nil, errors.New("invalid email or password")
	}
	return nil, fmt.Errorf("failed to find user: %w", err) // 其他错误（如 DB 挂了）
}
```

---

## 5. HTTP 层（handler）— 请求进出的边界

```go
type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// gin 的 handler 统一签名：func(c *gin.Context)。
// c 封装了这次请求的一切：读参数、写响应都通过它。
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest // 准备一个空的请求结构体
	// ShouldBindJSON 把请求体 JSON 反序列化进 req，并按 tag 做校验。
	// 校验失败 → 返回 400。
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return // 记得 return，否则会继续往下执行
	}

	// 调 service 做真正的业务。c.Request.Context() 取出本次请求的 context。
	resp, err := h.authService.Register(c.Request.Context(), req)
	if err != nil {
		status := http.StatusInternalServerError // 默认 500
		code := "REGISTRATION_FAILED"
		if err.Error() == "registration failed" { // 邮箱冲突 → 改成 409
			status = http.StatusConflict
		}
		respondError(c, status, code, "registration failed")
		return
	}

	// c.JSON(状态码, 数据)：把 resp 序列化成 JSON 写回，状态码 201。
	c.JSON(http.StatusCreated, resp)
}
```

`Me` 里演示了 **interface 类型断言**：

```go
func (h *AuthHandler) Me(c *gin.Context) {
	// c.Get 返回 (interface{}, bool)。中间件把 user_id 塞进了 context。
	userID, exists := c.Get("user_id")
	if !exists {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	// 类型断言：userID 是 interface{}（任意类型），
	// .(string) 断言它其实是 string。ok 表示断言是否成功。
	uid, ok := userID.(string)
	if !ok {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "invalid user id type")
		return
	}
	// ...
}
```

---

## 6. 常用库总览（你在这个项目里会反复见到）

| 库 | 作用 | 典型调用 |
|---|---|---|
| `net/http` | HTTP 状态码/常量 | `http.StatusOK`、`http.StatusUnauthorized` |
| `github.com/gin-gonic/gin` | Web 框架：路由、中间件、请求解析 | `gin.Default()`、`c.JSON()`、`c.ShouldBindJSON()` |
| `gorm.io/gorm` | ORM，操作数据库 | `db.Where().First()`、`db.Create()` |
| `github.com/golang-jwt/jwt/v5` | 签发/校验 JWT | `jwt.NewWithClaims()`、`jwt.ParseWithClaims()` |
| `golang.org/x/crypto/bcrypt` | 密码哈希 | `bcrypt.GenerateFromPassword()`、`CompareHashAndPassword()` |
| `github.com/google/uuid` | 生成 UUID | `uuid.New()`、`uuid.Parse()` |
| `context` | 传递取消/超时/请求级数据 | `ctx context.Context` |
| `crypto/rand` | 密码学安全随机数 | `rand.Read(bytes)` |

### JWT 里两个值得记的点

```go
// 生成 token：把自定义 claims + 标准 claims 打包，用密钥 HS256 签名。
token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
tokenString, err := token.SignedString([]byte(cfg.SecretKey))

// 校验 token：注意这个回调里显式检查签名算法，
// 防止攻击者把 alg 改成 none 或换算法绕过校验（经典 JWT 漏洞）。
token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	}
	return []byte(cfg.SecretKey), nil
})
```

### 用密码学随机数造 refresh token

```go
func GenerateRefreshToken() (string, error) {
	bytes := make([]byte, 32)          // make 创建长度 32 的字节切片
	if _, err := rand.Read(bytes); err != nil { // 填充 32 字节随机数
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return hex.EncodeToString(bytes), nil // 转成 64 位十六进制字符串
}
```

---

## 7. 启动流程串起来（main.go 的下半段）

```go
// 1) 读端口，给个默认值
port := os.Getenv("API_PORT")
if port == "" {
	port = "8080"
}

// 2) 创建 Gin 引擎（自带 Logger + Recovery 中间件）
r := gin.Default()

// 3) 注册全局中间件：r.Use(fn) 让每个请求都先过这个函数。
//    这里手写了一个 CORS 中间件。
r.Use(func(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", ...)
	if c.Request.Method == "OPTIONS" {
		c.AbortWithStatus(http.StatusNoContent) // 预检请求直接返回，不再往下走
		return
	}
	c.Next() // 放行，进入下一个中间件/handler
})

// 4) 路由分组：把公共前缀提出来，结构更清晰。
authGroup := r.Group("/api/v1/auth")
{ // 这对花括号只是"视觉分组"，不是语法要求
	authGroup.POST("/register", authHandler.Register) // POST /api/v1/auth/register
	authGroup.POST("/login", authHandler.Login)
}

// 5) 受保护路由：先挂鉴权中间件，再注册路由。
protected := r.Group("/api/v1")
protected.Use(middleware.AuthMiddleware(jwtConfig, userRepo, sessionCache))
{
	protected.GET("/me", authHandler.Me) // 请求必须带合法 JWT 才能进来
}

// 6) 启动服务器（阻塞，直到进程退出）。
//    fmt.Sprintf(":%s", port) 拼成 ":8080"。
log.Printf("BodySense API starting on :%s", port)
if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
	log.Fatal(err)
}
```

**依赖装配顺序**（main.go 上半段做的事）：先连 DB/Redis → 造 repository（注入 db）→ 造 service（注入 repository）→ 造 handler（注入 service）→ 注册路由。这条"自底向上"的组装链，就是依赖注入的手动版本。

---

## 8. 小结：Go 基础清单（自测）

读完能回答这些，Go 基础就通了：

- [ ] `package main` 和普通包的区别？`internal/` 目录有什么特殊？
- [ ] `var`、`=`、`:=`、`const` 分别什么时候用？
- [ ] 为什么函数最后一个返回值几乎总是 `error`？`if err != nil` 为什么到处都是？
- [ ] `fmt.Errorf("...%w", err)` 的 `%w` 有什么用？`errors.Is` 怎么配合？
- [ ] struct tag（`gorm:""` / `json:""`）是给谁看的？`json:"-"` 为什么重要？
- [ ] 指针接收者 `(r *UserRepository)` 和值接收者 `(User)` 的区别？
- [ ] `NewXxx` 函数 + 依赖注入解决了什么问题？
- [ ] interface 类型断言 `x.(string)` 的两返回值写法？
- [ ] `context.Context` 为什么要一路往下传？

> 下一步（Next rep）：打开真实的 `apps/api/internal/handler/auth_handler.go`，对照本文，试着**自己给 `Login` 方法逐行写一遍注释**，再和这里对比。
