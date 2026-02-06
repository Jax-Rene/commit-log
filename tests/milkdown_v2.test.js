const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const MILKDOWN_PATH = path.resolve(__dirname, "../web/frontend/milkdown_v2.js");

function stripImports(source) {
  return source
    .replace(/^import\s+[^;]+;\s*/gm, "")
    .replace(/^import\s+['"][^'"]+['"];\s*/gm, "");
}

function loadMilkdownTestingModule(overrides = {}) {
  const raw = fs.readFileSync(MILKDOWN_PATH, "utf-8");
  const sanitized = stripImports(raw);

  const context = {
    console: {
      log() {},
      warn() {},
      error() {},
    },
    URL,
    URLSearchParams,
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
      throw new Error("fetch stub not implemented");
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
        return "# ";
      }
    },
    editorViewCtx: Symbol("editorViewCtx"),
    editorViewOptionsCtx: Symbol("editorViewOptionsCtx"),
    listenerCtx: Symbol("listenerCtx"),
    cursor: Symbol("cursor"),
    upload: Symbol("upload"),
    uploadConfig: { key: Symbol("uploadConfig") },
    replaceAll(markdown, flush) {
      return { markdown, flush };
    },
    $nodeSchema(_name, factory) {
      const ctx = {
        get() {
          return () => ({});
        },
      };
      const schema = typeof factory === "function" ? factory(ctx) : {};
      return {
        node: { name: _name, schema },
        type() {
          return {
            name: _name,
            create(attrs, content) {
              return { attrs: attrs || {}, content, type: { name: _name } };
            },
            createAndFill(attrs) {
              return { attrs: attrs || {}, type: { name: _name } };
            },
          };
        },
        ctx,
      };
    },
    $remark(_name, factory) {
      return typeof factory === "function" ? factory() : {};
    },
    $view(_node, factory) {
      return typeof factory === "function" ? factory() : {};
    },
    TextSelection: {
      near() {
        return {};
      },
    },
    commonStyleUrl: "common.css",
    nordStyleUrl: "nord.css",
    structuredClone: global.structuredClone,
  };
  context.Crepe.Feature = {
    BlockEdit: "block-edit",
    Toolbar: "toolbar",
    Placeholder: "placeholder",
  };
  Object.assign(context, overrides);
  context.globalThis = context;

  const scriptContent = `${sanitized}\n;globalThis.__milkdownTesting = {\n        normalizePreviewFeatures,\n        applyMarkdownToInstance,\n        ensureSetMarkdown,\n        calculateContentMetrics,\n        splitMarkdownTableRow,\n        parseMarkdownTable,\n        buildTableNodeFromMarkdown,\n        handleMarkdownTablePaste,\n        handleVideoEmbedPaste,\n        alignmentFromToken,\n        clamp,\n        normalizeSelectionContent,\n        pickProperty,\n        cloneValue,\n        coerceNumber,\n        coerceString,\n        readInlineSelection,\n        applyInlineAIResult,\n        createToast,\n        parseVideoEmbedSource,\n        videoEmbedSandboxForPlatform,\n        extractVideoEmbedNode,\n        buildVideoEmbedTitle,\n        parseCalloutMarker,\n        buildCalloutMarker,\n        normalizeCalloutEmoji,\n        readParagraphText,\n        buildCalloutNodeFromBlockquote,\n        convertBlockquoteCallouts,\n        convertParagraphVideoEmbeds,\n        initializeVideoEmbedLoadingState,\n        DEFAULT_CHANGE_POLL_INTERVAL,\n        DEFAULT_AUTOSAVE_INTERVAL,\n        PostDraftController,\n};`;

  const script = new vm.Script(scriptContent, { filename: "milkdown_v2.js" });
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
      this.operations.push({ type: "insertText", text, start, end });
      return this;
    },
    replaceSelectionWith(node) {
      this.operations.push({ type: "replaceSelectionWith", node });
      return this;
    },
    scrollIntoView() {
      this.operations.push({ type: "scrollIntoView" });
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

test("normalizePreviewFeatures keeps defaults disabled but allows overrides", () => {
  const { api, context } = loadMilkdownTestingModule();
  const overrides = { [context.Crepe.Feature.Toolbar]: true };
  const normalized = api.normalizePreviewFeatures(overrides);
  assert.strictEqual(normalized[context.Crepe.Feature.BlockEdit], false);
  assert.strictEqual(normalized[context.Crepe.Feature.Placeholder], false);
  assert.strictEqual(normalized[context.Crepe.Feature.Toolbar], true);
});

test("applyMarkdownToInstance forwards markdown to replaceAll via editor action", () => {
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
  const success = api.applyMarkdownToInstance(instance, "**bold**", {
    flush: true,
  });
  assert.ok(success);
  assert.deepStrictEqual(invokeArgs, { markdown: "**bold**", flush: true });
  assert.strictEqual(actionCalled, true);
});

test("ensureSetMarkdown patches instance once and honors overrides", async () => {
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
  const patched = api.ensureSetMarkdown(instance, {
    flush: true,
    silent: true,
  });
  assert.strictEqual(typeof patched.setMarkdown, "function");
  const result = await patched.setMarkdown("Hello", { silent: false });
  assert.strictEqual(result, true);
  assert.ok(callLog[0].invoked);
  assert.strictEqual(callLog[0].flush, true);
  assert.ok(patched.setMarkdown.__commitLogPatched__);
  const same = api.ensureSetMarkdown(patched, { flush: false });
  assert.strictEqual(same.setMarkdown, patched.setMarkdown);
});

test("calculateContentMetrics counts CJK characters and words", () => {
  const { api } = loadMilkdownTestingModule();
  const sample =
    "# 标题\n\nThis is code `const a = 1;` 和中文混合。\n\n另一个段落\n\n```js\nconsole.log(1);\n```";
  const metrics = api.calculateContentMetrics(sample);
  assert.strictEqual(metrics.words, 15);
  assert.strictEqual(metrics.characters, 22);
  assert.strictEqual(metrics.paragraphs, 3);
});

test("splitMarkdownTableRow respects escaped pipes and trimming", () => {
  const { api } = loadMilkdownTestingModule();
  const cells = api.splitMarkdownTableRow(" | foo \\| bar | baz | ");
  assert.strictEqual(cells.length, 2);
  assert.strictEqual(cells[0], "foo | bar");
  assert.strictEqual(cells[1], "baz");
});

test("parseMarkdownTable returns normalized structure", () => {
  const { api } = loadMilkdownTestingModule();
  const table = `| 名称 | 数量 |\n| :--- | ---: |\n| 苹果 | 3 |\n| 香蕉 | 7 |`;
  const parsed = api.parseMarkdownTable(table);
  assert.ok(parsed);
  assert.strictEqual(parsed.headers[0], "名称");
  assert.strictEqual(parsed.headers[1], "数量");
  assert.strictEqual(parsed.alignments[0], "left");
  assert.strictEqual(parsed.alignments[1], "right");
  assert.strictEqual(parsed.rows.length, 2);
  assert.strictEqual(parsed.rows[0][0], "苹果");
  assert.strictEqual(parsed.rows[0][1], "3");
  assert.strictEqual(parsed.rows[1][0], "香蕉");
  assert.strictEqual(parsed.rows[1][1], "7");
});

test("buildTableNodeFromMarkdown constructs schema nodes with alignment", () => {
  const { api } = loadMilkdownTestingModule();
  const schema = {
    nodes: {
      table: {
        create: (attrs, content) => ({ type: "table", attrs, content }),
      },
      table_header_row: {
        create: (attrs, content) => ({
          type: "headerRow",
          attrs,
          content,
        }),
      },
      table_row: {
        create: (attrs, content) => ({ type: "row", attrs, content }),
      },
      table_header: {
        create: (attrs, content) => ({
          type: "headerCell",
          attrs,
          content,
        }),
      },
      table_cell: {
        create: (attrs, content) => ({
          type: "bodyCell",
          attrs,
          content,
        }),
      },
      paragraph: {
        create: (attrs, content) => ({
          type: "paragraph",
          attrs,
          content: Array.isArray(content) ? content : [content],
        }),
        createAndFill() {
          return { type: "paragraph", attrs: null, content: [] };
        },
      },
    },
    text(text) {
      return { type: "text", text };
    },
  };
  const tableData = {
    headers: ["名称", "数量"],
    alignments: ["left", "center"],
    rows: [
      ["苹果", "3"],
      ["梨", "5"],
    ],
  };
  const node = api.buildTableNodeFromMarkdown(schema, tableData);
  assert.ok(node);
  assert.strictEqual(node.type, "table");
  const [headerRow, ...bodyRows] = node.content;
  assert.strictEqual(headerRow.content[1].attrs.alignment, "center");
  const lastRow = bodyRows[bodyRows.length - 1];
  const firstBodyCell = lastRow.content[0];
  const paragraph = firstBodyCell.content[0];
  assert.strictEqual(paragraph.content[0].text, "梨");
});

test("handleMarkdownTablePaste intercepts plain text tables", () => {
  const { api, context } = loadMilkdownTestingModule();
  const schema = {
    nodes: {
      table: { create: (attrs, content) => ({ type: "table", content }) },
      table_header_row: {
        create: (attrs, content) => ({ type: "headerRow", content }),
      },
      table_row: {
        create: (attrs, content) => ({ type: "row", content }),
      },
      table_header: {
        create: (attrs, content) => ({
          type: "header",
          attrs,
          content,
        }),
      },
      table_cell: {
        create: (attrs, content) => ({ type: "cell", attrs, content }),
      },
      paragraph: {
        create: (attrs, content) => ({
          type: "paragraph",
          content: Array.isArray(content) ? content : [content],
        }),
        createAndFill() {
          return { type: "paragraph", content: [] };
        },
      },
    },
    text(text) {
      return { type: "text", text };
    },
  };
  const { editor, view } = createProseMirrorStubs(context, "示例文档");
  view.state.schema = schema;
  const table = `| A | B |\n| --- | --- |\n| 1 | 2 |`;
  const event = {
    clipboardData: {
      types: ["text/plain"],
      getData(type) {
        if (type === "text/plain") {
          return table;
        }
        return "";
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
  const operation = view.state.tr.operations.find(
    (op) => op.type === "replaceSelectionWith",
  );
  assert.ok(operation);
  assert.strictEqual(operation.node.type, "table");
});

test("handleVideoEmbedPaste inserts video embed node", () => {
  const { api } = loadMilkdownTestingModule();
  const videoType = {
    create: (attrs) => ({ type: "video_embed", attrs }),
  };
  const tr = {
    operations: [],
    replaceSelectionWith(node) {
      this.operations.push({ type: "replaceSelectionWith", node });
      return this;
    },
    scrollIntoView() {
      this.operations.push({ type: "scrollIntoView" });
      return this;
    },
  };
  const view = {
    state: { schema: { nodes: { video_embed: videoType } }, tr },
    dispatchCalls: [],
    dispatch(payload) {
      this.dispatchCalls.push(payload);
    },
    focusCalls: 0,
    focus() {
      this.focusCalls += 1;
    },
  };
  const event = {
    clipboardData: {
      types: ["text/plain"],
      getData(type) {
        if (type === "text/plain") {
          return "https://www.youtube.com/watch?v=dQw4w9WgXcQ";
        }
        return "";
      },
    },
    preventDefaultCalled: false,
    preventDefault() {
      this.preventDefaultCalled = true;
    },
  };

  const handled = api.handleVideoEmbedPaste(view, event);
  assert.strictEqual(handled, true);
  assert.strictEqual(event.preventDefaultCalled, true);
  assert.strictEqual(view.dispatchCalls.length, 1);
  const operation = tr.operations.find(
    (op) => op.type === "replaceSelectionWith",
  );
  assert.ok(operation);
  assert.strictEqual(operation.node.type, "video_embed");
  assert.strictEqual(tr.operations[1].type, "scrollIntoView");
  assert.strictEqual(view.focusCalls, 1);
});

test("alignmentFromToken interprets colon placement", () => {
  const { api } = loadMilkdownTestingModule();
  assert.strictEqual(api.alignmentFromToken(":---:"), "center");
  assert.strictEqual(api.alignmentFromToken("---:"), "right");
  assert.strictEqual(api.alignmentFromToken(":---"), "left");
});

test("utility helpers normalize values", () => {
  const { api } = loadMilkdownTestingModule();
  assert.strictEqual(api.clamp("5", 0, 10), 5);
  assert.strictEqual(api.clamp("999", 0, 10), 10);
  assert.strictEqual(
    api.normalizeSelectionContent("a\r\n b\u00a0c"),
    "a\n b c",
  );
  assert.strictEqual(api.coerceNumber("8", 1), 8);
  assert.strictEqual(api.coerceNumber("NaN", 3), 3);
  assert.strictEqual(api.coerceString(42, "x"), "42");
  assert.strictEqual(api.coerceString(null, "fallback"), "fallback");
  assert.strictEqual(
    api.pickProperty({ foo: 1, bar: 2 }, ["baz", "bar"], 9),
    2,
  );
});

test("default autosave intervals throttle draft updates", () => {
  const { api } = loadMilkdownTestingModule();
  assert.strictEqual(api.DEFAULT_CHANGE_POLL_INTERVAL, 3000);
  assert.strictEqual(api.DEFAULT_AUTOSAVE_INTERVAL, 60000);
});

test("cloneValue falls back when structuredClone unavailable", () => {
  const { api, context } = loadMilkdownTestingModule({
    structuredClone: undefined,
  });
  const original = { nested: { value: 1 } };
  const cloned = api.cloneValue(original);
  assert.notStrictEqual(cloned, original);
  assert.strictEqual(cloned.nested.value, 1);
  const circular = {};
  circular.self = circular;
  const same = api.cloneValue(circular);
  assert.strictEqual(same, circular);
});

test("createToast prefers AdminUI.toast when available", () => {
  const { api, context } = loadMilkdownTestingModule();
  let message = null;
  context.window.AdminUI = {
    toast(payload) {
      message = payload;
    },
  };
  const toast = api.createToast();
  toast({ type: "success", message: "ok" });
  assert.deepStrictEqual(message, { type: "success", message: "ok" });
});

test("readInlineSelection returns normalized selection details", () => {
  const { api, context } = loadMilkdownTestingModule();
  const { editor, view } = createProseMirrorStubs(
    context,
    "Hello world\n\nParagraph",
  );
  view.state.selection.from = 0;
  view.state.selection.to = 5;
  const controller = { autoSaveRevision: "4" };
  const info = api.readInlineSelection(editor, controller);
  assert.ok(info);
  assert.strictEqual(info.normalizedText, "Hello");
  assert.strictEqual(info.revision, 4);
  assert.ok(info.coords.width > 0);
});

test("applyInlineAIResult replaces expected range and validates content", () => {
  const { api, context } = loadMilkdownTestingModule();
  const { editor, view } = createProseMirrorStubs(context, "Hello world");
  view.state.selection.from = 0;
  view.state.selection.to = 5;
  const applied = api.applyInlineAIResult(editor, {
    from: 0,
    to: 5,
    expected: "Hello",
    replacement: "Hi",
  });
  assert.strictEqual(applied, true);
  const insertOp = view.state.tr.operations.find(
    (op) => op.type === "insertText",
  );
  assert.ok(insertOp);
  assert.deepStrictEqual(insertOp, {
    type: "insertText",
    text: "Hi",
    start: 0,
    end: 5,
  });
  assert.strictEqual(view.dispatchCalls.length, 1);
});

test("applyInlineAIResult throws when expected text mismatches", () => {
  const { api, context } = loadMilkdownTestingModule();
  const { editor, view } = createProseMirrorStubs(context, "Hello world");
  view.state.selection.from = 0;
  view.state.selection.to = 5;
  assert.throws(() => {
    api.applyInlineAIResult(editor, {
      from: 0,
      to: 5,
      expected: "Other",
      replacement: "Hi",
    });
  }, /已发生变化/);
});

test("applyInlineAIResult raises error when editor missing", () => {
  const { api } = loadMilkdownTestingModule();
  assert.throws(() => {
    api.applyInlineAIResult(null, {});
  }, /尚未就绪/);
});

test("resetToLatestPublication reports error when publication missing", async () => {
  const { api, context } = loadMilkdownTestingModule();
  const { PostDraftController } = api;
  const toasts = [];
  context.window.AdminUI = {
    toast(payload) {
      toasts.push(payload);
    },
  };
  const crepe = {
    editor: {},
    getMarkdown() {
      return "草稿内容";
    },
    setMarkdown() {
      throw new Error("should not update markdown without publication");
    },
  };
  const controller = new PostDraftController(crepe, {
    post: { Content: "草稿内容" },
  });
  const result = await controller.resetToLatestPublication();
  assert.strictEqual(result, false);
  assert.strictEqual(controller.autoSavePending, false);
  assert.ok(
    toasts.some((payload) => {
      const message = payload?.message || "";
      return payload?.type === "error" || message.includes("线上版本");
    }),
  );
});

test("resetToLatestPublication syncs published snapshot and marks draft dirty", async () => {
  const { api, context } = loadMilkdownTestingModule();
  const { PostDraftController } = api;
  const toasts = [];
  context.window.AdminUI = {
    toast(payload) {
      toasts.push(payload);
    },
  };
  const publication = {
    Content: "# 发布版\n\n这是线上版本内容",
    Summary: "线上摘要",
    CoverURL: "https://img.example.com/cover.jpg",
    CoverWidth: 1280,
    CoverHeight: 720,
    Tags: [{ id: 2, name: "Golang" }],
  };
  const crepe = {
    editor: {},
    calls: [],
    getMarkdown() {
      return "旧草稿";
    },
    setMarkdown(markdown, options) {
      this.calls.push({ markdown, options });
      return Promise.resolve(true);
    },
  };
  const controller = new PostDraftController(crepe, {
    post: {
      Content: "旧草稿",
      Summary: "旧摘要",
      CoverURL: "",
      CoverWidth: 0,
      CoverHeight: 0,
      Tags: [],
    },
    latestPublication: publication,
  });
  const initialRevision = controller.autoSaveRevision;
  const initialLastSaved = controller.lastSavedContent;
  const result = await controller.resetToLatestPublication();
  assert.strictEqual(result, true);
  assert.strictEqual(controller.currentContent, publication.Content);
  assert.strictEqual(controller.postData.Content, publication.Content);
  assert.strictEqual(controller.postData.Summary, publication.Summary);
  assert.strictEqual(controller.postData.CoverURL, publication.CoverURL);
  assert.strictEqual(controller.postData.CoverWidth, publication.CoverWidth);
  assert.strictEqual(controller.postData.CoverHeight, publication.CoverHeight);
  assert.strictEqual(controller.postData.Tags[0].Name, "Golang");
  assert.ok(controller.contentMetrics.words > 0);
  assert.strictEqual(controller.autoSavePending, true);
  assert.strictEqual(controller.autoSaveRevision, initialRevision + 1);
  assert.strictEqual(controller.lastSavedContent, initialLastSaved);
  assert.deepStrictEqual(crepe.calls[0].markdown, publication.Content);
  assert.ok(
    toasts.length === 0 || toasts.every((payload) => payload.type !== "error"),
  );
});

test("buildPayload includes draft session id", () => {
  const { api } = loadMilkdownTestingModule();
  const { PostDraftController } = api;
  const crepe = {
    editor: {},
    getMarkdown() {
      return "# 标题\n\n内容";
    },
    setMarkdown() {
      return true;
    },
  };
  const controller = new PostDraftController(crepe, {
    post: { Content: "# 标题\n\n内容" },
    draft_session_id: "session-123",
  });
  const payload = controller.buildPayload("# 标题\n\n内容");
  assert.strictEqual(payload.draft_session_id, "session-123");
});

test("parseVideoEmbedSource resolves YouTube watch links", () => {
  const { api } = loadMilkdownTestingModule();
  const embed = api.parseVideoEmbedSource(
    "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  );
  assert.ok(embed);
  assert.strictEqual(embed.platform, "youtube");
  assert.ok(embed.embed.includes("https://www.youtube.com/embed/dQw4w9WgXcQ"));
  assert.ok(embed.embed.includes("modestbranding=1"));
  assert.ok(embed.embed.includes("rel=0"));
  assert.ok(embed.embed.includes("playsinline=1"));
});

test("parseVideoEmbedSource resolves Bilibili video links", () => {
  const { api } = loadMilkdownTestingModule();
  const embed = api.parseVideoEmbedSource(
    "https://www.bilibili.com/video/BV1x5411c7mD",
  );
  assert.ok(embed);
  assert.strictEqual(embed.platform, "bilibili");
  assert.ok(embed.embed.includes("player.bilibili.com/player.html"));
  assert.ok(embed.embed.includes("autoplay=0"));
});

test("videoEmbedSandboxForPlatform returns strict sandbox for bilibili", () => {
  const { api } = loadMilkdownTestingModule();
  assert.strictEqual(
    api.videoEmbedSandboxForPlatform("bilibili"),
    "allow-scripts allow-same-origin allow-presentation",
  );
  assert.strictEqual(api.videoEmbedSandboxForPlatform("youtube"), "");
});

test("parseVideoEmbedSource resolves Douyin share links", () => {
  const { api } = loadMilkdownTestingModule();
  const embed = api.parseVideoEmbedSource(
    "https://www.iesdouyin.com/share/video/7234567890123456789",
  );
  assert.ok(embed);
  assert.strictEqual(embed.platform, "douyin");
  assert.ok(embed.embed.includes("iesdouyin.com/share/video/7234567890123456789"));
});

test("parseVideoEmbedSource resolves Douyin modal_id links", () => {
  const { api } = loadMilkdownTestingModule();
  const embed = api.parseVideoEmbedSource(
    "douyin.com/modal_id=7602245594001771802",
  );
  assert.ok(embed);
  assert.strictEqual(embed.platform, "douyin");
  assert.ok(embed.embed.includes("iesdouyin.com/share/video/7602245594001771802"));
});

test("parseVideoEmbedSource supports angle-bracket autolink", () => {
  const { api } = loadMilkdownTestingModule();
  const embed = api.parseVideoEmbedSource(
    "<https://www.youtube.com/watch?v=dQw4w9WgXcQ>",
  );
  assert.ok(embed);
  assert.strictEqual(embed.platform, "youtube");
});

test("parseVideoEmbedSource rejects lookalike domains", () => {
  const { api } = loadMilkdownTestingModule();
  assert.strictEqual(
    api.parseVideoEmbedSource("https://notyoutube.com/watch?v=dQw4w9WgXcQ"),
    null,
  );
  assert.strictEqual(
    api.parseVideoEmbedSource("https://notbilibili.com/video/BV1x5411c7mD"),
    null,
  );
  assert.strictEqual(
    api.parseVideoEmbedSource(
      "https://notdouyin.com/video/7234567890123456789",
    ),
    null,
  );
});

test("extractVideoEmbedNode converts a single-url paragraph", () => {
  const { api } = loadMilkdownTestingModule();
  const node = {
    type: "paragraph",
    children: [
      { type: "text", value: "https://www.youtube.com/watch?v=dQw4w9WgXcQ" },
    ],
  };
  const embed = api.extractVideoEmbedNode(node);
  assert.ok(embed);
  assert.strictEqual(embed.type, "video_embed");
  assert.strictEqual(embed.platform, "youtube");
});

test("extractVideoEmbedNode ignores mixed paragraph content", () => {
  const { api } = loadMilkdownTestingModule();
  const node = {
    type: "paragraph",
    children: [
      { type: "text", value: "前缀 " },
      { type: "text", value: "https://www.youtube.com/watch?v=dQw4w9WgXcQ" },
    ],
  };
  const embed = api.extractVideoEmbedNode(node);
  assert.strictEqual(embed, null);
});

test("extractVideoEmbedNode skips paragraphs inside list and quote", () => {
  const { api } = loadMilkdownTestingModule();
  const node = {
    type: "paragraph",
    children: [
      { type: "text", value: "https://www.youtube.com/watch?v=dQw4w9WgXcQ" },
    ],
  };

  assert.strictEqual(api.extractVideoEmbedNode(node, "listItem"), null);
  assert.strictEqual(api.extractVideoEmbedNode(node, "blockquote"), null);
});

test("callout marker parsing extracts emoji and trailing text", () => {
  const { api } = loadMilkdownTestingModule();
  const parsed = api.parseCalloutMarker("[!callout] 💡 Q1 总主题");
  assert.ok(parsed);
  assert.strictEqual(parsed.emoji, "💡");
  assert.strictEqual(parsed.text, "Q1 总主题");
});

test("callout marker parsing supports case-insensitive marker", () => {
  const { api } = loadMilkdownTestingModule();
  const parsed = api.parseCalloutMarker(" [!CALLOUT]   ⚠️  注意事项 ");
  assert.ok(parsed);
  assert.strictEqual(parsed.emoji, "⚠️");
  assert.strictEqual(parsed.text, "注意事项");
});

test("callout marker parsing returns empty emoji when missing", () => {
  const { api } = loadMilkdownTestingModule();
  const parsed = api.parseCalloutMarker("[!callout] 总结");
  assert.ok(parsed);
  assert.strictEqual(parsed.emoji, "");
  assert.strictEqual(parsed.text, "总结");
});

test("callout marker parsing ignores unrelated text", () => {
  const { api } = loadMilkdownTestingModule();
  assert.strictEqual(api.parseCalloutMarker("普通内容"), null);
});

test("callout emoji normalization only accepts configured emojis", () => {
  const { api } = loadMilkdownTestingModule();
  assert.strictEqual(api.normalizeCalloutEmoji("💡"), "💡");
  assert.strictEqual(api.normalizeCalloutEmoji("🙂"), "");
});

test("callout marker builder only includes normalized emoji", () => {
  const { api } = loadMilkdownTestingModule();
  assert.strictEqual(api.buildCalloutMarker("💡"), "[!callout] 💡");
  assert.strictEqual(api.buildCalloutMarker("🙂"), "[!callout]");
  assert.strictEqual(api.buildCalloutMarker(""), "[!callout]");
});

function createSchemaStubs() {
  const paragraphType = {
    name: "paragraph",
    create(attrs, content) {
      const children = Array.isArray(content)
        ? content
        : content
          ? [content]
          : [];
      return {
        type: { name: "paragraph" },
        attrs: attrs || {},
        content: children,
      };
    },
  };
  const calloutType = {
    name: "callout",
    create(attrs, content) {
      const children = Array.isArray(content)
        ? content
        : content
          ? [content]
          : [];
      return {
        type: { name: "callout" },
        attrs: attrs || {},
        content: children,
      };
    },
  };
  const blockquoteType = { name: "blockquote" };
  return {
    nodes: {
      paragraph: paragraphType,
      callout: calloutType,
      blockquote: blockquoteType,
    },
    text(value) {
      return { type: { name: "text" }, text: value };
    },
  };
}

test("buildCalloutNodeFromBlockquote converts marker blockquote", () => {
  const { api } = loadMilkdownTestingModule();
  const schema = createSchemaStubs();
  const markerParagraph = {
    type: schema.nodes.paragraph,
    content: { size: 16 },
    textBetween() {
      return "[!callout] 💡 hello";
    },
  };
  const extraParagraph = { type: schema.nodes.paragraph };
  const blockquote = {
    type: schema.nodes.blockquote,
    firstChild: markerParagraph,
    childCount: 2,
    child(index) {
      return index === 0 ? markerParagraph : extraParagraph;
    },
  };

  const callout = api.buildCalloutNodeFromBlockquote(blockquote, schema);
  assert.ok(callout);
  assert.strictEqual(callout.type.name, "callout");
  assert.strictEqual(callout.attrs.emoji, "💡");
  assert.strictEqual(callout.content[0].type.name, "paragraph");
  assert.strictEqual(callout.content[1], extraParagraph);
});

test("convertBlockquoteCallouts replaces matching blockquotes", () => {
  const { api } = loadMilkdownTestingModule();
  const schema = createSchemaStubs();
  const markerParagraph = {
    type: schema.nodes.paragraph,
    content: { size: 12 },
    textBetween() {
      return "[!callout]";
    },
  };
  const blockquote = {
    type: schema.nodes.blockquote,
    firstChild: markerParagraph,
    childCount: 1,
    child() {
      return markerParagraph;
    },
    nodeSize: 4,
  };
  const doc = {
    descendants(callback) {
      callback(blockquote, 0);
    },
  };
  const tr = {
    docChanged: false,
    replaceWith(from, to, node) {
      this.docChanged = true;
      this.last = { from, to, node };
      return this;
    },
  };
  let dispatched = null;
  const view = {
    state: { doc, schema, tr },
    dispatch(payload) {
      dispatched = payload;
    },
  };

  const result = api.convertBlockquoteCallouts(view);
  assert.ok(result);
  assert.ok(dispatched);
  assert.strictEqual(dispatched.last.node.type.name, "callout");
});

test("convertParagraphVideoEmbeds replaces top-level video paragraphs", () => {
  const { api } = loadMilkdownTestingModule();
  const paragraphType = { name: "paragraph" };
  const videoType = {
    name: "video_embed",
    create(attrs) {
      return { type: { name: "video_embed" }, attrs };
    },
  };
  const paragraph = {
    type: paragraphType,
    content: { size: 43 },
    nodeSize: 4,
    textBetween() {
      return "https://www.youtube.com/watch?v=dQw4w9WgXcQ";
    },
  };
  const nestedParagraph = {
    type: paragraphType,
    content: { size: 43 },
    nodeSize: 4,
    textBetween() {
      return "https://www.youtube.com/watch?v=dQw4w9WgXcQ";
    },
  };
  const doc = {
    descendants(callback) {
      callback(paragraph, 0, doc);
      callback(nestedParagraph, 12, { type: { name: "blockquote" } });
    },
  };
  const tr = {
    docChanged: false,
    replaceWith(from, to, node) {
      this.docChanged = true;
      this.last = { from, to, node };
      return this;
    },
  };
  let dispatched = null;
  const view = {
    state: {
      doc,
      schema: { nodes: { paragraph: paragraphType, video_embed: videoType } },
      tr,
    },
    dispatch(payload) {
      dispatched = payload;
    },
  };

  const result = api.convertParagraphVideoEmbeds(view);
  assert.ok(result);
  assert.ok(dispatched);
  assert.strictEqual(dispatched.last.node.type.name, "video_embed");
  assert.strictEqual(dispatched.last.node.attrs.platform, "youtube");
});

test("initializeVideoEmbedLoadingState toggles loading classes after iframe load", () => {
  const { api } = loadMilkdownTestingModule();
  const classSet = new Set(["video-embed", "is-loading"]);
  const listeners = new Map();
  const iframe = {
    addEventListener(event, handler) {
      listeners.set(event, handler);
    },
  };
  const container = {
    dataset: {},
    classList: {
      add(name) {
        classSet.add(name);
      },
      remove(name) {
        classSet.delete(name);
      },
      contains(name) {
        return classSet.has(name);
      },
    },
    querySelector(selector) {
      if (selector === "iframe") {
        return iframe;
      }
      return null;
    },
  };
  const root = {
    querySelectorAll(selector) {
      if (selector === "[data-video-embed]") {
        return [container];
      }
      return [];
    },
  };

  const count = api.initializeVideoEmbedLoadingState(root);
  assert.strictEqual(count, 1);
  assert.strictEqual(container.dataset.videoEmbedLoadingBound, "true");
  assert.ok(classSet.has("is-loading"));

  const onLoad = listeners.get("load");
  assert.strictEqual(typeof onLoad, "function");
  onLoad();
  assert.ok(!classSet.has("is-loading"));
  assert.ok(classSet.has("is-ready"));
});
