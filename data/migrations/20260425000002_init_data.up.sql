BEGIN;

SET @init_time = NOW();

-- 初始化系统数据

-- 初始密码 123456
INSERT INTO `admin_user` (`id`, `nickname`, `username`, `password`, `phone_number`, `full_phone_number`,
                            `country_code`, `email`, `avatar`, `status`,
                            `is_super_admin`,
                            `created_at`, `updated_at`, `deleted_at`)
VALUES (1, '超级管理员', 'super_admin', '$2a$10$OuKQoJGH7xkCgwFISmDve.euBDbOCnYEJX6R22QMeLxCLwdoJ4iyi', '18888888888',
        '8618888888888', '86', 'admin@go-layout.com', 'https://avatars.githubusercontent.com/u/48752601?v=4', 1, 1,
        @init_time, @init_time, 0);

INSERT INTO `department` (`id`, `code`, `is_system`, `pid`, `pids`, `name`, `description`, `level`, `sort`,
                          `children_num`, `user_number`, `created_at`, `updated_at`, `deleted_at`)
VALUES (1, 'default_department', 1, 0, '0', '默认部门', '系统默认部门', 1, 100, 0, 1,
        @init_time, @init_time, 0);

INSERT INTO `role` (`id`, `code`, `is_system`, `name`, `description`, `sort`, `status`, `created_at`, `updated_at`, `deleted_at`)
VALUES (1, 'super_admin', 1, '超级管理员', '系统默认超级管理员角色', 100, 1,
        @init_time, @init_time, 0);

INSERT INTO `admin_user_department_map` (`uid`, `dept_id`, `is_admin`, `created_at`, `updated_at`)
VALUES (1, 1, 1, @init_time, @init_time);

INSERT INTO `admin_user_role_map` (`uid`, `role_id`, `created_at`, `updated_at`)
VALUES (1, 1, @init_time, @init_time);

-- 初始化权限分组数据
INSERT INTO `api_group` (`id`, `pid`, `code`, `name`, `created_at`, `updated_at`)
VALUES (1, 0, 'other', '其他', @init_time, @init_time),
       (2, 0, 'login', '登录模块', @init_time, @init_time),
       (3, 0, 'auth', '权限模块', @init_time, @init_time),
       (4, 3, 'adminUser', '管理员模块', @init_time, @init_time),
       (5, 3, 'api', 'API模块', @init_time, @init_time),
       (6, 3, 'role', '角色模块', @init_time, @init_time),
       (7, 3, 'menu', '菜单模块', @init_time, @init_time),
       (8, 0, 'common', '公共模块', @init_time, @init_time),
       (9, 0, 'log', '日志模块', @init_time, @init_time),
       (16, 0, 'product', '产品模块', @init_time, @init_time);

