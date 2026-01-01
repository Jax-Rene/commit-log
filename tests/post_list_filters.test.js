const test = require('node:test');
const assert = require('node:assert/strict');
const { JSDOM } = require('jsdom');

function createDom(html = '<!doctype html><html><body></body></html>') {
        const dom = new JSDOM(html, { url: 'https://example.com/' });
        global.window = dom.window;
        global.document = dom.window.document;
        return () => {
                dom.window.close();
                delete global.window;
                delete global.document;
        };
}

test('initPostListDatePickers returns null when flatpickr missing', async () => {
        const cleanup = createDom();
        try {
                const { initPostListDatePickers } = await import('../web/frontend/post_list_filters.js');
                const result = initPostListDatePickers({ flatpickrInstance: null });
                assert.equal(result, null);
        } finally {
                cleanup();
        }
});

test('initPostListDatePickers wires inputs and triggers', async () => {
        const cleanup = createDom(`<!doctype html><html><body>
                <input id="start_date" />
                <button data-date-trigger="start"></button>
                <input id="end_date" />
                <button data-date-trigger="end"></button>
        </body></html>`);

        try {
                const { initPostListDatePickers } = await import('../web/frontend/post_list_filters.js');

                const instances = new Map();
                let startOptions;
                let endOptions;

                const flatpickrInstance = (input, options) => {
                        const instance = {
                                openCount: 0,
                                setCalls: [],
                                open() {
                                        this.openCount += 1;
                                },
                                set(key, value) {
                                        this.setCalls.push([key, value]);
                                },
                        };
                        if (input.id === 'start_date') {
                                startOptions = options;
                        }
                        if (input.id === 'end_date') {
                                endOptions = options;
                        }
                        instances.set(input.id, instance);
                        return instance;
                };

                const result = initPostListDatePickers({ flatpickrInstance });
                assert.ok(result);
                assert.equal(instances.size, 2);
                assert.ok(startOptions);
                assert.ok(endOptions);

                const startTrigger = document.querySelector('[data-date-trigger="start"]');
                const endTrigger = document.querySelector('[data-date-trigger="end"]');

                startTrigger.click();
                endTrigger.click();

                assert.equal(instances.get('start_date').openCount, 1);
                assert.equal(instances.get('end_date').openCount, 1);

                const startDate = new Date('2024-01-02T00:00:00Z');
                const endDate = new Date('2024-02-03T00:00:00Z');

                startOptions.onChange([startDate]);
                endOptions.onChange([endDate]);

                assert.deepEqual(instances.get('end_date').setCalls[0], ['minDate', startDate]);
                assert.deepEqual(instances.get('start_date').setCalls[0], ['maxDate', endDate]);
        } finally {
                cleanup();
        }
});
