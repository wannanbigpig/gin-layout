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
    `status`            tinyint(1)                                                   NOT NULL DEFAULT '1' COMMENT '状态 1启用 2禁用',
    `is_super_admin`    tinyint(1)                                                   NOT NULL DEFAULT '1' COMMENT '是否超级管理员 1是 2不是（拥有所有权限）',
    `last_login`        datetime                                                              DEFAULT NULL COMMENT '最后登录时间',
    `last_ip`           varchar(45) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT '最后登录IP',
    `created_at`        datetime                                                              DEFAULT NULL,
    `updated_at`        datetime                                                              DEFAULT NULL,
    `deleted_at`        int                                                          NOT NULL DEFAULT '0' COMMENT '删除时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `adu_u_d` (`username`, `deleted_at`),
    UNIQUE KEY `adu_f_p_n_d` (`full_phone_number`, `deleted_at`),
    UNIQUE KEY `adu_e_d` (`email`, `deleted_at`),
    KEY `adu_s` (`status`),
    KEY `adu_d` (`deleted_at`)
) ENGINE = InnoDB
  AUTO_INCREMENT = 10000
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci COMMENT ='后台管理用户表';

-- 初始密码 123456
INSERT INTO `a_admin_user` (`id`, `nickname`, `username`, `password`, `phone_number`, `full_phone_number`,
                            `country_code`, `email`, `avatar`, `status`,
                            `is_super_admin`,
                            `created_at`, `updated_at`, `deleted_at`)
VALUES (1, '超级管理员', 'super_admin', '$2a$10$OuKQoJGH7xkCgwFISmDve.euBDbOCnYEJX6R22QMeLxCLwdoJ4iyi', '18888888888',
        '8618888888888', '86', 'admin@go-layout.com', 'https://avatars.githubusercontent.com/u/48752601?v=4', 1, 1,
        '2023-05-01 00:00:00', '2023-05-01 00:00:00', 0);

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
INSERT INTO `a_api_group` (`id`, `pid`, `code`, `name`, `created_at`, `updated_at`)
VALUES (1, 0, 'other', '其他', '2025-04-26 18:00:00', '2025-04-26 18:00:00'),
       (2, 0, 'login', '登录模块', '2025-04-26 18:00:00', '2025-04-26 18:00:00'),
       (3, 0, 'auth', '权限模块', '2025-04-26 18:00:00', '2025-04-26 18:00:00'),
       (4, 3, 'adminUser', '管理员模块', '2025-04-26 18:00:00', '2025-04-26 18:00:00'),
       (5, 3, 'api', 'API模块', '2025-04-26 18:00:00', '2025-04-26 18:00:00'),
       (6, 3, 'role', '角色模块', '2025-04-26 18:00:00', '2025-04-26 18:00:00'),
       (7, 3, 'menu', '菜单模块', '2025-04-26 18:00:00', '2025-04-26 18:00:00');

