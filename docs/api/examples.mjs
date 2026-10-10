const parameter = (name, location, type, required, description, example) => ({
  name,
  location,
  type,
  required,
  description,
  example
})

const query = (name, type, required, description, example) => parameter(name, 'Query', type, required, description, example)
const body = (name, type, required, description, example) => parameter(name, 'Body', type, required, description, example)
const path = (name, type, required, description, example) => parameter(name, 'Path', type, required, description, example)
const header = (name, type, required, description, example) => parameter(name, 'Header', type, required, description, example)

const paginate = [
  query('page', 'integer', false, '页码，从 1 开始', 1),
  query('per_page', 'integer', false, '每页条数，最大 200', 10)
]
const idQuery = [query('id', 'integer', true, '资源 ID', 1)]
const idBody = [body('id', 'integer', true, '资源 ID', 1)]
const dateRange = [
  query('start_time', 'string', false, '开始时间', '2026-09-01 00:00:00'),
  query('end_time', 'string', false, '结束时间', '2026-09-30 23:59:59')
]

const success = (data = {}) => ({ code: 0, msg: 'OK', data: data == null ? {} : (Array.isArray(data) || typeof data !== 'object' ? { result: data } : data), cost: '2.31ms', request_id: '7d9d6d0e-51a3-4b80-a3b2-6ec759f5cd30' })
const collection = (item, extra = {}) => success({ total: 1, per_page: 10, current_page: 1, last_page: 1, data: [item], ...extra })

const samples = {
  admin: { id: 1, username: 'zhangsan', nickname: '张三', phone_number: '138****8000', email: 'z***@example.com', status: 1, status_name: '启用', dept_ids: [1], role_ids: [1], created_at: '2026-09-16 10:20:30' },
  permission: { id: 12, name: '产品列表', method: 'GET', route: '/admin/v1/product/list', is_auth: 1, is_effective: 1, sort: 100 },
  menu: { id: 10, title: '产品管理', name: 'Product', code: 'product', path: '/product', type: 2, status: 1, is_show: 1, api_list: [12, 13] },
  role: { id: 2, name: '运营管理员', code: 'operator', status: 1, data_scope: 2, menu_list: [10, 11], dept_ids: [1] },
  department: { id: 1, name: '总部', pid: 0, sort: 100, description: '默认组织', children: [] },
  product: { id: 1, name: '示例产品', description: '用于演示后台 CRUD', price: 19900, status: 1, status_name: '启用', dept_id: 1, dept_name: '总部', created_at: '2026-09-16 10:20:30' },
  config: { id: 8, config_key: 'system.name', config_name: '系统名称', config_value: 'Gin Layout', value_type: 'string', group_code: 'base', status: 1 },
  dictType: { id: 3, type_code: 'system_status', type_name: '系统状态', status: 1, sort: 100 },
  dictItem: { id: 9, type_code: 'system_status', label: '启用', value: '1', color: '#22c55e', status: 1, sort: 100 },
  file: { id: 21, uuid: 'a73d42cfe53b4d19b8d65470a1c5ad89', origin_name: 'avatar.png', mime_type: 'image/png', file_type: 'image', size: 245760, is_public: 1, storage_driver: 'local', folder_id: 0, created_at: '2026-09-16 10:20:30' },
  notification: { id: 15, category: 'system', title: '任务执行完成', content: '导出任务已完成', is_read: false, action_url: '/system/export', created_at: '2026-09-16 10:20:30' },
  exportRecord: { public_id: 'c6317363-7e0f-42f6-90cd-30469f046bcd', export_name: '请求日志', scene: 'request_log', status: 'success', stage: 'download_ready', progress: 100, total_count: 100, processed_count: 100, file_uuid: 'a73d42cf-e53b-4d19-b8d6-5470a1c5ad89', file_name: 'request-log-20260916.xlsx', file_size: 245760, file_expires_at: '2026-09-23 10:20:31', file_expired: false, download_ready: true, format: 'xlsx', created_at: '2026-09-16 10:20:30', finished_at: '2026-09-16 10:20:31' },
  requestLog: { id: 31, request_id: 'req_01J8ABCD', method: 'POST', base_url: '/admin/v1/product/update', operation_name: '更新产品', operator_account: 'zhangsan', operation_status: 0, is_high_risk: 0, execution_time_us: 23840, created_at: '2026-09-16 10:20:30' },
  loginLog: { id: 18, username: 'zhangsan', ip: '127.0.0.1', os: 'macOS', browser: 'Chrome', login_status: 1, login_status_name: '成功', created_at: '2026-09-16 10:20:30' },
  task: { id: 4, code: 'system.cleanup', name: '系统清理', kind: 'cron', status: 1, allow_manual: 1, allow_retry: 1, cron_spec: '0 3 * * *' },
  taskRun: { id: 42, task_code: 'system.cleanup', kind: 'cron', source: 'manual', status: 'success', started_at: '2026-09-16 10:20:30', finished_at: '2026-09-16 10:20:31', duration_ms: 862 },
  cronState: { task_code: 'system.cleanup', last_status: 'success', last_run_at: '2026-09-16 03:00:00', next_run_at: '2026-09-17 03:00:00' }
}

