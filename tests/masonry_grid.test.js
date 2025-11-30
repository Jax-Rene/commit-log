const test = require('node:test');
const assert = require('node:assert/strict');
const { JSDOM } = require('jsdom');

function createDom() {
        const dom = new JSDOM('<!doctype html><html><body></body></html>', {
                url: 'https://example.com/',
        });
        const { window } = dom;
        global.window = window;
        global.document = window.document;
        global.Node = window.Node;
        global.HTMLElement = window.HTMLElement;
        window.requestAnimationFrame = cb => {
                cb();
                return 1;
        };
        window.cancelAnimationFrame = () => {};
        window.getComputedStyle = () => ({
                gridAutoRows: '10',
                rowGap: '0',
        });
        global.requestAnimationFrame = window.requestAnimationFrame;
        global.cancelAnimationFrame = window.cancelAnimationFrame;
        return () => {
                dom.window.close();
                delete global.window;
                delete global.document;
                delete global.Node;
                delete global.HTMLElement;
                delete global.requestAnimationFrame;
                delete global.cancelAnimationFrame;
        };
}

function buildPostContent(heights) {
        const host = document.createElement('div');
        host.id = 'post-content';
        const container = document.createElement('div');
        container.id = 'post-grid';
        container.style.gridAutoRows = '10';
        container.style.rowGap = '0';
        host.appendChild(container);

        heights.forEach((height, index) => {
                const card = document.createElement('article');
                card.dataset.postCard = '';
                card.id = `card-${index}`;
                card.getBoundingClientRect = () => ({
                        top: 0,
                        left: 0,
                        width: 300,
                        height,
                        right: 300,
                        bottom: height,
                });
                container.appendChild(card);
        });

        return { host, container };
}

test('masonry recalculates rows after HTMX swaps', async () => {
        const cleanup = createDom();
        try {
                const { createMasonryGridController } = await import('../web/frontend/masonry_grid.js');

                const initial = buildPostContent([120, 200]);
                document.body.appendChild(initial.host);

                const controller = createMasonryGridController();
                controller.refresh();

                let cards = initial.container.querySelectorAll('[data-post-card]');
                assert.equal(cards[0].style.gridRowEnd, 'span 12');
                assert.equal(cards[1].style.gridRowEnd, 'span 20');

                const swapped = buildPostContent([80, 160]);
                document.body.replaceChild(swapped.host, initial.host);

                document.dispatchEvent(new window.CustomEvent('htmx:afterSwap'));

                cards = swapped.container.querySelectorAll('[data-post-card]');
                assert.equal(cards[0].style.gridRowEnd, 'span 8');
                assert.equal(cards[1].style.gridRowEnd, 'span 16');

                controller.destroy();
        } finally {
                cleanup();
        }
});
