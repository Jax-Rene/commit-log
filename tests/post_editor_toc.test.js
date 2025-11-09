const test = require('node:test');
const assert = require('node:assert/strict');
const { JSDOM } = require('jsdom');

function loadTocModule() {
        return import('../web/frontend/toc_controller.js');
}

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
        window.performance = window.performance || {};
        window.performance.now = () => Date.now();
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

function createRect(top) {
        return {
                top,
                bottom: top + 32,
                left: 0,
                right: 200,
                width: 200,
                height: 32,
        };
}

function buildEditorDom() {
        document.body.innerHTML = '';
        const scrollContainer = document.createElement('div');
        scrollContainer.dataset.editorScroll = '';
        Object.defineProperties(scrollContainer, {
                scrollHeight: { configurable: true, value: 2000, writable: true },
                clientHeight: { configurable: true, value: 800, writable: true },
                scrollTop: { configurable: true, value: 0, writable: true },
        });
        scrollContainer.style.overflowY = 'auto';
        scrollContainer.style.maxHeight = '600px';
        scrollContainer.getBoundingClientRect = () => ({
                top: 0,
                bottom: 600,
                left: 0,
                right: 400,
                width: 400,
                height: 600,
        });
        document.body.appendChild(scrollContainer);

        const contentHost = document.createElement('div');
        contentHost.dataset.editorTocContent = '';
        scrollContainer.appendChild(contentHost);

        const proseMirror = document.createElement('div');
        proseMirror.className = 'ProseMirror';
        contentHost.appendChild(proseMirror);

        const headings = ['绪论', '第二节', '第三节', '结论'];
        headings.forEach((text, index) => {
                const tag = index === 0 ? 'h1' : 'h2';
                const heading = document.createElement(tag);
                heading.textContent = text;
                const mockTop = 80 + index * 320;
                heading.getBoundingClientRect = () => createRect(mockTop);
                proseMirror.appendChild(heading);
        });

        const card = document.createElement('div');
        card.dataset.editorTocCard = '';
        card.className = 'hidden';
        const list = document.createElement('ol');
        list.dataset.tocList = '';
        card.appendChild(list);
        document.body.appendChild(card);

        return { scrollContainer, list };
}

function setScrollState(scrollContainer, scrollTop) {
        scrollContainer.scrollTop = scrollTop;
}

test('editor TOC highlights the last heading near scroll end', async () => {
        const cleanup = createDomEnvironment();
        try {
                const { createEditorTocController } = await loadTocModule();
                const { scrollContainer, list } = buildEditorDom();
                const controller = createEditorTocController({
                        scrollContainer,
                        enableTestAPI: true,
                });
                assert.ok(controller, 'controller should be created');
                controller.refresh();

                setScrollState(scrollContainer, 1180);
                controller.__test.runUpdate();

        const items = list.querySelectorAll('.toc-item');
                assert.ok(items.length >= 4, 'toc should contain heading entries');
                assert.ok(items[items.length - 1].classList.contains('is-active'), 'last heading should be active');

                controller.destroy();
        } catch (error) {
                console.error(error);
                throw error;
        } finally {
                cleanup();
        }
});

test('editor TOC keeps clicked heading active within lock tolerance', async () => {
        const cleanup = createDomEnvironment();
        try {
                const { createEditorTocController } = await loadTocModule();
                const { scrollContainer, list } = buildEditorDom();
                const controller = createEditorTocController({
                        scrollContainer,
                        enableTestAPI: true,
                });
                controller.refresh();

                setScrollState(scrollContainer, 600);
                controller.__test.runUpdate();

                const links = list.querySelectorAll('.toc-link');
                assert.ok(links.length >= 3, 'toc should have multiple links');
                const target = links[1];
                target.dispatchEvent(new window.MouseEvent('click', { bubbles: true }));

                setScrollState(scrollContainer, 720);
                controller.__test.runUpdate();
                const items = list.querySelectorAll('.toc-item');
                assert.ok(items[1].classList.contains('is-active'), 'clicked heading remains active within tolerance');

                setScrollState(scrollContainer, 980);
	controller.__test.runUpdate();
        const updatedItems = list.querySelectorAll('.toc-item');
                assert.ok(
                        !updatedItems[1].classList.contains('is-active') || updatedItems[updatedItems.length - 1].classList.contains('is-active'),
                        'locking should release after exceeding tolerance',
                );

                controller.destroy();
        } catch (error) {
                console.error(error);
                throw error;
        } finally {
                cleanup();
        }
});