const families = [
  {
    prefix: '/admin/v1/admin-user/', sample: samples.admin,
    list: [...paginate, query('username', 'string', false, '登录账号', 'zhangsan'), query('nickname', 'string', false, '昵称', '张三'), query('status', 'integer', false, '状态：0=禁用，1=启用', 1), query('dept_id', 'integer', false, '部门 ID', 1)],
    create: [body('username', 'string', true, '用户名，3~20 位字母、数字或下划线', 'zhangsan'), body('nickname', 'string', true, '昵称', '张三'), body('password', 'string', true, '密码，6~32 位', 'ChangeMe123!'), body('email', 'string', false, '邮箱', 'zhangsan@example.com'), body('status', 'integer', false, '状态：0=禁用，1=启用', 1), body('dept_ids', 'integer[]', false, '部门 ID 列表', [1])]
  },
  {
    prefix: '/admin/v1/permission/', sample: samples.permission,
    list: [...paginate, query('keyword', 'string', false, '名称或路由关键词', 'product'), query('method', 'string', false, 'HTTP 方法', 'GET'), query('is_auth', 'integer', false, '鉴权模式', 1), query('is_effective', 'integer', false, '是否已授权：0=否，1=是', 1)],
    create: [body('name', 'string', true, '接口名称', '产品列表'), body('description', 'string', false, '接口说明', '查询产品分页列表'), body('sort', 'integer', true, '排序值', 100)]
  },
  {
    prefix: '/admin/v1/menu/', sample: samples.menu,
    list: [...paginate, query('keyword', 'string', false, '标题、名称或编码关键词', '产品'), query('is_auth', 'integer', false, '是否鉴权：0=否，1=是', 1), query('status', 'integer', false, '状态：0=禁用，1=启用', 1)],
    create: [body('title_i18n', 'object', true, '多语言标题', { 'zh-CN': '产品管理', 'en-US': 'Products' }), body('type', 'integer', true, '菜单类型：1=目录，2=菜单，3=按钮', 2), body('name', 'string', false, '前端路由名称', 'Product'), body('path', 'string', false, '前端路由地址', '/product'), body('code', 'string', false, '按钮权限标识，type=3 时必填', 'product:list'), body('api_list', 'integer[]', false, '绑定的接口 ID', [12, 13]), body('status', 'integer', false, '状态：0=禁用，1=启用', 1)]
  },
  {
    prefix: '/admin/v1/role/', sample: samples.role,
    list: [...paginate, query('name', 'string', false, '角色名称', '运营'), query('code', 'string', false, '角色编码', 'operator'), query('status', 'integer', false, '状态：0=禁用，1=启用', 1)],
    create: [body('name', 'string', true, '角色名称', '运营管理员'), body('code', 'string', false, '角色编码', 'operator'), body('description', 'string', false, '角色说明', '负责日常运营'), body('status', 'integer', false, '状态：0=禁用，1=启用', 1), body('data_scope', 'integer', false, '数据范围：1~5', 2), body('menu_list', 'integer[]', false, '菜单 ID 列表', [10, 11]), body('dept_ids', 'integer[]', false, '部门 ID 列表', [1])]
  },
  {
    prefix: '/admin/v1/department/', sample: samples.department,
    list: [...paginate, query('name', 'string', false, '部门名称', '总部'), query('pid', 'integer', false, '上级部门 ID', 0)],
    create: [body('name', 'string', true, '部门名称', '研发部'), body('pid', 'integer', false, '上级部门 ID，0 表示根节点', 1), body('description', 'string', false, '部门说明', '产品研发团队'), body('sort', 'integer', false, '排序值', 100)]
  },
  {
    prefix: '/admin/v1/product/', sample: samples.product,
    list: [...paginate, query('name', 'string', false, '产品名称', '示例产品'), query('status', 'integer', false, '状态：0=禁用，1=启用', 1)],
    create: [body('name', 'string', true, '产品名称，最长 200 字符', '示例产品'), body('description', 'string', false, '产品说明', '用于演示后台 CRUD'), body('price', 'integer', false, '价格，单位由业务约定', 19900), body('status', 'integer', false, '状态：0=禁用，1=启用', 1), body('dept_id', 'integer', false, '所属部门；不传使用用户默认部门', 1)]
  },
  {
    prefix: '/admin/v1/system/config/', sample: samples.config,
    list: [...paginate, query('config_key', 'string', false, '参数键名', 'system.name'), query('config_name', 'string', false, '参数名称', '系统名称'), query('group_code', 'string', false, '参数分组', 'base'), query('value_type', 'string', false, '值类型：string/number/bool/json', 'string'), query('status', 'integer', false, '状态：0=禁用，1=启用', 1)],
    create: [body('config_key', 'string', true, '参数键名', 'system.name'), body('config_name_i18n', 'object', true, '多语言参数名称', { 'zh-CN': '系统名称', 'en-US': 'System name' }), body('config_value', 'string', false, '参数值', 'Gin Layout'), body('value_type', 'string', true, '值类型：string/number/bool/json', 'string'), body('group_code', 'string', false, '参数分组', 'base'), body('status', 'integer', false, '状态：0=禁用，1=启用', 1)]
  },
  {
    prefix: '/admin/v1/system/dict/type/', sample: samples.dictType,
    list: [...paginate, query('type_code', 'string', false, '字典类型编码', 'system_status'), query('type_name', 'string', false, '字典类型名称', '系统状态'), query('status', 'integer', false, '状态：0=禁用，1=启用', 1)],
    create: [body('type_code', 'string', true, '字典类型编码', 'system_status'), body('type_name_i18n', 'object', true, '多语言名称', { 'zh-CN': '系统状态', 'en-US': 'System status' }), body('status', 'integer', false, '状态：0=禁用，1=启用', 1), body('sort', 'integer', false, '排序值', 100)]
  },
  {
    prefix: '/admin/v1/system/dict/item/', sample: samples.dictItem,
    list: [...paginate, query('type_code', 'string', true, '字典类型编码', 'system_status'), query('label', 'string', false, '字典标签', '启用'), query('value', 'string', false, '字典值', '1'), query('status', 'integer', false, '状态：0=禁用，1=启用', 1)],
    create: [body('type_code', 'string', true, '字典类型编码', 'system_status'), body('label_i18n', 'object', true, '多语言标签', { 'zh-CN': '启用', 'en-US': 'Enabled' }), body('value', 'string', true, '字典值', '1'), body('color', 'string', false, '展示颜色', '#22c55e'), body('is_default', 'integer', false, '是否默认：0=否，1=是', 0), body('status', 'integer', false, '状态：0=禁用，1=启用', 1)]
  }
]

const requestBodyFrom = (parameters) => Object.fromEntries(parameters.filter((item) => item.location === 'Body' && item.example !== undefined).map((item) => [item.name, item.example]))

