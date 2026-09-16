# Contributing

`gin-layout` 是 Gin 后台管理脚手架。贡献应保持简单、稳定、工程化和易于二次开发，不把项目扩展成 SaaS、微服务、DDD 或 ERP 成品。

## 开发流程

1. 先阅读 `AGENTS.md`、`.aitasks/lessons.md` 和相关源码。
2. Issue 与提交说明写清触发条件、预期行为、兼容影响和验证方法。
3. 使用 Conventional Commits：`feat:`、`fix:`、`refactor:`、`test:`、`docs:`、`ci:`、`chore:`。
4. 只在确有独立业务语义、复用或测试价值时拆函数；PR 必须说明业务流程的文件/函数跳转成本是否变化。
5. 新接口遵循 form → service → resource → controller → route，并同步路由元数据和测试。

## 提交前验证

```bash
gofmt -w <changed-go-files>
go test ./...
go vet ./...
go mod verify
go run main.go -c ./config.yaml command migrate check
git diff --check
```

需要真实 MySQL 或外部服务的测试必须明确环境边界，不得通过吞错或弱化断言制造通过。
