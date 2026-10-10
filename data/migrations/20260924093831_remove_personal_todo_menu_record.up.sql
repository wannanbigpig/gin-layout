BEGIN;

-- 个人待办由首页入口和前端隐藏路由承载，不再占用菜单树节点。
-- 保留原记录及关联，便于回滚且不影响已有菜单权限数据。
UPDATE `menu`
SET `deleted_at` = 1790242186,
    `updated_at` = NOW()
WHERE `id` = 89
  AND `component_key` = 'todo:index'
  AND `full_path` = '/todo/list'
  AND `type` = 2
  AND `pid` = 0
  AND `is_auth` = 0
  AND `deleted_at` = 0;

COMMIT;
