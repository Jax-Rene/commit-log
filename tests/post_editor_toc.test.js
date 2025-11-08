const test = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { JSDOM } = require('jsdom');

const TEMPLATE_PATH = path.resolve(__dirname, '../web/template/admin/post_edit.html');

function loadSetupPostEditorToc() {
        const html = readFileSync(TEMPLATE_PATH, 'utf-8');
        const startToken = 'function setupPostEditorToc()';
        const endToken = "document.addEventListener('alpine:init'";
        const start = html.indexOf(startToken);
        if (start === -1) {
                throw new Error('无法在模板中找到 setupPostEditorToc 定义');
        }
        const end = html.indexOf(endToken, start);
        if (end === -1) {
                throw new Error('无法定位 setupPostEditorToc 结束位置');
        }
        let source = html.slice(start, end);
        const marker = 'let lockedTargetId = null; // 记录用户点击的标题，避免自动激活越界';
        if (source.includes(marker)) {
                const hook = `${marker}\n        if (typeof window !== 'undefined') {\n                window.__TEST_TOC_STATE__ = {\n                        getLockedId: () => lockedTargetId,\n                        getLockOffset: () => lockScrollOffset,\n                        runUpdate: null,\n                };\n        }`;
                source = source.replace(marker, hook);
        }
        const updateHookTarget = '\n        const requestUpdate = () => {';
        if (source.includes(updateHookTarget)) {
                const updateHook = `\n        if (typeof window !== 'undefined' && window.__TEST_TOC_STATE__) {\n                window.__TEST_TOC_STATE__.runUpdate = () => updateActive();\n        }${updateHookTarget}`;
                source = source.replace(updateHookTarget, updateHook);
        }
        const factory = new Function(`${source}\nreturn setupPostEditorToc;`);
        return factory();
}

const setupPostEditorToc = loadSetupPostEditorToc();

function createDomEnvironment() {
        const dom = new JSDOM('<!doctype html><html lang="zh-CN"><body></body></html>', {
                url: 'https://example.com/editor',
        });
        const { window } = dom;
        global.window = window;
        global.document = window.document;
        global.Node = window.Node;
        global.Element = window.Element;
        global.HTMLElement = window.HTMLElement;
        global.Event = window.Event;
        global.MouseEvent = window.MouseEvent;
        global.MutationObserver = window.MutationObserver;
        window.requestAnimationFrame = cb => {
                cb();
                return 1;
        };
        window.cancelAnimationFrame = () => {};
        window.scrollTo = () => {};
        window.matchMedia = () => ({
                matches: true,
                addEventListener() {},
                removeEventListener() {},
                addListener() {},
                removeListener() {},
        });
        window.setTimeout = cb => {
                cb();
                return 1;
        };
        window.clearTimeout = () => {};
        window.history.replaceState = () => {};
        return () => {
                dom.window.close();
                delete global.window;
                delete global.document;
                delete global.Node;
                delete global.Element;
                delete global.HTMLElement;
                delete global.Event;
                delete global.MouseEvent;
                delete global.MutationObserver;
        };
}

function mockScrollEnvironment({ scrollHeight = 2000, innerHeight = 800, scrollY = 0 } = {}) {
        Object.defineProperty(window, 'innerHeight', { configurable: true, value: innerHeight });
        Object.defineProperty(window, 'scrollY', { configurable: true, value: scrollY });
        Object.defineProperty(document, 'scrollingElement', {
                configurable: true,
                value: document.documentElement,
        });
        Object.defineProperty(document.documentElement, 'scrollHeight', {
                configurable: true,
                value: scrollHeight,
        });
        Object.defineProperty(document.documentElement, 'scrollTop', {
                configurable: true,
                value: scrollY,
        });
        Object.defineProperty(document.body, 'scrollTop', {
                configurable: true,
                value: scrollY,
        });
}

function createRect(top) {
        return {
                top,
                bottom: top + 24,
                left: 0,
                right: 200,
                width: 200,
                height: 24,
        };
}

