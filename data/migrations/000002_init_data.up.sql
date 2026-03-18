BEGIN;

-- 初始化系统数据

-- 初始密码 123456
INSERT INTO `admin_user` (`id`, `nickname`, `username`, `password`, `phone_number`, `full_phone_number`,
                            `country_code`, `email`, `avatar`, `status`,
                            `is_super_admin`,
                            `created_at`, `updated_at`, `deleted_at`)
VALUES (1, '超级管理员', 'super_admin', '$2a$10$OuKQoJGH7xkCgwFISmDve.euBDbOCnYEJX6R22QMeLxCLwdoJ4iyi', '18888888888',
        '8618888888888', '86', 'admin@go-layout.com', 'https://avatars.githubusercontent.com/u/48752601?v=4', 1, 1,
        '2023-05-01 00:00:00', '2023-05-01 00:00:00', 0);

INSERT INTO `department` (`id`, `code`, `is_system`, `pid`, `pids`, `name`, `description`, `level`, `sort`,
                          `children_num`, `user_number`, `created_at`, `updated_at`, `deleted_at`)
VALUES (1, 'default_department', 1, 0, '0', '默认部门', '系统默认部门', 1, 100, 0, 1,
        '2023-05-01 00:00:00', '2023-05-01 00:00:00', 0);

INSERT INTO `role` (`id`, `code`, `is_system`, `pid`, `pids`, `name`, `description`, `level`, `sort`, `children_num`,
                    `status`, `created_at`, `updated_at`, `deleted_at`)
VALUES (1, 'super_admin', 1, 0, '0', '超级管理员', '系统默认超级管理员角色', 1, 100, 0, 1,
        '2023-05-01 00:00:00', '2023-05-01 00:00:00', 0);

INSERT INTO `admin_user_department_map` (`uid`, `dept_id`, `is_admin`, `created_at`, `updated_at`)
VALUES (1, 1, 1, '2023-05-01 00:00:00', '2023-05-01 00:00:00');

INSERT INTO `admin_user_role_map` (`uid`, `role_id`, `created_at`, `updated_at`)
VALUES (1, 1, '2023-05-01 00:00:00', '2023-05-01 00:00:00');

-- 初始化权限分组数据
INSERT INTO `api_group` (`id`, `pid`, `code`, `name`, `created_at`, `updated_at`)
VALUES (1, 0, 'other', '其他', '2025-04-26 18:00:00', '2025-04-26 18:00:00'),
       (2, 0, 'login', '登录模块', '2025-04-26 18:00:00', '2025-04-26 18:00:00'),
       (3, 0, 'auth', '权限模块', '2025-04-26 18:00:00', '2025-04-26 18:00:00'),
       (4, 3, 'adminUser', '管理员模块', '2025-04-26 18:00:00', '2025-04-26 18:00:00'),
       (5, 3, 'api', 'API模块', '2025-04-26 18:00:00', '2025-04-26 18:00:00'),
       (6, 3, 'role', '角色模块', '2025-04-26 18:00:00', '2025-04-26 18:00:00'),
       (7, 3, 'menu', '菜单模块', '2025-04-26 18:00:00', '2025-04-26 18:00:00'),
       (8, 0, 'common', '公共模块', '2025-04-26 18:00:00', '2025-04-26 18:00:00'),
       (9, 0, 'log', '日志模块', '2025-04-26 18:00:00', '2025-04-26 18:00:00');