// 更新表单逐接口对应后端 form，不能从创建参数推断必填性或不可修改字段。
const updateParameters = {
  '/admin/v1/admin-user/': [
    ...idBody,
    body('username', 'string', false, '用户名，3~20 位字母、数字或下划线', 'zhangsan'),
    body('nickname', 'string', false, '昵称', '张三'),
    body('password', 'string', false, '新密码，6~32 位；不改密码时省略', undefined),
    body('phone_number', 'string', false, '手机号', '13800138000'),
    body('country_code', 'string', false, '国家代码：86 或 +86', '86'),
    body('email', 'string', false, '邮箱', 'zhangsan@example.com'),
    body('status', 'integer', false, '状态：0=禁用，1=启用', 1),
    body('is_super_admin', 'integer', false, '超级管理员标记：0=否，1=是；受服务端权限限制', 0),
    body('avatar', 'string', false, '头像地址', undefined),
    body('dept_ids', 'integer[]', false, '部门 ID 列表；不传保持原值', [1])
  ],
  '/admin/v1/permission/': [
    ...idBody,
    body('name', 'string', true, '接口名称，最长 60 字符', '产品列表'),
    body('description', 'string', false, '接口说明', '查询产品分页列表'),
    body('sort', 'integer', true, '排序值，不能为 0', 100)
  ],
  '/admin/v1/menu/': [
    ...idBody,
    body('title_i18n', 'object', true, '完整多语言标题；省略已有语言表示删除，zh-CN/en-US 至少一种非空', { 'zh-CN': '产品管理', 'en-US': 'Products' }),
    body('type', 'integer', true, '菜单类型：1=目录，2=菜单，3=按钮', 2),
    body('name', 'string', false, '前端路由名称；type=2 时必填', 'Product'),
    body('code', 'string', false, '按钮权限标识；type=3 时必填', ''),
    body('path', 'string', false, '前端路由地址', '/business/product'),
    body('component_key', 'string', false, '前端 componentMap 注册的语义标识，不是组件文件路径', 'product:index'),
    body('pid', 'integer', false, '上级菜单 ID，0=根节点', 0),
    body('icon', 'string', false, '菜单图标', ''),
    body('api_list', 'integer[]', false, '绑定的接口 ID', [12, 13]),
    body('is_show', 'integer', false, '是否显示：0=否，1=是', 1),
    body('is_auth', 'integer', false, '是否需要授权：0=否，1=是', 1),
    body('is_new_window', 'integer', false, '新窗口打开：0=否，1=是', 0),
    body('is_external_links', 'integer', false, '是否外链：0=否，1=是', 0),
    body('redirect', 'string', false, '重定向地址', ''),
    body('status', 'integer', false, '状态：0=禁用，1=启用', 1),
    body('sort', 'integer', false, '排序值', 100),
    body('description', 'string', false, '菜单说明', ''),
    body('animate_enter', 'string', false, '进入动画类名', ''),
    body('animate_leave', 'string', false, '离开动画类名', ''),
    body('animate_duration', 'number', false, '动画持续时间', 0)
  ],
  '/admin/v1/role/': [
    ...idBody,
    body('name', 'string', true, '角色名称；角色编码创建后不能修改', '运营管理员'),
    body('description', 'string', false, '角色说明', '负责日常运营'),
    body('status', 'integer', false, '状态：0=禁用，1=启用', 1),
    body('sort', 'integer', false, '排序值', 100),
    body('data_scope', 'integer', false, '数据范围：1=全部，2=本部门及子部门，3=本部门，4=本人，5=自定义部门', 2),
    body('menu_list', 'integer[]', false, '真实授权菜单或按钮 ID 列表', [10, 11]),
    body('dept_ids', 'integer[]', false, '自定义部门 ID 列表，data_scope=5 时使用', [])
  ],
  '/admin/v1/department/': [
    ...idBody,
    body('name', 'string', true, '部门名称', '研发部'),
    body('pid', 'integer', false, '上级部门 ID，0=根部门', 0),
    body('description', 'string', false, '部门说明', '产品研发团队'),
    body('sort', 'integer', false, '排序值', 100)
  ],
  '/admin/v1/product/': [
    ...idBody,
    body('name', 'string', true, '产品名称，最长 200 字符', '示例产品'),
    body('description', 'string', false, '产品说明', '用于演示后台 CRUD'),
    body('price', 'integer', false, '价格，非负整数', 19900),
    body('status', 'integer', false, '状态：0=禁用，1=启用', 1),
    body('dept_id', 'integer', false, '所属部门 ID', 1)
  ],
  '/admin/v1/system/config/': [
    ...idBody,
    body('config_key', 'string', true, '参数键名，最长 100 字符', 'system.name'),
    body('config_name_i18n', 'object', true, '多语言名称，按提交的语言合并', { 'zh-CN': '系统名称', 'en-US': 'System name' }),
    body('config_value', 'string', false, '参数值', 'Gin Layout'),
    body('value_type', 'string', true, '值类型：string=文本，number=数字，bool=布尔，json=JSON', 'string'),
    body('group_code', 'string', false, '参数分组', 'base'),
    body('is_sensitive', 'integer', false, '是否敏感：0=否，1=是', 0),
    body('is_visible', 'integer', false, '是否展示：0=否，1=是', 1),
    body('manage_tab', 'string', false, '专属配置页签', ''),
    body('status', 'integer', false, '状态：0=禁用，1=启用', 1),
    body('sort', 'integer', false, '排序值', 100),
    body('remark', 'string', false, '备注，最长 255 字符', '')
  ],
  '/admin/v1/system/dict/type/': [
    ...idBody,
    body('type_name_i18n', 'object', true, '多语言名称，按提交的语言合并；类型编码创建后不能修改', { 'zh-CN': '系统状态', 'en-US': 'System status' }),
    body('status', 'integer', false, '状态：0=禁用，1=启用', 1),
    body('sort', 'integer', false, '排序值', 100),
    body('remark', 'string', false, '备注，最长 255 字符', '')
  ],
  '/admin/v1/system/dict/item/': [
    ...idBody,
    body('type_code', 'string', true, '字典类型编码，最长 100 字符', 'system_status'),
    body('label_i18n', 'object', true, '多语言标签，按提交的语言合并', { 'zh-CN': '启用', 'en-US': 'Enabled' }),
    body('value', 'string', true, '字典值，最长 100 字符', '1'),
    body('color', 'string', false, '展示颜色', '#22c55e'),
    body('tag_type', 'string', false, '标签类型', 'success'),
    body('is_default', 'integer', false, '是否默认：0=否，1=是', 0),
    body('status', 'integer', false, '状态：0=禁用，1=启用', 1),
    body('sort', 'integer', false, '排序值', 100),
    body('remark', 'string', false, '备注，最长 255 字符', '')
  ]
}

const familyDocumentation = (endpoint) => {
  const family = families.find((item) => endpoint.path.startsWith(item.prefix))
  if (!family) return null
  const action = endpoint.path.slice(family.prefix.length)
  if (action === 'list') return { parameters: family.list, responseExample: collection(family.sample), source: 'form' }
  if (action === 'options') return { parameters: [query('keyword', 'string', false, '远程搜索关键词', '')], responseExample: success([{ id: family.sample.id, name: family.sample.name || family.sample.title || '示例选项' }]), source: 'form' }
  if (action === 'detail') return { parameters: idQuery, responseExample: success(family.sample), source: 'form' }
  if (action === 'create') return { parameters: family.create, requestBody: requestBodyFrom(family.create), responseExample: success({}), source: 'form' }
  if (action === 'update' && updateParameters[family.prefix]) {
    const parameters = updateParameters[family.prefix]
    return { parameters, requestBody: requestBodyFrom(parameters), responseExample: success({}), source: 'form' }
  }
  if (action === 'delete') return { parameters: idBody, requestBody: { id: 1 }, responseExample: success({}), source: 'form' }
  return null
}

