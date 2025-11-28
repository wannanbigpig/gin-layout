BEGIN;

-- 创建管理员表
CREATE TABLE IF NOT EXISTS `a_admin_user`
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
CREATE TABLE IF NOT EXISTS `a_api_group`
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
CREATE TABLE IF NOT EXISTS `a_api`
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
    `is_auth`      tinyint unsigned                                       NOT NULL DEFAULT '1' COMMENT '是否鉴权 1是 0否',
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
CREATE TABLE IF NOT EXISTS `a_menu`
(
    `id`                int                                                    NOT NULL AUTO_INCREMENT,
    `icon`              varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '图标',
    `title`             varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '中文标题',
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

-- 创建组织表
CREATE TABLE IF NOT EXISTS `a_department`
(
    `id`          int unsigned                                           NOT NULL AUTO_INCREMENT,
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
    KEY `idx_name_deleted_at` (`name`, `deleted_at`),
    KEY `idx_pid_deleted_at_sort_id` (`pid`, `deleted_at`, `sort`, `id`),
    KEY `idx_pids_deleted_at` (`pids`, `deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='部门表';

-- 创建角色表
CREATE TABLE IF NOT EXISTS `a_role`
(
    `id`          int unsigned                                           NOT NULL AUTO_INCREMENT,
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
    KEY `idx_name_deleted_at` (`name`, `deleted_at`),
    KEY `idx_pid_deleted_at_sort_id` (`pid`, `deleted_at`, `sort`, `id`),
    KEY `idx_status_deleted_at` (`status`, `deleted_at`),
    KEY `idx_pids_deleted_at` (`pids`, `deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='角色表';

-- 创建用户部门映射表
CREATE TABLE IF NOT EXISTS `a_admin_user_department_map`
(
    `id`         int unsigned NOT NULL AUTO_INCREMENT,
    `uid`        int unsigned NOT NULL DEFAULT '0' COMMENT 'admin_users表id',
    `dept_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '部门id，a_department表id',
    `is_admin`   tinyint      NOT NULL DEFAULT '0' COMMENT '是否管理员，1是，0否',
    `created_at` datetime              DEFAULT NULL,
    `updated_at` datetime              DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    KEY `idx_uid` (`uid`),
    KEY `idx_dept_id` (`dept_id`),
    KEY `idx_uid_dept_id` (`uid`, `dept_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='用户部门映射表';

-- 创建菜单权限映射表
CREATE TABLE IF NOT EXISTS `a_menu_api_map`
(
    `id`         int unsigned NOT NULL AUTO_INCREMENT,
    `menu_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '菜单id,对应menu表id',
    `api_id`     int unsigned NOT NULL DEFAULT '0' COMMENT '接口id,对应api表id',
    `created_at` datetime              DEFAULT NULL,
    `updated_at` datetime              DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `idx_menu_id_api_id` (`menu_id`, `api_id`),
    KEY `idx_menu_id` (`menu_id`),
    KEY `idx_api_id` (`api_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='菜单权限映射表';

-- 初始化菜单API关联表，根据casbin_rule表自动生成关联（不依赖ID）
-- 从casbin_rule表中提取菜单ID（从v0字段的'menu:10'格式中提取10）和API的route+method来关联
-- 请先执行 ‘go-layout server -R’ 生成a_api表数据后执行下面的sql

-- INSERT INTO `a_menu_api_map` (`menu_id`, `api_id`, `created_at`, `updated_at`)
-- SELECT 
--     CAST(SUBSTRING_INDEX(c.v0, ':', -1) AS UNSIGNED) AS menu_id,
--     a.id AS api_id,
--     NOW() AS created_at,
--     NOW() AS updated_at
-- FROM `casbin_rule` c
-- INNER JOIN `a_api` a ON a.route = c.v1 AND a.method = c.v2 AND a.deleted_at = 0
-- INNER JOIN `a_menu` m ON m.id = CAST(SUBSTRING_INDEX(c.v0, ':', -1) AS UNSIGNED) AND m.deleted_at = 0
-- WHERE c.ptype = 'p' 
--   AND c.v0 LIKE 'menu:%'
--   AND c.v1 != ''
--   AND c.v2 != ''
-- ON DUPLICATE KEY UPDATE `updated_at` = NOW();

-- 创建角色菜单映射表
CREATE TABLE IF NOT EXISTS `a_role_menu_map`
(
    `id`         int unsigned NOT NULL AUTO_INCREMENT,
    `role_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '角色id,对应roles表id',
    `menu_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '菜单id,对应menu表id',
    `created_at` datetime              DEFAULT NULL,
    `updated_at` datetime              DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    KEY `idx_role_id` (`role_id`),
    KEY `idx_menu_id` (`menu_id`),
    KEY `idx_role_id_menu_id` (`role_id`, `menu_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='角色菜单映射表';

-- 创建用户菜单映射表
CREATE TABLE IF NOT EXISTS `a_admin_user_menu_map`
(
    `id`         int unsigned NOT NULL AUTO_INCREMENT,
    `uid`        int unsigned NOT NULL DEFAULT '0' COMMENT 'uid,admin_users表id',
    `menu_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '菜单id',
    `created_at` datetime              DEFAULT NULL,
    `updated_at` datetime              DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    KEY `idx_uid` (`uid`),
    KEY `idx_menu_id` (`menu_id`),
    KEY `idx_uid_menu_id` (`uid`, `menu_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='用户菜单映射表';

-- 创建部门角色映射表
CREATE TABLE IF NOT EXISTS `a_department_role_map`
(
    `id`         int unsigned NOT NULL AUTO_INCREMENT,
    `dept_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '部门id,对应a_department表id',
    `role_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '角色id,对应roles表id',
    `created_at` datetime              DEFAULT NULL,
    `updated_at` datetime              DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    KEY `idx_dept_id` (`dept_id`),
    KEY `idx_role_id` (`role_id`),
    KEY `idx_dept_id_role_id` (`dept_id`, `role_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='部门角色映射表';

-- 创建用户角色映射表
CREATE TABLE IF NOT EXISTS `a_admin_user_role_map`
(
    `id`         int unsigned NOT NULL AUTO_INCREMENT,
    `uid`        int unsigned NOT NULL DEFAULT '0' COMMENT 'uid,admin_users表id',
    `role_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '角色id',
    `created_at` datetime              DEFAULT NULL,
    `updated_at` datetime              DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    KEY `idx_uid` (`uid`),
    KEY `idx_role_id` (`role_id`),
    KEY `idx_uid_role_id` (`uid`, `role_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='用户角色映射表';

-- 创建管理员登录日志表
CREATE TABLE IF NOT EXISTS `a_admin_login_logs`
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
    `user_agent`         varchar(255)     NOT NULL DEFAULT '' COMMENT '用户代理（浏览器/设备信息）',
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
    KEY `aall_jwt_id` (`jwt_id`),
    KEY `aall_uid_deleted_at_is_revoked_created_at` (`uid`, `deleted_at`, `is_revoked`, `created_at`),
    KEY `aall_username_deleted_at_created_at` (`username`, `deleted_at`, `created_at`),
    KEY `aall_type_deleted_at_created_at` (`type`, `deleted_at`, `created_at`),
    KEY `aall_login_status_deleted_at_created_at` (`login_status`, `deleted_at`, `created_at`),
    KEY `aall_token_hash_deleted_at` (`token_hash`, `deleted_at`),
    KEY `aall_refresh_token_hash_deleted_at` (`refresh_token_hash`, `deleted_at`),
    KEY `aall_token_expires_deleted_at` (`token_expires`, `deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='管理员登录日志表';

-- 创建请求日志表
CREATE TABLE IF NOT EXISTS `a_request_logs`
(
    `id`              bigint(20)   NOT NULL AUTO_INCREMENT COMMENT '日志ID',
    `request_id`      varchar(64)  NOT NULL DEFAULT '' COMMENT '请求唯一标识',
    `jwt_id`          varchar(36)  NOT NULL DEFAULT '' COMMENT '请求授权的jwtId',
    `operator_id`     bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '操作ID（用户ID）',
    `ip`              varchar(45)  NOT NULL DEFAULT '' COMMENT '客户端IP地址',
    `user_agent`      varchar(255) NOT NULL DEFAULT '' COMMENT '用户代理（浏览器/设备信息）',
    `os`             varchar(50)  NOT NULL DEFAULT '' COMMENT '操作系统',
    `browser`        varchar(50)  NOT NULL DEFAULT '' COMMENT '浏览器',
    `method`          varchar(10)  NOT NULL DEFAULT '' COMMENT 'HTTP请求方法（GET/POST等）',
    `base_url`        varchar(160) NOT NULL DEFAULT '' COMMENT '请求基础URL',
    `operation_name`  varchar(255) NOT NULL DEFAULT '' COMMENT '操作名称',
    `operation_status` int(11) NOT NULL DEFAULT '0' COMMENT '操作状态码（响应返回的code，0=成功，其他=失败）',
    `operator_account` varchar(50) NOT NULL DEFAULT '' COMMENT '操作账号',
    `operator_name`   varchar(50)  NOT NULL DEFAULT '' COMMENT '操作人员',
    `request_headers` text                  DEFAULT NULL COMMENT '请求头（JSON格式）',
    `request_query`   text                  DEFAULT NULL COMMENT '请求参数',
    `request_body`    text                  DEFAULT NULL COMMENT '请求体',
    `response_status` int(11)      NOT NULL DEFAULT '0' COMMENT '响应状态码',
    `response_body`   text                  DEFAULT NULL COMMENT '响应体',
    `response_header` text                  DEFAULT NULL COMMENT '响应头',
    `execution_time`  DECIMAL(10,4) NOT NULL DEFAULT '0.0000' COMMENT '执行时间（毫秒，支持小数，最多4位）',
    `created_at`      datetime              DEFAULT NULL COMMENT '创建时间',
    `updated_at`      datetime              DEFAULT NULL COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY `rl_request_id` (`request_id`),
    KEY `rl_operator_id_created_at` (`operator_id`, `created_at`),
    KEY `rl_base_url_method` (`base_url`, `method`),
    KEY `rl_response_status_operator_id_created_at` (`response_status`, `operator_id`, `created_at`),
    KEY `rl_operator_account_created_at` (`operator_account`, `created_at`),
    KEY `rl_created_at` (`created_at`),
    KEY `rl_jwt_id` (`jwt_id`)
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

CREATE TABLE IF NOT EXISTS `a_upload_files`
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
    KEY `idx_hash` (`hash`),
    KEY `idx_uuid` (`uuid`),
    KEY `idx_is_public_uuid` (`is_public`, `uuid`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='上传文件表';

COMMIT;