ALTER TABLE `notification_preferences`
    ADD COLUMN `lead_minutes` smallint unsigned NOT NULL DEFAULT 1440 COMMENT '个人待办提前提醒分钟数，0表示到期时提醒' AFTER `sms_enabled`;

-- 将旧系统级设置迁入已有用户，保留原来支持的 0-43200 分钟范围。
-- 配置缺失、停用或不合法时沿用运行时默认值，不改变已有渠道开关。
SET @legacy_todo_lead_minutes = COALESCE((
    SELECT CASE
        WHEN TRIM(`config_value`) REGEXP '^[+]?[0-9]{1,10}$'
             AND CAST(TRIM(`config_value`) AS UNSIGNED) <= 43200
        THEN CAST(TRIM(`config_value`) AS UNSIGNED)
        ELSE 1440
    END
    FROM `sys_config`
    WHERE `config_key` = 'todo.reminder_lead_minutes' AND `deleted_at` = 0 AND `status` = 1
    ORDER BY `id` DESC
    LIMIT 1
), 1440);

UPDATE `notification_preferences`
SET `lead_minutes` = @legacy_todo_lead_minutes
WHERE `scene` = 'todo_due';

INSERT INTO `notification_preferences`
    (`user_id`, `scene`, `in_app_enabled`, `sms_enabled`, `lead_minutes`, `created_at`, `updated_at`)
SELECT u.`id`, 'todo_due', 1, 0, @legacy_todo_lead_minutes, NOW(), NOW()
FROM `admin_user` AS u
LEFT JOIN `notification_preferences` AS p ON p.`user_id` = u.`id` AND p.`scene` = 'todo_due'
WHERE u.`deleted_at` = 0 AND p.`id` IS NULL;

-- 仅隐藏旧入口；保留值和启用状态，回滚后仍可使用升级前的配置。
UPDATE `sys_config`
SET `is_visible` = 0
WHERE `config_key` = 'todo.reminder_lead_minutes' AND `deleted_at` = 0;
