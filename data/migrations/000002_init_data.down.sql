BEGIN;

-- 回滚系统数据

-- 删除casbin_rule表数据
DELETE FROM `casbin_rule` WHERE `id` BETWEEN 1 AND 40;

-- 删除菜单数据
DELETE FROM `a_menu` WHERE `id` BETWEEN 1 AND 40;

-- 删除权限分组数据
DELETE FROM `a_api_group` WHERE `id` BETWEEN 1 AND 9;

-- 删除管理员用户数据
DELETE FROM `a_admin_user` WHERE `id` = 1;

COMMIT;