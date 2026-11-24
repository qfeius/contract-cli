const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

// SKILL_TEST_ROOT also checks the files exported by `skills install`.
const root = process.env.SKILL_TEST_ROOT || path.join(__dirname, '..', 'skills');
const read = name => fs.readFileSync(path.join(root, name, 'SKILL.md'), 'utf8');

test('expiry Skill uses production guidance and platform-day bounds for a UTC business date', () => {
  const guide = read('contract-expiry-query');
  const protocol = fs.readFileSync(path.join(root, 'contract-expiry-query/references/query-api.md'), 'utf8');
  assert.doesNotMatch(guide + protocol, /contract-test/);
  assert.match(guide, /GMT\+8/);
  const range = protocol.match(/UTC[^\n]*2026-11-19[^\n]*\[(\d+),\s*(\d+)\]/);
  assert.ok(range, 'missing single-day UTC example with the actual searchValue');
  const [, start, end] = range.map(Number);
  assert.equal(start, Date.parse('2026-11-19T00:00:00+08:00'));
  assert.equal(end, Date.parse('2026-11-19T23:59:59.999+08:00'));
});

test('every shipped skill has unambiguous top-level frontmatter keys', () => {
  for (const entry of fs.readdirSync(root, { withFileTypes: true })) {
    if (!entry.isDirectory() || !fs.existsSync(path.join(root, entry.name, 'SKILL.md'))) continue;
    const frontmatter = read(entry.name).trimStart().match(/^---\r?\n([\s\S]*?)\r?\n---/);
    assert.ok(frontmatter, `${entry.name}: missing frontmatter`);
    const keys = [...frontmatter[1].matchAll(/^([\w-]+):/gm)].map(match => match[1]);
    assert.equal(new Set(keys).size, keys.length, `${entry.name}: duplicate metadata keys: ${keys}`);
    assert.ok(keys.includes('name') && keys.includes('description'), entry.name);
  }
});

test('shared skill gives contract search one destination while preserving review and directory routes', () => {
  const shared = read('contract-cli-shared');
  const decisions = shared.split('## 快速决策')[1].split('\n## ')[0];
  const searchRoutes = decisions.split('\n').filter(line => line.startsWith('- 合同搜索'));
  assert.equal(searchRoutes.length, 1, 'contract search must not route to two skills');
  assert.ok(searchRoutes[0].includes('../contract-cli-contract-search/SKILL.md'));
  for (const name of ['contract-cli-contract-search', 'contract-cli-contract', 'contract-cli-review', 'contract-cli-review-rule', 'contract-cli-employee', 'contract-cli-department', 'contract-cli-rule']) {
    assert.ok(decisions.includes(`../${name}/SKILL.md`), `missing route ${name}`);
    assert.ok(fs.existsSync(path.join(root, name, 'SKILL.md')), `missing destination ${name}`);
  }
});

test('Device quick flow keeps the selected profile through authorization', () => {
  const flow = read('auth').split('### 豆包 / WorkBuddy / Codex Device 授权')[1].split('### `user` 登录')[0];
  const commands = [...flow.matchAll(/^contract-cli auth (status|init|complete) --profile (\S+)/gm)];
  assert.deepEqual(commands.map(match => match[1]), ['status', 'init', 'complete']);
  assert.deepEqual([...new Set(commands.map(match => match[2]))], ['<profile>']);
  assert.ok(!flow.includes('config add --env prod'));
});

test('shared auth instructions limit the release to production without redirecting unsupported requests', () => {
  const shared = read('contract-cli-shared');
  const auth = read('auth');
  assert.ok(shared.includes('仅支持 `prod`'));
  assert.ok(shared.includes('所有业务 Skill 中的 `--profile contract` 示例'));
  assert.ok(shared.includes('不自动改用 prod'));
  assert.ok(auth.includes('仅内置 `prod`'));
  assert.ok(auth.includes('关键地址变化后必须重新授权'));
  assert.ok(auth.includes('https://open.qfei.cn'));
  assert.ok(auth.includes('https://myaccount.qfei.cn'));
  assert.doesNotMatch(auth, /--env test|contract-test|https:\/\/test-(open|myaccount)\.qtech\.cn/);
  assert.doesNotMatch(shared, /--env test|contract-test/);
  assert.ok(!shared.includes('默认启用 `blue`'));
  assert.ok(!shared.includes('contract-blue'));
  assert.ok(!auth.includes('contract-blue'));
});

test('app authorization examples do not enable Blue configuration', () => {
  const flow = read('auth').split('### `app` 登录')[1].split('## 状态、退出与切换')[0];
  const commands = flow.split('\n').filter(line => line.startsWith('contract-cli auth '));
  assert.ok(commands.length >= 3);
  for (const command of commands) {
    assert.doesNotMatch(command, /--env\s+blue|contract-blue/, command);
  }
});

test('Blue profile setup remains unsupported by the Skill bundle', () => {
  const shared = read('contract-cli-shared');
  const auth = read('auth');
  assert.ok(!shared.includes('contract-cli config add --env blue'));
  assert.ok(!auth.includes('contract-cli config add --env blue'));
});

test('shared decisions reach every delivered business workflow', () => {
  const shared = read('contract-cli-shared');
  const decisions = shared.split('## 快速决策')[1].split('\n## ')[0];
  for (const name of ['contract-approval-efficiency', 'contract-cli-approval-actions', 'contract-cooperation-chat-group', 'contract-handover']) {
    assert.ok(decisions.includes(`../${name}/SKILL.md`), `missing business route ${name}`);
    assert.ok(fs.existsSync(path.join(root, name, 'SKILL.md')));
  }
  const modules = shared.split('## 当前已实现模块')[1].split('\n## ')[0];
  assert.ok(modules.includes('review pre-audit'));
  assert.ok(!modules.includes('upload-url'));
});

test('authorization guidance does not reject supported JSON status output', () => {
  for (const name of ['auth', 'contract-cli-contract-search']) {
    assert.doesNotMatch(read(name), /auth status`?\s*不支持\s*`?--output/);
  }
});

test('cooperation request example is an object accepted as a JSON input file', () => {
  const content = fs.readFileSync(path.join(root, 'contract-cli-contract', 'references', 'cooperation-search-parameters.md'), 'utf8');
  const examples = [...content.matchAll(/```json\s*\n([\s\S]*?)```/g)];
  assert.ok(examples.length > 0);
  for (const [, body] of examples) {
    const request = JSON.parse(body);
    assert.ok(request && typeof request === 'object' && !Array.isArray(request));
    assert.equal(request.page_size, 10);
    assert.ok(Array.isArray(request.confirmable_status_codes));
  }
});