-- 初始化菜单数据
INSERT INTO `menu` (`id`, `icon`, `code`, `path`, `full_path`, `redirect`, `name`, `component_key`, `animate_enter`, `animate_leave`, `animate_duration`, `is_show`, `status`, `is_auth`, `is_external_links`, `is_new_window`, `sort`, `type`, `pid`, `level`, `pids`, `children_num`, `description`, `created_at`, `updated_at`, `deleted_at`) VALUES
(1, 'ep:menu', '', '', '/', '', 'Home', 'home:index', '', '', 0.00, 1, 1, 0, 0, 0, 100, 2, 0, 1, '0', 3, '', @init_time, @init_time, 0),
(2, 'ant-design:lock-outlined', '', 'permission', '/permission', 'AdminUserList', 'Permission', '', '', '', 0.00, 1, 1, 1, 0, 0, 99, 1, 0, 1, '0', 0, '', @init_time, @init_time, 0),
(3, 'ant-design:api-outlined', '', 'list', '/permission/list', '', 'PermissionList', 'permission:api', '', '', 0.00, 1, 1, 1, 0, 0, 100, 2, 2, 2, '0,2', 2, '', @init_time, @init_time, 0),
(4, 'ant-design:menu-outlined', '', 'menu-list', '/permission/menu-list', '', 'MenuList', 'permission:menuList', '', '', 0.00, 1, 1, 1, 0, 0, 105, 2, 2, 2, '0,2', 6, '', @init_time, @init_time, 0),
(5, 'lucide:info', '', '/about', '/about', '', 'About', 'about:index', '', '', 0.00, 1, 1, 1, 0, 0, 78, 2, 0, 1, '0', 0, '', @init_time, @init_time, 0),
(7, 'ep:user', '', 'admin-user-list', '/permission/admin-user-list', '', 'AdminUserList', 'permission:adminUser', '', '', 0.00, 1, 1, 1, 0, 0, 120, 2, 2, 2, '0,2', 5, '', @init_time, @init_time, 0),
(8, 'ant-design:usergroup-add-outlined', '', 'role-list', '/permission/role-list', '', 'RoleList', 'permission:role', '', '', 0.00, 1, 1, 1, 0, 0, 115, 2, 2, 2, '0,2', 4, '', @init_time, @init_time, 0),
(9, 'tdesign:tree-square-dot', '', 'department-list', '/permission/department-list', '', 'DepartmentList', 'permission:department', '', '', 0.00, 1, 1, 1, 0, 0, 115, 2, 2, 2, '0,2', 6, '', @init_time, @init_time, 0),
(10, 'ant-design:edit-filled', 'adminUser:update', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 7, 3, '0,2,7', 0, '', @init_time, @init_time, 0),
(11, 'ant-design:plus-outlined', 'adminUser:add', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 7, 3, '0,2,7', 0, '', @init_time, @init_time, 0),
(12, 'ant-design:user-switch-outlined', 'adminUser:bindRole', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 7, 3, '0,2,7', 0, '', @init_time, @init_time, 0),
(13, 'ant-design:delete-outlined', 'adminUser:delete', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 7, 3, '0,2,7', 0, '', @init_time, @init_time, 0),
(14, 'ant-design:plus-outlined', 'menu:add', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 4, 3, '0,2,4', 0, '', @init_time, @init_time, 0),
(15, 'ant-design:plus-circle-outlined', 'menu:addChild', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 4, 3, '0,2,4', 0, '', @init_time, @init_time, 0),
(16, 'ant-design:edit-filled', 'menu:update', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 4, 3, '0,2,4', 0, '', @init_time, @init_time, 0),
(17, 'ant-design:delete-outlined', 'menu:delete', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 4, 3, '0,2,4', 0, '', @init_time, @init_time, 0),
(27, 'ep:refresh', 'menu:refreshPermissionCache', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 90, 3, 4, 3, '0,2,4', 0, '', @init_time, @init_time, 0),
(18, 'ant-design:plus-outlined', 'role:add', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 8, 3, '0,2,8', 0, '', @init_time, @init_time, 0),
(19, 'ant-design:edit-filled', 'role:update', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 8, 3, '0,2,8', 0, '', @init_time, @init_time, 0),
(20, 'ant-design:delete-outlined', 'role:delete', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 8, 3, '0,2,8', 0, '', @init_time, @init_time, 0),
(21, 'ant-design:plus-outlined', 'department:add', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 9, 3, '0,2,9', 0, '', @init_time, @init_time, 0),
(22, 'ant-design:plus-circle-outlined', 'department:addChild', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 9, 3, '0,2,9', 0, '', @init_time, @init_time, 0),
(23, 'ant-design:edit-filled', 'department:update', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 9, 3, '0,2,9', 0, '', @init_time, @init_time, 0),
(24, 'ant-design:user-switch-outlined', 'department:bindRole', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 9, 3, '0,2,9', 0, '', @init_time, @init_time, 0),
(25, 'ant-design:delete-outlined', 'department:delete', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 9, 3, '0,2,9', 0, '', @init_time, @init_time, 0),
(26, 'ant-design:edit-filled', 'api:update', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 3, 3, '0,2,3', 0, '', @init_time, @init_time, 0),
(29, 'ep:tickets', '', 'log', '/log', 'RequestLog', 'Log', '', '', '', 0.00, 1, 1, 1, 0, 0, 98, 1, 0, 1, '0', 3, '', @init_time, @init_time, 0),
(30, 'ep:document', '', 'request-log', '/log/request-log', '', 'RequestLog', 'log:request', '', '', 0.00, 1, 1, 1, 0, 0, 100, 2, 29, 2, '0,29', 3, '', @init_time, @init_time, 0),
(31, 'ep:document', '', 'admin-login-log', '/log/admin-login-log', '', 'AdminLoginLog', 'log:adminLogin', '', '', 0.00, 1, 1, 1, 0, 0, 100, 2, 29, 2, '0,29', 2, '', @init_time, @init_time, 0),
(32, 'ep:document', 'adminLoginLog:detail', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 31, 3, '0,29,31', 0, '', @init_time, @init_time, 0),
(33, 'ep:document', 'requestLog:detail', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 30, 3, '0,29,30', 0, '', @init_time, @init_time, 0),
(34, 'ant-design:search-outlined', 'adminUser:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 7, 3, '0,2,7', 0, '', @init_time, @init_time, 0),
(35, 'ant-design:search-outlined', 'department:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 9, 3, '0,2,9', 0, '', @init_time, @init_time, 0),
(36, 'ant-design:search-outlined', 'role:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 8, 3, '0,2,8', 0, '', @init_time, @init_time, 0),
(37, 'ant-design:search-outlined', 'menu:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 4, 3, '0,2,4', 0, '', @init_time, @init_time, 0),
(38, 'ant-design:search-outlined', 'api:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 3, 3, '0,2,3', 0, '', @init_time, @init_time, 0),
(39, 'ant-design:search-outlined', 'adminLoginLog:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 31, 3, '0,29,31', 0, '', @init_time, @init_time, 0),
(40, 'ant-design:search-outlined', 'requestLog:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 30, 3, '0,29,30', 0, '', @init_time, @init_time, 0),
(42, 'ep:setting', '', 'system', '/system', 'SysConfig', 'System', '', '', '', 0.00, 1, 1, 1, 0, 0, 96, 1, 0, 1, '0', 5, '', @init_time, @init_time, 0),
(43, 'ep:operation', '', 'config', '/system/config', '', 'SysConfig', 'system:config', '', '', 0.00, 1, 1, 1, 0, 0, 100, 2, 42, 2, '0,42', 10, '', @init_time, @init_time, 0),
(44, 'ep:collection-tag', '', 'dict', '/system/dict', '', 'SysDict', 'system:dict', '', '', 0.00, 1, 1, 1, 0, 0, 90, 2, 42, 2, '0,42', 4, '', @init_time, @init_time, 0),
(45, 'ant-design:search-outlined', 'sysConfig:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 43, 3, '0,42,43', 0, '', @init_time, @init_time, 0),
(46, 'ant-design:plus-outlined', 'sysConfig:add', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 90, 3, 43, 3, '0,42,43', 0, '', @init_time, @init_time, 0),
(47, 'ant-design:edit-filled', 'sysConfig:update', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 80, 3, 43, 3, '0,42,43', 0, '', @init_time, @init_time, 0),
(48, 'ant-design:delete-outlined', 'sysConfig:delete', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 70, 3, 43, 3, '0,42,43', 0, '', @init_time, @init_time, 0),
(49, 'ep:refresh', 'sysConfig:refresh', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 60, 3, 43, 3, '0,42,43', 0, '', @init_time, @init_time, 0),
(50, 'ant-design:search-outlined', 'sysDict:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 44, 3, '0,42,44', 0, '', @init_time, @init_time, 0),
(51, 'ant-design:plus-outlined', 'sysDict:add', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 90, 3, 44, 3, '0,42,44', 0, '', @init_time, @init_time, 0),
(52, 'ant-design:edit-filled', 'sysDict:update', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 80, 3, 44, 3, '0,42,44', 0, '', @init_time, @init_time, 0),
(53, 'ant-design:delete-outlined', 'sysDict:delete', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 70, 3, 44, 3, '0,42,44', 0, '', @init_time, @init_time, 0),
(54, 'ri:task-line', '', 'task', '/task', 'TaskCenter', 'Task', '', '', '', 0.00, 1, 1, 1, 0, 0, 97, 1, 0, 1, '0', 1, '', @init_time, @init_time, 0),
(55, 'ri:task-line', '', 'center', '/task/center', '', 'TaskCenter', 'system:task', '', '', 0.00, 1, 1, 1, 0, 0, 100, 2, 54, 2, '0,54', 7, '', @init_time, @init_time, 0),
(56, 'ant-design:search-outlined', 'task:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 55, 3, '0,54,55', 0, '', @init_time, @init_time, 0),
(57, 'ep:video-play', 'task:trigger', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 90, 3, 55, 3, '0,54,55', 0, '', @init_time, @init_time, 0),
(58, 'ep:refresh-right', 'task:retry', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 80, 3, 55, 3, '0,54,55', 0, '', @init_time, @init_time, 0),
(59, 'ep:circle-close', 'task:cancel', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 70, 3, 55, 3, '0,54,55', 0, '', @init_time, @init_time, 0),
(60, 'ep:view', 'task:detail', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 60, 3, 55, 3, '0,54,55', 0, '', @init_time, @init_time, 0),
(61, 'ant-design:search-outlined', 'requestLog:export', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 80, 3, 30, 3, '0,29,30', 0, '', @init_time, @init_time, 0),
(62, 'ep:setting', 'requestLog:maskConfig', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 70, 3, 43, 3, '0,42,43', 0, '', @init_time, @init_time, 0),
(63, 'ep:folder-opened', '', 'file', '/system/file', '', 'FileResource', 'system:file', '', '', 0.00, 1, 1, 1, 0, 0, 80, 2, 42, 2, '0,42', 2, '', @init_time, @init_time, 0),
(64, 'ant-design:search-outlined', 'file:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 63, 3, '0,42,63', 0, '', @init_time, @init_time, 0),
(65, 'ant-design:delete-outlined', 'file:delete', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 90, 3, 63, 3, '0,42,63', 0, '', @init_time, @init_time, 0),
(66, 'ep:monitor', '', 'session', '/log/session', '', 'OnlineSession', 'log:session', '', '', 0.00, 1, 1, 1, 0, 0, 90, 2, 29, 2, '0,29', 2, '', @init_time, @init_time, 0),
(67, 'ant-design:search-outlined', 'session:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 66, 3, '0,29,66', 0, '', @init_time, @init_time, 0),
(68, 'ep:circle-close', 'session:revoke', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 90, 3, 66, 3, '0,29,66', 0, '', @init_time, @init_time, 0),
(70, 'ant-design:search-outlined', 'storage:config', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 43, 3, '0,42,43', 0, '', @init_time, @init_time, 0),
(71, 'ant-design:edit-filled', 'storage:update', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 90, 3, 43, 3, '0,42,43', 0, '', @init_time, @init_time, 0),
(72, 'ep:connection', 'storage:test', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 80, 3, 43, 3, '0,42,43', 0, '', @init_time, @init_time, 0),
(77, 'ant-design:eye-outlined', 'storage:secret', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 70, 3, 43, 3, '0,42,43', 0, '', @init_time, @init_time, 0),
(73, 'ep:refresh-left', 'file:restore', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 80, 3, 63, 3, '0,42,63', 0, '', @init_time, @init_time, 0),
(74, 'ant-design:delete-filled', 'file:destroy', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 70, 3, 63, 3, '0,42,63', 0, '', @init_time, @init_time, 0),
(75, 'ant-design:edit-filled', 'file:update', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 85, 3, 63, 3, '0,42,63', 0, '', @init_time, @init_time, 0),
(76, 'ant-design:plus-outlined', 'file:create', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 95, 3, 63, 3, '0,42,63', 0, '', @init_time, @init_time, 0),
(78, 'ep:data-line', 'dashboard:overview', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 1, 2, '0,1', 0, '全局运营概览 / Global operational overview', @init_time, @init_time, 0),
(79, 'ep:trend-charts', 'dashboard:statistics', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 90, 3, 1, 2, '0,1', 0, '全局趋势与运行统计 / Global trends and operational statistics', @init_time, @init_time, 0),
(80, 'ep:monitor', 'dashboard:monitor', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 80, 3, 1, 2, '0,1', 0, '服务器运行状态监控 / Server health monitoring', @init_time, @init_time, 0),
(81, 'ep:message', '', 'notification-manage', '/system/notification-manage', '', 'NotificationManage', 'system:notificationManage', '', '', 0.00, 1, 1, 1, 0, 0, 55, 2, 42, 2, '0,42', 1, '', @init_time, @init_time, 0),
(82, 'ep:promotion', 'notification:send', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 81, 3, '0,42,81', 0, '', @init_time, @init_time, 0),
(83, 'ep:goods', '', 'business', '/business', 'Product', 'Business', '', '', '', 0.00, 1, 1, 1, 0, 0, 95, 1, 0, 1, '0', 1, '产品示例业务分组', @init_time, @init_time, 0),
(84, 'ep:box', '', 'product', '/business/product', '', 'Product', 'product:index', '', '', 0.00, 1, 1, 1, 0, 0, 100, 2, 83, 2, '0,83', 4, '产品数据权限示例菜单', @init_time, @init_time, 0),
(85, 'ant-design:search-outlined', 'product:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 84, 3, '0,83,84', 0, '', @init_time, @init_time, 0),
(86, 'ant-design:plus-outlined', 'product:create', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 90, 3, 84, 3, '0,83,84', 0, '', @init_time, @init_time, 0),
(87, 'ant-design:edit-filled', 'product:update', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 80, 3, 84, 3, '0,83,84', 0, '', @init_time, @init_time, 0),
(88, 'ant-design:delete-outlined', 'product:delete', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 70, 3, 84, 3, '0,83,84', 0, '', @init_time, @init_time, 0),
(89, 'ep:list', '', 'todo/list', '/todo/list', '', 'TodoList', 'todo:index', '', '', 0.00, 1, 1, 0, 0, 0, 97, 2, 0, 1, '0', 0, '当前登录用户的个人与业务待办', @init_time, @init_time, 0);

INSERT INTO `menu` (`icon`, `code`, `path`, `full_path`, `redirect`, `name`, `component_key`, `animate_enter`, `animate_leave`, `animate_duration`, `is_show`, `status`, `is_auth`, `is_external_links`, `is_new_window`, `sort`, `type`, `pid`, `level`, `pids`, `children_num`, `description`, `created_at`, `updated_at`, `deleted_at`)
SELECT 'ant-design:setting-outlined', 'task:update', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 50, 3, parent.`id`, parent.`level` + 1, CONCAT(parent.`pids`, ',', parent.`id`), 0, '', @init_time, @init_time, 0
FROM `menu` parent
WHERE parent.`component_key` = 'system:task'
  AND parent.`deleted_at` = 0
LIMIT 1
ON DUPLICATE KEY UPDATE `updated_at` = VALUES(`updated_at`);

INSERT INTO `menu` (`icon`, `code`, `path`, `full_path`, `redirect`, `name`, `component_key`, `animate_enter`, `animate_leave`, `animate_duration`, `is_show`, `status`, `is_auth`, `is_external_links`, `is_new_window`, `sort`, `type`, `pid`, `level`, `pids`, `children_num`, `description`, `created_at`, `updated_at`, `deleted_at`)
SELECT 'ep:data-analysis', 'task:stats', 'stats', '/task/stats', '', 'TaskStats', 'system:taskStats', '', '', 0.00, 1, 1, 1, 0, 0, 90, 2, parent.`id`, parent.`level` + 1, CONCAT(parent.`pids`, ',', parent.`id`), 0, '任务执行全量统计', @init_time, @init_time, 0
FROM `menu` parent
WHERE parent.`full_path` = '/task'
  AND parent.`type` = 1
  AND parent.`deleted_at` = 0
LIMIT 1
ON DUPLICATE KEY UPDATE
    `icon` = VALUES(`icon`),
    `path` = VALUES(`path`),
    `full_path` = VALUES(`full_path`),
    `name` = VALUES(`name`),
    `component_key` = VALUES(`component_key`),
    `sort` = VALUES(`sort`),
    `type` = VALUES(`type`),
    `pid` = VALUES(`pid`),
    `level` = VALUES(`level`),
    `pids` = VALUES(`pids`),
    `description` = VALUES(`description`),
    `updated_at` = VALUES(`updated_at`);

UPDATE `menu`
SET `children_num` = (
    SELECT COUNT(*)
    FROM (
        SELECT child.`id`
        FROM `menu` child
        WHERE child.`pid` = (SELECT parent.`id` FROM `menu` parent WHERE parent.`component_key` = 'system:task' AND parent.`deleted_at` = 0 LIMIT 1)
          AND child.`deleted_at` = 0
    ) children
)
WHERE `component_key` = 'system:task'
  AND `deleted_at` = 0;

UPDATE `menu`
SET `children_num` = (
    SELECT COUNT(*)
    FROM (
        SELECT child.`id`
        FROM `menu` child
        WHERE child.`pid` = (SELECT parent.`id` FROM `menu` parent WHERE parent.`full_path` = '/task' AND parent.`type` = 1 AND parent.`deleted_at` = 0 LIMIT 1)
          AND child.`deleted_at` = 0
    ) children
)
WHERE `full_path` = '/task'
  AND `type` = 1
  AND `deleted_at` = 0;

INSERT INTO `menu_i18n` (`menu_id`, `locale`, `title`, `created_at`, `updated_at`)
SELECT `id`,
       'zh-CN',
       CASE `id`
           WHEN 1 THEN '首页'
           WHEN 2 THEN '权限管理'
           WHEN 3 THEN '接口'
           WHEN 4 THEN '菜单'
           WHEN 5 THEN '关于'
           WHEN 7 THEN '管理员'
           WHEN 8 THEN '角色'
           WHEN 9 THEN '部门'
           WHEN 10 THEN '编辑'
           WHEN 11 THEN '新增管理员'
           WHEN 12 THEN '绑定角色'
           WHEN 13 THEN '删除'
           WHEN 14 THEN '新增菜单'
           WHEN 15 THEN '新增下级'
           WHEN 16 THEN '编辑'
           WHEN 17 THEN '删除'
           WHEN 27 THEN '刷新权限缓存'
           WHEN 18 THEN '新增角色'
           WHEN 19 THEN '编辑'
           WHEN 20 THEN '删除'
           WHEN 21 THEN '新增部门'
           WHEN 22 THEN '新增'
           WHEN 23 THEN '编辑'
           WHEN 24 THEN '绑定角色'
           WHEN 25 THEN '删除'
           WHEN 26 THEN '编辑'
           WHEN 29 THEN '日志管理'
           WHEN 30 THEN '请求日志'
           WHEN 31 THEN '管理员登录日志'
           WHEN 32 THEN '详情'
           WHEN 33 THEN '详情'
           WHEN 34 THEN '列表'
           WHEN 35 THEN '列表'
           WHEN 36 THEN '列表'
           WHEN 37 THEN '列表'
           WHEN 38 THEN '列表'
           WHEN 39 THEN '列表'
           WHEN 40 THEN '列表'
           WHEN 42 THEN '系统管理'
           WHEN 43 THEN '系统参数'
           WHEN 44 THEN '字典管理'
           WHEN 45 THEN '列表'
           WHEN 46 THEN '新增'
           WHEN 47 THEN '编辑'
           WHEN 48 THEN '删除'
           WHEN 49 THEN '刷新缓存'
           WHEN 50 THEN '列表'
           WHEN 51 THEN '新增'
           WHEN 52 THEN '编辑'
           WHEN 53 THEN '删除'
           WHEN 54 THEN '任务中心'
           WHEN 55 THEN '任务管理'
           WHEN 56 THEN '列表'
           WHEN 57 THEN '触发任务'
           WHEN 58 THEN '重试任务'
           WHEN 59 THEN '取消任务'
           WHEN 60 THEN '详情'
           WHEN 61 THEN '导出'
           WHEN 62 THEN '脱敏配置'
           WHEN 63 THEN '文件资源'
           WHEN 64 THEN '列表'
           WHEN 65 THEN '删除'
           WHEN 66 THEN '在线会话'
           WHEN 67 THEN '列表'
           WHEN 68 THEN '撤销'
           WHEN 70 THEN '配置'
           WHEN 71 THEN '更新'
           WHEN 72 THEN '测试'
           WHEN 77 THEN '查看密钥'
           WHEN 73 THEN '恢复'
           WHEN 74 THEN '硬删除'
           WHEN 75 THEN '编辑'
           WHEN 76 THEN '新增'
           WHEN 78 THEN '全局概览'
           WHEN 79 THEN '全局统计'
           WHEN 80 THEN '全局服务器监控'
           WHEN 81 THEN '通知管理'
           WHEN 82 THEN '发送通知'
           WHEN 83 THEN '业务管理'
           WHEN 84 THEN '产品示例'
           WHEN 85 THEN '列表'
           WHEN 86 THEN '新增'
           WHEN 87 THEN '编辑'
           WHEN 88 THEN '删除'
           WHEN 89 THEN '我的待办'
           ELSE ''
           END,
       `created_at`,
       `updated_at`
FROM `menu`
WHERE `deleted_at` = 0;

INSERT INTO `menu_i18n` (`menu_id`, `locale`, `title`, `created_at`, `updated_at`)
SELECT `id`, 'zh-CN', '任务配置', @init_time, @init_time
FROM `menu`
WHERE `code` = 'task:update'
  AND `deleted_at` = 0
ON DUPLICATE KEY UPDATE `title` = VALUES(`title`), `updated_at` = VALUES(`updated_at`);

INSERT INTO `menu_i18n` (`menu_id`, `locale`, `title`, `created_at`, `updated_at`)
SELECT `id`, 'zh-CN', '任务统计', @init_time, @init_time
FROM `menu`
WHERE `code` = 'task:stats'
  AND `deleted_at` = 0
ON DUPLICATE KEY UPDATE `title` = VALUES(`title`), `updated_at` = VALUES(`updated_at`);

INSERT INTO `menu_i18n` (`menu_id`, `locale`, `title`, `created_at`, `updated_at`)
SELECT `id`,
       'en-US',
       CASE `id`
           WHEN 1 THEN 'Home'
           WHEN 2 THEN 'Permission'
           WHEN 3 THEN 'API'
           WHEN 4 THEN 'Menu'
           WHEN 5 THEN 'About'
           WHEN 7 THEN 'Administrators'
           WHEN 8 THEN 'Roles'
           WHEN 9 THEN 'Departments'
           WHEN 10 THEN 'Edit'
           WHEN 11 THEN 'Add Administrator'
           WHEN 12 THEN 'Bind Roles'
           WHEN 13 THEN 'Delete'
           WHEN 14 THEN 'Add Menu'
           WHEN 15 THEN 'Add Child'
           WHEN 16 THEN 'Edit'
           WHEN 17 THEN 'Delete'
           WHEN 27 THEN 'Refresh Permission Cache'
           WHEN 18 THEN 'Add Role'
           WHEN 19 THEN 'Edit'
           WHEN 20 THEN 'Delete'
           WHEN 21 THEN 'Add Department'
           WHEN 22 THEN 'Add'
           WHEN 23 THEN 'Edit'
           WHEN 24 THEN 'Bind Roles'
           WHEN 25 THEN 'Delete'
           WHEN 26 THEN 'Edit'
           WHEN 29 THEN 'Log Management'
           WHEN 30 THEN 'Request Logs'
           WHEN 31 THEN 'Admin Login Logs'
           WHEN 32 THEN 'Detail'
           WHEN 33 THEN 'Detail'
           WHEN 34 THEN 'List'
           WHEN 35 THEN 'List'
           WHEN 36 THEN 'List'
           WHEN 37 THEN 'List'
           WHEN 38 THEN 'List'
           WHEN 39 THEN 'List'
           WHEN 40 THEN 'List'
           WHEN 42 THEN 'System'
           WHEN 43 THEN 'System Config'
           WHEN 44 THEN 'Dictionary'
           WHEN 45 THEN 'List'
           WHEN 46 THEN 'Add'
           WHEN 47 THEN 'Edit'
           WHEN 48 THEN 'Delete'
           WHEN 49 THEN 'Refresh Cache'
           WHEN 50 THEN 'List'
           WHEN 51 THEN 'Add'
           WHEN 52 THEN 'Edit'
           WHEN 53 THEN 'Delete'
           WHEN 54 THEN 'Task Center'
           WHEN 55 THEN 'Task Management'
           WHEN 56 THEN 'List'
           WHEN 57 THEN 'Trigger Task'
           WHEN 58 THEN 'Retry Task'
           WHEN 59 THEN 'Cancel Task'
           WHEN 60 THEN 'Detail'
           WHEN 61 THEN 'Export'
           WHEN 62 THEN 'Mask Config'
           WHEN 63 THEN 'File Resources'
           WHEN 64 THEN 'List'
           WHEN 65 THEN 'Delete'
           WHEN 66 THEN 'Online Sessions'
           WHEN 67 THEN 'List'
           WHEN 68 THEN 'Revoke'
           WHEN 70 THEN 'Config'
           WHEN 71 THEN 'Update'
           WHEN 72 THEN 'Test'
           WHEN 77 THEN 'View Secret'
           WHEN 73 THEN 'Restore'
           WHEN 74 THEN 'Destroy'
           WHEN 75 THEN 'Edit'
           WHEN 76 THEN 'Create'
           WHEN 78 THEN 'Global Overview'
           WHEN 79 THEN 'Global Statistics'
           WHEN 80 THEN 'Global Server Monitor'
           WHEN 81 THEN 'Notification Management'
           WHEN 82 THEN 'Send Notification'
           WHEN 83 THEN 'Business'
           WHEN 84 THEN 'Reference Products'
           WHEN 85 THEN 'List'
           WHEN 86 THEN 'Create'
           WHEN 87 THEN 'Edit'
           WHEN 88 THEN 'Delete'
           WHEN 89 THEN 'My Todos'
           ELSE ''
           END,
       `created_at`,
       `updated_at`
FROM `menu`
WHERE `deleted_at` = 0;

INSERT INTO `menu_i18n` (`menu_id`, `locale`, `title`, `created_at`, `updated_at`)
SELECT `id`, 'en-US', 'Task Config', @init_time, @init_time
FROM `menu`
WHERE `code` = 'task:update'
  AND `deleted_at` = 0
ON DUPLICATE KEY UPDATE `title` = VALUES(`title`), `updated_at` = VALUES(`updated_at`);

INSERT INTO `menu_i18n` (`menu_id`, `locale`, `title`, `created_at`, `updated_at`)
SELECT `id`, 'en-US', 'Task Stats', @init_time, @init_time
FROM `menu`
WHERE `code` = 'task:stats'
  AND `deleted_at` = 0
ON DUPLICATE KEY UPDATE `title` = VALUES(`title`), `updated_at` = VALUES(`updated_at`);

INSERT INTO `role_menu_map` (`role_id`, `menu_id`, `created_at`, `updated_at`)
SELECT 1, m.`id`, @init_time, @init_time
FROM `menu` m
WHERE m.`deleted_at` = 0
  AND m.`status` = 1
  AND m.`is_auth` = 1
  AND (
    m.`type` = 3
        OR (
        m.`type` = 2
            AND NOT EXISTS (
            SELECT 1
            FROM `menu` child
            WHERE child.`deleted_at` = 0
              AND child.`status` = 1
              AND child.`is_auth` = 1
              AND child.`type` IN (2, 3)
              AND FIND_IN_SET(m.`id`, child.`pids`) > 0
        )
    )
    );

INSERT INTO `sys_config` (`config_key`, `config_value`, `value_type`, `group_code`, `is_system`,
                          `is_sensitive`, `is_visible`, `manage_tab`, `status`, `sort`, `remark`, `created_at`, `updated_at`, `deleted_at`)
VALUES ('auth.login_lock_enabled', 'true', 'bool', 'auth', 1, 0, 1, '', 1, 89, '是否开启登录失败锁定', @init_time, @init_time, 0),
       ('auth.login_max_failures', '5', 'number', 'auth', 1, 0, 1, '', 1, 88, '登录连续失败阈值', @init_time, @init_time, 0),
       ('auth.login_lock_minutes', '15', 'number', 'auth', 1, 0, 1, '', 1, 87, '登录锁定时长（分钟）', @init_time, @init_time, 0),
       ('task.cron_demo_enabled', 'false', 'bool', 'task', 1, 0, 1, '', 1, 80, '是否启用演示定时任务，默认关闭', @init_time, @init_time, 0),
       ('audit.sensitive_fields', '{"common":["password","pwd","passwd","pass","secret","token","access_token","refresh_token","api_key","apikey","apiKey","pin","cvv","cvc","cvv2","security_code"],"request_header":["authorization","auth","cookie","x-api-key"],"request_body":["phone","mobile","tel","telephone","phone_number","mobile_number","email","mail","id_card","idcard","identity","id_number","bank_card","bankcard","card_number","card_no","ssn","social_security","real_name","realname","name"],"response_header":["set-cookie","authorization","auth","x-api-key","refresh-exp","cookie"],"response_body":["phone","mobile","tel","telephone","phone_number","mobile_number","email","mail","id_card","idcard","identity","id_number","bank_card","bankcard","card_number","card_no","ssn","social_security"]}', 'json', 'audit', 1, 1, 0, 'audit_mask', 1, 95, '请求日志脱敏字段配置', @init_time, @init_time, 0),
       ('storage.active_driver', 'local', 'string', 'storage', 1, 0, 0, 'storage', 1, 90, '当前启用的文件存储驱动', @init_time, @init_time, 0),
       ('storage.config', '{"local":{},"aliyun_oss":{},"signed_url_ttl_seconds":300,"max_file_size_mb":10,"allowed_mime_types":[]}', 'json', 'storage', 1, 1, 0, 'storage', 1, 80, '文件存储配置', @init_time, @init_time, 0)
ON DUPLICATE KEY UPDATE `updated_at` = VALUES(`updated_at`);

INSERT INTO `sys_dict_type` (`type_code`, `is_system`, `status`, `sort`, `remark`, `created_at`,
                             `updated_at`, `deleted_at`)
VALUES ('common_status', 1, 1, 100, '0=禁用,1=启用', @init_time, @init_time, 0),
       ('yes_no', 1, 1, 90, '0=否,1=是', @init_time, @init_time, 0),
       ('menu_type', 1, 1, 80, '1=目录,2=菜单,3=按钮', @init_time, @init_time, 0),
       ('api_auth_mode', 1, 1, 70, '0=无需登录,1=需要登录,2=需要接口权限', @init_time, @init_time, 0),
       ('http_method', 1, 1, 60, '常用 HTTP 方法', @init_time, @init_time, 0),
       ('task_kind', 1, 1, 50, '任务类型', @init_time, @init_time, 0),
       ('task_source', 1, 1, 40, '任务来源', @init_time, @init_time, 0),
       ('task_run_status', 1, 1, 30, '任务执行状态', @init_time, @init_time, 0),
       ('task_record_success_mode', 1, 1, 20, '任务成功明细记录策略', @init_time, @init_time, 0),
       ('task_detail_record_mode', 1, 1, 10, '任务明细记录模式', @init_time, @init_time, 0)
ON DUPLICATE KEY UPDATE `updated_at` = VALUES(`updated_at`);

INSERT INTO `sys_dict_item` (`type_code`, `value`, `color`, `tag_type`, `is_default`, `is_system`, `status`,
                             `sort`, `remark`, `created_at`, `updated_at`, `deleted_at`)
VALUES ('common_status', '0', '#909399', 'info', 0, 1, 1, 10, '', @init_time, @init_time, 0),
       ('common_status', '1', '#67c23a', 'success', 1, 1, 1, 20, '', @init_time, @init_time, 0),
       ('yes_no', '0', '#909399', 'info', 1, 1, 1, 10, '', @init_time, @init_time, 0),
       ('yes_no', '1', '#67c23a', 'success', 0, 1, 1, 20, '', @init_time, @init_time, 0),
       ('menu_type', '1', '#409eff', 'primary', 0, 1, 1, 30, '', @init_time, @init_time, 0),
       ('menu_type', '2', '#67c23a', 'success', 1, 1, 1, 20, '', @init_time, @init_time, 0),
       ('menu_type', '3', '#e6a23c', 'warning', 0, 1, 1, 10, '', @init_time, @init_time, 0),
       ('api_auth_mode', '0', '#909399', 'info', 0, 1, 1, 30, '', @init_time, @init_time, 0),
       ('api_auth_mode', '1', '#409eff', 'primary', 1, 1, 1, 20, '', @init_time, @init_time, 0),
       ('api_auth_mode', '2', '#f56c6c', 'danger', 0, 1, 1, 10, '', @init_time, @init_time, 0),
       ('http_method', 'GET', '#67c23a', 'success', 1, 1, 1, 70, '', @init_time, @init_time, 0),
       ('http_method', 'POST', '#409eff', 'primary', 0, 1, 1, 60, '', @init_time, @init_time, 0),
       ('http_method', 'PUT', '#e6a23c', 'warning', 0, 1, 1, 50, '', @init_time, @init_time, 0),
       ('http_method', 'DELETE', '#f56c6c', 'danger', 0, 1, 1, 40, '', @init_time, @init_time, 0),
       ('http_method', 'PATCH', '#909399', 'info', 0, 1, 1, 30, '', @init_time, @init_time, 0),
       ('http_method', 'OPTIONS', '#909399', 'info', 0, 1, 1, 20, '', @init_time, @init_time, 0),
       ('http_method', 'HEAD', '#909399', 'info', 0, 1, 1, 10, '', @init_time, @init_time, 0),
       ('task_kind', 'async', '#909399', 'info', 1, 1, 1, 20, '', @init_time, @init_time, 0),
       ('task_kind', 'cron', '#e6a23c', 'warning', 0, 1, 1, 10, '', @init_time, @init_time, 0),
       ('task_source', 'queue', '#409eff', 'primary', 1, 1, 1, 30, '', @init_time, @init_time, 0),
       ('task_source', 'cron', '#e6a23c', 'warning', 0, 1, 1, 20, '', @init_time, @init_time, 0),
       ('task_source', 'manual', '#67c23a', 'success', 0, 1, 1, 10, '', @init_time, @init_time, 0),
       ('task_run_status', 'pending', '#909399', 'info', 1, 1, 1, 60, '', @init_time, @init_time, 0),
       ('task_run_status', 'running', '#e6a23c', 'warning', 0, 1, 1, 50, '', @init_time, @init_time, 0),
       ('task_run_status', 'success', '#67c23a', 'success', 0, 1, 1, 40, '', @init_time, @init_time, 0),
       ('task_run_status', 'failed', '#f56c6c', 'danger', 0, 1, 1, 30, '', @init_time, @init_time, 0),
       ('task_run_status', 'canceled', '#909399', 'info', 0, 1, 1, 20, '', @init_time, @init_time, 0),
       ('task_run_status', 'retrying', '#e6a23c', 'warning', 0, 1, 1, 10, '', @init_time, @init_time, 0),
       ('task_run_status', 'timeout', '#f56c6c', 'danger', 0, 1, 1, 8, '', @init_time, @init_time, 0),
       ('task_run_status', 'interrupted', '#909399', 'info', 0, 1, 1, 6, '', @init_time, @init_time, 0),
       ('task_record_success_mode', 'all', '#67c23a', 'success', 1, 1, 1, 40, '', @init_time, @init_time, 0),
       ('task_record_success_mode', 'sample', '#409eff', 'primary', 0, 1, 1, 30, '', @init_time, @init_time, 0),
       ('task_record_success_mode', 'interval', '#e6a23c', 'warning', 0, 1, 1, 20, '', @init_time, @init_time, 0),
       ('task_record_success_mode', 'none', '#909399', 'info', 0, 1, 1, 10, '', @init_time, @init_time, 0),
       ('task_detail_record_mode', 'all', '#67c23a', 'success', 1, 1, 1, 40, '', @init_time, @init_time, 0),
       ('task_detail_record_mode', 'sampled', '#409eff', 'primary', 0, 1, 1, 30, '', @init_time, @init_time, 0),
       ('task_detail_record_mode', 'forced', '#e6a23c', 'warning', 0, 1, 1, 20, '', @init_time, @init_time, 0),
       ('task_detail_record_mode', 'none', '#909399', 'info', 0, 1, 1, 10, '', @init_time, @init_time, 0)
ON DUPLICATE KEY UPDATE `updated_at` = VALUES(`updated_at`);

INSERT INTO `sys_config_i18n` (`config_id`, `locale`, `config_name`, `created_at`, `updated_at`)
SELECT `id`,
       'zh-CN',
       CASE
           WHEN `config_key` = 'auth.login_lock_enabled' THEN '登录失败锁定开关'
           WHEN `config_key` = 'auth.login_max_failures' THEN '登录失败锁定阈值'
           WHEN `config_key` = 'auth.login_lock_minutes' THEN '登录失败锁定时长（分钟）'
           WHEN `config_key` = 'task.cron_demo_enabled' THEN '演示定时任务开关'
           WHEN `config_key` = 'audit.sensitive_fields' THEN '请求日志脱敏配置'
           WHEN `config_key` = 'storage.active_driver' THEN '当前存储驱动'
           WHEN `config_key` = 'storage.config' THEN '文件存储配置'
           ELSE ''
           END,
       @init_time,
       @init_time
FROM `sys_config`
WHERE `deleted_at` = 0
  AND `config_key` IN ('auth.login_lock_enabled', 'auth.login_max_failures', 'auth.login_lock_minutes', 'task.cron_demo_enabled', 'audit.sensitive_fields', 'storage.active_driver', 'storage.config')
ON DUPLICATE KEY UPDATE `config_name` = VALUES(`config_name`), `updated_at` = VALUES(`updated_at`);

INSERT INTO `sys_config_i18n` (`config_id`, `locale`, `config_name`, `created_at`, `updated_at`)
SELECT `id`,
       'en-US',
       CASE
           WHEN `config_key` = 'auth.login_lock_enabled' THEN 'Login Lock Enabled'
           WHEN `config_key` = 'auth.login_max_failures' THEN 'Login Lock Max Failures'
           WHEN `config_key` = 'auth.login_lock_minutes' THEN 'Login Lock Minutes'
           WHEN `config_key` = 'task.cron_demo_enabled' THEN 'Cron Demo Enabled'
           WHEN `config_key` = 'audit.sensitive_fields' THEN 'Request Log Mask Config'
           WHEN `config_key` = 'storage.active_driver' THEN 'Active Storage Driver'
           WHEN `config_key` = 'storage.config' THEN 'File Storage Config'
           ELSE ''
           END,
       @init_time,
       @init_time
FROM `sys_config`
WHERE `deleted_at` = 0
  AND `config_key` IN ('auth.login_lock_enabled', 'auth.login_max_failures', 'auth.login_lock_minutes', 'task.cron_demo_enabled', 'audit.sensitive_fields', 'storage.active_driver', 'storage.config')
ON DUPLICATE KEY UPDATE `config_name` = VALUES(`config_name`), `updated_at` = VALUES(`updated_at`);

INSERT INTO `sys_dict_type_i18n` (`dict_type_id`, `locale`, `type_name`, `created_at`, `updated_at`)
SELECT `id`,
       'zh-CN',
       CASE
           WHEN `type_code` = 'common_status' THEN '通用状态'
           WHEN `type_code` = 'yes_no' THEN '是否选项'
           WHEN `type_code` = 'menu_type' THEN '菜单类型'
           WHEN `type_code` = 'api_auth_mode' THEN '接口鉴权模式'
           WHEN `type_code` = 'http_method' THEN 'HTTP 方法'
           WHEN `type_code` = 'task_kind' THEN '任务类型'
           WHEN `type_code` = 'task_source' THEN '任务来源'
           WHEN `type_code` = 'task_run_status' THEN '任务执行状态'
           WHEN `type_code` = 'task_record_success_mode' THEN '任务成功明细记录策略'
           WHEN `type_code` = 'task_detail_record_mode' THEN '任务明细记录模式'
           ELSE ''
           END,
       @init_time,
       @init_time
FROM `sys_dict_type`
WHERE `deleted_at` = 0
  AND `type_code` IN ('common_status', 'yes_no', 'menu_type', 'api_auth_mode', 'http_method', 'task_kind', 'task_source', 'task_run_status', 'task_record_success_mode', 'task_detail_record_mode')
ON DUPLICATE KEY UPDATE `type_name` = VALUES(`type_name`), `updated_at` = VALUES(`updated_at`);

INSERT INTO `sys_dict_type_i18n` (`dict_type_id`, `locale`, `type_name`, `created_at`, `updated_at`)
SELECT `id`,
       'en-US',
       CASE
           WHEN `type_code` = 'common_status' THEN 'Common Status'
           WHEN `type_code` = 'yes_no' THEN 'Yes/No'
           WHEN `type_code` = 'menu_type' THEN 'Menu Type'
           WHEN `type_code` = 'api_auth_mode' THEN 'API Auth Mode'
           WHEN `type_code` = 'http_method' THEN 'HTTP Method'
           WHEN `type_code` = 'task_kind' THEN 'Task Kind'
           WHEN `type_code` = 'task_source' THEN 'Task Source'
           WHEN `type_code` = 'task_run_status' THEN 'Task Run Status'
           WHEN `type_code` = 'task_record_success_mode' THEN 'Task Success Detail Policy'
           WHEN `type_code` = 'task_detail_record_mode' THEN 'Task Detail Record Mode'
           ELSE ''
           END,
       @init_time,
       @init_time
FROM `sys_dict_type`
WHERE `deleted_at` = 0
  AND `type_code` IN ('common_status', 'yes_no', 'menu_type', 'api_auth_mode', 'http_method', 'task_kind', 'task_source', 'task_run_status', 'task_record_success_mode', 'task_detail_record_mode')
ON DUPLICATE KEY UPDATE `type_name` = VALUES(`type_name`), `updated_at` = VALUES(`updated_at`);

INSERT INTO `sys_dict_item_i18n` (`dict_item_id`, `locale`, `label`, `created_at`, `updated_at`)
SELECT `id`,
       'zh-CN',
       CASE
           WHEN `type_code` = 'common_status' AND `value` = '0' THEN '禁用'
           WHEN `type_code` = 'common_status' AND `value` = '1' THEN '启用'
           WHEN `type_code` = 'yes_no' AND `value` = '0' THEN '否'
           WHEN `type_code` = 'yes_no' AND `value` = '1' THEN '是'
           WHEN `type_code` = 'menu_type' AND `value` = '1' THEN '目录'
           WHEN `type_code` = 'menu_type' AND `value` = '2' THEN '菜单'
           WHEN `type_code` = 'menu_type' AND `value` = '3' THEN '按钮'
           WHEN `type_code` = 'api_auth_mode' AND `value` = '0' THEN '无需登录'
           WHEN `type_code` = 'api_auth_mode' AND `value` = '1' THEN '需要登录'
           WHEN `type_code` = 'api_auth_mode' AND `value` = '2' THEN '需要接口权限'
           WHEN `type_code` = 'http_method' THEN `value`
           WHEN `type_code` = 'task_kind' AND `value` = 'async' THEN '异步'
           WHEN `type_code` = 'task_kind' AND `value` = 'cron' THEN '定时'
           WHEN `type_code` = 'task_source' AND `value` = 'queue' THEN '队列'
           WHEN `type_code` = 'task_source' AND `value` = 'cron' THEN '定时'
           WHEN `type_code` = 'task_source' AND `value` = 'manual' THEN '手动'
           WHEN `type_code` = 'task_run_status' AND `value` = 'pending' THEN '等待中'
           WHEN `type_code` = 'task_run_status' AND `value` = 'running' THEN '执行中'
           WHEN `type_code` = 'task_run_status' AND `value` = 'success' THEN '成功'
           WHEN `type_code` = 'task_run_status' AND `value` = 'failed' THEN '失败'
           WHEN `type_code` = 'task_run_status' AND `value` = 'canceled' THEN '已取消'
           WHEN `type_code` = 'task_run_status' AND `value` = 'retrying' THEN '重试中'
           WHEN `type_code` = 'task_run_status' AND `value` = 'timeout' THEN '超时'
           WHEN `type_code` = 'task_run_status' AND `value` = 'interrupted' THEN '已中断'
           WHEN `type_code` = 'task_record_success_mode' AND `value` = 'all' THEN '全部记录'
           WHEN `type_code` = 'task_record_success_mode' AND `value` = 'sample' THEN '按比例抽样'
           WHEN `type_code` = 'task_record_success_mode' AND `value` = 'interval' THEN '按间隔记录'
           WHEN `type_code` = 'task_record_success_mode' AND `value` = 'none' THEN '不记录成功明细'
           WHEN `type_code` = 'task_detail_record_mode' AND `value` = 'all' THEN '全部记录'
           WHEN `type_code` = 'task_detail_record_mode' AND `value` = 'sampled' THEN '抽样记录'
           WHEN `type_code` = 'task_detail_record_mode' AND `value` = 'forced' THEN '强制记录'
           WHEN `type_code` = 'task_detail_record_mode' AND `value` = 'none' THEN '未记录明细'
           ELSE ''
           END,
       @init_time,
       @init_time
FROM `sys_dict_item`
WHERE `deleted_at` = 0
  AND (
    (`type_code` = 'common_status' AND `value` IN ('0', '1')) OR
    (`type_code` = 'yes_no' AND `value` IN ('0', '1')) OR
    (`type_code` = 'menu_type' AND `value` IN ('1', '2', '3')) OR
    (`type_code` = 'api_auth_mode' AND `value` IN ('0', '1', '2')) OR
    (`type_code` = 'http_method' AND `value` IN ('GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'OPTIONS', 'HEAD')) OR
    (`type_code` = 'task_kind' AND `value` IN ('async', 'cron')) OR
    (`type_code` = 'task_source' AND `value` IN ('queue', 'cron', 'manual')) OR
    (`type_code` = 'task_run_status' AND `value` IN ('pending', 'running', 'success', 'failed', 'canceled', 'retrying', 'timeout', 'interrupted')) OR
    (`type_code` = 'task_record_success_mode' AND `value` IN ('all', 'sample', 'interval', 'none')) OR
    (`type_code` = 'task_detail_record_mode' AND `value` IN ('all', 'sampled', 'forced', 'none'))
    )
ON DUPLICATE KEY UPDATE `label` = VALUES(`label`), `updated_at` = VALUES(`updated_at`);

INSERT INTO `sys_dict_item_i18n` (`dict_item_id`, `locale`, `label`, `created_at`, `updated_at`)
SELECT `id`,
       'en-US',
       CASE
           WHEN `type_code` = 'common_status' AND `value` = '0' THEN 'Disabled'
           WHEN `type_code` = 'common_status' AND `value` = '1' THEN 'Enabled'
           WHEN `type_code` = 'yes_no' AND `value` = '0' THEN 'No'
           WHEN `type_code` = 'yes_no' AND `value` = '1' THEN 'Yes'
           WHEN `type_code` = 'menu_type' AND `value` = '1' THEN 'Directory'
           WHEN `type_code` = 'menu_type' AND `value` = '2' THEN 'Menu'
           WHEN `type_code` = 'menu_type' AND `value` = '3' THEN 'Button'
           WHEN `type_code` = 'api_auth_mode' AND `value` = '0' THEN 'No Auth'
           WHEN `type_code` = 'api_auth_mode' AND `value` = '1' THEN 'Login Required'
           WHEN `type_code` = 'api_auth_mode' AND `value` = '2' THEN 'Permission Required'
           WHEN `type_code` = 'http_method' THEN `value`
           WHEN `type_code` = 'task_kind' AND `value` = 'async' THEN 'Async'
           WHEN `type_code` = 'task_kind' AND `value` = 'cron' THEN 'Cron'
           WHEN `type_code` = 'task_source' AND `value` = 'queue' THEN 'Queue'
           WHEN `type_code` = 'task_source' AND `value` = 'cron' THEN 'Cron'
           WHEN `type_code` = 'task_source' AND `value` = 'manual' THEN 'Manual'
           WHEN `type_code` = 'task_run_status' AND `value` = 'pending' THEN 'Pending'
           WHEN `type_code` = 'task_run_status' AND `value` = 'running' THEN 'Running'
           WHEN `type_code` = 'task_run_status' AND `value` = 'success' THEN 'Success'
           WHEN `type_code` = 'task_run_status' AND `value` = 'failed' THEN 'Failed'
           WHEN `type_code` = 'task_run_status' AND `value` = 'canceled' THEN 'Canceled'
           WHEN `type_code` = 'task_run_status' AND `value` = 'retrying' THEN 'Retrying'
           WHEN `type_code` = 'task_run_status' AND `value` = 'timeout' THEN 'Timeout'
           WHEN `type_code` = 'task_run_status' AND `value` = 'interrupted' THEN 'Interrupted'
           WHEN `type_code` = 'task_record_success_mode' AND `value` = 'all' THEN 'Record All'
           WHEN `type_code` = 'task_record_success_mode' AND `value` = 'sample' THEN 'Sample'
           WHEN `type_code` = 'task_record_success_mode' AND `value` = 'interval' THEN 'Interval'
           WHEN `type_code` = 'task_record_success_mode' AND `value` = 'none' THEN 'No Success Detail'
           WHEN `type_code` = 'task_detail_record_mode' AND `value` = 'all' THEN 'All'
           WHEN `type_code` = 'task_detail_record_mode' AND `value` = 'sampled' THEN 'Sampled'
           WHEN `type_code` = 'task_detail_record_mode' AND `value` = 'forced' THEN 'Forced'
           WHEN `type_code` = 'task_detail_record_mode' AND `value` = 'none' THEN 'No Detail'
           ELSE ''
           END,
       @init_time,
       @init_time
FROM `sys_dict_item`
WHERE `deleted_at` = 0
  AND (
    (`type_code` = 'common_status' AND `value` IN ('0', '1')) OR
    (`type_code` = 'yes_no' AND `value` IN ('0', '1')) OR
    (`type_code` = 'menu_type' AND `value` IN ('1', '2', '3')) OR
    (`type_code` = 'api_auth_mode' AND `value` IN ('0', '1', '2')) OR
    (`type_code` = 'http_method' AND `value` IN ('GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'OPTIONS', 'HEAD')) OR
    (`type_code` = 'task_kind' AND `value` IN ('async', 'cron')) OR
    (`type_code` = 'task_source' AND `value` IN ('queue', 'cron', 'manual')) OR
    (`type_code` = 'task_run_status' AND `value` IN ('pending', 'running', 'success', 'failed', 'canceled', 'retrying', 'timeout', 'interrupted')) OR
    (`type_code` = 'task_record_success_mode' AND `value` IN ('all', 'sample', 'interval', 'none')) OR
    (`type_code` = 'task_detail_record_mode' AND `value` IN ('all', 'sampled', 'forced', 'none'))
    )
ON DUPLICATE KEY UPDATE `label` = VALUES(`label`), `updated_at` = VALUES(`updated_at`);

INSERT INTO `api_group` (`id`, `pid`, `code`, `name`, `created_at`, `updated_at`)
VALUES (10, 0, 'system', '系统管理模块', @init_time, @init_time),
       (11, 10, 'sysConfig', '系统参数模块', @init_time, @init_time),
       (12, 10, 'sysDict', '系统字典模块', @init_time, @init_time),
       (13, 0, 'task', '任务中心模块', @init_time, @init_time),
       (14, 10, 'file', '文件资源模块', @init_time, @init_time),
       (15, 3, 'session', '在线会话模块', @init_time, @init_time),
       (16, 0, 'product', '产品模块', @init_time, @init_time)
ON DUPLICATE KEY UPDATE `name` = VALUES(`name`), `updated_at` = VALUES(`updated_at`);

COMMIT;
