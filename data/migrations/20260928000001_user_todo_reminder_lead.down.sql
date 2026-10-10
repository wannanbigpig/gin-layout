ALTER TABLE `notification_preferences` DROP COLUMN `lead_minutes`;

UPDATE `sys_config`
SET `is_visible` = 1
WHERE `config_key` = 'todo.reminder_lead_minutes' AND `deleted_at` = 0;
