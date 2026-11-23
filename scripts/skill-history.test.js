const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const test = require('node:test');
const { treeFingerprint, rewriteVersion, extendCatalog } = require('./skill-history');

test('fingerprints cover exact contents, paths and directory structure', () => {
  const bytes = Buffer.from('official');
  const expected = crypto.createHash('sha256').update('d:references\0').update('f:references/a.md\0' + crypto.createHash('sha256').update(bytes).digest('hex') + '\0').digest('hex');
  assert.equal(treeFingerprint(new Map([['references/a.md', bytes]])), expected);
  assert.notEqual(treeFingerprint(new Map([['references/a.md', Buffer.from('changed')]])), expected);
  assert.notEqual(treeFingerprint(new Map([['a.md', bytes]])), expected);
});

test('only the top-level package version is stamped', () => {
  const source = Buffer.from('---\nname: auth\nmetadata:\n  version: schema-v1\n---\nbody\n');
  assert.equal(rewriteVersion(source, '1.9.13').toString(), '---\nname: auth\nmetadata:\n  version: schema-v1\nversion: 1.9.13\n---\nbody\n');
});

test('catalog adopts only exact official snapshots and preserves older trusted entries', () => {
  const files = new Map([['auth/SKILL.md', Buffer.from('---\nname: auth\n---\nbody\n')], ['auth/references/a.md', Buffer.from('official')], ['disabled/references/a.md', Buffer.from('not a skill')]]);
  const result = extendCatalog({ auth: ['old-trusted'] }, [{ version: '1.9.13', files }]);
  assert.ok(result.auth.includes('old-trusted'));
  assert.ok(result.auth.includes(treeFingerprint(new Map([['SKILL.md', files.get('auth/SKILL.md')], ['references/a.md', files.get('auth/references/a.md')]]))));
  assert.equal(result.disabled, undefined);
  const edited = new Map([['SKILL.md', files.get('auth/SKILL.md')], ['references/a.md', Buffer.from('edited')]]);
  assert.ok(!result.auth.includes(treeFingerprint(edited)));
  assert.equal(extendCatalog(result, [{ version: '1.9.13', files }]).auth.length, result.auth.length);
});

test('1.9.5 through 1.9.7 installed review metadata matches the old installer bytes', () => {
  const source = Buffer.from('---\nmetadata:\n  version: old\n---\nbody\n');
  const files = new Map([['review/SKILL.md', source]]);
  for (const version of ['1.9.5', '1.9.6', '1.9.7']) {
    const expected = Buffer.from('---\nmetadata:\nversion: ' + version + '\n---\nbody\n');
    assert.ok(extendCatalog({}, [{ version, files }]).review.includes(treeFingerprint(new Map([['SKILL.md', expected]]))));
  }
});
