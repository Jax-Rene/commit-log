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

test("关键词输入框仅通过回车提交草稿", () => {
    assert.match(content, /x-ref="keywordInput"/);
    assert.match(content, /@keydown\.enter\.prevent="commitKeywordDraft\(\)"/);
    assert.doesNotMatch(content, /@blur="commitKeywordDraft\(\)"/);
});

test("关键词容器空白区域点击会聚焦输入框", () => {
    assert.match(
        content,
        /@click="\$refs\.keywordInput && \$refs\.keywordInput\.focus\(\)"/,
    );
});

test("关键词删除按钮阻止事件冒泡并按标签值删除", () => {
    assert.match(content, /@mousedown\.prevent\.stop/);
    assert.match(
        content,
        /@click\.prevent\.stop="removeKeywordTag\(tag\)"/,
    );
});

test("关键词区使用显式 for/id 绑定，避免整块 label 点击转发到删除按钮", () => {
    assert.match(content, /for="site-keywords-input"/);
    assert.match(content, /id="site-keywords-input"/);
    assert.match(
        content,
        /<div class="flex flex-col gap-2">[\s\S]*for="site-keywords-input"[\s\S]*>默认关键词<\/label\s*>/,
    );
});
