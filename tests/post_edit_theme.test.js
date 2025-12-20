const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const TEMPLATE_PATH = path.resolve(
    __dirname,
    "../web/template/admin/post_edit.html",
);
const TEMPLATE_SOURCE = fs.readFileSync(TEMPLATE_PATH, "utf-8");

test("slash menu popup height stays within viewport", () => {
    assert.match(
        TEMPLATE_SOURCE,
        /\.milkdown-immersive-shell\s+\.milkdown\s+\.milkdown-slash-menu\s*\{[^}]*max-height:\s*min\(420px,\s*calc\(100vh\s*-\s*5rem\)\);/i,
        "slash menu should cap its height to avoid exceeding viewport",
    );
    assert.match(
        TEMPLATE_SOURCE,
        /\.milkdown-immersive-shell\s+\.milkdown\s+\.milkdown-slash-menu\s*\{[^}]*overflow:\s*auto;/i,
        "slash menu should remain scrollable when height is constrained",
    );
});
