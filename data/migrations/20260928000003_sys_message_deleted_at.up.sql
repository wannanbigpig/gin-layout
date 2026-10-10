ALTER TABLE `sys_messages`
    ADD COLUMN `deleted_at` datetime DEFAULT NULL COMMENT '接收人删除时间' AFTER `read_at`,
    ADD KEY `sm_user_deleted_id` (`user_id`, `deleted_at`, `id`);
