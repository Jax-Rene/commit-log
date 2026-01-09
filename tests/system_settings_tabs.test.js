const test = require("node:test");
const assert = require("node:assert/strict");

const {
    normalizeTabId,
} = require("../web/static/js/system_settings_tabs");

test("normalizeTabId matches known tabs", () => {
    const allowed = ["basic", "gallery", "contacts", "ai"];

    assert.equal(normalizeTabId("#gallery", allowed, "basic"), "gallery");
    assert.equal(normalizeTabId("  #AI ", allowed, "basic"), "ai");
    assert.equal(normalizeTabId("contacts", allowed, "basic"), "contacts");
});

test("normalizeTabId falls back when input is empty or unknown", () => {
    const allowed = ["basic", "gallery", "contacts", "ai"];

    assert.equal(normalizeTabId("", allowed, "basic"), "basic");
    assert.equal(normalizeTabId("unknown", allowed, "basic"), "basic");
    assert.equal(normalizeTabId("unknown", allowed), "basic");
});

test("normalizeTabId handles empty allowed list", () => {
    assert.equal(normalizeTabId("#gallery", [], "basic"), "basic");
    assert.equal(normalizeTabId("", [], ""), "");
});
