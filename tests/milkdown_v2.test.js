const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const MILKDOWN_PATH = path.resolve(__dirname, '../web/frontend/milkdown_v2.js');

function stripImports(source) {
        return source
                .replace(/^import\s+[^;]+;\s*/gm, '')
                .replace(/^import\s+['"][^'"]+['"];\s*/gm, '');
}

function loadMilkdownTestingModule(overrides = {}) {
        const raw = fs.readFileSync(MILKDOWN_PATH, 'utf-8');
        const sanitized = stripImports(raw);

        const context = {
                console: {
                        log() {},
                        warn() {},
                        error() {},
                },
                window: { MilkdownV2: {} },
                document: {
                        head: { appendChild() {} },
                        createElement() {
                                return {
                                        setAttribute() {},
                                        appendChild() {},
                                        classList: { add() {}, remove() {} },
                                        style: {},
                                };
                        },
                        querySelector() {
                                return null;
                        },
                        getElementById() {
                                return null;
                        },
                },
                CustomEvent: class CustomEvent {
                        constructor(type, init = {}) {
                                this.type = type;
                                this.detail = init.detail;
                        }
                },
                FormData: class FormData {
                        constructor() {
                                this.fields = new Map();
                        }
                        append(key, value) {
                                this.fields.set(key, value);
                        }
                },
                fetch: async () => {
                        throw new Error('fetch stub not implemented');
                },
                performance: { now: () => 0 },
                setTimeout: () => 1,
                clearTimeout: () => {},
                setInterval: () => 1,
                clearInterval: () => {},
                Crepe: class {
                        constructor(options) {
                                this.options = options;
                                this.editor = {
                                        action(fn) {
                                                fn({
                                                        get() {
                                                                return null;
                                                        },
                                                });
                                        },
                                        use() {},
                                        config() {},
                                };
                        }
                        setReadonly() {}
                        create() {
                                return Promise.resolve();
                        }
                        setMarkdown() {
                                return true;
                        }
                        getMarkdown() {
                                return '# ';
                        }
                },
                editorViewCtx: Symbol('editorViewCtx'),
                editorViewOptionsCtx: Symbol('editorViewOptionsCtx'),
                listenerCtx: Symbol('listenerCtx'),
                cursor: Symbol('cursor'),
                upload: Symbol('upload'),
                uploadConfig: { key: Symbol('uploadConfig') },
                replaceAll(markdown, flush) {
                        return { markdown, flush };
                },
                commonStyleUrl: 'common.css',
                nordStyleUrl: 'nord.css',
                structuredClone: global.structuredClone,
        };
        context.Crepe.Feature = {
                BlockEdit: 'block-edit',
                Toolbar: 'toolbar',
                Placeholder: 'placeholder',
        };
        Object.assign(context, overrides);
        context.globalThis = context;

        const scriptContent = `${sanitized}\n;globalThis.__milkdownTesting = {\n        normalizePreviewFeatures,\n        applyMarkdownToInstance,\n        ensureSetMarkdown,\n        calculateContentMetrics,\n        splitMarkdownTableRow,\n        parseMarkdownTable,\n        buildTableNodeFromMarkdown,\n        handleMarkdownTablePaste,\n        alignmentFromToken,\n        clamp,\n        normalizeSelectionContent,\n        pickProperty,\n        cloneValue,\n        coerceNumber,\n        coerceString,\n        readInlineSelection,\n        applyInlineAIResult,\n        createToast,\n        DEFAULT_CHANGE_POLL_INTERVAL,\n        DEFAULT_AUTOSAVE_INTERVAL,\n};`;

        const script = new vm.Script(scriptContent, { filename: 'milkdown_v2.js' });
        const contextified = vm.createContext(context);
        script.runInContext(contextified);
        return { api: context.__milkdownTesting, context };
}

