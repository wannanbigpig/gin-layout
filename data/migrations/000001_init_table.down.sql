BEGIN;

-- 删除管理员表
DROP TABLE IF EXISTS `a_admin_user`;
-- 删除路由表
DROP TABLE IF EXISTS `a_api`;
-- 删除菜单表
DROP TABLE IF EXISTS `a_menu`;
-- 删除组织表
DROP TABLE IF EXISTS `a_group`;
-- 删除角色表
DROP TABLE IF EXISTS `a_role`;
-- 删除组织角色映射表
DROP TABLE IF EXISTS `a_group_role_map`;
-- 删除菜单权限映射表
DROP TABLE IF EXISTS `a_menu_api_map`;
-- 删除角色菜单映射表
DROP TABLE IF EXISTS `a_menu_role_map`;
-- 删除用户角色映射表
DROP TABLE IF EXISTS `a_user_role_map`;
-- 删除登录日志表
DROP TABLE IF EXISTS `a_login_log`;

COMMIT;