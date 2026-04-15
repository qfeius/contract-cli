# Contract Commands Reference

## 查询与创建

```bash
contract-cli contract get 7023646046559404327 --profile contract-group --as user
contract-cli contract search --profile contract-group --input-file contract-search.json
contract-cli contract search --profile contract-group --data '{"contract_number":"CN-001"}'
contract-cli contract create --profile contract-group --input-file contract-create.json
contract-cli contract create --profile contract-group --data '{"title":"示例合同"}'
contract-cli contract sync-user-groups --profile contract-group
contract-cli contract text 7023646046559404327 --profile contract-group --full-text
```

## 分类、模板、枚举

```bash
contract-cli contract category list --profile contract-group --lang zh-CN
contract-cli contract template list --profile contract-group --category-number CAT-1 --page-size 20
contract-cli contract template get tpl_123 --profile contract-group
contract-cli contract template instantiate --profile contract-group --input-file template-instance.json
contract-cli contract enum list --profile contract-group --type contract_status
```

## 已知限制

- 文件上传链路尚未实现
- `contract template fields` 尚未实现
- `contract create` 不自动帮你补模板信息；当前就是透传请求体