function createProseMirrorStubs(context, docText, selectionOverrides = {}) {
        const contentSize = docText.length;
        const doc = {
                content: { size: contentSize },
                textBetween(from, to) {
                        return docText.slice(from, to);
                },
        };
        const baseSelection = {
                from: 0,
                to: contentSize,
                empty: false,
                $from: {
                        blockRange() {
                                return { start: 0, end: contentSize };
                        },
                },
                $to: {
                        blockRange() {
                                return { start: 0, end: contentSize };
                        },
                },
        };
        const selection = { ...baseSelection, ...selectionOverrides };
        const tr = {
                operations: [],
                insertText(text, start, end) {
                        this.operations.push({ type: 'insertText', text, start, end });
                        return this;
                },
                replaceSelectionWith(node) {
                        this.operations.push({ type: 'replaceSelectionWith', node });
                        return this;
                },
                scrollIntoView() {
                        this.operations.push({ type: 'scrollIntoView' });
                        return this;
                },
        };
        const view = {
                state: { doc, selection, tr },
                dispatchCalls: [],
                dispatch(payload) {
                        this.dispatchCalls.push(payload);
                },
                focusCalls: 0,
                focus() {
                        this.focusCalls += 1;
                },
                coordsAtPos(pos) {
                        return { left: pos, right: pos + 10, top: pos, bottom: pos + 20 };
                },
        };
        const editor = {
                action(fn) {
                        return fn({
                                get(key) {
                                        if (key === context.editorViewCtx) {
                                                return view;
                                        }
                                        return null;
                                },
                        });
                },
        };
        return { editor, view, doc, tr, selection };
}

test('normalizePreviewFeatures keeps defaults disabled but allows overrides', () => {
        const { api, context } = loadMilkdownTestingModule();
        const overrides = { [context.Crepe.Feature.Toolbar]: true };
        const normalized = api.normalizePreviewFeatures(overrides);
        assert.strictEqual(normalized[context.Crepe.Feature.BlockEdit], false);
        assert.strictEqual(normalized[context.Crepe.Feature.Placeholder], false);
        assert.strictEqual(normalized[context.Crepe.Feature.Toolbar], true);
});

test('applyMarkdownToInstance forwards markdown to replaceAll via editor action', () => {
        let invokeArgs = null;
        let actionCalled = false;
        const { api } = loadMilkdownTestingModule({
                replaceAll(markdown, flush) {
                        invokeArgs = { markdown, flush };
                        return () => {
                                actionCalled = true;
                        };
                },
        });
        const instance = {
                editor: {
                        action(fn) {
                                fn({ get() {} });
                        },
                },
        };
        const success = api.applyMarkdownToInstance(instance, '**bold**', { flush: true });
        assert.ok(success);
        assert.deepStrictEqual(invokeArgs, { markdown: '**bold**', flush: true });
        assert.strictEqual(actionCalled, true);
});

test('ensureSetMarkdown patches instance once and honors overrides', async () => {
        const callLog = [];
        const { api } = loadMilkdownTestingModule({
                replaceAll(markdown, flush) {
                        callLog.push({ markdown, flush });
                        return () => {
                                callLog[callLog.length - 1].invoked = true;
                        };
                },
        });
        const instance = {
                editor: {
                        action(fn) {
                                fn({ get() {} });
                        },
                },
        };
        const patched = api.ensureSetMarkdown(instance, { flush: true, silent: true });
        assert.strictEqual(typeof patched.setMarkdown, 'function');
        const result = await patched.setMarkdown('Hello', { silent: false });
        assert.strictEqual(result, true);
        assert.ok(callLog[0].invoked);
        assert.strictEqual(callLog[0].flush, true);
        assert.ok(patched.setMarkdown.__commitLogPatched__);
        const same = api.ensureSetMarkdown(patched, { flush: false });
        assert.strictEqual(same.setMarkdown, patched.setMarkdown);
});

test('calculateContentMetrics counts CJK characters and words', () => {
        const { api } = loadMilkdownTestingModule();
        const sample = '# 标题\n\nThis is code `const a = 1;` 和中文混合。\n\n另一个段落\n\n```js\nconsole.log(1);\n```';
        const metrics = api.calculateContentMetrics(sample);
        assert.strictEqual(metrics.words, 15);
        assert.strictEqual(metrics.characters, 22);
        assert.strictEqual(metrics.paragraphs, 3);
});

test('splitMarkdownTableRow respects escaped pipes and trimming', () => {
        const { api } = loadMilkdownTestingModule();
        const cells = api.splitMarkdownTableRow(' | foo \\| bar | baz | ');
        assert.strictEqual(cells.length, 2);
        assert.strictEqual(cells[0], 'foo | bar');
        assert.strictEqual(cells[1], 'baz');
});

