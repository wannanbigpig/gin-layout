BEGIN;

-- 删除系统参数与字典相关表
DROP TABLE IF EXISTS `sys_dict_item_i18n`;
DROP TABLE IF EXISTS `sys_dict_type_i18n`;
DROP TABLE IF EXISTS `sys_config_i18n`;
DROP TABLE IF EXISTS `sys_dict_item`;
DROP TABLE IF EXISTS `sys_dict_type`;
DROP TABLE IF EXISTS `sys_config`;
-- 删除任务中心相关表
DROP TABLE IF EXISTS `cron_task_states`;
DROP TABLE IF EXISTS `task_run_events`;
DROP TABLE IF EXISTS `task_runs`;
DROP TABLE IF EXISTS `task_definitions`;
-- 删除管理员表
DROP TABLE IF EXISTS `admin_user`;
-- 删除路由表
DROP TABLE IF EXISTS `api`;
-- 删除路由分组表
DROP TABLE IF EXISTS `api_group`;
-- 删除菜单多语言标题表
DROP TABLE IF EXISTS `menu_i18n`;
-- 删除菜单表
DROP TABLE IF EXISTS `menu`;
-- 删除部门表
DROP TABLE IF EXISTS `department`;
-- 删除角色表
DROP TABLE IF EXISTS `role`;
-- 删除用户部门映射表
DROP TABLE IF EXISTS `admin_user_department_map`;
-- 删除部门角色映射表
DROP TABLE IF EXISTS `department_role_map`;
-- 删除用户菜单映射表
DROP TABLE IF EXISTS `admin_user_menu_map`;
-- 删除菜单权限映射表
DROP TABLE IF EXISTS `menu_api_map`;
-- 删除角色菜单映射表
DROP TABLE IF EXISTS `role_menu_map`;
-- 删除用户角色映射表
DROP TABLE IF EXISTS `admin_user_role_map`;
-- 删除请求日志表
DROP TABLE IF EXISTS `request_logs`;
-- 删除登录安全状态表
DROP TABLE IF EXISTS `login_security_state`;
-- 删除管理员登录日志表
DROP TABLE IF EXISTS `admin_login_logs`;
-- 删除casbin规则表
DROP TABLE IF EXISTS `casbin_rule`;
-- 删除文件上传表
DROP TABLE IF EXISTS `upload_files`;

COMMIT;
