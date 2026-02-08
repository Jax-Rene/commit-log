(function (root, factory) {
  if (typeof module === 'object' && module.exports) {
    module.exports = factory();
  } else {
    root.SystemSettingsKeywords = factory();
  }
})(this, function () {
  function splitKeywordTokens(input) {
    const trimmed = String(input || '').trim();
    if (!trimmed) {
      return [];
    }

    return trimmed
      .split(/[\s,，;；]+/)
      .map((token) => token.trim())
      .filter(Boolean);
  }

  function normalizeKeywordList(input) {
    const rawTokens = Array.isArray(input) ? input : splitKeywordTokens(input);
    const seen = new Set();
    const normalized = [];

    for (const token of rawTokens) {
      const candidate = String(token || '').trim();
      if (!candidate) {
        continue;
      }
      const lowered = candidate.toLowerCase();
      if (seen.has(lowered)) {
        continue;
      }
      seen.add(lowered);
      normalized.push(candidate);
    }

    return normalized;
  }

  function mergeKeywordList(currentList, draftInput) {
    const current = Array.isArray(currentList) ? currentList : [];
    return normalizeKeywordList([...current, ...splitKeywordTokens(draftInput)]);
  }

  function removeKeywordToken(currentList, tokenToRemove) {
    const current = Array.isArray(currentList) ? currentList : [];
    const target = String(tokenToRemove || '')
      .trim()
      .toLowerCase();
    if (!target) {
      return normalizeKeywordList(current);
    }

    let removed = false;
    const next = [];

    for (const token of current) {
      const candidate = String(token || '').trim();
      if (!candidate) {
        continue;
      }
      if (!removed && candidate.toLowerCase() === target) {
        removed = true;
        continue;
      }
      next.push(candidate);
    }

    return normalizeKeywordList(next);
  }

  function stringifyKeywordList(input) {
    return normalizeKeywordList(input).join(', ');
  }

  return {
    splitKeywordTokens,
    normalizeKeywordList,
    mergeKeywordList,
    removeKeywordToken,
    stringifyKeywordList,
  };
});