test('parseMarkdownTable returns normalized structure', () => {
        const { api } = loadMilkdownTestingModule();
        const table = `| 名称 | 数量 |\n| :--- | ---: |\n| 苹果 | 3 |\n| 香蕉 | 7 |`;
        const parsed = api.parseMarkdownTable(table);
        assert.ok(parsed);
        assert.strictEqual(parsed.headers[0], '名称');
        assert.strictEqual(parsed.headers[1], '数量');
        assert.strictEqual(parsed.alignments[0], 'left');
        assert.strictEqual(parsed.alignments[1], 'right');
        assert.strictEqual(parsed.rows.length, 2);
        assert.strictEqual(parsed.rows[0][0], '苹果');
        assert.strictEqual(parsed.rows[0][1], '3');
        assert.strictEqual(parsed.rows[1][0], '香蕉');
        assert.strictEqual(parsed.rows[1][1], '7');
});

test('buildTableNodeFromMarkdown constructs schema nodes with alignment', () => {
        const { api } = loadMilkdownTestingModule();
        const schema = {
                nodes: {
                        table: { create: (attrs, content) => ({ type: 'table', attrs, content }) },
                        table_header_row: {
                                create: (attrs, content) => ({ type: 'headerRow', attrs, content }),
                        },
                        table_row: { create: (attrs, content) => ({ type: 'row', attrs, content }) },
                        table_header: {
                                create: (attrs, content) => ({ type: 'headerCell', attrs, content }),
                        },
                        table_cell: {
                                create: (attrs, content) => ({ type: 'bodyCell', attrs, content }),
                        },
                        paragraph: {
                                create: (attrs, content) => ({
                                        type: 'paragraph',
                                        attrs,
                                        content: Array.isArray(content) ? content : [content],
                                }),
                                createAndFill() {
                                        return { type: 'paragraph', attrs: null, content: [] };
                                },
                        },
                },
                text(text) {
                        return { type: 'text', text };
                },
        };
        const tableData = {
                headers: ['名称', '数量'],
                alignments: ['left', 'center'],
                rows: [
                        ['苹果', '3'],
                        ['梨', '5'],
                ],
        };
        const node = api.buildTableNodeFromMarkdown(schema, tableData);
        assert.ok(node);
        assert.strictEqual(node.type, 'table');
        const [headerRow, ...bodyRows] = node.content;
        assert.strictEqual(headerRow.content[1].attrs.alignment, 'center');
        const lastRow = bodyRows[bodyRows.length - 1];
        const firstBodyCell = lastRow.content[0];
        const paragraph = firstBodyCell.content[0];
        assert.strictEqual(paragraph.content[0].text, '梨');
});

test('handleMarkdownTablePaste intercepts plain text tables', () => {
        const { api, context } = loadMilkdownTestingModule();
        const schema = {
                nodes: {
                        table: { create: (attrs, content) => ({ type: 'table', content }) },
                        table_header_row: { create: (attrs, content) => ({ type: 'headerRow', content }) },
                        table_row: { create: (attrs, content) => ({ type: 'row', content }) },
                        table_header: { create: (attrs, content) => ({ type: 'header', attrs, content }) },
                        table_cell: {
                                create: (attrs, content) => ({ type: 'cell', attrs, content }),
                        },
                        paragraph: {
                                create: (attrs, content) => ({
                                        type: 'paragraph',
                                        content: Array.isArray(content) ? content : [content],
                                }),
                                createAndFill() {
                                        return { type: 'paragraph', content: [] };
                                },
                        },
                },
                text(text) {
                        return { type: 'text', text };
                },
        };
        const { editor, view } = createProseMirrorStubs(context, '示例文档');
        view.state.schema = schema;
        const table = `| A | B |\n| --- | --- |\n| 1 | 2 |`;
        const event = {
                clipboardData: {
                        types: ['text/plain'],
                        getData(type) {
                                if (type === 'text/plain') {
                                        return table;
                                }
                                return '';
                        },
                },
                preventDefaultCalled: false,
                preventDefault() {
                        this.preventDefaultCalled = true;
                },
        };
        const handled = api.handleMarkdownTablePaste(view, event);
        assert.strictEqual(handled, true);
        assert.strictEqual(event.preventDefaultCalled, true);
        assert.strictEqual(view.dispatchCalls.length, 1);
        const operation = view.state.tr.operations.find(op => op.type === 'replaceSelectionWith');
        assert.ok(operation);
        assert.strictEqual(operation.node.type, 'table');
});

