const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { execFileSync } = require('node:child_process');

const digest = bytes => crypto.createHash('sha256').update(bytes).digest('hex');

function treeFingerprint(files) {
  const root = new Map();
  for (const [name, bytes] of files) {
    const parts = name.split('/');
    assertPath(parts);
    let parent = root;
    for (const part of parts.slice(0, -1)) {
      if (!parent.has(part)) parent.set(part, new Map());
      parent = parent.get(part);
    }
    parent.set(parts.at(-1), bytes);
  }
  const hash = crypto.createHash('sha256');
  function walk(tree, prefix) {
    for (const name of [...tree.keys()].sort()) {
      const relative = prefix + name;
      const value = tree.get(name);
      if (value instanceof Map) { hash.update(`d:${relative}\0`); walk(value, relative + '/'); }
      else hash.update(`f:${relative}\0${digest(value)}\0`);
    }
  }
  walk(root, '');
  return hash.digest('hex');
}

function assertPath(parts) {
  if (parts.some(part => !part || part === '.' || part === '..')) throw new Error('invalid snapshot path');
}

function rewriteVersion(content, version, legacyIndented = false) {
  const lines = content.toString().split('\n');
  if (lines[0].trim() !== '---') return content;
  for (let i = 1; i < lines.length; i++) {
    if (lines[i].trim() === '---') { lines.splice(i, 0, `version: ${version}`); return Buffer.from(lines.join('\n')); }
    // 1.9.5-7 matched the key after trimming indentation, then wrote it at column zero.
    const line = legacyIndented ? lines[i].trim() : lines[i];
    if (/^version[ \t]*:/.test(line)) { lines[i] = `version: ${version}`; return Buffer.from(lines.join('\n')); }
  }
  return content;
}

function extendCatalog(existing, releases) {
  const result = Object.fromEntries(Object.entries(existing).map(([name, hashes]) => [name, [...hashes]]));
  for (const { version, files } of releases) {
    for (const skill of [...files.keys()].filter(name => /^[^/]+\/SKILL\.md$/.test(name)).map(name => name.split('/')[0])) {
      const prefix = skill + '/';
      const tree = new Map([...files].filter(([name]) => name.startsWith(prefix)).map(([name, bytes]) => [name.slice(prefix.length), bytes]));
      const hashes = new Set(result[skill] || []);
      hashes.add(treeFingerprint(tree));
      for (const stamp of [version, 'v' + version]) {
        const stamped = new Map(tree);
        stamped.set('SKILL.md', rewriteVersion(tree.get('SKILL.md'), stamp, /^1\.9\.[567]$/.test(version)));
        hashes.add(treeFingerprint(stamped));
      }
      result[skill] = [...hashes].sort();
    }
  }
  return Object.fromEntries(Object.entries(result).sort(([a], [b]) => a.localeCompare(b)));
}

function readOfficialRelease(root, release) {
  if (!/^1\.9\.\d+$/.test(release.version) || !/^[a-f0-9]{40}$/.test(release.commit)) throw new Error('invalid pinned release');
  const git = args => execFileSync('git', ['-C', root, ...args], { maxBuffer: 32 << 20 });
  const actual = git(['rev-parse', `refs/tags/v${release.version}^{}`]).toString().trim();
  if (actual !== release.commit) throw new Error(`official tag differs from pinned commit: ${release.version}`);
  const files = new Map();
  for (const record of git(['ls-tree', '-r', '-z', release.commit, '--', 'skills/']).toString().split('\0').filter(Boolean)) {
    const tab = record.indexOf('\t');
    const [mode, type, oid] = record.slice(0, tab).split(' ');
    const name = record.slice(tab + 1).slice('skills/'.length);
    if (type !== 'blob' || !['100644', '100755'].includes(mode)) throw new Error('non-regular official snapshot entry');
    files.set(name, git(['cat-file', 'blob', oid]));
  }
  return { ...release, files };
}

if (require.main === module) {
  const mode = process.argv[2];
  if (!['--write', '--check'].includes(mode)) throw new Error('usage: node scripts/skill-history.js --check|--write');
  const root = path.resolve(__dirname, '..');
  const target = path.join(root, 'internal/skillsync/history_fingerprints.json');
  const sources = JSON.parse(fs.readFileSync(path.join(root, 'internal/skillsync/history_sources.json'), 'utf8'));
  const current = JSON.parse(fs.readFileSync(target, 'utf8'));
  const catalog = extendCatalog(current, sources.map(source => readOfficialRelease(root, source)));
  const content = JSON.stringify(catalog, null, 2) + '\n';
  if (mode === '--write') fs.writeFileSync(target, content);
  else if (JSON.stringify(current) !== JSON.stringify(catalog)) throw new Error('official historical catalog needs regeneration');
  console.log(`Verified ${sources.length} pinned GitHub releases; ${Object.keys(catalog).length} historical Skill names`);
}

module.exports = { treeFingerprint, rewriteVersion, extendCatalog };
