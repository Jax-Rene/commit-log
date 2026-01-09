(function (root, factory) {
  if (typeof module === "object" && module.exports) {
    module.exports = factory();
  } else {
    root.SystemSettingsTabs = factory();
  }
})(this, function () {
  function normalizeTabId(raw, allowed, fallback) {
    if (!Array.isArray(allowed) || allowed.length === 0) {
      return fallback || "";
    }

    const trimmed = String(raw || "")
      .trim()
      .replace(/^#/, "");
    if (!trimmed) {
      return fallback || allowed[0];
    }

    const normalized = trimmed.toLowerCase();
    const match = allowed.find((tab) => tab.toLowerCase() === normalized);
    return match || fallback || allowed[0];
  }

  return { normalizeTabId };
});
