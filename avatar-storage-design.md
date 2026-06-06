# 头像存储方案对比

## 背景

头像类图片存在跨账号展示需求（用户列表、消息通知、审计日志都要显示别人头像），但图片选择器不应让 A 账号浏览 B 账号的头像图库。需要确定头像的存储、上传、访问方案。

---

## 方案一：系统隐藏目录 + 专用接口

头像继续走文件资源系统，但用系统保留隐藏目录隔离。

### 设计

| 维度 | 做法 |
|---|---|
| 存储目录 | `/system/avatars`，`is_hidden=1`, `created_by=0` |
| 数据库 | 复用 `file_resource` 表，新增 `is_hidden` 字段 |
| 上传接口 | `POST /admin/v1/admin-user/avatar/upload`，AuthModeLogin |
| 后端约束 | folder 固定隐藏目录，is_public=1，upload_scene=avatar，max_size=2MB，仅 jpg/jpeg/png/webp |
| 更新逻辑 | 上传后直接更新用户 avatar 字段 + BindReference |
| 文件选择器 | 默认排除 `is_hidden=1` 目录及文件 |
| 管理员维护 | 通过 `include_hidden=1` 参数查看隐藏目录 |
| 访问控制 | URL 公开访问（is_public=1） |

### 落地步骤

1. 迁移：`file_resource_folder` 表新增 `is_hidden` 字段，插入 `/system/avatars` 记录
2. 后端：新增头像上传接口，固定业务规则
3. 后端：上传后更新用户 avatar + BindReference
4. 后端：文件列表接口默认过滤 `is_hidden=1`
5. 前端：头像组件调专用接口
6. 前端：文件选择器排除隐藏目录

### 优点

- 头像仍可在文件资源系统中统一管理（管理员视角）
- BindReference 机制复用，引用关系清晰
- 文件存储层统一，不分散

### 缺点

- `is_hidden` 是为头像一个场景引入的通用机制，过度设计
- 头像本质是用户属性，塞进文件资源系统再打补丁排除，职责不清
- 文件选择器、文件列表、目录查询三处需要配合过滤逻辑
- 后续如果其他业务也需要"隐藏"语义，`is_hidden` 的含义容易膨胀

---

## 方案二：独立存储 + 专用接口

头像彻底脱离文件资源系统，独立存储和管理。

### 设计

| 维度 | 做法 |
|---|---|
| 存储路径 | 磁盘 `/data/avatars/{user_id}/{filename}` 或 OSS `avatars/{user_id}/` |
| 数据库 | 新建 `admin_user_avatar` 表（user_id, path, file_size, mime_type, created_at）或扩展 `admin_user` 表 |
| 上传接口 | `POST /admin/v1/admin-user/avatar/upload`，AuthModeLogin |
| 后端约束 | max_size=2MB，仅 jpg/jpeg/png/webp，路径后端生成 |
| 更新逻辑 | 上传成功直接覆盖旧记录，删除旧文件 |
| 文件选择器 | 不涉及，头像组件独立 |
| 访问控制 | URL 公开访问 |

### 表结构参考

```sql
CREATE TABLE admin_user_avatar (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id     BIGINT UNSIGNED NOT NULL UNIQUE,
    path        VARCHAR(500) NOT NULL,
    file_size   INT UNSIGNED NOT NULL DEFAULT 0,
    mime_type   VARCHAR(50) NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

或直接扩展 `admin_user` 表：

```sql
ALTER TABLE admin_user ADD COLUMN avatar_path VARCHAR(500) DEFAULT NULL AFTER avatar;
```

### 落地步骤

1. 迁移：新建 `admin_user_avatar` 表或扩展 `admin_user` 表
2. 后端：新增头像上传接口，写入独立存储
3. 后端：上传成功后删除旧文件、更新记录
4. 后端：用户信息接口返回头像 URL
5. 前端：头像组件独立，调专用接口

### 优点

- 职责清晰：头像是用户属性，不污染文件资源系统
- 文件选择器无需改动，不存在跨账号浏览问题
- 无需新增 `is_hidden` 字段，不引入额外概念
- 耦合点少：只有头像上传接口一个入口
- 查询简单：直接关联用户表，不需要 JOIN 文件资源表

### 缺点

- 头像不在文件资源系统中，管理员无法通过文件管理页统一查看
- 需要单独写头像存储/删除逻辑，不能复用文件资源的清理机制
- 如果未来需要"从文件库选图作头像"，需要额外打通

---

## 对比总结

| 对比项 | 方案一（隐藏目录） | 方案二（独立存储） |
|---|---|---|
| 职责划分 | 文件系统承载用户属性，打补丁 | 各管各的，边界清晰 |
| 文件资源表 | 需要 `is_hidden` 字段 | 干净，只存用户文件 |
| 文件选择器 | 需要排除逻辑 | 不需要改动 |
| 新增概念 | `is_hidden` 隐藏机制 | 无 |
| 跨账号防护 | 目录隔离 + 隐藏标记 + 选择器过滤 | 根本不存在此问题 |
| 管理员可见 | 可通过参数查看 | 不在文件管理中（可加单独页面） |
| 代码耦合 | 文件系统 + 选择器 + 头像接口 | 仅头像接口 |
| 复用 BindReference | 可以 | 不需要，直接管理 |
| 未来扩展"从文件库选图" | 天然支持 | 需额外打通 |

## 推荐

**方案二（独立存储）**。

理由：
1. 头像是用户属性，不是文件资源，强行塞入再排除是职责倒置
2. `is_hidden` 为单一场景引入通用机制，收益不足
3. 方案二耦合点最少，维护成本最低
4. "从文件库选图作头像"的需求目前不存在，不应为假设需求增加复杂度
