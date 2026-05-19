START TRANSACTION;

CREATE TABLE IF NOT EXISTS `sys_messages`
(
    `id`         bigint unsigned NOT NULL AUTO_INCREMENT,
    `user_id`    bigint unsigned NOT NULL DEFAULT '0' COMMENT '接收人用户ID',
    `category`   varchar(60)     NOT NULL DEFAULT '' COMMENT '消息分类',
    `topic`      varchar(80)     NOT NULL DEFAULT '' COMMENT '消息主题',
    `title`      varchar(255)    NOT NULL DEFAULT '' COMMENT '消息标题',
    `content`    text                     DEFAULT NULL COMMENT '消息内容',
    `payload`    text                     DEFAULT NULL COMMENT '附加数据 JSON',
    `is_read`    tinyint unsigned NOT NULL DEFAULT '0' COMMENT '是否已读 0未读 1已读',
    `read_at`    datetime                 DEFAULT NULL COMMENT '已读时间',
    `created_at` datetime                 DEFAULT NULL,
    `updated_at` datetime                 DEFAULT NULL,
    PRIMARY KEY (`id`),
    KEY `sm_user_id_created_at` (`user_id`, `created_at`),
    KEY `sm_user_id_is_read_created_at` (`user_id`, `is_read`, `created_at`),
    KEY `sm_category_created_at` (`category`, `created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='系统消息表';

COMMIT;
