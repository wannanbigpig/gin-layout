ALTER TABLE `cron_task_states`
    ADD COLUMN `active_status` tinyint(1) NOT NULL DEFAULT '0' COMMENT '当前调度生效状态 1启用 0停用' AFTER `cron_spec`,
    ADD COLUMN `active_cron_spec` varchar(120) NOT NULL DEFAULT '' COMMENT '当前调度进程实际生效的 Cron 表达式' AFTER `active_status`,
    ADD COLUMN `last_sync_at` datetime DEFAULT NULL COMMENT '最近调度同步时间' AFTER `active_cron_spec`,
    ADD COLUMN `last_sync_status` varchar(20) NOT NULL DEFAULT '' COMMENT '最近调度同步状态 success/failed' AFTER `last_sync_at`,
    ADD COLUMN `last_sync_error` text DEFAULT NULL COMMENT '最近调度同步错误' AFTER `last_sync_status`,
    ADD COLUMN `scheduler_instance` varchar(120) NOT NULL DEFAULT '' COMMENT '调度进程实例标识' AFTER `last_sync_error`;
