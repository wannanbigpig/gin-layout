# 依赖替代方案说明

本文档列出了项目中存在许可证问题的依赖及其替代方案。

## 📋 问题依赖概览

### 1. GPL 许可证依赖（modernc.org 系列）

**依赖项：**
- `modernc.org/libc` (v1.66.10) - GPL-2.0/GPL-3.0
- `modernc.org/sqlite` (v1.40.0) - GPL-2.0/GPL-3.0
- `modernc.org/mathutil` (v1.7.1) - GPL-2.0/GPL-3.0
- `modernc.org/memory` (v1.11.0) - GPL-2.0/GPL-3.0

**引入路径：**
```
casbin/gorm-adapter/v3 → glebarez/sqlite → modernc.org/sqlite → modernc.org/libc
```

**问题：**
- GPL 是 copyleft 许可证，可能要求项目也采用 GPL
- 项目实际只使用 MySQL，不需要 SQLite 支持

### 2. FreeType 许可证依赖

**依赖项：**
- `github.com/golang/freetype` (v0.0.0-20170609003504-e2365dfdc4a0)

**引入路径：**
```
mojocn/base64Captcha → golang/freetype
```

**用途：** 验证码生成

### 3. MPL-2.0 许可证依赖

**依赖项：**
- `github.com/go-sql-driver/mysql` (v1.9.3) - MPL-2.0

**说明：** MPL-2.0 相对宽松，通常与 MIT 兼容，但需要注意许可证要求。

---

## 🔄 替代方案

### 方案一：替换 Casbin GORM Adapter（推荐）

**问题：** `casbin/gorm-adapter/v3` 强制包含 SQLite 支持，导致引入 GPL 依赖。

**替代方案：**

#### 1. 使用 Casbin 的 MySQL 专用适配器

```go
// 使用 casbin 的 xorm-adapter（如果支持 MySQL）
import (
    xormadapter "github.com/casbin/xorm-adapter/v2"
    _ "github.com/go-sql-driver/mysql"
)

adapter, err := xormadapter.NewAdapter("mysql", dsn)
```

**注意：** 需要检查 xorm-adapter 是否也包含 SQLite 支持。

#### 2. 实现自定义 MySQL Adapter

创建一个只支持 MySQL 的自定义适配器：

```go
// internal/pkg/utils/casbin/mysql_adapter.go
package casbinx

import (
    "github.com/casbin/casbin/v2/persist"
    "gorm.io/gorm"
)

type MySQLAdapter struct {
    db *gorm.DB
}

func NewMySQLAdapter(db *gorm.DB) persist.Adapter {
    return &MySQLAdapter{db: db}
}

// 实现 persist.Adapter 接口的所有方法
func (a *MySQLAdapter) LoadPolicy(model casbin.Model) error {
    // 从 MySQL 加载策略
}

func (a *MySQLAdapter) SavePolicy(model casbin.Model) error {
    // 保存策略到 MySQL
}

// ... 其他方法
```

**优点：**
- 完全控制依赖
- 无 GPL 许可证问题
- 只包含需要的功能

**缺点：**
- 需要自己实现和维护
- 需要测试兼容性

#### 3. 使用文件或内存适配器（不推荐用于生产）

```go
import (
    "github.com/casbin/casbin/v2"
    fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
)

// 文件适配器
adapter := fileadapter.NewAdapter("policy.csv")

// 内存适配器
adapter := casbin.NewEnforcer(model) // 无持久化
```

**缺点：**
- 文件适配器不适合多实例部署
- 内存适配器不支持持久化

---

### 方案二：替换验证码库

**问题：** `mojocn/base64Captcha` 依赖 `golang/freetype`。

**替代方案：**

#### 1. 使用 `github.com/mojocn/base64Captcha` 的纯 Go 实现

检查是否有不依赖 freetype 的版本或分支。

#### 2. 使用其他验证码库

**选项 A：`github.com/dchest/captcha`** (MIT License)
```go
import "github.com/dchest/captcha"

// 生成验证码
id := captcha.New()
captcha.WriteImage(w, id, 200, 80)

// 验证
if captcha.VerifyString(id, value) {
    // 验证通过
}
```

**选项 B：`github.com/steambap/captcha`** (MIT License)
```go
import "github.com/steambap/captcha"

img, err := captcha.New(200, 80, func(options *captcha.Options) {
    options.CharPreset = "0123456789"
})
```

**选项 C：自实现简单验证码**

如果需求简单，可以自己实现一个不依赖外部字体的验证码生成器。

---

### 方案三：替换 MySQL 驱动（可选）

**当前：** `github.com/go-sql-driver/mysql` (MPL-2.0)

**替代方案：**

#### 使用 `github.com/go-sql-driver/mysql` 的替代品

实际上，Go 的 MySQL 驱动选择有限，`go-sql-driver/mysql` 是最成熟和广泛使用的。

**其他选项：**
- `github.com/ziutek/mymysql` - 但可能不再维护
- `github.com/go-mysql-org/go-mysql` - 主要用于复制，不适合常规使用

**建议：** MPL-2.0 许可证相对宽松，通常可以接受。如果必须避免，可以考虑使用 GORM 的抽象层，但底层仍可能使用该驱动。

---

## 🎯 推荐方案

### 短期方案（最小改动）

1. **保持现状** - 接受 GPL 间接依赖
   - 在文档中明确说明
   - 这些是间接依赖，通常不影响项目许可证
   - 如果项目本身就是 MIT，可能需要咨询法律顾问

2. **替换验证码库** - 使用 `dchest/captcha` 或 `steambap/captcha`
   - 相对简单的替换
   - 可以完全避免 freetype 依赖

### 长期方案（彻底解决）

1. **实现自定义 MySQL Adapter**
   - 完全控制依赖
   - 无许可证问题
   - 需要一定开发工作

2. **Fork 并修改 casbin/gorm-adapter**
   - 移除 SQLite 支持
   - 只保留 MySQL 支持
   - 维护成本较高

---

## 📝 实施步骤

### 替换验证码库（推荐先做）

1. 安装新库：
```bash
go get github.com/dchest/captcha
# 或
go get github.com/steambap/captcha
```

2. 修改 `pkg/utils/captcha/captcha.go`

3. 修改 `internal/controller/admin_v1/auth.go`

4. 运行测试确保功能正常

5. 移除旧依赖：
```bash
go mod tidy
```

### 替换 Casbin Adapter（需要更多工作）

1. 实现自定义 MySQL Adapter
2. 修改 `internal/pkg/utils/casbin/casbin.go`
3. 充分测试权限功能
4. 移除 `casbin/gorm-adapter/v3` 依赖

---

## ⚠️ 注意事项

1. **许可证兼容性**
   - GPL 是 copyleft 许可证
   - 如果项目需要保持 MIT 许可证，需要完全避免 GPL 依赖
   - 间接依赖的法律影响需要咨询法律顾问

2. **功能兼容性**
   - 替换依赖前需要充分测试
   - 确保新库的功能满足需求

3. **维护成本**
   - 自定义实现需要长期维护
   - 需要考虑团队的技术能力

---

## 📚 参考资源

- [Casbin 适配器列表](https://casbin.org/docs/en/adapters)
- [Go 验证码库对比](https://github.com/topics/captcha?l=go)
- [GPL 许可证说明](https://www.gnu.org/licenses/gpl-faq.html)
- [MPL-2.0 许可证说明](https://www.mozilla.org/en-US/MPL/2.0/)

