const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

// SKILL_TEST_ROOT also checks the files exported by `skills install`.
const root = process.env.SKILL_TEST_ROOT || path.join(__dirname, '..', 'skills');
const read = name => fs.readFileSync(path.join(root, name, 'SKILL.md'), 'utf8');

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

test('shared auth instructions select services from profile addresses without reusing credentials across targets', () => {
  const shared = read('contract-cli-shared');
  const auth = read('auth');
  assert.ok(shared.includes('实际访问目标由 profile 中的地址决定'));
  assert.ok(shared.includes('所有业务 Skill 中的 `--profile contract` 示例'));
  assert.ok(shared.includes('不自动改用 prod'));
  assert.ok(auth.includes('`prod` 和 `test` 是地址预设'));
  assert.ok(auth.includes('关键地址变化后必须重新授权'));
  assert.ok(auth.includes('contract-cli config add --env test --name contract-test'));
  assert.ok(auth.includes('https://test-open.qtech.cn'));
  assert.ok(auth.includes('https://test-myaccount.qtech.cn'));
  assert.ok(auth.includes('config add --env prod --name contract-test'));
  assert.ok(auth.includes('--open-platform-base-url https://test-open.qtech.cn'));
  assert.ok(auth.includes('--authorization-server-metadata-url https://test-myaccount.qtech.cn/.well-known/oauth-authorization-server/contract'));
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
