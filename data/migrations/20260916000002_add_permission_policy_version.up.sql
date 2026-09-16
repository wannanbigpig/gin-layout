CREATE TABLE IF NOT EXISTS `permission_policy_versions`
(
    `id`         tinyint unsigned NOT NULL,
    `version`    bigint unsigned NOT NULL DEFAULT '1',
    `updated_at` datetime        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='权限策略全局版本';

INSERT INTO `permission_policy_versions` (`id`, `version`)
VALUES (1, 1)
ON DUPLICATE KEY UPDATE `id` = `id`;
