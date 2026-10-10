ALTER TABLE `sys_messages`
    DROP KEY `sm_user_deleted_id`,
    DROP COLUMN `deleted_at`;
