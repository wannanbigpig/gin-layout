BEGIN;

-- 删除管理员表
DROP TABLE IF EXISTS `a_admin_user`;
-- 删除路由表
DROP TABLE IF EXISTS `a_api`;
-- 删除路由分组表
DROP TABLE IF EXISTS `a_api_group`;
-- 删除菜单表
DROP TABLE IF EXISTS `a_menu`;
-- 删除部门表
DROP TABLE IF EXISTS `a_department`;
-- 删除角色表
DROP TABLE IF EXISTS `a_role`;
-- 删除用户部门映射表
DROP TABLE IF EXISTS `a_user_department_map`;
-- 删除菜单权限映射表
DROP TABLE IF EXISTS `a_menu_api_map`;
-- 删除角色菜单映射表
DROP TABLE IF EXISTS `a_menu_role_map`;
-- 删除用户角色映射表
DROP TABLE IF EXISTS `a_user_role_map`;
-- 删除用户菜单映射表
DROP TABLE IF EXISTS `a_user_menu_map`;
-- 删除用户认证令牌及登录日志表
DROP TABLE IF EXISTS `a_auth_tokens`;
-- 删除请求日志表
DROP TABLE IF EXISTS `a_request_logs`;

COMMIT;