-- 初始化菜单数据
INSERT INTO `menu` (`id`, `icon`, `title`, `code`, `path`, `full_path`, `redirect`, `name`, `component`, `animate_enter`, `animate_leave`, `animate_duration`, `is_show`, `status`, `is_auth`, `is_external_links`, `is_new_window`, `sort`, `type`, `pid`, `level`, `pids`, `children_num`, `description`, `created_at`, `updated_at`, `deleted_at`) VALUES
(1, 'ep:menu', '首页', '', '', '/', '', 'Home', 'home/index.vue', '', '', 0.00, 1, 1, 0, 0, 0, 100, 2, 0, 1, '0', 0, '', '2024-09-27 13:36:50', '2025-11-15 14:36:40', 0),
(2, 'ant-design:lock-outlined', '权限管理', '', 'permission', '/permission', 'AdminUserList', 'Permission', '', '', '', 0.00, 1, 1, 1, 0, 0, 99, 1, 0, 1, '0', 0, '', '2025-04-16 15:36:33', '2025-04-22 18:16:25', 0),
(3, 'ant-design:api-outlined', '接口', '', 'list', '/permission/list', '', 'PermissionList', 'permission/api.vue', '', '', 0.00, 1, 1, 1, 0, 0, 100, 2, 2, 2, '0,2', 2, '', '2025-04-16 15:41:54', '2025-11-25 17:23:53', 0),
(4, 'ant-design:menu-outlined', '菜单', '', 'menu-list', '/permission/menu-list', '', 'MenuList', 'permission/menuList.vue', '', '', 0.00, 1, 1, 1, 0, 0, 105, 2, 2, 2, '0,2', 5, '', '2025-04-16 15:45:31', '2025-11-25 17:23:44', 0),
(5, 'ix:about', '关于', '', '/about', '/about', '', 'About', 'about/index.vue', '', '', 0.00, 1, 1, 1, 0, 0, 90, 2, 0, 1, '0', 0, '', '2025-04-16 16:47:58', '2025-04-23 15:01:05', 0),
(6, 'ix:about', 'CSDN', '', 'https://blog.csdn.net/u010324331', 'https://blog.csdn.net/u010324331', '', 'CSDN', '', '', '', 0.00, 1, 1, 1, 1, 0, 80, 2, 0, 1, '0', 0, '', '2025-04-16 16:51:17', '2025-04-18 18:08:51', 0),
(7, 'ep:user', '管理员', '', 'admin-user-list', '/permission/admin-user-list', '', 'AdminUserList', 'permission/adminUser.vue', '', '', 0.00, 1, 1, 1, 0, 0, 120, 2, 2, 2, '0,2', 5, '', '2025-04-19 11:19:36', '2025-11-25 17:20:23', 0),
(8, 'ant-design:usergroup-add-outlined', '角色', '', 'role-list', '/permission/role-list', '', 'RoleList', 'permission/role.vue', '', '', 0.00, 1, 1, 1, 0, 0, 115, 2, 2, 2, '0,2', 5, '', '2025-04-21 16:51:22', '2025-11-25 17:22:21', 0),
(9, 'tdesign:tree-square-dot', '部门', '', 'department-list', '/permission/department-list', '', 'DepartmentList', 'permission/department.vue', '', '', 0.00, 1, 1, 1, 0, 0, 115, 2, 2, 2, '0,2', 6, '', '2025-04-21 16:51:22', '2025-11-25 17:21:30', 0),
(10, 'ant-design:edit-filled', '编辑', 'adminUser:update', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 7, 3, '0,2,7', 0, '', '2025-11-13 16:45:19', '2025-11-18 17:14:25', 0),
(11, 'ant-design:plus-outlined', '新增管理员', 'adminUser:add', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 7, 3, '0,2,7', 0, '', '2025-11-15 10:06:52', '2025-11-18 17:22:41', 0),
(12, 'ant-design:user-switch-outlined', '绑定角色', 'adminUser:bindRole', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 7, 3, '0,2,7', 0, '', '2025-11-15 10:06:52', '2025-11-18 17:20:53', 0),
(13, 'ant-design:delete-outlined', '删除', 'adminUser:delete', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 7, 3, '0,2,7', 0, '', '2025-11-15 10:06:52', '2025-11-18 17:24:34', 0),
(14, 'ant-design:plus-outlined', '新增菜单', 'menu:add', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 4, 3, '0,2,4', 0, '', '2025-11-15 10:06:52', '2025-11-18 17:39:20', 0),
(15, 'ant-design:plus-circle-outlined', '新增下级', 'menu:addChild', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 4, 3, '0,2,4', 0, '', '2025-11-15 10:06:52', '2025-11-18 17:39:03', 0),
(16, 'ant-design:edit-filled', '编辑', 'menu:update', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 4, 3, '0,2,4', 0, '', '2025-11-15 10:06:52', '2025-11-18 17:38:12', 0),
(17, 'ant-design:delete-outlined', '删除', 'menu:delete', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 4, 3, '0,2,4', 0, '', '2025-11-15 10:06:52', '2025-11-18 17:37:02', 0),
(18, 'ant-design:plus-outlined', '新增角色', 'role:add', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 8, 3, '0,2,8', 0, '', '2025-11-15 10:06:52', '2025-11-18 17:36:08', 0),
(19, 'ant-design:edit-filled', '编辑', 'role:update', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 8, 3, '0,2,8', 0, '', '2025-11-15 10:06:52', '2025-11-18 17:36:22', 0),
(20, 'ant-design:delete-outlined', '删除', 'role:delete', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 8, 3, '0,2,8', 0, '', '2025-11-15 10:06:52', '2025-11-18 17:32:12', 0),
(21, 'ant-design:plus-outlined', '新增部门', 'department:add', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 9, 3, '0,2,9', 0, '', '2025-11-15 10:06:52', '2025-11-18 17:29:43', 0),
(22, 'ant-design:plus-circle-outlined', '新增', 'department:addChild', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 9, 3, '0,2,9', 0, '', '2025-11-15 10:06:52', '2025-11-18 17:29:07', 0),
(23, 'ant-design:edit-filled', '编辑', 'department:update', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 9, 3, '0,2,9', 0, '', '2025-11-15 10:06:52', '2025-11-18 17:33:40', 0),
(24, 'ant-design:user-switch-outlined', '绑定角色', 'department:bindRole', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 9, 3, '0,2,9', 0, '', '2025-11-15 10:06:52', '2025-11-18 17:27:57', 0),
(25, 'ant-design:delete-outlined', '删除', 'department:delete', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 9, 3, '0,2,9', 0, '', '2025-11-15 10:06:52', '2025-11-18 17:26:52', 0),
(26, 'ant-design:edit-filled', '编辑', 'api:update', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 3, 3, '0,2,3', 0, '', '2025-11-15 10:06:52', '2025-11-18 17:39:39', 0),
(28, 'ant-design:plus-circle-outlined', '新增', 'role:addChild', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 8, 3, '0,2,8', 0, '', '2025-11-17 17:46:21', '2025-11-18 17:41:45', 0),
(29, 'ep:tickets', '日志管理', '', 'log', '/log', 'RequestLog', 'Log', '', '', '', 0.00, 1, 1, 1, 0, 0, 90, 1, 0, 1, '0', 2, '', '2025-11-20 16:16:47', '2025-11-20 16:17:04', 0),
(30, 'ep:document', '请求日志', '', 'request-log', '/log/request-log', '', 'RequestLog', 'log/request.vue', '', '', 0.00, 1, 1, 1, 0, 0, 100, 2, 29, 2, '0,29', 2, '', '2025-11-20 16:14:39', '2025-11-25 17:25:13', 0),
(31, 'ep:document', '管理员登录日志', '', 'admin-login-log', '/log/admin-login-log', '', 'AdminLoginLog', 'log/adminLogin.vue', '', '', 0.00, 1, 1, 1, 0, 0, 100, 2, 29, 2, '0,29', 2, '', '2025-11-20 16:16:47', '2025-11-25 17:24:30', 0),
(32, 'ep:document', '详情', 'adminLoginLog:detail', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 31, 3, '0,29,31', 0, '', '2025-11-22 11:48:10', '2025-11-22 11:48:10', 0),
(33, 'ep:document', '详情', 'requestLog:detail', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 30, 3, '0,29,30', 0, '', '2025-11-22 11:48:44', '2025-11-22 11:48:44', 0),
(34, 'ant-design:search-outlined', '列表', 'adminUser:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 7, 3, '0,2,7', 0, '', '2025-11-25 17:20:23', '2025-11-25 17:20:23', 0),
(35, 'ant-design:search-outlined', '列表', 'department:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 9, 3, '0,2,9', 0, '', '2025-11-25 17:21:22', '2025-11-25 17:21:22', 0),
(36, 'ant-design:search-outlined', '列表', 'role:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 8, 3, '0,2,8', 0, '', '2025-11-25 17:22:13', '2025-11-25 17:22:13', 0),
(37, 'ant-design:search-outlined', '列表', 'menu:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 4, 3, '0,2,4', 0, '', '2025-11-25 17:23:02', '2025-11-25 17:23:02', 0),
(38, 'ant-design:search-outlined', '列表', 'api:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 3, 3, '0,2,3', 0, '', '2025-11-25 17:23:35', '2025-11-25 17:23:35', 0),
(39, 'ant-design:search-outlined', '列表', 'adminLoginLog:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 31, 3, '0,29,31', 0, '', '2025-11-25 17:24:20', '2025-11-25 17:24:20', 0),
(40, 'ant-design:search-outlined', '列表', 'requestLog:list', '', '', '', '', '', '', '', 0.00, 1, 1, 1, 0, 0, 100, 3, 30, 3, '0,29,30', 0, '', '2025-11-25 17:25:04', '2025-11-25 17:25:04', 0),
(41, 'mdi:github', 'GITHUB', '', 'https://github.com/wannanbigpig/gin-layout', 'https://github.com/wannanbigpig/gin-layout', '', 'GITHUB', '', '', '', 0.00, 1, 1, 1, 1, 1, 80, 2, 0, 1, '0', 0, '', '2025-04-16 16:51:17', '2025-04-18 18:08:51', 0);

INSERT INTO `role_menu_map` (`role_id`, `menu_id`, `created_at`, `updated_at`)
SELECT 1, `id`, '2023-05-01 00:00:00', '2023-05-01 00:00:00'
FROM `menu`
WHERE `deleted_at` = 0;

COMMIT;