-- 创建权限表
CREATE TABLE IF NOT EXISTS `a_api`
(
    `id`           int unsigned                                           NOT NULL AUTO_INCREMENT,
    `code`         varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '权限唯一code码',
    `group_code`   varchar(36) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '分组唯一code码',
    `name`         varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '权限名称',
    `desc`         varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '描述',
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
    UNIQUE KEY `a_api_c_d_u` (`code`, `deleted_at`) USING BTREE,
    KEY `a_api_route` (`route`),
    KEY `a_api_method` (`method`),
    KEY `a_api_is_auth` (`is_auth`)
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
    `status`            tinyint                                                NOT NULL DEFAULT '0' COMMENT '状态，0正常 1禁用',
    `is_auth`           tinyint                                                NOT NULL DEFAULT '0' COMMENT '是否需要授权，1是 0否 ',
    `is_external_links` tinyint                                                NOT NULL DEFAULT '0' COMMENT '是否外链，1是 0否 ',
    `is_new_window`     tinyint                                                NOT NULL DEFAULT '0' COMMENT '是否新窗口打开, 1是 0否',
    `sort`              int                                                    NOT NULL DEFAULT '0' COMMENT '排序，数字越大，排名越靠前',
    `type`              tinyint                                                NOT NULL DEFAULT '1' COMMENT '菜单类型，1目录，2菜单，3按钮',
    `pid`               int                                                    NOT NULL DEFAULT '0' COMMENT '上级菜单id',
    `level`             tinyint                                                NOT NULL DEFAULT '0' COMMENT '层级',
    `pids`              varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '层级序列，多个用英文逗号隔开',
    `desc`              varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '描述',
    `created_at`        datetime                                                        DEFAULT NULL COMMENT '创建时间',
    `updated_at`        datetime                                                        DEFAULT NULL COMMENT '更新时间',
    `deleted_at`        int                                                    NOT NULL DEFAULT '0' COMMENT '删除时间戳',
    PRIMARY KEY (`id`) USING BTREE,
    KEY `a_m_c_d` (`code`),
    KEY `a_m_n_d` (`name`),
    KEY `a_m_p_d` (`path`),
    KEY `a_m_s_d` (`status`),
    KEY `a_m_i_s` (`is_auth`),
    KEY `a_m_deleted_at` (`deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='菜单表';

INSERT INTO `a_menu` (`id`, `icon`, `title`, `code`, `path`, `full_path`, `redirect`, `name`, `component`,
                      `animate_enter`, `animate_leave`, `animate_duration`, `is_show`, `status`, `is_auth`,
                      `is_external_links`, `is_new_window`, `sort`, `type`, `pid`, `level`, `pids`, `desc`,
                      `created_at`, `updated_at`, `deleted_at`)
VALUES (1, 'ep:menu', '首页', '', '', '/', '', 'Home', 'home/index.vue', '', '', 0.00, 1, 1, 1, 0, 0, 100, 2, 0, 1,
        '0', '首页', '2024-09-27 13:36:50', '2025-04-23 11:02:33', 0),
       (2, 'ant-design:lock-outlined', '权限管理', '', 'permission', '/permission', 'AdminUserList', 'Permission', '',
        '', '', 0.00, 1, 1, 1, 0, 0, 99, 1, 0, 1, '0', '', '2025-04-16 15:36:33', '2025-04-22 18:16:25', 0),
       (3, 'ant-design:api-outlined', '接口', '', 'list', '/permission/list', '', 'PermissionList',
        'permission/index.vue', '', '', 0.00, 1, 1, 1, 0, 0, 100, 2, 2, 2, '0,2', '', '2025-04-16 15:41:54',
        '2025-04-22 16:02:59', 0),
       (4, 'ant-design:menu-outlined', '菜单', '', 'menu-list', '/permission/menu-list', '', 'MenuList',
        'permission/menuList.vue', '', '', 0.00, 1, 1, 1, 0, 0, 105, 2, 2, 2, '0,2', '', '2025-04-16 15:45:31',
        '2025-04-22 16:03:10', 0),
       (5, 'ix:about', '关于', '', '/about', '/about', '', 'About', 'about/index.vue', '', '', 0.00, 1, 1, 1, 0, 0, 90,
        2, 0, 1, '0', '', '2025-04-16 16:47:58', '2025-04-23 15:01:05', 0),
       (6, 'ix:about', 'CSDN', '', 'https://blog.csdn.net/u010324331', 'https://blog.csdn.net/u010324331', '', 'CSDN',
        '', '', '', 0.00, 1, 1, 1, 1, 0, 80, 2, 0, 1, '0', '', '2025-04-16 16:51:17', '2025-04-18 18:08:51', 0),
       (7, 'ep:user', '管理员', '', 'admin-user-list', '/permission/admin-user-list', '', 'AdminUserList',
        'permission/adminUser.vue', '', '', 0.00, 1, 1, 1, 0, 0, 120, 2, 2, 2, '0,2', '', '2025-04-19 11:19:36',
        '2025-04-22 16:03:22', 0),
       (8, 'ant-design:usergroup-add-outlined', '角色', '', 'role-list', '/permission/role-list', '', 'RoleList',
        'permission/role.vue', '', '', 0.00, 1, 1, 1, 0, 0, 115, 2, 2, 2, '0,2', '', '2025-04-21 16:51:22',
        '2025-04-21 18:23:35', 0);

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
    `created_at`  datetime                                                        DEFAULT NULL,
    `updated_at`  datetime                                                        DEFAULT NULL,
    `deleted_at`  int                                                    NOT NULL DEFAULT '0' COMMENT '删除时间戳',
    PRIMARY KEY (`id`) USING BTREE,
    KEY `ag_deleted_at` (`deleted_at`),
    KEY `ag_pid` (`pid`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='组织表';

-- 创建角色表
CREATE TABLE IF NOT EXISTS `a_role`
(
    `id`          int unsigned                                           NOT NULL AUTO_INCREMENT,
    `pid`         int unsigned                                           NOT NULL DEFAULT '0' COMMENT '上级id',
    `name`        varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '角色名称',
    `description` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '描述',
    `level`       tinyint                                                NOT NULL DEFAULT '1' COMMENT '层级',
    `sort`        mediumint                                              NOT NULL DEFAULT '0' COMMENT '排序',
    `status`      tinyint                                                NOT NULL DEFAULT '1' COMMENT '是否启用状态,1启用，2不启用',
    `created_at`  datetime                                                        DEFAULT NULL,
    `updated_at`  datetime                                                        DEFAULT NULL,
    `deleted_at`  int                                                    NOT NULL DEFAULT '0' COMMENT '删除时间戳',
    PRIMARY KEY (`id`) USING BTREE,
    KEY `ar_deleted_at` (`deleted_at`),
    KEY `ar_pid` (`pid`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='角色表';

-- 创建用户菜单映射表
CREATE TABLE IF NOT EXISTS `a_user_menu_map`
(
    `id`         int unsigned NOT NULL AUTO_INCREMENT,
    `uid`        int unsigned NOT NULL DEFAULT '0' COMMENT '用户ID',
    `menu_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '菜单ID',
    `created_at` datetime              DEFAULT NULL,
    `updated_at` datetime              DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    KEY `a_g_r_m_uid_index` (`uid`),
    KEY `a_g_r_m_menu_id_index` (`menu_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='组织角色映射表';

-- 创建用户部门映射表
CREATE TABLE IF NOT EXISTS `a_user_department_map`
(
    `id`            int unsigned NOT NULL AUTO_INCREMENT,
    `uid`           int unsigned NOT NULL DEFAULT '0' COMMENT 'admin_users表id',
    `department_id` int unsigned NOT NULL DEFAULT '0' COMMENT '部门id，a_department表id',
    `is_admin`      tinyint      NOT NULL DEFAULT '0' COMMENT '是否管理员，1是，0否',
    `created_at`    datetime              DEFAULT NULL,
    `updated_at`    datetime              DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    KEY `a_u_d_m_u` (`uid`),
    KEY `a_u_d_m_d_i` (`department_id`)
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
    KEY `a_m_a_m_menu_id_index` (`menu_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='菜单权限映射表';

-- 创建角色菜单映射表
CREATE TABLE IF NOT EXISTS `a_menu_role_map`
(
    `id`         int unsigned NOT NULL AUTO_INCREMENT,
    `role_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '角色id,对应roles表id',
    `menu_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '菜单id,对应menu表id',
    `created_at` datetime              DEFAULT NULL,
    `updated_at` datetime              DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    KEY `a_m_r_m_role_id_index` (`role_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='角色菜单映射表';

-- 创建用户角色映射表
CREATE TABLE IF NOT EXISTS `a_user_role_map`
(
    `id`         int unsigned NOT NULL AUTO_INCREMENT,
    `uid`        int unsigned NOT NULL DEFAULT '0' COMMENT 'uid,admin_users表id',
    `role_id`    int unsigned NOT NULL DEFAULT '0' COMMENT '角色id',
    `created_at` datetime              DEFAULT NULL,
    `updated_at` datetime              DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    KEY `a_u_r_m_uid_index` (`uid`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_bin
  ROW_FORMAT = DYNAMIC COMMENT ='用户角色映射表';

-- 创建用户认证令牌及登录日志表
CREATE TABLE IF NOT EXISTS `a_auth_tokens`
(
    `id`                 bigint unsigned  NOT NULL AUTO_INCREMENT,
    `uid`                int unsigned     NOT NULL COMMENT '用户ID',
    `user_type`          tinyint unsigned NOT NULL DEFAULT '0' COMMENT '用户类型：1=管理员(admin_users表), 2=普通用户(users表)',
    `client_type`        tinyint unsigned NOT NULL DEFAULT '0' COMMENT '客户端类型：1=Web, 2=iOS, 3=Android, 4=小程序',
    `device_id`          varchar(64)      NOT NULL DEFAULT '' COMMENT '设备唯一标识',
    `device_name`        varchar(100)     NOT NULL DEFAULT '' COMMENT '设备名称(如iPhone 15)',
    `jwt_id`             char(36)         NOT NULL DEFAULT '' COMMENT 'JWT唯一标识(jti claim)',
    `access_token`       text                      DEFAULT NULL COMMENT '访问令牌（加密保存）',
    `refresh_token`      text                      DEFAULT NULL COMMENT '刷新令牌（加密保存）',
    `token_hash`         char(64)         NOT NULL DEFAULT '' COMMENT 'Token的SHA256哈希值',
    `refresh_token_hash` char(64)         NOT NULL DEFAULT '' COMMENT 'Refresh Token的哈希值',
    `ip`                 varchar(45)      NOT NULL DEFAULT '' COMMENT '登录IP(支持IPv6)',
    `is_revoked`         tinyint(1)       NOT NULL DEFAULT '0' COMMENT '是否被撤销：0=否, 1=是',
    `revoked_code`       tinyint(1)       NOT NULL DEFAULT '0' COMMENT '撤销原因码：1=用户主动登出（退出登录）, 2=系统强制登出（账号被封）, 3=系统刷新token, 4=用户禁用（针对某个设备下线操作） 5=其他原因',
    `revoked_reason`     varchar(255)     NOT NULL DEFAULT '' COMMENT '撤销原因',
    `revoked_at`         datetime                  DEFAULT NULL COMMENT '撤销时间',
    `token_expires`      datetime                  DEFAULT NULL COMMENT 'Token过期时间',
    `refresh_expires`    datetime                  DEFAULT NULL COMMENT 'Refresh Token过期时间',
    `created_at`         datetime                  DEFAULT NULL,
    `updated_at`         datetime                  DEFAULT NULL,
    `deleted_at`         int              NOT NULL DEFAULT '0' COMMENT '删除时间戳',
    PRIMARY KEY (`id`),
    UNIQUE KEY `aat_jwt_id` (`jwt_id`),
    KEY `aat_created_at` (`uid`, `is_revoked`, `created_at`),
    KEY `aat_token_hash` (`token_hash`),
    KEY `aat_token_expire` (`token_expires`),
    KEY `aat_deleted_at` (`deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='用户认证令牌及登录日志';

-- 创建请求日志表
CREATE TABLE `a_request_logs`
(
    `id`              bigint(20)   NOT NULL AUTO_INCREMENT COMMENT '日志ID',
    `request_id`      varchar(64)  NOT NULL DEFAULT '' COMMENT '请求唯一标识',
    `jwt_id`          varchar(36)  NOT NULL DEFAULT '' COMMENT '请求授权的jwtId',
    `uid`             bigint(20)   NOT NULL DEFAULT '0' COMMENT '用户ID（如适用）',
    `ip`              varchar(45)  NOT NULL DEFAULT '' COMMENT '客户端IP地址',
    `user_agent`      varchar(255) NOT NULL DEFAULT '' COMMENT '用户代理（浏览器/设备信息）',
    `method`          varchar(10)  NOT NULL DEFAULT '' COMMENT 'HTTP请求方法（GET/POST等）',
    `base_url`        varchar(160) NOT NULL DEFAULT '' COMMENT '请求基础URL',
    `request_headers` text                  DEFAULT NULL COMMENT '请求头（JSON格式）',
    `request_query`   text                  DEFAULT NULL COMMENT '请求参数',
    `request_body`    text                  DEFAULT NULL COMMENT '请求体',
    `response_status` int(11)      NOT NULL DEFAULT '0' COMMENT '响应状态码',
    `response_body`   text                  DEFAULT NULL COMMENT '响应体',
    `response_header` text                  DEFAULT NULL COMMENT '响应头',
    `execution_time`  int(11)      NOT NULL DEFAULT '0' COMMENT '执行时间（毫秒）',
    `created_at`      datetime              DEFAULT NULL COMMENT '创建时间',
    `updated_at`      datetime              DEFAULT NULL COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY `rl_request_id` (`request_id`),
    KEY `rl_uid_id` (`uid`),
    KEY `rl_url_base_url_method` (`base_url`, `method`),
    KEY `rl_created_at_uid_r_s` (`created_at`, `uid`, `response_status`),
    KEY `rl_response_status` (`response_status`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4 COMMENT ='请求日志表';

COMMIT;