ALTER TABLE `cron_task_states`
    DROP COLUMN `scheduler_instance`,
    DROP COLUMN `last_sync_error`,
    DROP COLUMN `last_sync_status`,
    DROP COLUMN `last_sync_at`,
    DROP COLUMN `active_cron_spec`,
    DROP COLUMN `active_status`;
