ALTER TABLE `api`
    MODIFY COLUMN `is_auth` tinyint unsigned NOT NULL DEFAULT '1' COMMENT '是否鉴权 1是 0否';
