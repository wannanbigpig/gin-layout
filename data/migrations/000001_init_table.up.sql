BEGIN;

-- 创建管理员表
CREATE TABLE IF NOT EXISTS `a_admin_user` (
    `id` int unsigned NOT NULL AUTO_INCREMENT,
    `nickname` varchar(30) NOT NULL DEFAULT '' COMMENT '昵称',
    `username` varchar(30) NOT NULL DEFAULT '' COMMENT '用户名',
    `password` varchar(255) NOT NULL DEFAULT '' COMMENT '密码',
    `mobile` varchar(15) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT '手机号',
    `email` varchar(120) NOT NULL DEFAULT '' COMMENT '邮箱',
    `avatar` varchar(255) NOT NULL DEFAULT '' COMMENT '头像',
    `status` tinyint(1) NOT NULL DEFAULT '1' COMMENT '状态 1启用 2禁用',
    `is_admin` tinyint(1) NOT NULL DEFAULT '1' COMMENT '是否管理员 1是 2不是',
    `created_at` datetime DEFAULT NULL,
    `updated_at` datetime DEFAULT NULL,
    `deleted_at` int NOT NULL DEFAULT '0' COMMENT '删除时间',
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `a_u_u_d` (`username`,`deleted_at`) USING BTREE,
    UNIQUE KEY `a_u_p_d` (`mobile`,`deleted_at`) USING BTREE,
    UNIQUE KEY `a_u_e_d` (`email`,`deleted_at`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=16 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='后台管理用户表';

-- 初始密码 123456
INSERT INTO `a_admin_user` (`id`, `nickname`, `username`, `password`, `mobile`, `email`, `avatar`, `status`, `is_admin`, `created_at`, `updated_at`, `deleted_at`) VALUES (1, '超级管理员', 'super_admin', '$2a$10$OuKQoJGH7xkCgwFISmDve.euBDbOCnYEJX6R22QMeLxCLwdoJ4iyi', '18888888888', 'admin@go-layout.com', '', 1, 1, '2023-05-01 00:00:00', '2023-05-01 00:00:00', 0);

-- 创建权限表
CREATE TABLE IF NOT EXISTS `a_permission` (
    `id` int unsigned NOT NULL AUTO_INCREMENT,
    `name` varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '权限名称',
    `desc` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '描述',
    `method` varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '接口请求方法',
    `route` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '接口路由',
    `func` varchar(80) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '接口方法',
    `func_path` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '接口方法路径',
    `is_auth` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '是否鉴权 1是 2否',
    `sort` mediumint unsigned NOT NULL DEFAULT '0' COMMENT '排序',
    `created_at` datetime DEFAULT NULL,
    `updated_at` datetime DEFAULT NULL,
    `deleted_at` int NOT NULL DEFAULT '0' COMMENT '删除时间戳',
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `p_m_r` (`method`,`route`,`deleted_at`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC COMMENT='权限表';

-- 创建菜单表
CREATE TABLE IF NOT EXISTS `a_menu` (
    `id` int NOT NULL AUTO_INCREMENT,
    `zh_name` varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '中文名称',
    `en_name` varchar(160) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '英文名称',
    `code` varchar(120) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '前端权限标识',
    `route` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '前端路由',
    `is_show` tinyint NOT NULL DEFAULT '1' COMMENT '是否隐藏，1显示 2隐藏',
    `is_outer_chain` tinyint NOT NULL DEFAULT '2' COMMENT '是否外链，1是 2不是',
    `sort` int NOT NULL COMMENT '排序，数字越大，排名越靠前',
    `type` tinyint NOT NULL DEFAULT '1' COMMENT '菜单类型，1目录，2菜单，3按钮',
    `pid` int NOT NULL DEFAULT '0' COMMENT '上级菜单id',
    `level` tinyint NOT NULL DEFAULT '0' COMMENT '层级',
    `pids` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '层级序列，多个用英文逗号隔开',
    `desc` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '描述',
    `created_at` datetime DEFAULT NULL COMMENT '创建时间',
    `updated_at` datetime DEFAULT NULL COMMENT '更新时间',
    `deleted_at` int NOT NULL DEFAULT '0' COMMENT '删除时间戳',
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `a_m_c_d` (`code`,`deleted_at`) USING BTREE,
    KEY `a_m_route` (`route`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC COMMENT='菜单表';

-- 创建组织表
CREATE TABLE IF NOT EXISTS `a_group` (
    `id` int unsigned NOT NULL AUTO_INCREMENT,
    `pid` int unsigned NOT NULL DEFAULT '0' COMMENT '上级组织id',
    `pids` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '',
    `name` varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '组织名称',
    `description` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '描述',
    `level` tinyint NOT NULL DEFAULT '1' COMMENT '层级',
    `sort` int NOT NULL DEFAULT '0' COMMENT '排序',
    `created_at` datetime DEFAULT NULL,
    `updated_at` datetime DEFAULT NULL,
    `deleted_at` int NOT NULL DEFAULT '0' COMMENT '删除时间戳',
    PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC COMMENT='组织表';

-- 创建角色表
CREATE TABLE IF NOT EXISTS `a_role` (
    `id` int unsigned NOT NULL AUTO_INCREMENT,
    `pid` int NOT NULL DEFAULT '0' COMMENT '上级id',
    `name` varchar(60) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '角色名称',
    `description` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '描述',
    `level` tinyint NOT NULL DEFAULT '1' COMMENT '层级',
    `sort` mediumint NOT NULL DEFAULT '0' COMMENT '排序',
    `status` tinyint NOT NULL DEFAULT '1' COMMENT '是否启用状态,1启用，2不启用',
    `created_at` datetime DEFAULT NULL,
    `updated_at` datetime DEFAULT NULL,
    `deleted_at` int NOT NULL DEFAULT '0' COMMENT '删除时间戳',
    PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC COMMENT='角色表';

-- 创建组织角色映射表
CREATE TABLE IF NOT EXISTS `a_group_role_map` (
    `id` int unsigned NOT NULL AUTO_INCREMENT,
    `group_id` int unsigned NOT NULL DEFAULT '0' COMMENT '组织id，对应yh_groups表id',
    `role_id` int unsigned NOT NULL DEFAULT '0' COMMENT '角色表，yh_roles表id',
    `created_at` datetime DEFAULT NULL,
    `updated_at` datetime DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    KEY `a_g_r_m_group_id_index` (`group_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC COMMENT='组织角色映射表';

-- 创建菜单权限映射表
CREATE TABLE IF NOT EXISTS `a_menu_permission_map` (
    `id` int unsigned NOT NULL AUTO_INCREMENT,
    `menu_id` int unsigned NOT NULL DEFAULT '0' COMMENT '菜单id,对应menu表id',
    `permission_id` int unsigned NOT NULL DEFAULT '0' COMMENT '接口id,对应permission表id',
    `created_at` datetime DEFAULT NULL,
    `updated_at` datetime DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    KEY `a_m_a_m_menu_id_index` (`menu_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC COMMENT='菜单权限映射表';

-- 创建角色菜单映射表
CREATE TABLE IF NOT EXISTS `a_menu_role_map` (
    `id` int unsigned NOT NULL AUTO_INCREMENT,
    `role_id` int unsigned NOT NULL DEFAULT '0' COMMENT '角色id,对应roles表id',
    `menu_id` int unsigned NOT NULL DEFAULT '0' COMMENT '菜单id,对应menu表id',
    `created_at` datetime DEFAULT NULL,
    `updated_at` datetime DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE,
    KEY `a_m_r_m_role_id_index` (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC COMMENT='角色菜单映射表';

-- 创建用户角色映射表
CREATE TABLE IF NOT EXISTS `a_user_role_map` (
   `id` int unsigned NOT NULL AUTO_INCREMENT,
   `user_id` int unsigned NOT NULL DEFAULT '0' COMMENT 'user_id,对应yh_merchant_users表id或者yh_admin_users表id',
   `role_id` int unsigned NOT NULL DEFAULT '0' COMMENT '角色id,对应yh_roles表id',
   `created_at` datetime DEFAULT NULL,
   `updated_at` datetime DEFAULT NULL,
   PRIMARY KEY (`id`) USING BTREE,
   KEY `a_u_r_m_user_id_index` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC COMMENT='用户角色映射表';

-- 创建登录日志表
CREATE TABLE IF NOT EXISTS `a_login_log` (
   `id` int unsigned NOT NULL AUTO_INCREMENT,
   `uid` int unsigned NOT NULL COMMENT '用户id',
   `user_type` tinyint NOT NULL DEFAULT '0' COMMENT '用户类别：1、后台管理用户（admin_user）',
   `client` tinyint NOT NULL COMMENT '登录客户端 1、PC 2、APP',
   `token_md5` char(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '用户 TOKEN_MD5，用来判断Token是否登出',
   `ip` varchar(15) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '登录IP',
   `is_log_out` tinyint(1) NOT NULL DEFAULT '0' COMMENT '是否登出：1是，2否',
   `log_out` datetime DEFAULT NULL COMMENT '登出时间',
   `created_at` datetime DEFAULT NULL,
   `updated_at` datetime DEFAULT NULL,
   PRIMARY KEY (`id`) USING BTREE,
   UNIQUE KEY `token_id` (`uid`,`client`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ROW_FORMAT=DYNAMIC COMMENT='登录日志表';

COMMIT;