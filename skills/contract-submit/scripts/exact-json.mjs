/** Strict JSON with duplicate-key rejection and lossless numeric literals. */
const MAX_JSON = 32 * 1024 * 1024;
const MAX_DEPTH = 96;
const MAX_ITEMS = 300000;
class ExactJSONError extends Error {
  constructor(code) { super(code); this.name='ExactJSONError'; this.code=code; }
}
function requireThat(condition, code) { if(!condition) throw new ExactJSONError(code); }
const safeKey = key => !['__proto__','prototype','constructor'].includes(key);
const has = (value,key) => Object.prototype.hasOwnProperty.call(value,key);

function parse(source) {
  requireThat(typeof JSON.rawJSON === 'function', 'LOSSLESS_JSON_RUNTIME_REQUIRED');
  requireThat(typeof source === 'string' && Buffer.byteLength(source) <= MAX_JSON,
    'JSON_INPUT_SIZE_INVALID');
  let position = 0;
  let count = 0;
  function whitespace() {
    while (position < source.length && /[\x20\t\r\n]/.test(source[position])) position++;
  }
  function string() {
    const start = position++;
    while (position < source.length) {
      const char = source[position++];
      if (char === '"') return JSON.parse(source.slice(start, position));
      if (char === '\\') position++;
    }
    throw new ExactJSONError('INVALID_JSON');
  }
  function value(depth) {
    requireThat(depth <= MAX_DEPTH && ++count <= MAX_ITEMS, 'JSON_COMPLEXITY_LIMIT');
    whitespace();
    const char = source[position];
    if (char === '"') return string();
    if (char === '{') {
      position++;
      const result = {};
      whitespace();
      if (source[position] === '}') { position++; return result; }
      while (position < source.length) {
        whitespace();
        requireThat(source[position] === '"', 'INVALID_JSON');
        const key = string();
        requireThat(safeKey(key), 'UNSAFE_JSON_KEY');
        requireThat(!has(result, key), 'DUPLICATE_JSON_KEY');
        whitespace();
        requireThat(source[position++] === ':', 'INVALID_JSON');
        Object.defineProperty(result, key, {
          value: value(depth + 1), enumerable: true, writable: true, configurable: true,
        });
        whitespace();
        const next = source[position++];
        if (next === '}') return result;
        requireThat(next === ',', 'INVALID_JSON');
      }
      throw new ExactJSONError('INVALID_JSON');
    }
    if (char === '[') {
      position++;
      const result = [];
      whitespace();
      if (source[position] === ']') { position++; return result; }
      while (position < source.length) {
        result.push(value(depth + 1));
        whitespace();
        const next = source[position++];
        if (next === ']') return result;
        requireThat(next === ',', 'INVALID_JSON');
      }
      throw new ExactJSONError('INVALID_JSON');
    }
    for (const [literal, result] of [['true', true], ['false', false], ['null', null]]) {
      if (source.startsWith(literal, position)) { position += literal.length; return result; }
    }
    const match = /^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?/.exec(source.slice(position));
    requireThat(Boolean(match), 'INVALID_JSON');
    position += match[0].length;
    const number = Number(match[0]);
    return /^-?(?:0|[1-9][0-9]*)$/.test(match[0]) && Number.isSafeInteger(number)
      && match[0] !== '-0' ? number : JSON.rawJSON(match[0]);
  }
  const result = value(0);
  whitespace();
  requireThat(position === source.length, 'INVALID_JSON');
  return result;
}
export function parseExact(source) {
  try { return parse(source); }
  catch(error) {
    if(error instanceof ExactJSONError) throw error;
    // Never return invalid JSON snippets or private response values in errors.
    throw new ExactJSONError('INVALID_JSON');
  }
}
