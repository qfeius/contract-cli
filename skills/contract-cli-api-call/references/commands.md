# Open Web API Call Transport Reference

`contract-cli api call --channel web` 是业务 Skill 的通用执行通道，不是独立的接口发现 Skill。

业务 Skill 已提供 method、Open Web path、query/body 和响应映射后，可按以下形式调用：

```bash
contract-cli api call \
  --channel web \
  --profile contract \
  --as user \
  --output json \
  POST /open-web/web/clm/statistic/contractInfoByTradingParty \
  --data '<JSON>'
```

不得根据页面名称猜 path，也不得通过 `--header` 传入授权或可信身份头。
