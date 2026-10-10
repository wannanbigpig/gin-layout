import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import assert from 'node:assert/strict'
import test from 'node:test'
import { assertCatalog, generateDocumentation } from './generate-api-documentation.mjs'

const catalog = [{ method: 'GET', path: '/example/:id', title: '示例', auth: '公开', parameters: [], responseExample: { code: 0 } }]
const openapi = 'openapi: "3.0.0"\ninfo:\n  version: "0.10.0"\npaths:\n  "/example/{id}":\n    get:\n      summary: "示例"\n      x-security-policy: "public"\n'
test('release documentation captures business fields, errors and upgrade guide for its backend version', (t) => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'backend-api-docs-'))
    t.after(() => fs.rmSync(root, { recursive: true, force: true }))
    const write = (name, value) => { const file = path.join(root, name); fs.mkdirSync(path.dirname(file), { recursive: true }); fs.writeFileSync(file, value) }
    write('internal/global/common.go', 'var Version = "0.10.0"')
    write('docs/openapi.yaml', openapi)
    write('docs/release.json', '{"compatibleFrontend":{"min":"0.10.0","maxExclusive":"0.11.0"},"upgradeGuide":"docs/upgrades/0.10.0.md"}')
    write('docs/error_codes.json', '{"groups":[]}')
    write('docs/upgrades/0.10.0.md', '# Upgrade guide')
    const source = { apiCatalog: catalog, apiGroups: ['全部', '示例'], authDescription: { 公开: '无需凭证' } }
    const result = generateDocumentation(root, source)
    assert.equal(result.backendVersion, '0.10.0')
    assert.deepEqual(result.apiCatalog, catalog)
    assert.equal(result.upgradeGuide, '# Upgrade guide')
    assert.equal(result.openapi, openapi)
    write('internal/global/common.go', 'var Version = "0.11.0"')
    assert.throws(() => generateDocumentation(root, source), /版本/)
})
test('a release cannot retain an old catalog after routes or security change', () => {
    assert.doesNotThrow(() => assertCatalog(openapi, catalog))
    for (const change of [{ method: 'POST' }, { path: '/old' }, { title: '旧标题' }, { auth: '接口权限' }]) {
        assert.throws(() => assertCatalog(openapi, [{ ...catalog[0], ...change }]), /不一致/)
    }
    assert.throws(() => assertCatalog(openapi, [...catalog, ...catalog]), /数量/)
})
test('placeholder documentation cannot appear as completed release documentation', () => {
    for (const change of [{ source: 'fallback' }, { parameters: null }, { responseExample: null }]) {
        assert.throws(() => assertCatalog(openapi, [{ ...catalog[0], ...change }]), /尚未完善/)
    }
})
