import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

export function assertCatalog(openapi, catalog) {
    const operations = new Map()
    let route = '', key = ''
    for (const line of openapi.split('\n')) {
        const p = line.match(/^  "([^"]+)":/)
        if (p) { route = p[1]; key = '' }
        const m = line.match(/^    (get|post|put|patch|delete|head|options|trace):/)
        if (m && route) {
            key = `${m[1].toUpperCase()} ${route.replace(/\{([^}]+)\}/g, ':$1')}`
            if (operations.has(key)) throw new Error(`重复 OpenAPI 端点：${key}`)
            operations.set(key, {})
        }
        const title = line.match(/^      summary: ("(?:\\.|[^"])*")$/)
        const policy = line.match(/^      x-security-policy: "([^"]+)"$/)
        if (key && title) operations.get(key).title = JSON.parse(title[1])
        if (key && policy) operations.get(key).policy = policy[1]
    }
    const policies = { 公开: 'public', 'Access Token': 'access_token', 'Refresh Cookie': 'refresh_cookie', 'WebSocket Ticket': 'websocket_ticket', 接口权限: 'access_token_permission' }
    const keys = new Set(catalog.map((item) => `${item.method.toUpperCase()} ${item.path}`))
    if (!operations.size || operations.size !== keys.size || keys.size !== catalog.length) throw new Error('API 业务目录端点数量与 OpenAPI 不一致')
    for (const item of catalog) {
        const key = `${item.method.toUpperCase()} ${item.path}`, op = operations.get(key)
        if (!op || op.title !== item.title || op.policy !== policies[item.auth]) throw new Error(`API 业务目录标题、路径或安全策略不一致：${key}`)
        if (item.source === 'fallback' || !Array.isArray(item.parameters) || !item.responseExample) throw new Error(`API 参数或响应示例尚未完善：${key}`)
    }
}

export function generateDocumentation(root, { apiCatalog, apiGroups, authDescription }) {
    const read = (name) => fs.readFileSync(path.join(root, name), 'utf8')
    const backendVersion = read('internal/global/common.go').match(/Version\s*=\s*"([^"]+)"/)?.[1]
    const openapi = read('docs/openapi.yaml')
    if (!backendVersion || !openapi.includes(`  version: "${backendVersion}"`)) throw new Error('OpenAPI 版本与源码版本不一致')
    const policy = JSON.parse(read('docs/release.json'))
    if (policy.upgradeGuide !== `docs/upgrades/${backendVersion}.md`) throw new Error('升级说明必须对应当前后端版本')
    assertCatalog(openapi, apiCatalog)
    if (!Array.isArray(apiGroups) || !authDescription) throw new Error('缺少 API 分组或凭证说明')
    return {
        schemaVersion: 1, backendVersion, compatibleFrontend: policy.compatibleFrontend,
        schemaCoverage: 'routes-and-security', apiCatalog, apiGroups, authDescription,
        openapi, errorCodes: JSON.parse(read('docs/error_codes.json')), upgradeGuide: read(policy.upgradeGuide)
    }
}

async function main() {
    const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
    const args = process.argv.slice(2)
    if (args.some((item) => item !== '--check')) throw new Error('Usage: node scripts/generate-api-documentation.mjs [--check]')
    const source = await import(pathToFileURL(path.join(root, 'docs/api/catalog.mjs')))
    const documentation = generateDocumentation(root, source)
    const bytes = JSON.stringify(documentation, null, 2) + '\n'
    const target = path.join(root, 'docs/api-documentation.json')
    if (args.includes('--check')) {
        if (!fs.existsSync(target) || fs.readFileSync(target, 'utf8') !== bytes) throw new Error('API 版本文档未同步，请执行 node scripts/generate-api-documentation.mjs')
    } else fs.writeFileSync(target, bytes)
    console.log(`API version documentation ${documentation.backendVersion}: ${documentation.apiCatalog.length} endpoints ${args.includes('--check') ? 'verified' : 'generated'}`)
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main().catch((error) => { console.error(error.message); process.exitCode = 1 })