const requestLogParams = [...paginate, query('request_id', 'string', false, '请求 ID', 'req_01J8ABCD'), query('operator_account', 'string', false, '操作账号', 'zhangsan'), query('operation_status', 'integer', false, '操作状态：0=成功，1=失败', 0), query('is_high_risk', 'integer', false, '是否高风险：0=否，1=是', 0), query('method', 'string', false, 'HTTP 方法', 'POST'), ...dateRange]
const fileListParams = [...paginate, query('origin_name', 'string', false, '原始文件名', 'avatar.png'), query('file_type', 'string', false, '文件类型，支持逗号分隔多值', 'image'), query('folder_id', 'integer', false, '目录 ID', 0), query('storage_driver', 'string', false, '存储驱动：local/aliyun_oss', 'local'), query('is_deleted', 'integer', false, '是否已删除：0=否，1=是', 0)]

const jsonPost = (parameters, responseExample = success({})) => ({ parameters, requestBody: requestBodyFrom(parameters), responseExample, source: 'form' })
const queryGet = (parameters, responseExample) => ({ parameters, responseExample, source: 'form' })
const noParams = (responseExample, responseKind = 'JSON') => ({ parameters: [], responseExample, responseKind, source: 'none' })
const todoSample = { id: 12, group_id: 3, title: '核对订单', description: '今天完成', priority: 'normal', status: 'pending', source_type: 'manual', due_at: '2026-09-24 18:00:00', can_edit: true, can_delete: true, can_update_status: true }
const todoSummarySample = { total: 5, pending: 3, processing: 1, completed: 1, canceled: 0, overdue: 1, due_today: 2 }
const exportSummarySample = { total: 7, pending: 1, running: 1, retrying: 1, success: 2, failed: 1, canceled: 1 }
const todoGroupQuery = query('group_id', 'integer', false, '分组 ID；不传查全部，0=未分组', 3)
const smsConfigSample = {
  enabled: true,
  providers: { aliyun: { enabled: true, access_key_id: '<access_key_id>', access_key_secret: '********', endpoint: 'https://dysmsapi.aliyuncs.com' } },
  templates: { aliyun: { login_otp: { sign_name: '示例签名', template_code: 'SMS_123456789', variables: ['code'] } } },
  routes: { login_otp: { provider: 'aliyun', template: 'login_otp' } },
  otp: { ttl_seconds: 300, resend_interval_seconds: 60, max_attempts: 5, ip_limit_per_10_minutes: 10, phone_daily_limit: 20, global_daily_limit: 1000 }
}

