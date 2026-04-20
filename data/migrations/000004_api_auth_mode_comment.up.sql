ALTER TABLE `api`
    MODIFY COLUMN `is_auth` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '接口鉴权模式 0无需登录 1需要登录 2需要登录且需要API权限';
