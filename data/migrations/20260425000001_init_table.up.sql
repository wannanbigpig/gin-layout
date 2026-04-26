BEGIN;

-- 创建管理员表
CREATE TABLE IF NOT EXISTS `admin_user`
(
    `id`                int unsigned                                                 NOT NULL AUTO_INCREMENT,
    `nickname`          varchar(30)                                                  NOT NULL DEFAULT '' COMMENT '昵称',
    `username`          varchar(30)                                                  NOT NULL DEFAULT '' COMMENT '用户名',
    `password`          varchar(255)                                                 NOT NULL DEFAULT '' COMMENT '密码',
    `phone_number`      varchar(15) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT '手机号',
    `full_phone_number` varchar(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT '带区号的手机号',
    `country_code`      varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT '国际区号',
    `email`             varchar(120)                                                 NOT NULL DEFAULT '' COMMENT '邮箱',
    `avatar`            varchar(255)                                                 NOT NULL DEFAULT '' COMMENT '头像',
    `status`            tinyint(1)                                                   NOT NULL DEFAULT '1' COMMENT '状态 1启用 0禁用',
    `is_super_admin`    tinyint(1)                                                   NOT NULL DEFAULT '1' COMMENT '是否超级管理员（拥有所有权限） 1是 0不是',
    `last_login`        datetime                                                              DEFAULT NULL COMMENT '最后登录时间',
    `last_ip`           varchar(45) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT '最后登录IP',
    `created_at`        datetime                                                              DEFAULT NULL,
    `updated_at`        datetime                                                              DEFAULT NULL,
    `deleted_at`        int                                                          NOT NULL DEFAULT '0' COMMENT '删除时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `adu_u_d` (`username`, `deleted_at`),
    KEY `idx_status_deleted_at` (`status`, `deleted_at`),
    KEY `idx_full_phone_number_deleted_at` (`full_phone_number`, `deleted_at`),
    KEY `idx_email_deleted_at` (`email`, `deleted_at`),
    KEY `idx_created_at_deleted_at` (`created_at`, `deleted_at`)
) ENGINE = InnoDB
  AUTO_INCREMENT = 10000
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='后台管理用户表';

-- 创建权限分组表
CREATE TABLE IF NOT EXISTS `api_group`
(
    `id`         int unsigned                                          NOT NULL AUTO_INCREMENT,
    `pid`        int unsigned                                          NOT NULL DEFAULT '0' COMMENT '上级组织id',
    `code`       varchar(30) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT 'code',
    `name`       varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '分组名称',
    `created_at` datetime                                                       DEFAULT NULL,
    `updated_at` datetime                                                       DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `ag_code` (`code`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='权限分组表';

-- 创建权限表
CREATE TABLE IF NOT EXISTS `api`
(
    `id`           int unsigned                                           NOT NULL AUTO_INCREMENT,
    `code`         varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '权限唯一code码',
    `group_code`   varchar(36) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '分组唯一code码',
    `name`         varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '权限名称',
    `description`  varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '描述',
    `method`       varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '接口请求方法',
    `route`        varchar(160) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '接口路由',
    `func`         varchar(80) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '接口方法',
    `func_path`    varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '接口方法路径',
    `is_auth`      tinyint unsigned                                       NOT NULL DEFAULT '1' COMMENT '接口鉴权模式 0无需登录 1需要登录 2需要登录且需要API权限',
    `is_effective` tinyint unsigned                                       NOT NULL DEFAULT '1' COMMENT '接口是否可用 1是 0否',
    `sort`         int unsigned                                           NOT NULL DEFAULT '0' COMMENT '排序',
    `created_at`   datetime                                                        DEFAULT NULL,
    `updated_at`   datetime                                                        DEFAULT NULL,
    `deleted_at`   int                                                    NOT NULL DEFAULT '0' COMMENT '删除时间戳',
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `api_uniq_code_del` (`code`, `deleted_at`) USING BTREE,
    KEY `api_idx_route_method_deleted_at` (`route`, `method`, `deleted_at`) USING BTREE,
    KEY `idx_group_code_deleted_at_sort` (`group_code`, `deleted_at`, `sort`) USING BTREE,
    KEY `idx_is_auth_deleted_at` (`is_auth`, `deleted_at`) USING BTREE,
    KEY `idx_updated_at` (`updated_at`) USING BTREE
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='权限表';

-- 创建菜单表
CREATE TABLE IF NOT EXISTS `menu`
(
    `id`                int                                                    NOT NULL AUTO_INCREMENT,
    `icon`              varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '图标',
    `code`              varchar(120) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '前端权限标识',
    `path`              varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '前端路由路径',
    `full_path`         varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '完整前端路由路径',
    `redirect`          varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '重定向路由名称',
    `name`              varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '前端路由名称',
    `component`         varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '前端组件路径',
    `animate_enter`     varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '进入动画，动画类参考https://animate.style/',
    `animate_leave`     varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '离开动画，动画类参考https://animate.style/',
    `animate_duration`  float(2, 2)                                            NOT NULL DEFAULT '0.00' COMMENT '动画持续时间',
    `is_show`           tinyint                                                NOT NULL DEFAULT '0' COMMENT '是否显示，1是 0否',
    `status`            tinyint                                                NOT NULL DEFAULT '0' COMMENT '状态，1正常 0禁用',
    `is_auth`           tinyint                                                NOT NULL DEFAULT '0' COMMENT '是否需要授权，1是 0否 ',
    `is_external_links` tinyint                                                NOT NULL DEFAULT '0' COMMENT '是否外链，1是 0否 ',
    `is_new_window`     tinyint                                                NOT NULL DEFAULT '0' COMMENT '是否新窗口打开, 1是 0否',
    `sort`              int                                                    NOT NULL DEFAULT '0' COMMENT '排序，数字越大，排名越靠前',
    `type`              tinyint                                                NOT NULL DEFAULT '1' COMMENT '菜单类型，1目录，2菜单，3按钮',
    `pid`               int                                                    NOT NULL DEFAULT '0' COMMENT '上级菜单id',
    `level`             tinyint                                                NOT NULL DEFAULT '0' COMMENT '层级',
    `pids`              varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '层级序列，多个用英文逗号隔开',
    `children_num`      int                                                    NOT NULL DEFAULT '0' COMMENT '子集数量',
    `description`       varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '描述',
    `created_at`        datetime                                                        DEFAULT NULL COMMENT '创建时间',
    `updated_at`        datetime                                                        DEFAULT NULL COMMENT '更新时间',
    `deleted_at`        int                                                    NOT NULL DEFAULT '0' COMMENT '删除时间戳',
    PRIMARY KEY (`id`) USING BTREE,
    KEY `uniq_code_del` (`code`, `deleted_at`) USING BTREE,
    KEY `idx_name_del` (`name`, `deleted_at`) USING BTREE,
    KEY `idx_path_del` (`path`, `deleted_at`) USING BTREE,
    KEY `idx_is_auth_del` (`is_auth`, `deleted_at`) USING BTREE,
    KEY `idx_status_del` (`status`, `deleted_at`) USING BTREE,
    KEY `idx_pid_deleted_at_sort_id` (`pid`, `deleted_at`, `sort`, `id`) USING BTREE,
    KEY `idx_pids_deleted_at` (`pids`, `deleted_at`) USING BTREE
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='菜单表';

-- 创建菜单多语言标题表
CREATE TABLE IF NOT EXISTS `menu_i18n`
(
    `id`         int unsigned                                           NOT NULL AUTO_INCREMENT,
    `menu_id`    int unsigned                                           NOT NULL DEFAULT '0' COMMENT '菜单ID',
    `locale`     varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '语言代码，如 zh-CN、en-US',
    `title`      varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '菜单标题',
    `created_at` datetime                                                        DEFAULT NULL,
    `updated_at` datetime                                                        DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `uniq_menu_id_locale` (`menu_id`, `locale`) USING BTREE,
    KEY `idx_locale_menu_id` (`locale`, `menu_id`) USING BTREE
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='菜单多语言标题表';

-- 创建组织表
CREATE TABLE IF NOT EXISTS `department`
(
    `id`          int unsigned                                           NOT NULL AUTO_INCREMENT,
    `code`        varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '部门业务编码',
    `is_system`   tinyint                                                NOT NULL DEFAULT '0' COMMENT '是否系统保留对象,1是 0否',
    `pid`         int unsigned                                           NOT NULL DEFAULT '0' COMMENT '上级部门id',
    `pids`        varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '',
    `name`        varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '部门名称',
    `description` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '描述',
    `level`       tinyint                                                NOT NULL DEFAULT '1' COMMENT '层级',
    `sort`        int                                                    NOT NULL DEFAULT '0' COMMENT '排序',
    `children_num` int                                                   NOT NULL DEFAULT '0' COMMENT '子集数量',
    `user_number` int                                                    NOT NULL DEFAULT '0' COMMENT '部门用户数量',
    `created_at`  datetime                                                        DEFAULT NULL,
    `updated_at`  datetime                                                        DEFAULT NULL,
    `deleted_at`  int                                                    NOT NULL DEFAULT '0' COMMENT '删除时间戳',
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `uniq_code_deleted_at` (`code`, `deleted_at`),
    KEY `idx_name_deleted_at` (`name`, `deleted_at`),
    KEY `idx_is_system_deleted_at` (`is_system`, `deleted_at`),
    KEY `idx_pid_deleted_at_sort_id` (`pid`, `deleted_at`, `sort`, `id`),
    KEY `idx_pids_deleted_at` (`pids`, `deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='部门表';

-- 创建角色表
CREATE TABLE IF NOT EXISTS `role`
(
    `id`          int unsigned                                           NOT NULL AUTO_INCREMENT,
    `code`        varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '角色业务编码',
    `is_system`   tinyint                                                NOT NULL DEFAULT '0' COMMENT '是否系统保留对象,1是 0否',
    `pid`         int unsigned                                           NOT NULL DEFAULT '0' COMMENT '上级id',
    `pids`        varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '上级id路径链',
    `name`        varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '角色名称',
    `description` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '描述',
    `level`       tinyint                                                NOT NULL DEFAULT '1' COMMENT '层级',
    `sort`        mediumint                                              NOT NULL DEFAULT '0' COMMENT '排序',
    `children_num` int unsigned                                          NOT NULL DEFAULT '0' COMMENT '子集数量',
    `status`      tinyint                                                NOT NULL DEFAULT '0' COMMENT '是否启用状态,1是，0否',
    `created_at`  datetime                                                        DEFAULT NULL,
    `updated_at`  datetime                                                        DEFAULT NULL,
    `deleted_at`  int                                                    NOT NULL DEFAULT '0' COMMENT '删除时间戳',
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `uniq_code_deleted_at` (`code`, `deleted_at`),
    KEY `idx_name_deleted_at` (`name`, `deleted_at`),
    KEY `idx_is_system_deleted_at` (`is_system`, `deleted_at`),
    KEY `idx_pid_deleted_at_sort_id` (`pid`, `deleted_at`, `sort`, `id`),
    KEY `idx_status_deleted_at` (`status`, `deleted_at`),
    KEY `idx_pids_deleted_at` (`pids`, `deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='角色表';

-- 创建用户部门映射表
CREATE TABLE IF NOT EXISTS `admin_user_department_map`
(
    `id`         int unsigned NOT NULL AUTO_INCREMENT,
    `uid`        int unsigned NOT NULL DEFAULT '0' COMMENT 'admin_users表id',
    `dept_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '部门id，department表id',
    `is_admin`   tinyint      NOT NULL DEFAULT '0' COMMENT '是否管理员，1是，0否',
    `created_at` datetime              DEFAULT NULL,
    `updated_at` datetime              DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `idx_uid_dept_id` (`uid`, `dept_id`),
    KEY `idx_dept_id_uid` (`dept_id`, `uid`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='用户部门映射表';

-- 创建菜单权限映射表
CREATE TABLE IF NOT EXISTS `menu_api_map`
(
    `id`         int unsigned NOT NULL AUTO_INCREMENT,
    `menu_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '菜单id,对应menu表id',
    `api_id`     int unsigned NOT NULL DEFAULT '0' COMMENT '接口id,对应api表id',
    `created_at` datetime              DEFAULT NULL,
    `updated_at` datetime              DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `idx_menu_id_api_id` (`menu_id`, `api_id`),
    KEY `idx_api_id_menu_id` (`api_id`, `menu_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='菜单权限映射表';

-- menu_api_map 维护菜单与 API 的静态关系。
-- api 表由 `go-layout command api-route` 或 `init-system` 写入后，
-- 默认菜单与 API 的初始绑定由 Go 初始化逻辑幂等补齐。

-- 创建角色菜单映射表
CREATE TABLE IF NOT EXISTS `role_menu_map`
(
    `id`         int unsigned NOT NULL AUTO_INCREMENT,
    `role_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '角色id,对应roles表id',
    `menu_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '菜单id,对应menu表id',
    `created_at` datetime              DEFAULT NULL,
    `updated_at` datetime              DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `idx_role_id_menu_id` (`role_id`, `menu_id`),
    KEY `idx_menu_id_role_id` (`menu_id`, `role_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='角色菜单映射表';

-- 创建用户菜单映射表
CREATE TABLE IF NOT EXISTS `admin_user_menu_map`
(
    `id`         int unsigned NOT NULL AUTO_INCREMENT,
    `uid`        int unsigned NOT NULL DEFAULT '0' COMMENT 'uid,admin_users表id',
    `menu_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '菜单id',
    `created_at` datetime              DEFAULT NULL,
    `updated_at` datetime              DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `idx_uid_menu_id` (`uid`, `menu_id`),
    KEY `idx_menu_id_uid` (`menu_id`, `uid`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='用户菜单映射表';

-- 创建部门角色映射表
CREATE TABLE IF NOT EXISTS `department_role_map`
(
    `id`         int unsigned NOT NULL AUTO_INCREMENT,
    `dept_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '部门id,对应department表id',
    `role_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '角色id,对应roles表id',
    `created_at` datetime              DEFAULT NULL,
    `updated_at` datetime              DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `idx_dept_id_role_id` (`dept_id`, `role_id`),
    KEY `idx_role_id_dept_id` (`role_id`, `dept_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='部门角色映射表';

-- 创建用户角色映射表
CREATE TABLE IF NOT EXISTS `admin_user_role_map`
(
    `id`         int unsigned NOT NULL AUTO_INCREMENT,
    `uid`        int unsigned NOT NULL DEFAULT '0' COMMENT 'uid,admin_users表id',
    `role_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '角色id',
    `created_at` datetime              DEFAULT NULL,
    `updated_at` datetime              DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `idx_uid_role_id` (`uid`, `role_id`),
    KEY `idx_role_id_uid` (`role_id`, `uid`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='用户角色映射表';

-- 创建管理员登录日志表
CREATE TABLE IF NOT EXISTS `admin_login_logs`
(
    `id`                 bigint unsigned  NOT NULL AUTO_INCREMENT,
    `uid`                int unsigned     NOT NULL DEFAULT '0' COMMENT '用户ID（登录失败时为0）',
    `username`           varchar(50)      NOT NULL DEFAULT '' COMMENT '登录账号',
    `jwt_id`             char(36)         NOT NULL DEFAULT '' COMMENT 'JWT唯一标识(jti claim)',
    `access_token`       text                      DEFAULT NULL COMMENT '访问令牌（加密保存）',
    `refresh_token`      text                      DEFAULT NULL COMMENT '刷新令牌（加密保存）',
    `token_hash`         char(64)         NOT NULL DEFAULT '' COMMENT 'Token的SHA256哈希值',
    `refresh_token_hash` char(64)         NOT NULL DEFAULT '' COMMENT 'Refresh Token的哈希值',
    `ip`                 varchar(45)      NOT NULL DEFAULT '' COMMENT '登录IP(支持IPv6)',
    `user_agent`         varchar(1024)    NOT NULL DEFAULT '' COMMENT '用户代理（浏览器/设备信息）',
    `os`                 varchar(50)      NOT NULL DEFAULT '' COMMENT '操作系统',
    `browser`            varchar(50)      NOT NULL DEFAULT '' COMMENT '浏览器',
    `execution_time`     int(11)          NOT NULL DEFAULT '0' COMMENT '登录耗时（毫秒）',
    `login_status`       tinyint(1)       NOT NULL DEFAULT '1' COMMENT '登录状态：1=成功, 0=失败',
    `login_fail_reason`  varchar(255)     NOT NULL DEFAULT '' COMMENT '登录失败原因',
    `type`               tinyint(1)       NOT NULL DEFAULT '1' COMMENT '操作类型：1=登录操作, 2=刷新token',
    `is_revoked`         tinyint(1)       NOT NULL DEFAULT '0' COMMENT '是否被撤销：0=否, 1=是',
    `revoked_code`       tinyint(1)       NOT NULL DEFAULT '0' COMMENT '撤销原因码：1=用户主动登出（退出登录）, 2=系统强制登出（账号被封）, 3=系统刷新token, 4=用户禁用（针对某个设备下线操作）, 5=其他原因, 6=用户自己修改密码, 7=管理员修改用户密码',
    `revoked_reason`     varchar(255)     NOT NULL DEFAULT '' COMMENT '撤销原因',
    `revoked_at`         datetime                  DEFAULT NULL COMMENT '撤销时间',
    `token_expires`      datetime                  DEFAULT NULL COMMENT 'Token过期时间',
    `refresh_expires`    datetime                  DEFAULT NULL COMMENT 'Refresh Token过期时间',
    `created_at`         datetime                  DEFAULT NULL,
    `updated_at`         datetime                  DEFAULT NULL,
    `deleted_at`         int              NOT NULL DEFAULT '0' COMMENT '删除时间戳',
    PRIMARY KEY (`id`),
    KEY `aall_deleted_at_created_at` (`deleted_at`, `created_at`),
    KEY `aall_login_status_deleted_at_created_at` (`login_status`, `deleted_at`, `created_at`),
    KEY `aall_is_revoked_deleted_at_revoked_at` (`is_revoked`, `deleted_at`, `revoked_at`),
    KEY `aall_uid_deleted_at_is_revoked_login_status_token_expires` (`uid`, `deleted_at`, `is_revoked`, `login_status`, `token_expires`),
    KEY `aall_jwt_id_deleted_at_is_revoked` (`jwt_id`, `deleted_at`, `is_revoked`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='管理员登录日志表';

-- 创建登录安全状态表
CREATE TABLE IF NOT EXISTS `login_security_state`
(
    `id`             int unsigned NOT NULL AUTO_INCREMENT,
    `username`       varchar(50)  NOT NULL DEFAULT '' COMMENT '登录账号',
    `fail_count`     int unsigned NOT NULL DEFAULT '0' COMMENT '连续失败次数',
    `lock_until`     datetime              DEFAULT NULL COMMENT '锁定截止时间',
    `last_failed_at` datetime              DEFAULT NULL COMMENT '最近失败时间',
    `created_at`     datetime              DEFAULT NULL,
    `updated_at`     datetime              DEFAULT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `lss_username` (`username`),
    KEY `lss_lock_until` (`lock_until`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='登录安全状态表';

-- 创建请求日志表
CREATE TABLE IF NOT EXISTS `request_logs`
(
    `id`              bigint(20)   NOT NULL AUTO_INCREMENT COMMENT '日志ID',
    `request_id`      varchar(64)  NOT NULL DEFAULT '' COMMENT '请求唯一标识',
    `jwt_id`          varchar(36)  NOT NULL DEFAULT '' COMMENT '请求授权的jwtId',
    `operator_id`     bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '操作ID（用户ID）',
    `ip`              varchar(45)  NOT NULL DEFAULT '' COMMENT '客户端IP地址',
    `user_agent`      varchar(1024) NOT NULL DEFAULT '' COMMENT '用户代理（浏览器/设备信息）',
    `os`             varchar(50)  NOT NULL DEFAULT '' COMMENT '操作系统',
    `browser`        varchar(50)  NOT NULL DEFAULT '' COMMENT '浏览器',
    `method`          varchar(10)  NOT NULL DEFAULT '' COMMENT 'HTTP请求方法（GET/POST等）',
    `base_url`        varchar(160) NOT NULL DEFAULT '' COMMENT '请求基础URL',
    `operation_name`  varchar(255) NOT NULL DEFAULT '' COMMENT '操作名称',
    `operation_status` int(11) NOT NULL DEFAULT '0' COMMENT '操作状态码（响应返回的code，0=成功，其他=失败）',
    `is_high_risk`    tinyint(1)    NOT NULL DEFAULT '0' COMMENT '是否高危操作 1是 0否',
    `operator_account` varchar(50) NOT NULL DEFAULT '' COMMENT '操作账号',
    `operator_name`   varchar(50)  NOT NULL DEFAULT '' COMMENT '操作人员',
    `request_headers` text                  DEFAULT NULL COMMENT '请求头（JSON格式）',
    `request_query`   text                  DEFAULT NULL COMMENT '请求参数',
    `request_body`    text                  DEFAULT NULL COMMENT '请求体',
    `change_diff`     longtext              DEFAULT NULL COMMENT '关键变更前后差异（JSON）',
    `response_status` int(11)      NOT NULL DEFAULT '0' COMMENT '响应状态码',
    `response_body`   text                  DEFAULT NULL COMMENT '响应体',
    `response_header` text                  DEFAULT NULL COMMENT '响应头',
    `execution_time`  DECIMAL(10,4) NOT NULL DEFAULT '0.0000' COMMENT '执行时间（毫秒，支持小数，最多4位）',
    `created_at`      datetime              DEFAULT NULL COMMENT '创建时间',
    `updated_at`      datetime              DEFAULT NULL COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY `rl_request_id` (`request_id`),
    KEY `rl_operator_id_created_at` (`operator_id`, `created_at`),
    KEY `rl_base_url_method_created_at` (`base_url`, `method`, `created_at`),
    KEY `rl_operation_status_created_at` (`operation_status`, `created_at`),
    KEY `rl_response_status_operator_id_created_at` (`response_status`, `operator_id`, `created_at`),
    KEY `rl_operator_account_created_at` (`operator_account`, `created_at`),
    KEY `rl_created_at` (`created_at`),
    KEY `rl_jwt_id` (`jwt_id`),
    KEY `rl_is_high_risk_created_at` (`is_high_risk`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='请求日志表';


CREATE TABLE IF NOT EXISTS `casbin_rule`
(
    `id`    bigint unsigned NOT NULL AUTO_INCREMENT,
    `ptype` varchar(100) DEFAULT NULL,
    `v0`    varchar(100) DEFAULT NULL,
    `v1`    varchar(100) DEFAULT NULL,
    `v2`    varchar(100) DEFAULT NULL,
    `v3`    varchar(100) DEFAULT NULL,
    `v4`    varchar(100) DEFAULT NULL,
    `v5`    varchar(100) DEFAULT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_casbin_rule` (`ptype`, `v0`, `v1`, `v2`, `v3`, `v4`, `v5`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='casbin_rule表';

CREATE TABLE IF NOT EXISTS `upload_files`
(
    `id`          int unsigned NOT NULL AUTO_INCREMENT,
    `uid`         int unsigned NOT NULL DEFAULT '0' COMMENT '用户ID',
    `origin_name` varchar(255) NOT NULL DEFAULT '' COMMENT '文件源名称',
    `name`        varchar(255) NOT NULL DEFAULT '' COMMENT '文件名称（UUID+扩展名）',
    `path`        varchar(255) NOT NULL DEFAULT '' COMMENT '文件相对路径（相对于storage/public或storage/private）',
    `size`        int unsigned NOT NULL DEFAULT '0' COMMENT '文件大小（字节）',
    `ext`         varchar(20)  NOT NULL DEFAULT '' COMMENT '文件扩展名',
    `hash`        varchar(64)  NOT NULL DEFAULT '' COMMENT '文件SHA256哈希值（用于去重）',
    `uuid`        varchar(32)  NOT NULL DEFAULT '' COMMENT '文件UUID（用于URL访问，32位十六进制字符串，不带连字符）',
    `mime_type`   varchar(100) NOT NULL DEFAULT '' COMMENT 'MIME类型（如：image/jpeg, application/pdf）',
    `is_public`   tinyint      NOT NULL DEFAULT '0' COMMENT '是否公开访问: 0否 1是',
    `created_at`  datetime              DEFAULT NULL COMMENT '创建时间',
    `updated_at`  datetime              DEFAULT NULL COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY `idx_uid_created_at` (`uid`, `created_at`),
    KEY `idx_hash_is_public` (`hash`, `is_public`),
    UNIQUE KEY `idx_uuid` (`uuid`),
    KEY `idx_is_public_uuid` (`is_public`, `uuid`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='上传文件表';

CREATE TABLE IF NOT EXISTS `sys_config`
(
    `id`           int unsigned                                           NOT NULL AUTO_INCREMENT,
    `config_key`   varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '参数键名',
    `config_value` text                                                            DEFAULT NULL COMMENT '参数值',
    `value_type`   varchar(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT 'string' COMMENT '值类型:string,number,bool,json',
    `group_code`   varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT 'default' COMMENT '参数分组',
    `is_system`    tinyint unsigned                                       NOT NULL DEFAULT '0' COMMENT '是否系统内置:0否,1是',
    `is_sensitive` tinyint unsigned                                       NOT NULL DEFAULT '0' COMMENT '是否敏感配置:0否,1是',
    `status`       tinyint unsigned                                       NOT NULL DEFAULT '1' COMMENT '状态:0禁用,1启用',
    `sort`         int unsigned                                           NOT NULL DEFAULT '0' COMMENT '排序',
    `remark`       varchar(255)                                           NOT NULL DEFAULT '' COMMENT '备注',
    `created_at`   datetime                                                        DEFAULT NULL,
    `updated_at`   datetime                                                        DEFAULT NULL,
    `deleted_at`   int unsigned                                           NOT NULL DEFAULT '0' COMMENT '删除时间戳',
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `uniq_config_key_deleted_at` (`config_key`, `deleted_at`) USING BTREE,
    KEY `idx_group_status_deleted_at_sort` (`group_code`, `status`, `deleted_at`, `sort`) USING BTREE,
    KEY `idx_status_deleted_at` (`status`, `deleted_at`) USING BTREE
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='系统参数表';

CREATE TABLE IF NOT EXISTS `sys_dict_type`
(
    `id`          int unsigned                                           NOT NULL AUTO_INCREMENT,
    `type_code`   varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '字典类型编码',
    `is_system`   tinyint unsigned                                       NOT NULL DEFAULT '0' COMMENT '是否系统内置:0否,1是',
    `status`      tinyint unsigned                                       NOT NULL DEFAULT '1' COMMENT '状态:0禁用,1启用',
    `sort`        int unsigned                                           NOT NULL DEFAULT '0' COMMENT '排序',
    `remark`      varchar(255)                                           NOT NULL DEFAULT '' COMMENT '备注',
    `created_at`  datetime                                                        DEFAULT NULL,
    `updated_at`  datetime                                                        DEFAULT NULL,
    `deleted_at`  int unsigned                                           NOT NULL DEFAULT '0' COMMENT '删除时间戳',
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `uniq_type_code_deleted_at` (`type_code`, `deleted_at`) USING BTREE,
    KEY `idx_status_deleted_at_sort` (`status`, `deleted_at`, `sort`) USING BTREE
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='系统字典类型表';

CREATE TABLE IF NOT EXISTS `sys_dict_item`
(
    `id`         int unsigned                                           NOT NULL AUTO_INCREMENT,
    `type_code`  varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '字典类型编码',
    `value`      varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '字典值',
    `color`      varchar(30)                                            NOT NULL DEFAULT '' COMMENT '展示颜色',
    `tag_type`   varchar(30)                                            NOT NULL DEFAULT '' COMMENT '前端标签类型',
    `is_default` tinyint unsigned                                       NOT NULL DEFAULT '0' COMMENT '是否默认项:0否,1是',
    `is_system`  tinyint unsigned                                       NOT NULL DEFAULT '0' COMMENT '是否系统内置:0否,1是',
    `status`     tinyint unsigned                                       NOT NULL DEFAULT '1' COMMENT '状态:0禁用,1启用',
    `sort`       int unsigned                                           NOT NULL DEFAULT '0' COMMENT '排序',
    `remark`     varchar(255)                                           NOT NULL DEFAULT '' COMMENT '备注',
    `created_at` datetime                                                        DEFAULT NULL,
    `updated_at` datetime                                                        DEFAULT NULL,
    `deleted_at` int unsigned                                           NOT NULL DEFAULT '0' COMMENT '删除时间戳',
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `uniq_type_value_deleted_at` (`type_code`, `value`, `deleted_at`) USING BTREE,
    KEY `idx_type_status_deleted_at_sort` (`type_code`, `status`, `deleted_at`, `sort`) USING BTREE,
    KEY `idx_status_deleted_at` (`status`, `deleted_at`) USING BTREE
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='系统字典项表';

CREATE TABLE IF NOT EXISTS `sys_config_i18n`
(
    `id`          int unsigned                                           NOT NULL AUTO_INCREMENT,
    `config_id`   int unsigned                                           NOT NULL DEFAULT '0' COMMENT '系统参数ID',
    `locale`      varchar(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '语言编码',
    `config_name` varchar(100)                                           NOT NULL DEFAULT '' COMMENT '参数名称',
    `created_at`  datetime                                                        DEFAULT NULL,
    `updated_at`  datetime                                                        DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `uniq_config_id_locale` (`config_id`, `locale`) USING BTREE,
    KEY `idx_locale_config_name` (`locale`, `config_name`) USING BTREE
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='系统参数多语言表';

CREATE TABLE IF NOT EXISTS `sys_dict_type_i18n`
(
    `id`           int unsigned                                           NOT NULL AUTO_INCREMENT,
    `dict_type_id` int unsigned                                           NOT NULL DEFAULT '0' COMMENT '字典类型ID',
    `locale`       varchar(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '语言编码',
    `type_name`    varchar(100)                                           NOT NULL DEFAULT '' COMMENT '字典类型名称',
    `created_at`   datetime                                                        DEFAULT NULL,
    `updated_at`   datetime                                                        DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `uniq_dict_type_id_locale` (`dict_type_id`, `locale`) USING BTREE,
    KEY `idx_locale_type_name` (`locale`, `type_name`) USING BTREE
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='系统字典类型多语言表';

CREATE TABLE IF NOT EXISTS `sys_dict_item_i18n`
(
    `id`           int unsigned                                           NOT NULL AUTO_INCREMENT,
    `dict_item_id` int unsigned                                           NOT NULL DEFAULT '0' COMMENT '字典项ID',
    `locale`       varchar(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '语言编码',
    `label`        varchar(100)                                           NOT NULL DEFAULT '' COMMENT '字典标签',
    `created_at`   datetime                                                        DEFAULT NULL,
    `updated_at`   datetime                                                        DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `uniq_dict_item_id_locale` (`dict_item_id`, `locale`) USING BTREE,
    KEY `idx_locale_label` (`locale`, `label`) USING BTREE
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='系统字典项多语言表';

CREATE TABLE IF NOT EXISTS `task_definitions`
(
    `id`             int unsigned NOT NULL AUTO_INCREMENT,
    `code`           varchar(120) NOT NULL DEFAULT '' COMMENT '任务唯一编码',
    `name`           varchar(120) NOT NULL DEFAULT '' COMMENT '任务名称',
    `kind`           varchar(20)  NOT NULL DEFAULT '' COMMENT '任务类型 async/cron/manual',
    `queue`          varchar(60)  NOT NULL DEFAULT '' COMMENT '队列名称',
    `cron_spec`      varchar(120) NOT NULL DEFAULT '' COMMENT 'Cron 表达式',
    `handler`        varchar(255) NOT NULL DEFAULT '' COMMENT '处理器标识',
    `status`         tinyint(1)   NOT NULL DEFAULT '1' COMMENT '状态 1启用 0停用',
    `allow_manual`   tinyint(1)   NOT NULL DEFAULT '0' COMMENT '是否允许手动触发 1是 0否',
    `allow_retry`    tinyint(1)   NOT NULL DEFAULT '1' COMMENT '是否允许手动重试 1是 0否',
    `is_high_risk`   tinyint(1)   NOT NULL DEFAULT '0' COMMENT '是否高危任务 1是 0否',
    `remark`         varchar(255) NOT NULL DEFAULT '' COMMENT '备注',
    `created_at`     datetime             DEFAULT NULL,
    `updated_at`     datetime             DEFAULT NULL,
    `deleted_at`     int          NOT NULL DEFAULT '0' COMMENT '删除时间戳',
    PRIMARY KEY (`id`),
    UNIQUE KEY `td_code_deleted_at` (`code`, `deleted_at`),
    KEY `td_kind_status_deleted_at` (`kind`, `status`, `deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='任务定义表';

CREATE TABLE IF NOT EXISTS `task_runs`
(
    `id`              bigint unsigned NOT NULL AUTO_INCREMENT,
    `task_code`       varchar(120)    NOT NULL DEFAULT '' COMMENT '任务唯一编码',
    `kind`            varchar(20)     NOT NULL DEFAULT '' COMMENT '任务类型 async/cron/manual',
    `source`          varchar(20)     NOT NULL DEFAULT '' COMMENT '来源 queue/cron/manual',
    `source_id`       varchar(120)    NOT NULL DEFAULT '' COMMENT '来源任务ID，如 Asynq task id',
    `queue`           varchar(60)     NOT NULL DEFAULT '' COMMENT '队列名称',
    `trigger_user_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '触发人ID',
    `trigger_account` varchar(60)     NOT NULL DEFAULT '' COMMENT '触发人账号',
    `status`          varchar(20)     NOT NULL DEFAULT '' COMMENT '执行状态 pending/running/success/failed/canceled/retrying',
    `attempt`         int             NOT NULL DEFAULT '0' COMMENT '当前尝试次数',
    `max_retry`       int             NOT NULL DEFAULT '0' COMMENT '最大重试次数',
    `payload`         mediumtext               DEFAULT NULL COMMENT '任务 payload',
    `error_message`   text                     DEFAULT NULL COMMENT '失败原因',
    `started_at`      datetime                 DEFAULT NULL COMMENT '开始时间',
    `finished_at`     datetime                 DEFAULT NULL COMMENT '结束时间',
    `duration_ms`     decimal(10, 4)  NOT NULL DEFAULT '0.0000' COMMENT '执行耗时毫秒',
    `created_at`      datetime                 DEFAULT NULL,
    `updated_at`      datetime                 DEFAULT NULL,
    PRIMARY KEY (`id`),
    KEY `tr_task_code_created_at` (`task_code`, `created_at`),
    KEY `tr_status_created_at` (`status`, `created_at`),
    KEY `tr_source_source_id` (`source`, `source_id`),
    KEY `tr_kind_created_at` (`kind`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='任务执行记录表';

CREATE TABLE IF NOT EXISTS `task_run_events`
(
    `id`         bigint unsigned NOT NULL AUTO_INCREMENT,
    `run_id`     bigint unsigned NOT NULL DEFAULT '0' COMMENT '任务执行记录ID',
    `event_type` varchar(30)     NOT NULL DEFAULT '' COMMENT '事件类型 enqueue/start/retry/fail/success/cancel',
    `message`    text                    DEFAULT NULL COMMENT '事件说明',
    `meta`       text                    DEFAULT NULL COMMENT '事件元数据 JSON',
    `created_at` datetime                DEFAULT NULL,
    `updated_at` datetime                DEFAULT NULL,
    PRIMARY KEY (`id`),
    KEY `tre_run_id_created_at` (`run_id`, `created_at`),
    KEY `tre_event_type_created_at` (`event_type`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='任务执行事件表';

CREATE TABLE IF NOT EXISTS `cron_task_states`
(
    `id`               bigint unsigned NOT NULL AUTO_INCREMENT,
    `task_code`        varchar(120)    NOT NULL DEFAULT '' COMMENT '任务唯一编码',
    `cron_spec`        varchar(120)    NOT NULL DEFAULT '' COMMENT 'Cron 表达式',
    `last_run_id`      bigint unsigned NOT NULL DEFAULT '0' COMMENT '最近执行记录ID',
    `last_status`      varchar(20)     NOT NULL DEFAULT '' COMMENT '最近执行状态',
    `last_started_at`  datetime                 DEFAULT NULL COMMENT '最近开始时间',
    `last_finished_at` datetime                 DEFAULT NULL COMMENT '最近结束时间',
    `next_run_at`      datetime                 DEFAULT NULL COMMENT '下次执行时间',
    `last_error`       text                     DEFAULT NULL COMMENT '最近失败原因',
    `created_at`       datetime                 DEFAULT NULL,
    `updated_at`       datetime                 DEFAULT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `cts_task_code` (`task_code`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='定时任务最近状态表';

COMMIT;
