const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const TEMPLATE_PATH = path.resolve(
    __dirname,
    "../web/template/admin/post_edit.html",
);
const TEMPLATE_SOURCE = fs.readFileSync(TEMPLATE_PATH, "utf-8");

test("post editor provides a publish-like article preview", () => {
    assert.match(
        TEMPLATE_SOURCE,
        /@click="openArticlePreviewPage\(\)"/,
        "preview button should open preview page",
    );
    assert.match(
        TEMPLATE_SOURCE,
        /文章预览/,
        "preview button label should be visible in the template",
    );
    assert.match(
        TEMPLATE_SOURCE,
        /action="\/admin\/posts\/preview"/,
        "preview form should post to preview endpoint",
    );
    assert.match(
        TEMPLATE_SOURCE,
        /target="_blank"/,
        "preview form should open in a new page",
    );
    assert.match(
        TEMPLATE_SOURCE,
        /name="content"/,
        "preview form should include content field",
    );
});
