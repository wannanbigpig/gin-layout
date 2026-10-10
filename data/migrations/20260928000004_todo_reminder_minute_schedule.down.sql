UPDATE `task_definitions`
SET `cron_spec` = '0 */5 * * * *', `updated_at` = NOW()
WHERE `code` = 'cron:todo-reminder'
  AND `deleted_at` = 0
  AND `cron_spec` = '0 * * * * *';