test('alignmentFromToken interprets colon placement', () => {
        const { api } = loadMilkdownTestingModule();
        assert.strictEqual(api.alignmentFromToken(':---:'), 'center');
        assert.strictEqual(api.alignmentFromToken('---:'), 'right');
        assert.strictEqual(api.alignmentFromToken(':---'), 'left');
});

test('utility helpers normalize values', () => {
        const { api } = loadMilkdownTestingModule();
        assert.strictEqual(api.clamp('5', 0, 10), 5);
        assert.strictEqual(api.clamp('999', 0, 10), 10);
        assert.strictEqual(api.normalizeSelectionContent('a\r\n b\u00a0c'), 'a\n b c');
        assert.strictEqual(api.coerceNumber('8', 1), 8);
        assert.strictEqual(api.coerceNumber('NaN', 3), 3);
        assert.strictEqual(api.coerceString(42, 'x'), '42');
        assert.strictEqual(api.coerceString(null, 'fallback'), 'fallback');
        assert.strictEqual(api.pickProperty({ foo: 1, bar: 2 }, ['baz', 'bar'], 9), 2);
});

test('cloneValue falls back when structuredClone unavailable', () => {
        const { api, context } = loadMilkdownTestingModule({ structuredClone: undefined });
        const original = { nested: { value: 1 } };
        const cloned = api.cloneValue(original);
        assert.notStrictEqual(cloned, original);
        assert.strictEqual(cloned.nested.value, 1);
        const circular = {};
        circular.self = circular;
        const same = api.cloneValue(circular);
        assert.strictEqual(same, circular);
});

test('createToast prefers AdminUI.toast when available', () => {
        const { api, context } = loadMilkdownTestingModule();
        let message = null;
        context.window.AdminUI = {
                toast(payload) {
                        message = payload;
                },
        };
        const toast = api.createToast();
        toast({ type: 'success', message: 'ok' });
        assert.deepStrictEqual(message, { type: 'success', message: 'ok' });
});

test('readInlineSelection returns normalized selection details', () => {
        const { api, context } = loadMilkdownTestingModule();
        const { editor, view } = createProseMirrorStubs(context, 'Hello world\n\nParagraph');
        view.state.selection.from = 0;
        view.state.selection.to = 5;
        const controller = { autoSaveRevision: '4' };
        const info = api.readInlineSelection(editor, controller);
        assert.ok(info);
        assert.strictEqual(info.normalizedText, 'Hello');
        assert.strictEqual(info.revision, 4);
        assert.ok(info.coords.width > 0);
});

test('applyInlineAIResult replaces expected range and validates content', () => {
        const { api, context } = loadMilkdownTestingModule();
        const { editor, view } = createProseMirrorStubs(context, 'Hello world');
        view.state.selection.from = 0;
        view.state.selection.to = 5;
        const applied = api.applyInlineAIResult(editor, {
                from: 0,
                to: 5,
                expected: 'Hello',
                replacement: 'Hi',
        });
        assert.strictEqual(applied, true);
        const insertOp = view.state.tr.operations.find(op => op.type === 'insertText');
        assert.ok(insertOp);
        assert.deepStrictEqual(insertOp, { type: 'insertText', text: 'Hi', start: 0, end: 5 });
        assert.strictEqual(view.dispatchCalls.length, 1);
});

test('applyInlineAIResult throws when expected text mismatches', () => {
        const { api, context } = loadMilkdownTestingModule();
        const { editor, view } = createProseMirrorStubs(context, 'Hello world');
        view.state.selection.from = 0;
        view.state.selection.to = 5;
        assert.throws(() => {
                api.applyInlineAIResult(editor, {
                        from: 0,
                        to: 5,
                        expected: 'Other',
                        replacement: 'Hi',
                });
        }, /已发生变化/);
});

test('applyInlineAIResult raises error when editor missing', () => {
        const { api } = loadMilkdownTestingModule();
        assert.throws(() => {
                api.applyInlineAIResult(null, {});
        }, /尚未就绪/);
});

