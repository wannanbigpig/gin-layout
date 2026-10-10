import { buildEndpointDocumentation } from './examples.mjs'

const api = (group, basePath, auth, entries) =>
  entries.map(([method, path, title, description = '']) => {
    const endpoint = {
      group,
      method,
      path: `/${[basePath, path].filter(Boolean).join('/')}`.replace(/\/{2,}/g, '/'),
      title,
      description,
      auth
    }
    return { ...endpoint, ...buildEndpointDocumentation(endpoint) }
  })

export const apiGroups = [
  '全部',
  '基础探针',
  '认证会话',
  '个人中心',
  '仪表盘',
  '管理员',
  '权限菜单',
  '组织角色',
  '产品示例',
  '系统配置',
  '字典',
  '文件中心',
  '通知导出',
  '存储',
  '短信服务',
  '日志审计',
  '任务中心'
]

export const apiCatalog = [
  ...api('基础探针', '', '公开', [
    ['GET', '/ping', 'ping', '确认 HTTP 进程已经启动'],
    ['GET', '/health/readiness', 'readiness', '确认当前配置要求的依赖已经就绪']
  ]),
  ...api('认证会话', '/admin/v1', '公开', [
    ['GET', 'login-captcha', '验证码', '返回验证码 id 与图片 b64s；answer 仅在 local/test 环境非空'],
    ['POST', 'login', '登录', '换取 access token 并建立 refresh 会话；expires_at 为过期时间的 Unix 秒数'],
    ['GET', 'login/options', '登录方式', '获取当前启用的登录方式'],
    ['POST', 'login/sms/send-code', '发送短信登录验证码', '向管理员手机号发送验证码'],
    ['POST', 'login/sms', '短信验证码登录', '通过手机号与短信验证码换取 token；expires_at 为过期时间的 Unix 秒数'],
    ['GET', 'file/:uuid', '获取文件']
  ]),
  ...api('认证会话', '/admin/v1/auth', 'Refresh Cookie', [
    ['POST', 'refresh', '刷新 Token', '通过 HttpOnly refresh_token Cookie 刷新访问令牌；expires_at 为过期时间的 Unix 秒数'],
    ['POST', 'logout', '退出登录', '撤销会话凭证并清除 refresh_token Cookie']
  ]),
  ...api('认证会话', '/admin/v1/auth', 'Access Token', [
    ['GET', 'check-token', '检查 Token']
  ]),
  ...api('认证会话', '/admin/v1/auth/session', '接口权限', [
    ['GET', 'list', '在线会话列表'],
    ['POST', 'revoke', '撤销在线会话']
  ]),
  ...api('个人中心', '/admin/v1/common', 'Access Token', [
    ['POST', 'upload', '上传文件'],
    ['POST', 'ws-ticket', '生成 WebSocket 临时票据']
  ]),
  ...api('个人中心', '/admin/v1/common', 'WebSocket Ticket', [
    ['GET', 'ws', '通用实时连接 WebSocket', '使用一次性 ticket 建立认证连接']
  ]),
  ...api('个人中心', '/admin/v1/admin-user', 'Access Token', [
    ['GET', 'get', '获取当前用户信息'],
    ['GET', 'user-menu-info', '获取用户权限信息'],
    ['POST', 'update-profile', '更新个人资料'],
    ['GET', 'profile-security/options', '个人资料安全选项', '查询当前是否需要短信验证'],
    ['POST', 'profile-security/send-code', '发送个人资料验证码', '改密发送到当前绑定号码，换号发送到新号码'],
    ['POST', 'change-phone', '换绑手机号', '校验当前密码，启用手机号登录时还需新号码验证码'],
    ['POST', 'avatar/upload', '上传头像']
  ]),
  ...api('个人中心', '/admin/v1/todo', 'Access Token', [
    ['GET', 'list', '我的待办列表', '查询分配给当前用户的待办事项'],
    ['GET', 'summary', '我的待办摘要', '统计个人待办状态分布'],
    ['GET', 'groups', '我的待办分组', '查询当前用户创建的待办分组'],
    ['GET', 'reminder-preference', '待办提醒偏好', '获取个人待办通知偏好配置'],
    ['POST', 'create', '创建个人待办', '新增待办事项'],
    ['POST', 'update', '更新个人待办', '修改待办内容、优先级或截止时间'],
    ['POST', 'status', '更新待办状态', '流转待办状态（待办/进行中/完成/取消）'],
    ['POST', 'delete', '删除个人待办', '删除指定待办事项'],
    ['POST', 'group/create', '创建待办分组', '创建当前用户的私有分组'],
    ['POST', 'group/rename', '重命名待办分组', '修改当前用户的分组名称'],
    ['POST', 'group/delete', '删除待办分组', '删除分组并将其中待办移回未分组'],
    ['POST', 'group/move', '调整待办分组', '将当前用户的待办移入指定分组或未分组'],
    ['POST', 'reminder-preference', '更新待办提醒偏好', '修改站内提醒和短信提醒开关']
  ]),
  ...api('仪表盘', '/admin/v1/dashboard', '接口权限', [
    ['GET', 'overview', '仪表盘概览'],
    ['GET', 'statistics', '仪表盘统计'],
    ['GET', 'monitor', '服务器监控']
  ]),
  ...api('仪表盘', '/admin/v1/dashboard', 'Access Token', [
    ['GET', 'personal', '个人工作台']
  ]),
  ...api('管理员', '/admin/v1/admin-user', '接口权限', [
    ['GET', 'list', '管理员列表'],
    ['GET', 'options', '管理员选项'],
    ['GET', 'detail', '管理员详情'],
    ['GET', 'get-full-phone', '获取完整手机号'],
    ['GET', 'get-full-email', '获取完整邮箱'],
    ['POST', 'create', '新增管理员'],
    ['POST', 'update', '更新管理员'],
    ['POST', 'delete', '删除管理员'],
    ['POST', 'bind-role', '绑定角色']
  ]),
  ...api('权限菜单', '/admin/v1/permission', '接口权限', [
    ['GET', 'list', '接口列表'],
    ['GET', 'options', '接口选项'],
    ['POST', 'update', '更新接口']
  ]),
  ...api('权限菜单', '/admin/v1/menu', '接口权限', [
    ['GET', 'list', '菜单列表'],
    ['GET', 'detail', '菜单详情'],
    ['POST', 'create', '新增菜单'],
    ['POST', 'update', '更新菜单'],
    ['POST', 'delete', '删除菜单'],
    ['POST', 'update-all-menu-permissions', '刷新菜单权限缓存']
  ]),
  ...api('组织角色', '/admin/v1/role', '接口权限', [
    ['GET', 'list', '角色列表'],
    ['GET', 'options', '角色选项'],
    ['GET', 'detail', '角色详情'],
    ['POST', 'create', '新增角色'],
    ['POST', 'update', '更新角色'],
    ['POST', 'delete', '删除角色']
  ]),
  ...api('组织角色', '/admin/v1/department', '接口权限', [
    ['GET', 'list', '部门列表'],
    ['GET', 'detail', '部门详情'],
    ['POST', 'create', '新增部门'],
    ['POST', 'update', '更新部门'],
    ['POST', 'delete', '删除部门'],
    ['POST', 'bind-role', '部门绑定角色']
  ]),
  ...api('产品示例', '/admin/v1/product', '接口权限', [
    ['GET', 'list', '产品列表'],
    ['GET', 'detail', '产品详情'],
    ['POST', 'create', '新增产品'],
    ['POST', 'update', '更新产品'],
    ['POST', 'delete', '删除产品']
  ]),
  ...api('系统配置', '/admin/v1/system/config', '接口权限', [
    ['GET', 'list', '系统参数列表'],
    ['GET', 'detail', '系统参数详情'],
    ['GET', 'value', '获取系统参数值'],
    ['POST', 'create', '新增系统参数'],
    ['POST', 'update', '更新系统参数'],
    ['POST', 'delete', '删除系统参数'],
    ['POST', 'refresh', '刷新系统参数缓存']
  ]),
  ...api('字典', '/admin/v1/system/dict', '接口权限', [
    ['GET', 'type/list', '字典类型列表'],
    ['GET', 'type/detail', '字典类型详情'],
    ['POST', 'type/create', '新增字典类型'],
    ['POST', 'type/update', '更新字典类型'],
    ['POST', 'type/delete', '删除字典类型'],
    ['GET', 'item/list', '字典项列表'],
    ['POST', 'item/create', '新增字典项'],
    ['POST', 'item/update', '更新字典项'],
    ['POST', 'item/delete', '删除字典项'],
    ['GET', 'options', '字典选项']
  ]),
  ...api('文件中心', '/admin/v1/system/file', '接口权限', [
    ['GET', 'list', '文件资源列表'],
    ['GET', 'detail', '文件资源详情'],
    ['GET', 'folder/tree', '文件目录树'],
    ['GET', 'folder/stats', '目录统计信息'],
    ['POST', 'folder/create', '创建文件目录'],
    ['POST', 'folder/update', '更新文件目录'],
    ['POST', 'folder/delete', '删除文件目录'],
    ['POST', 'folder/move', '移动文件目录'],
    ['POST', 'update', '更新文件资源'],
    ['POST', 'move', '移动文件资源'],
    ['GET', 'upload/options', '文件上传能力'],
    ['POST', 'upload/local', '本地上传文件资源'],
    ['POST', 'upload/credential', '获取文件直传凭证'],
    ['POST', 'upload/credential/batch', '批量获取文件直传凭证'],
    ['POST', 'upload/complete', '完成文件直传登记'],
    ['POST', 'upload/complete/batch', '批量完成文件直传登记'],
    ['POST', 'upload/multipart/init', '分片上传初始化'],
    ['POST', 'upload/multipart/complete', '分片上传完成'],
    ['POST', 'upload/multipart/abort', '分片上传中止'],
    ['POST', 'delete', '删除文件资源'],
    ['POST', 'batch-delete', '批量删除文件资源'],
    ['GET', 'trash/list', '文件回收站列表'],
    ['POST', 'trash/restore', '恢复文件资源'],
    ['POST', 'trash/batch-restore', '批量恢复文件资源'],
    ['POST', 'trash/destroy', '硬删除文件资源'],
    ['POST', 'trash/batch-destroy', '批量硬删除文件资源'],
    ['GET', 'references', '文件引用列表'],
    ['POST', 'export', '导出文件资源']
  ]),
  ...api('通知导出', '/admin/v1/system/notification', 'Access Token', [
    ['GET', 'list', '系统消息列表'],
    ['GET', 'unread-count', '系统消息未读数'],
    ['POST', 'read', '标记消息已读'],
    ['POST', 'delete', '删除个人消息', '删除当前用户的一条站内消息'],
    ['POST', 'read-all', '全部标记已读']
  ]),
  ...api('通知导出', '/admin/v1/system/notification', '接口权限', [
    ['POST', 'send', '发送系统通知']
  ]),
  ...api('通知导出', '/admin/v1/system/export', 'Access Token', [
    ['GET', 'list', '我的导出列表'],
    ['GET', 'summary', '我的导出摘要', '统计当前用户全部导出记录的状态分布'],
    ['GET', 'detail', '我的导出详情']
  ]),
  ...api('存储', '/admin/v1/system/storage', '接口权限', [
    ['GET', 'config', '存储配置'],
    ['GET', 'secret', '查看存储敏感信息'],
    ['POST', 'config', '保存存储配置'],
    ['POST', 'test', '测试存储配置']
  ]),
  ...api('短信服务', '/admin/v1/system/sms', '接口权限', [
    ['GET', 'config', '短信配置', '获取当前短信服务商及模板配置'],
    ['POST', 'config', '保存短信配置', '保存短信渠道密钥与模板'],
    ['POST', 'test', '测试短信发送', '使用草稿配置向测试手机号发送验证码']
  ]),
  ...api('日志审计', '/admin/v1/log/request', 'Access Token', [
    ['GET', 'self-list', '个人请求日志列表'],
    ['GET', 'self-detail', '个人请求日志详情']
  ]),
  ...api('日志审计', '/admin/v1/log/request', '接口权限', [
    ['GET', 'list', '请求日志列表'],
    ['GET', 'detail', '请求日志详情'],
    ['POST', 'export', '导出请求日志'],
    ['GET', 'mask-config', '获取数据脱敏配置'],
    ['POST', 'mask-config', '更新数据脱敏配置']
  ]),
  ...api('日志审计', '/admin/v1/log/login', '接口权限', [
    ['GET', 'list', '登录日志列表'],
    ['GET', 'detail', '登录日志详情']
  ]),
  ...api('任务中心', '/admin/v1/task', '接口权限', [
    ['GET', 'list', '任务定义列表'],
    ['POST', 'trigger', '手动触发任务'],
    ['POST', 'record-policy', '更新任务记录策略'],
    ['POST', 'operation-config', '更新任务运营配置']
  ]),
  ...api('任务中心', '/admin/v1/task/run', '接口权限', [
    ['GET', 'list', '任务执行记录列表'],
    ['GET', 'stats', '任务执行统计'],
    ['GET', 'stats/trend', '任务执行统计趋势'],
    ['GET', 'detail', '任务执行记录详情'],
    ['GET', 'events', '任务执行事件列表'],
    ['POST', 'retry', '重试失败任务'],
    ['POST', 'cancel', '取消任务']
  ]),
  ...api('任务中心', '/admin/v1/task/cron', '接口权限', [
    ['GET', 'state', '定时任务最近状态']
  ])
]

export const authDescription = {
  公开: '无需登录即可访问。',
  'Access Token': '请求头携带 Bearer access token，不额外校验接口权限。',
  'Refresh Cookie': '通过 HttpOnly refresh_token Cookie 识别会话，并携带 X-Requested-With: XMLHttpRequest。',
  'WebSocket Ticket': '使用一次性 ticket 建立认证 WebSocket 连接。',
  接口权限: '请求头携带 Bearer access token，并通过 Casbin 校验接口权限。'
}
