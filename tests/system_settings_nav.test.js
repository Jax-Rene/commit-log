const test = require("node:test");
const assert = require("node:assert/strict");

const {
    normalizeNavIndex,
    countNavButtonType,
    canUseNavButtonType,
} = require("../web/static/js/system_settings_nav");

test("normalizeNavIndex coerces string indices and rejects invalid values", () => {
    assert.equal(normalizeNavIndex(null), null);
    assert.equal(normalizeNavIndex(undefined), null);
    assert.equal(normalizeNavIndex("0"), 0);
    assert.equal(normalizeNavIndex(3), 3);
    assert.equal(normalizeNavIndex("not-a-number"), null);
});

test("countNavButtonType ignores the current index even when it is a string", () => {
    const navButtons = [
        { type: "about" },
        { type: "rss" },
        { type: "dashboard" },
        { type: "gallery" },
    ];

    assert.equal(countNavButtonType(navButtons, "dashboard", 2), 0);
    assert.equal(countNavButtonType(navButtons, "dashboard", "2"), 0);
    assert.equal(countNavButtonType(navButtons, "dashboard", null), 1);
    assert.equal(countNavButtonType(navButtons, "about", 2), 1);
});

test("canUseNavButtonType allows current type and blocks duplicates", () => {
    const navButtons = [
        { type: "about" },
        { type: "rss" },
        { type: "dashboard" },
    ];

    assert.equal(canUseNavButtonType(navButtons, "custom", 2), true);
    assert.equal(canUseNavButtonType(navButtons, "dashboard", 2), true);
    assert.equal(canUseNavButtonType(navButtons, "dashboard", 0), false);
});
