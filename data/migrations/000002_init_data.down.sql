BEGIN;

-- 回滚系统数据

-- 删除角色菜单映射
DELETE FROM `role_menu_map` WHERE `role_id` = 1;

-- 删除菜单数据
DELETE FROM `menu` WHERE `id` BETWEEN 1 AND 41;

-- 删除权限分组数据
DELETE FROM `api_group` WHERE `id` BETWEEN 1 AND 9;

-- 删除管理员用户部门/角色映射
DELETE FROM `admin_user_role_map` WHERE `uid` = 1 AND `role_id` = 1;
DELETE FROM `admin_user_department_map` WHERE `uid` = 1 AND `dept_id` = 1;

-- 删除默认角色和部门
DELETE FROM `role` WHERE `id` = 1;
DELETE FROM `department` WHERE `id` = 1;

-- 删除管理员用户数据
DELETE FROM `admin_user` WHERE `id` = 1;

COMMIT;
