const test = require('node:test');
const assert = require('node:assert/strict');

const {
  splitKeywordTokens,
  normalizeKeywordList,
  mergeKeywordList,
  stringifyKeywordList,
  removeKeywordToken,
} = require('../web/static/js/system_settings_keywords');

test('splitKeywordTokens supports spaces and punctuation separators', () => {
  const tokens = splitKeywordTokens('AI, 全栈工程师；博客  技术，Go;React');

  assert.deepEqual(tokens, ['AI', '全栈工程师', '博客', '技术', 'Go', 'React']);
});

test('normalizeKeywordList trims and deduplicates case-insensitively', () => {
  const normalized = normalizeKeywordList(['  AI ', '博客', 'ai', 'Go', 'GO', '', '  ']);

  assert.deepEqual(normalized, ['AI', '博客', 'Go']);
});

test('mergeKeywordList appends draft input and keeps unique order', () => {
  const merged = mergeKeywordList(['AI', '博客'], 'go, AI；TypeScript  Go');

  assert.deepEqual(merged, ['AI', '博客', 'go', 'TypeScript']);
});

test('stringifyKeywordList outputs normalized comma-separated string', () => {
  assert.equal(stringifyKeywordList(['AI', '博客', 'ai', 'Go']), 'AI, 博客, Go');
  assert.equal(stringifyKeywordList('AI，博客  Go'), 'AI, 博客, Go');
});

test('removeKeywordToken removes only the clicked token by value', () => {
  const next = removeKeywordToken(['AI', 'Go', 'TypeScript'], 'Go');

  assert.deepEqual(next, ['AI', 'TypeScript']);
});

test('removeKeywordToken matches case-insensitively and keeps other tags', () => {
  const next = removeKeywordToken(['AI', 'Go', 'TypeScript'], 'go');

  assert.deepEqual(next, ['AI', 'TypeScript']);
});

test('removeKeywordToken ignores unknown token', () => {
  const current = ['AI', 'Go'];
  const next = removeKeywordToken(current, 'Rust');

  assert.deepEqual(next, current);
});
