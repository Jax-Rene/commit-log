const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const templatePath = path.join(
    __dirname,
    "..",
    "web",
    "template",
    "admin",
    "system_settings.html",
);
const content = fs.readFileSync(templatePath, "utf8");

test("默认分享图区域包含上传控件", () => {
    assert.match(content, /x-ref="socialImageInput"/);
    assert.match(content, /@change="handleSocialImageSelected\(\$event\)"/);
    assert.match(content, /@click="triggerSocialImageUpload\(\)"/);
    assert.match(content, /@click="clearSocialImage\(\)"/);
});

test("默认分享图上传方法已定义", () => {
    assert.match(content, /socialImageUploading:\s*false/);
    assert.match(content, /triggerSocialImageUpload\(\)\s*{/);
    assert.match(content, /handleSocialImageSelected\(event\)\s*{/);
    assert.match(content, /async\s+uploadSocialImage\(file\)\s*{/);
    assert.match(content, /clearSocialImage\(\)\s*{/);
});

test("站点 Favicon 区域包含上传控件", () => {
    assert.match(content, /x-ref="faviconInput"/);
    assert.match(content, /@change="handleFaviconSelected\(\$event\)"/);
    assert.match(content, /@click="triggerFaviconUpload\(\)"/);
    assert.match(content, /@click="clearFavicon\(\)"/);
});

test("站点 Favicon 上传方法已定义", () => {
    assert.match(content, /faviconUploading:\s*false/);
    assert.match(content, /triggerFaviconUpload\(\)\s*{/);
    assert.match(content, /handleFaviconSelected\(event\)\s*{/);
    assert.match(content, /async\s+uploadFavicon\(file\)\s*{/);
    assert.match(content, /clearFavicon\(\)\s*{/);
});
