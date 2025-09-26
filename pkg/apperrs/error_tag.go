package apperrs

// ErrorTag 错误标签常量，用于错误分类和统计
// 遵循统一的标签命名规范，便于错误监控和追踪

// 系统层面标签
const (
	TagDatabase = "database" // 数据库相关错误
	TagCache    = "cache"    // 缓存相关错误
	TagServer   = "server"   // 服务器相关错误
	TagInternal = "internal" // 内部错误
	TagExternal = "external" // 外部服务错误
	TagAPI      = "api"      // API调用错误
	TagNetwork  = "network"  // 网络相关错误
)

// 业务层面标签
const (
	TagAuth       = "auth"       // 认证相关错误
	TagSecurity   = "security"   // 安全相关错误
	TagValidation = "validation" // 数据验证错误
	TagBusiness   = "business"   // 业务逻辑错误
	TagLogic      = "logic"      // 逻辑处理错误
	TagConflict   = "conflict"   // 资源冲突错误
	TagResource   = "resource"   // 资源相关错误
)

// 请求层面标签
const (
	TagClient  = "client"  // 客户端错误
	TagRequest = "request" // 请求相关错误
	TagInput   = "input"   // 输入参数错误
	TagOutput  = "output"  // 输出响应错误
)

// 功能模块标签
const (
	TagUser     = "user"     // 用户模块
	TagAccount  = "account"  // 账户模块
	TagProfile  = "profile"  // 用户资料模块
	TagPassword = "password" // 密码相关
	TagEmail    = "email"    // 邮箱相关
	TagUpload   = "upload"   // 文件上传
	TagDownload = "download" // 文件下载
)

// 操作类型标签
const (
	TagCreate = "create" // 创建操作
	TagRead   = "read"   // 读取操作
	TagUpdate = "update" // 更新操作
	TagDelete = "delete" // 删除操作
	TagQuery  = "query"  // 查询操作
)

// 性能相关标签
const (
	TagTimeout    = "timeout"    // 超时错误
	TagRateLimit  = "ratelimit"  // 频率限制
	TagOverload   = "overload"   // 系统过载
	TagMemory     = "memory"     // 内存相关
	TagDiskSpace  = "diskspace"  // 磁盘空间
	TagConcurrent = "concurrent" // 并发相关
)

// 数据相关标签
const (
	TagSQL     = "sql"     // SQL相关错误
	TagMigrate = "migrate" // 数据迁移
	TagBackup  = "backup"  // 数据备份
	TagRestore = "restore" // 数据恢复
	TagCorrupt = "corrupt" // 数据损坏
	TagConsist = "consist" // 数据一致性
)

// 第三方服务标签
const (
	TagPayment = "payment" // 支付服务
	TagSMS     = "sms"     // 短信服务
	TagEmail3  = "email3"  // 第三方邮件服务（区别于内部邮箱功能）
	TagOAuth   = "oauth"   // OAuth认证
	TagWeChat  = "wechat"  // 微信相关
	TagAlipay  = "alipay"  // 支付宝相关
)

// 常用标签组合预定义
var (
	// 认证安全相关标签组合
	TagsAuthSecurity = []string{TagAuth, TagSecurity}

	// 数据库操作相关标签组合
	TagsDatabaseOp = []string{TagDatabase, TagSQL}

	// 客户端请求错误标签组合
	TagsClientRequest = []string{TagClient, TagRequest}

	// 业务验证错误标签组合
	TagsBusinessValidation = []string{TagBusiness, TagValidation}

	// 外部API调用错误标签组合
	TagsExternalAPI = []string{TagExternal, TagAPI}

	// 服务器内部错误标签组合
	TagsServerInternal = []string{TagServer, TagInternal}
)