function buildEditorDom() {
        document.body.innerHTML = '';
        const card = document.createElement('div');
        card.dataset.editorTocCard = '';
        const list = document.createElement('ol');
        list.dataset.tocList = '';
        card.appendChild(list);
        document.body.appendChild(card);

        const contentHost = document.createElement('div');
        contentHost.dataset.editorTocContent = '';
        const proseMirror = document.createElement('div');
        proseMirror.className = 'ProseMirror';
        contentHost.appendChild(proseMirror);
        document.body.appendChild(contentHost);

        ['绪论', '第二节', '第三节', '结论'].forEach((text, index) => {
                const heading = document.createElement(index === 0 ? 'h1' : 'h2');
                heading.textContent = text;
                heading.scrollIntoView = () => {};
                heading.__mockTop = 60 + index * 260;
                heading.getBoundingClientRect = () => createRect(heading.__mockTop);
                proseMirror.appendChild(heading);
        });

        return { list };
}

function dispatchScroll() {
        window.dispatchEvent(new window.Event('scroll'));
}

test('TOC 在接近底部时高亮最后一个标题', () => {
        const cleanup = createDomEnvironment();
        const { list } = buildEditorDom();
        const controller = setupPostEditorToc();
        assert.ok(controller, 'TOC 控制器应成功创建');
        controller.refresh();

        mockScrollEnvironment({ scrollHeight: 2000, innerHeight: 800, scrollY: 1180 });
        dispatchScroll();

        const items = list.querySelectorAll('.toc-item');
        assert.ok(items.length > 0, '应生成目录项');
        assert.ok(items[items.length - 1].classList.contains('is-active'), '最后一个标题应被高亮');

        controller.destroy();
        cleanup();
});

test('TOC 点击后在容差范围内保持锁定', () => {
        const cleanup = createDomEnvironment();
        const { list } = buildEditorDom();
        const controller = setupPostEditorToc();
        controller.refresh();

        mockScrollEnvironment({ scrollHeight: 2000, innerHeight: 800, scrollY: 1180 });
        dispatchScroll();

        const links = list.querySelectorAll('.toc-link');
        assert.ok(links.length > 2, '至少包含 3 个目录链接');
        links[1].dispatchEvent(new window.MouseEvent('click', { bubbles: true }));

        dispatchScroll();
        const items = list.querySelectorAll('.toc-item');
        assert.ok(items[1].classList.contains('is-active'), '点击后的标题应保持激活');

        controller.destroy();
        cleanup();
});

test('TOC 超出锁定容差后允许重新激活最后标题', () => {
        const cleanup = createDomEnvironment();
        const { list } = buildEditorDom();
        const controller = setupPostEditorToc();
        controller.refresh();

        mockScrollEnvironment({ scrollHeight: 2000, innerHeight: 800, scrollY: 1180 });
        dispatchScroll();

        const links = list.querySelectorAll('.toc-link');
        assert.ok(links.length >= 3, '至少包含 3 个目录链接');
        const targetIndex = links.length - 3;
        links[targetIndex].dispatchEvent(new window.MouseEvent('click', { bubbles: true }));

        dispatchScroll();
        let items = list.querySelectorAll('.toc-item');
        const tocState = window.__TEST_TOC_STATE__;
        assert.ok(tocState && typeof tocState.getLockedId === 'function', '测试需要可观测锁状态');
        assert.strictEqual(tocState.getLockedId(), links[targetIndex].dataset.tocTarget, '点击后应锁定对应标题');
        assert.ok(items[targetIndex].classList.contains('is-active'), '点击后应锁定对应标题');

        mockScrollEnvironment({ scrollHeight: 2000, innerHeight: 800, scrollY: 50 });
        dispatchScroll();
        tocState.runUpdate?.();
        assert.strictEqual(tocState.getLockedId(), null, '超过容差后应释放锁定');

        mockScrollEnvironment({ scrollHeight: 2000, innerHeight: 800, scrollY: 1180 });
        dispatchScroll();
        tocState.runUpdate?.();

        items = list.querySelectorAll('.toc-item');
        assert.ok(items[items.length - 1].classList.contains('is-active'), '超过容差后应回到最后一个标题');

        controller.destroy();
        cleanup();
});