const specials = {
  'GET /ping': noParams(success({})),
  'GET /health/readiness': noParams(success({ status: 'ready', checks: { mysql: 'ok', redis: 'ok' } })),
  'GET /admin/v1/login-captcha': noParams(success({ id: 'b37d3e28-baf6-4280-913c-46f292a58b8e', b64s: 'data:image/png;base64,...', answer: '' })),
  'POST /admin/v1/login': jsonPost([body('username', 'string', true, '用户名，3~20 位', 'super_admin'), body('password', 'string', true, '密码，6~32 位', 'ChangeMe123!'), body('captcha', 'string', true, '验证码内容', '8m2k'), body('captcha_id', 'string', true, '验证码接口返回的 id', 'b37d3e28-baf6-4280-913c-46f292a58b8e')], success({ access_token: '<access_token>', token_type: 'Bearer', expires_at: 1790812800 })),
  'GET /admin/v1/login/options': noParams(success({ sms_login_enabled: true })),
  'POST /admin/v1/login/sms/send-code': jsonPost([body('country_code', 'string', true, '国家代码：86 或 +86', '86'), body('phone_number', 'string', true, '登录手机号', '13800138000'), body('captcha', 'string', true, '图形验证码', '8m2k'), body('captcha_id', 'string', true, '验证码接口返回的 id', 'b37d3e28-baf6-4280-913c-46f292a58b8e')], success({ challenge_id: 'b37d3e28-baf6-4280-913c-46f292a58b8e', retry_after: 60, expires_in: 300 })),
  'POST /admin/v1/login/sms': jsonPost([body('country_code', 'string', true, '国家代码：86 或 +86', '86'), body('phone_number', 'string', true, '登录手机号', '13800138000'), body('challenge_id', 'string', true, '发送验证码返回的 challenge_id', 'b37d3e28-baf6-4280-913c-46f292a58b8e'), body('sms_code', 'string', true, '6 位短信验证码', '123456')], success({ access_token: '<access_token>', token_type: 'Bearer', expires_at: 1790812800 })),
  'GET /admin/v1/file/:uuid': queryGet([path('uuid', 'string', true, '文件 UUID', 'a73d42cfe53b4d19b8d65470a1c5ad89')], 'Binary file stream'),
  'POST /admin/v1/auth/refresh': { ...noParams(success({ access_token: '<access_token>', token_type: 'Bearer', expires_at: 1790812800 })), parameters: [header('X-Requested-With', 'string', true, 'Cookie 接口的 CSRF 防护标记', 'XMLHttpRequest')], source: 'cookie' },
  'POST /admin/v1/auth/logout': { ...noParams(success({})), parameters: [header('X-Requested-With', 'string', true, 'Cookie 接口的 CSRF 防护标记', 'XMLHttpRequest')], source: 'cookie' },
  'GET /admin/v1/auth/check-token': noParams(success(true)),
  'GET /admin/v1/auth/session/list': queryGet([...paginate, query('uid', 'integer', false, '用户 ID', 1), query('username', 'string', false, '登录账号', 'zhangsan'), query('ip', 'string', false, '登录 IP', '127.0.0.1'), query('is_revoked', 'integer', false, '是否撤销：0=否，1=是', 0), ...dateRange], collection({ id: 1, uid: 1, username: 'zhangsan', jwt_id: 'session_01J8', ip: '127.0.0.1', os: 'macOS', browser: 'Chrome', is_revoked: 0 })),
  'POST /admin/v1/auth/session/revoke': jsonPost([body('id', 'integer', true, '会话 ID', 1), body('reason', 'string', false, '撤销原因', '账号安全处理')]),
  'POST /admin/v1/common/upload': { parameters: [body('file', 'file', true, '待上传文件', '@avatar.png')], requestBody: { file: '@avatar.png' }, responseExample: success(samples.file), contentType: 'multipart/form-data', source: 'form' },
  'POST /admin/v1/common/ws-ticket': noParams(success({ ticket: '<one_time_ticket>', expires_in: 60 })),
  'GET /admin/v1/common/ws': queryGet([query('ticket', 'string', true, '一次性 WebSocket ticket', '<one_time_ticket>')], { type: 'connected', request_id: 'req_01J8WS', data: { uid: 1 } }),
  'GET /admin/v1/admin-user/get': noParams(success(samples.admin)),
  'GET /admin/v1/admin-user/user-menu-info': noParams(success({ user: samples.admin, roles: ['operator'], menus: [samples.menu], permissions: ['product:list', 'product:create'] })),
  'POST /admin/v1/admin-user/update-profile': jsonPost([body('nickname', 'string', false, '昵称', '张三'), body('current_password', 'string', false, '修改密码时必填', 'OldPassword123!'), body('password', 'string', false, '新密码，6~32 位', 'NewPassword123!'), body('sms_challenge_id', 'string', false, '启用手机号登录且改密时必填的验证码标识', 'b37d3e28-baf6-4280-913c-46f292a58b8e'), body('sms_code', 'string', false, '启用手机号登录且改密时必填的 6 位验证码', '123456'), body('email', 'string', false, '邮箱', 'zhangsan@example.com')]),
  'GET /admin/v1/admin-user/profile-security/options': noParams(success({ sms_verification_required: true })),
  'POST /admin/v1/admin-user/profile-security/send-code': jsonPost([body('purpose', 'string', true, 'phone=换号，password=改密', 'phone'), body('country_code', 'string', false, '换号时的新号码区号，目前为 86 或 +86', '86'), body('phone_number', 'string', false, '换号时的新手机号', '13900139000'), body('current_password', 'string', false, '换号时的当前密码', 'OldPassword123!')], success({ challenge_id: 'b37d3e28-baf6-4280-913c-46f292a58b8e', retry_after: 60, expires_in: 300 })),
  'POST /admin/v1/admin-user/change-phone': jsonPost([body('country_code', 'string', true, '新号码区号，目前为 86 或 +86', '86'), body('phone_number', 'string', true, '新手机号', '13900139000'), body('current_password', 'string', true, '当前登录密码', 'OldPassword123!'), body('sms_challenge_id', 'string', false, '启用手机号登录时必填的新号码验证码标识', 'b37d3e28-baf6-4280-913c-46f292a58b8e'), body('sms_code', 'string', false, '启用手机号登录时必填的新号码 6 位验证码', '123456')]),
  'GET /admin/v1/todo/list': queryGet([...paginate, query('keyword', 'string', false, '标题或描述关键词', '订单'), query('status', 'string', false, '状态：pending/processing/completed/canceled', 'pending'), query('due_scope', 'string', false, '截止范围：overdue/today/upcoming/no_due', 'today'), todoGroupQuery], collection(todoSample, { summary: todoSummarySample })),
  'GET /admin/v1/todo/summary': queryGet([todoGroupQuery], success(todoSummarySample)),
  'GET /admin/v1/todo/groups': noParams(success({ groups: [{ id: 3, name: '工作' }] })),
  'POST /admin/v1/todo/group/create': jsonPost([body('name', 'string', true, '分组名称，去除首尾空白后 1~40 个字符；同用户名称唯一', '工作')], success({ id: 3, name: '工作' })),
  'POST /admin/v1/todo/group/rename': jsonPost([body('id', 'integer', true, '当前用户的分组 ID', 3), body('name', 'string', true, '新分组名称，去除首尾空白后 1~40 个字符', '项目工作')], success({ id: 3, name: '项目工作' })),
  'POST /admin/v1/todo/group/delete': jsonPost([body('id', 'integer', true, '当前用户的分组 ID；组内待办移回未分组', 3)]),
  'POST /admin/v1/todo/group/move': jsonPost([body('id', 'integer', true, '当前用户的待办 ID', 12), body('group_id', 'integer', false, '目标分组 ID；0 或不传表示移回未分组', 3)], success(todoSample)),
  'GET /admin/v1/todo/reminder-preference': noParams(success({ in_app_enabled: true, sms_enabled: false, sms_available: true, lead_minutes: 1440 })),
  'POST /admin/v1/todo/create': jsonPost([body('title', 'string', true, '待办标题，最长 255 字符', '核对订单'), body('description', 'string', false, '待办说明', '今天完成'), body('priority', 'string', false, '优先级：low/normal/high/urgent', 'normal'), body('due_at', 'string', false, '截止时间', '2026-09-24 18:00:00'), body('group_id', 'integer', false, '当前用户的分组 ID，0=未分组', 3)], success(todoSample)),
  'POST /admin/v1/todo/update': jsonPost([body('id', 'integer', true, '待办 ID', 12), body('title', 'string', false, '待办标题', '核对订单'), body('priority', 'string', false, '优先级：low/normal/high/urgent', 'high'), body('due_at', 'string', false, '截止时间；空字符串表示清除', '2026-09-24 18:00:00'), body('group_id', 'integer', false, '目标分组 ID；不传保持原分组，0=未分组', 3)], success({ ...todoSample, priority: 'high' })),
  'POST /admin/v1/todo/status': jsonPost([body('id', 'integer', true, '待办 ID', 12), body('status', 'string', true, '状态：pending/processing/completed/canceled', 'processing')], success({ ...todoSample, status: 'processing' })),
  'POST /admin/v1/todo/delete': jsonPost([body('id', 'integer', true, '待办 ID', 12)]),
  'POST /admin/v1/todo/reminder-preference': jsonPost([body('in_app_enabled', 'boolean', true, '是否开启站内提醒', true), body('sms_enabled', 'boolean', true, '是否开启短信提醒', false), body('lead_minutes', 'integer', false, '提前提醒分钟数，0~43200；0=到期时提醒，不传保留已有值', 1440)], success({ in_app_enabled: true, sms_enabled: false, sms_available: true, lead_minutes: 1440 })),
  'POST /admin/v1/admin-user/avatar/upload': { parameters: [body('file', 'file', true, '头像文件', '@avatar.png')], requestBody: { file: '@avatar.png' }, responseExample: success({ avatar: '/uploads/avatar/a73d42cf.png' }), contentType: 'multipart/form-data', source: 'form' },
  'GET /admin/v1/dashboard/overview': noParams(success({ user_total: 128, today_login_total: 42, file_total: 386, task_running_total: 3 })),
  'GET /admin/v1/dashboard/statistics': queryGet([query('range', 'string', false, '统计范围', '7d')], success({ dates: ['09-15', '09-16'], login_counts: [36, 42], request_counts: [1280, 1462] })),
  'GET /admin/v1/dashboard/monitor': noParams(success({ cpu_percent: 18.6, memory_percent: 42.1, goroutines: 83, uptime_seconds: 86400 })),
  'GET /admin/v1/dashboard/personal': noParams(success({ login_at: '2026-09-16 23:24:26', login_ip: '127.0.0.1', quick_entries: [samples.menu], recent_activities: [samples.requestLog] })),
  'GET /admin/v1/admin-user/get-full-phone': queryGet(idQuery, success({ phone_number: '13800138000' })),
  'GET /admin/v1/admin-user/get-full-email': queryGet(idQuery, success({ email: 'zhangsan@example.com' })),
  'POST /admin/v1/admin-user/bind-role': jsonPost([body('user_id', 'integer', true, '用户 ID', 1), body('role_ids', 'integer[]', true, '角色 ID 列表', [2])]),
  'POST /admin/v1/department/bind-role': jsonPost([body('dept_id', 'integer', true, '部门 ID', 1), body('role_ids', 'integer[]', true, '角色 ID 列表', [2])]),
  'POST /admin/v1/menu/update-all-menu-permissions': noParams(success({})),
  'GET /admin/v1/system/config/value': queryGet([query('config_key', 'string', true, '参数键名', 'system.name')], success({ result: 'Gin Layout' })),
  'POST /admin/v1/system/config/refresh': noParams(success({})),
  'GET /admin/v1/system/dict/options': queryGet([query('type_code', 'string', true, '字典类型编码', 'system_status')], success([{ label: '启用', value: '1', color: '#22c55e' }, { label: '禁用', value: '0', color: '#94a3b8' }])),
  'GET /admin/v1/system/file/list': queryGet(fileListParams, collection(samples.file)),
  'GET /admin/v1/system/file/detail': queryGet(idQuery, success(samples.file)),
  'GET /admin/v1/system/file/folder/tree': queryGet([query('include_hidden', 'integer', false, '是否包含隐藏目录：0=否，1=是', 0)], success([{ id: 1, parent_id: 0, name: '素材', logical_path: '/素材', children: [] }])),
  'GET /admin/v1/system/file/folder/stats': queryGet([query('id', 'integer', true, '目录 ID', 1)], success({ folder_id: 1, file_count: 12, folder_count: 3, total_size: 2457600 })),
  'POST /admin/v1/system/file/folder/create': jsonPost([body('parent_id', 'integer', false, '上级目录 ID', 0), body('name', 'string', true, '目录名称，最长 120 字符', '素材')], success({ id: 1, parent_id: 0, name: '素材' })),
  'POST /admin/v1/system/file/folder/update': jsonPost([body('id', 'integer', true, '目录 ID', 1), body('name', 'string', true, '新目录名称', '品牌素材')]),
  'POST /admin/v1/system/file/folder/delete': jsonPost([body('id', 'integer', true, '目录 ID', 1), body('deleted_reason', 'string', false, '删除原因', '目录清理'), body('confirm', 'integer', true, '确认标记，固定传 1', 1)]),
  'POST /admin/v1/system/file/folder/move': jsonPost([body('id', 'integer', true, '目录 ID', 1), body('target_parent_id', 'integer', false, '目标上级目录 ID', 2)]),
  'POST /admin/v1/system/file/update': jsonPost([body('id', 'integer', true, '文件资源 ID', 21), body('origin_name', 'string', true, '显示文件名', 'new-avatar.png')]),
  'POST /admin/v1/system/file/move': jsonPost([body('ids', 'integer[]', true, '文件资源 ID 列表', [21, 22]), body('folder_id', 'integer', false, '目标目录 ID', 1)]),
  'GET /admin/v1/system/file/upload/options': noParams(success({ max_size: 104857600, allowed_extensions: ['jpg', 'png', 'pdf'], drivers: ['local', 'aliyun_oss'], multipart_enabled: true })),
  'POST /admin/v1/system/file/upload/local': { parameters: [body('file', 'file', true, '待上传文件', '@report.pdf'), body('folder_id', 'integer', false, '目录 ID', 1), body('is_public', 'integer', false, '是否公开：0=否，1=是', 1), body('upload_scene', 'string', false, '上传场景', 'document')], requestBody: { file: '@report.pdf', folder_id: 1, is_public: 1, upload_scene: 'document' }, responseExample: success(samples.file), contentType: 'multipart/form-data', source: 'form' },
  'POST /admin/v1/system/file/upload/credential': jsonPost([body('origin_name', 'string', true, '原始文件名', 'report.pdf'), body('mime_type', 'string', true, 'MIME 类型', 'application/pdf'), body('size', 'integer', true, '文件大小，字节', 245760), body('hash', 'string', true, 'SHA-256，64 位十六进制', 'a'.repeat(64)), body('folder_id', 'integer', false, '目录 ID', 1), body('is_public', 'integer', false, '是否公开：0=否，1=是', 1), body('driver', 'string', false, '存储驱动，目前支持 aliyun_oss', 'aliyun_oss')], success({ upload_url: 'https://example-bucket.oss-cn.example.com/...', method: 'PUT', headers: { 'Content-Type': 'application/pdf' }, complete_token: '<complete_token>', expires_in: 900 })),
  'POST /admin/v1/system/file/upload/credential/batch': jsonPost([body('is_public', 'integer', false, '默认公开状态', 1), body('driver', 'string', false, '存储驱动', 'aliyun_oss'), body('items', 'array<object>', true, '文件信息；每项含 client_id、origin_name、mime_type、size、hash', [{ client_id: 'file-1', origin_name: 'report.pdf', mime_type: 'application/pdf', size: 245760, hash: 'a'.repeat(64) }])], success({ items: [{ client_id: 'file-1', upload_url: 'https://example-bucket.oss-cn.example.com/...', complete_token: '<complete_token>' }] })),
  'POST /admin/v1/system/file/upload/complete': jsonPost([body('complete_token', 'string', true, 'init/credential 阶段签发的完成令牌', '<complete_token>'), body('folder_id', 'integer', false, '目录 ID', 1), body('reuse', 'boolean', false, '是否复用已有物理对象', false), body('origin_name', 'string', false, '显示文件名', 'report.pdf')], success(samples.file)),
  'POST /admin/v1/system/file/upload/complete/batch': jsonPost([body('items', 'array<object>', true, '完成信息；每项含 client_id 和 complete_token', [{ client_id: 'file-1', complete_token: '<complete_token>' }])], success({ items: [{ client_id: 'file-1', file: samples.file }] })),
  'POST /admin/v1/system/file/upload/multipart/init': jsonPost([body('hash', 'string', true, 'SHA-256，64 位十六进制', 'a'.repeat(64)), body('origin_name', 'string', true, '原始文件名', 'video.mp4'), body('size', 'integer', true, '文件大小，字节', 52428800), body('mime_type', 'string', false, 'MIME 类型', 'video/mp4'), body('chunk_size', 'integer', false, '分片大小，字节', 5242880), body('part_count', 'integer', false, '分片数量', 10)], success({ upload_id: 'upload_01J8', complete_token: '<complete_token>', parts: [{ part_number: 1, upload_url: 'https://example-bucket.oss-cn.example.com/part/1' }] })),
  'POST /admin/v1/system/file/upload/multipart/complete': jsonPost([body('complete_token', 'string', true, '初始化阶段签发的完成令牌', '<complete_token>'), body('upload_id', 'string', false, '对象存储上传 ID', 'upload_01J8'), body('parts', 'array<object>', false, '已上传分片及 ETag', [{ part_number: 1, etag: 'etag-part-1' }])], success(samples.file)),
  'POST /admin/v1/system/file/upload/multipart/abort': jsonPost([body('complete_token', 'string', true, '初始化阶段签发的完成令牌', '<complete_token>')]),
  'POST /admin/v1/system/file/delete': jsonPost([body('id', 'integer', true, '文件资源 ID', 21), body('deleted_reason', 'string', false, '删除原因', '文件过期')]),
  'POST /admin/v1/system/file/batch-delete': jsonPost([body('ids', 'integer[]', true, '文件资源 ID 列表', [21, 22]), body('deleted_reason', 'string', false, '删除原因', '批量清理')]),
  'GET /admin/v1/system/file/trash/list': queryGet(fileListParams.map((item) => item.name === 'is_deleted' ? { ...item, description: '固定按已删除资源筛选', example: 1 } : item), collection(samples.file)),
  'POST /admin/v1/system/file/trash/restore': jsonPost([body('id', 'integer', true, '文件资源 ID', 21)]),
  'POST /admin/v1/system/file/trash/batch-restore': jsonPost([body('ids', 'integer[]', true, '文件资源 ID 列表', [21, 22])]),
  'POST /admin/v1/system/file/trash/destroy': jsonPost([body('id', 'integer', true, '文件资源 ID', 21)]),
  'POST /admin/v1/system/file/trash/batch-destroy': jsonPost([body('ids', 'integer[]', true, '文件资源 ID 列表', [21, 22])]),
  'GET /admin/v1/system/file/references': queryGet([...paginate, query('file_id', 'integer', false, '文件资源 ID', 21), query('uuid', 'string', false, '文件 UUID', samples.file.uuid), query('owner_type', 'string', false, '引用方类型', 'admin_user'), query('owner_id', 'integer', false, '引用方 ID', 1)], collection({ id: 5, file_id: 21, owner_type: 'admin_user', owner_id: 1, owner_field: 'avatar' })),
  'POST /admin/v1/system/file/export': jsonPost([body('file_type', 'string', false, '文件类型', 'image'), body('folder_id', 'integer', false, '目录 ID', 1), body('limit', 'integer', false, '最大导出条数，最多 50000', 1000)], success({ export_id: 7, status: 'pending' })),
  'GET /admin/v1/system/notification/list': queryGet([...paginate, query('category', 'string', false, '消息分类', 'system'), query('is_read', 'integer', false, '是否已读：0=未读，1=已读', 0)], collection(samples.notification)),
  'GET /admin/v1/system/notification/unread-count': noParams(success({ count: 3 })),
  'POST /admin/v1/system/notification/read': jsonPost([body('id', 'integer', true, '消息 ID', 15)]),
  'POST /admin/v1/system/notification/delete': jsonPost([body('id', 'integer', true, '当前用户的消息 ID', 15)]),
  'POST /admin/v1/system/notification/read-all': jsonPost([body('category', 'string', false, '只标记指定分类；不传表示全部', 'system')]),
  'POST /admin/v1/system/notification/send': jsonPost([body('title', 'string', true, '消息标题', '系统维护通知'), body('message', 'string', true, '消息内容', '系统将于今晚 23:00 维护'), body('category', 'string', false, '消息分类', 'system'), body('audience_type', 'string', true, '接收人类型：all/user_ids', 'user_ids'), body('user_ids', 'integer[]', false, 'audience_type=user_ids 时必填', [1, 2]), body('action_url', 'string', false, '操作链接', '/dashboard')], success({ batch_id: 'msg_01J8', recipient_count: 2 })),
  'GET /admin/v1/system/export/list': queryGet([...paginate, query('scene', 'string', false, '导出场景：request_log=请求日志，file_list=文件资源', 'request_log'), query('status', 'string', false, '状态：pending=排队，running=执行中，success=成功，failed=失败，canceled=取消，retrying=重试', 'success'), query('public_id', 'string', false, '导出记录公开标识，UUID', samples.exportRecord.public_id), query('sort_by', 'string', false, '排序字段：created_at=创建时间，finished_at=完成时间，file_size=文件大小', 'created_at'), query('sort_order', 'string', false, '排序：asc=升序，desc=降序', 'desc')], collection(samples.exportRecord, { summary: exportSummarySample })),
  'GET /admin/v1/system/export/summary': noParams(success(exportSummarySample)),
  'GET /admin/v1/system/export/detail': queryGet([query('public_id', 'string', true, '当前用户导出记录的公开标识，UUID', samples.exportRecord.public_id)], success(samples.exportRecord)),
  'GET /admin/v1/system/storage/config': noParams(success({ active_driver: 'local', config: { local: { root: './storage' }, aliyun_oss: { endpoint: 'oss-cn-hangzhou.aliyuncs.com', bucket: 'example-bucket' } } })),
  'GET /admin/v1/system/storage/secret': queryGet([query('driver', 'string', true, '存储驱动，目前仅 aliyun_oss', 'aliyun_oss'), query('field', 'string', true, '敏感字段，目前仅 access_key_secret', 'access_key_secret')], success({ value: '******' })),
  'POST /admin/v1/system/storage/config': jsonPost([body('active_driver', 'string', true, '当前驱动：local/aliyun_oss', 'aliyun_oss'), body('config', 'object', true, '驱动配置', { aliyun_oss: { endpoint: 'oss-cn-hangzhou.aliyuncs.com', bucket: 'example-bucket', access_key_id: '<access_key_id>', access_key_secret: '<access_key_secret>' } })]),
  'POST /admin/v1/system/storage/test': jsonPost([body('active_driver', 'string', true, '待测试驱动', 'aliyun_oss'), body('config', 'object', true, '待测试驱动配置', { aliyun_oss: { endpoint: 'oss-cn-hangzhou.aliyuncs.com', bucket: 'example-bucket' } })], success({ reachable: true, latency_ms: 86 })),
  'GET /admin/v1/system/sms/config': noParams(success(smsConfigSample)),
  'POST /admin/v1/system/sms/config': jsonPost([body('enabled', 'boolean', false, '是否启用短信服务', true), body('providers', 'object', true, '供应商配置', smsConfigSample.providers), body('templates', 'object', true, '短信模板配置', smsConfigSample.templates), body('routes', 'object', true, '场景路由', smsConfigSample.routes), body('otp', 'object', true, '验证码有效期、频控与次数', smsConfigSample.otp)]),
  'POST /admin/v1/system/sms/test': jsonPost([body('country_code', 'string', true, '国家代码：86 或 +86', '86'), body('phone_number', 'string', true, '测试手机号', '13800138000'), body('config', 'object', true, '待测试的完整短信配置草稿，不会保存', smsConfigSample)]),
  'GET /admin/v1/log/request/self-list': queryGet(requestLogParams, collection(samples.requestLog)),
  'GET /admin/v1/log/request/self-detail': queryGet(idQuery, success(samples.requestLog)),
  'GET /admin/v1/log/request/list': queryGet(requestLogParams, collection(samples.requestLog)),
  'GET /admin/v1/log/request/detail': queryGet(idQuery, success({ ...samples.requestLog, request_query: '{}', request_body: '{"id":1}', response_body: '{"code":0}' })),
  'POST /admin/v1/log/request/export': jsonPost([body('request_id', 'string', false, '请求 ID', 'req_01J8ABCD'), body('operator_account', 'string', false, '操作账号', 'zhangsan'), body('method', 'string', false, 'HTTP 方法', 'POST'), body('start_time', 'string', false, '开始时间', '2026-09-01 00:00:00'), body('end_time', 'string', false, '结束时间', '2026-09-30 23:59:59'), body('limit', 'integer', false, '导出上限，最大 5000', 1000)], success({ export_id: 8, status: 'pending' })),
  'GET /admin/v1/log/request/mask-config': noParams(success({ common: ['password', 'token'], request_header: ['authorization'], request_body: ['password'], response_header: [], response_body: ['access_token'] })),
  'POST /admin/v1/log/request/mask-config': jsonPost([body('common', 'string[]', false, '请求和响应共同脱敏字段', ['password', 'token']), body('request_header', 'string[]', false, '请求头脱敏字段', ['authorization']), body('request_body', 'string[]', false, '请求体脱敏字段', ['password']), body('response_header', 'string[]', false, '响应头脱敏字段', []), body('response_body', 'string[]', false, '响应体脱敏字段', ['access_token'])]),
  'GET /admin/v1/log/login/list': queryGet([...paginate, query('username', 'string', false, '登录账号', 'zhangsan'), query('login_status', 'integer', false, '登录状态：0=失败，1=成功', 1), query('ip', 'string', false, '登录 IP', '127.0.0.1'), ...dateRange], collection(samples.loginLog)),
  'GET /admin/v1/log/login/detail': queryGet(idQuery, success(samples.loginLog)),
  'GET /admin/v1/task/list': queryGet([...paginate, query('code', 'string', false, '任务编码', 'system.cleanup'), query('name', 'string', false, '任务名称', '系统清理'), query('kind', 'string', false, '类型：async/cron', 'cron'), query('status', 'integer', false, '状态：0=禁用，1=启用', 1), query('allow_manual', 'integer', false, '是否允许手动触发', 1)], collection(samples.task)),
  'POST /admin/v1/task/trigger': jsonPost([body('task_code', 'string', true, '任务编码', 'system.cleanup'), body('queue', 'string', false, '队列名', 'default'), body('task_id', 'string', false, '幂等任务 ID', 'manual-20260916-001'), body('payload', 'object', false, '任务载荷', { dry_run: true }), body('confirm', 'string', false, '高风险任务确认文本', 'CONFIRM'), body('reason', 'string', false, '触发原因', '人工验证')], success({ run_id: 42, status: 'pending' })),
  'POST /admin/v1/task/record-policy': jsonPost([body('task_code', 'string', true, '任务编码', 'system.cleanup'), body('record_success_mode', 'string', true, '成功记录模式：all/sample/interval/none', 'sample'), body('record_success_rate', 'integer', false, '抽样比例，0~100', 10), body('record_success_interval_seconds', 'integer', false, '间隔记录秒数', 300), body('record_detail_on_manual', 'integer', false, '手动任务记录详情：0/1', 1), body('record_detail_on_failure', 'integer', false, '失败任务记录详情：0/1', 1)]),
  'POST /admin/v1/task/operation-config': jsonPost([body('task_code', 'string', true, '任务编码', 'system.cleanup'), body('status', 'integer', true, '状态：0=禁用，1=启用', 1), body('allow_manual', 'integer', true, '是否允许手动触发：0/1', 1), body('allow_retry', 'integer', true, '是否允许重试：0/1', 1), body('cron_spec', 'string', false, 'Cron 表达式', '0 3 * * *')]),
  'GET /admin/v1/task/run/list': queryGet([...paginate, query('task_code', 'string', false, '任务编码', 'system.cleanup'), query('kind', 'string', false, '类型：async/cron', 'cron'), query('source', 'string', false, '来源：queue/cron/manual', 'manual'), query('status', 'string', false, '运行状态', 'success'), ...dateRange], collection(samples.taskRun)),
  'GET /admin/v1/task/run/stats': queryGet([query('task_code', 'string', false, '任务编码', 'system.cleanup'), query('kind', 'string', false, '类型：async/cron', 'cron'), query('source', 'string', false, '来源：queue/cron/manual', 'manual'), ...dateRange], success({ total: 120, success: 116, failed: 3, running: 1, success_rate: 96.67 })),
  'GET /admin/v1/task/run/stats/trend': queryGet([query('task_code', 'string', false, '任务编码', 'system.cleanup'), query('kind', 'string', false, '类型：async/cron', 'cron'), ...dateRange], success([{ date: '2026-09-15', success: 18, failed: 1 }, { date: '2026-09-16', success: 21, failed: 0 }])),
  'GET /admin/v1/task/run/detail': queryGet(idQuery, success(samples.taskRun)),
  'GET /admin/v1/task/run/events': queryGet([query('run_id', 'integer', true, '任务运行 ID', 42)], success([{ id: 1, run_id: 42, level: 'info', message: '任务开始执行', created_at: '2026-09-16 10:20:30' }])),
  'POST /admin/v1/task/run/retry': jsonPost([body('run_id', 'integer', true, '任务运行 ID', 42)], success({ run_id: 43, status: 'retrying' })),
  'POST /admin/v1/task/run/cancel': jsonPost([body('run_id', 'integer', true, '任务运行 ID', 42), body('reason', 'string', false, '取消原因', '任务已不再需要')]),
  'GET /admin/v1/task/cron/state': queryGet([...paginate, query('task_code', 'string', false, '任务编码', 'system.cleanup'), query('last_status', 'string', false, '最近运行状态', 'success')], collection(samples.cronState))
}

export function buildEndpointDocumentation(endpoint) {
  const key = `${endpoint.method} ${endpoint.path}`
  const special = specials[key]
  if (special) return special

  const family = familyDocumentation(endpoint)
  if (family) return family

  return {
    parameters: [],
    responseExample: success({}),
    source: 'fallback'
  }
}
