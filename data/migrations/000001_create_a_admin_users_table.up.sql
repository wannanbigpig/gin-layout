CREATE TABLE IF NOT EXISTS `a_admin_user` (
    `id` int unsigned NOT NULL AUTO_INCREMENT,
    `nickname` varchar(30) NOT NULL DEFAULT '' COMMENT '昵称',
    `username` varchar(30) NOT NULL DEFAULT '' COMMENT '用户名',
    `password` varchar(255) NOT NULL DEFAULT '' COMMENT '密码',
    `mobile` varchar(15) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT '手机号',
    `email` varchar(120) NOT NULL DEFAULT '' COMMENT '邮箱',
    `avatar` varchar(255) NOT NULL DEFAULT '' COMMENT '头像',
    `status` tinyint(1) NOT NULL DEFAULT '1' COMMENT '状态 0禁用，1启用',
    `created_at` datetime DEFAULT NULL,
    `updated_at` datetime DEFAULT NULL,
    `deleted_at` int NOT NULL DEFAULT '0' COMMENT '删除时间',
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE KEY `a_u_u_d` (`username`,`deleted_at`) USING BTREE,
    UNIQUE KEY `a_u_p_d` (`mobile`,`deleted_at`) USING BTREE,
    UNIQUE KEY `a_u_e_d` (`email`,`deleted_at`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=16 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='后台管理用户表';

INSERT INTO `a_admin_user` (`id`, `nickname`, `username`, `password`, `mobile`, `email`, `avatar`, `status`, `created_at`, `updated_at`, `deleted_at`) VALUES (1, '超级管理员', 'admin', '$2a$10$K52COmufcXarNTnsQw.7P.ji.uMA.wt26n3/7PqlKgiI4qNBiuQZC', '13200000000', '723164417@qq.com', 'https://avatars.githubusercontent.com/u/48752601?v=4', 1, '2022-07-10 12:01:02', '2022-07-10 12:01:02', 0);