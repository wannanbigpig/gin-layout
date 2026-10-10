CREATE TABLE IF NOT EXISTS `user_todo_groups`
(
    `id`         bigint unsigned NOT NULL AUTO_INCREMENT,
    `user_id`    bigint unsigned NOT NULL DEFAULT 0 COMMENT '所属用户ID',
    `name`       varchar(40)     NOT NULL COMMENT '分组名称',
    `created_at` datetime                 DEFAULT NULL,
    `updated_at` datetime                 DEFAULT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `utg_user_name` (`user_id`, `name`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci COMMENT ='个人待办分组';

ALTER TABLE `user_todos`
    ADD COLUMN `group_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '个人分组ID，0为未分组' AFTER `creator_id`,
    ADD KEY `ut_assignee_group_deleted` (`assignee_id`, `group_id`, `deleted_at`);
