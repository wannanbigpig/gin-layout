BEGIN;

UPDATE `menu`
SET `deleted_at` = 0,
    `updated_at` = NOW()
WHERE `id` = 89
  AND `component_key` = 'todo:index'
  AND `full_path` = '/todo/list'
  AND `type` = 2
  AND `pid` = 0
  AND `is_auth` = 0
  AND `deleted_at` = 1790242186;

COMMIT;
