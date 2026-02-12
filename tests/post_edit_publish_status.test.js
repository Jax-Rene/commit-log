const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const TEMPLATE_PATH = path.resolve(
    __dirname,
    "../web/template/admin/post_edit.html",
);
const TEMPLATE_SOURCE = fs.readFileSync(TEMPLATE_PATH, "utf-8");

function extractPostEditorScript() {
    const match = TEMPLATE_SOURCE.match(
        /<script>\s*function setupPostEditorToc\(\)\s*\{[\s\S]*?<\/script>/,
    );
    assert.ok(
        match && match[0],
        "expected to find post editor Alpine script block",
    );
    return match[0]
        .replace(/^<script>\s*/, "")
        .replace(/\s*<\/script>\s*$/, "");
}

function createPostEditorShell() {
    let shellFactory = null;
    const alpineInitListeners = [];
    const context = {
        window: {
            CommitLog: {},
            AdminUI: null,
            MilkdownV2: {},
            confirm: () => true,
            setTimeout,
            clearTimeout,
            addEventListener() {},
            removeEventListener() {},
        },
        document: {
            addEventListener(event, listener) {
                if (event === "alpine:init") {
                    alpineInitListeners.push(listener);
                }
            },
        },
        Alpine: {
            data(name, factory) {
                if (name === "postEditorShell") {
                    shellFactory = factory;
                }
            },
        },
        Intl,
        Date,
        URL,
        setTimeout,
        clearTimeout,
        setInterval,
        clearInterval,
        console: {
            log() {},
            warn() {},
            error() {},
        },
    };
    context.globalThis = context;
    context.window.window = context.window;
    context.window.document = context.document;
    context.window.Alpine = context.Alpine;

    const script = new vm.Script(extractPostEditorScript(), {
        filename: "post_edit.html<script>",
    });
    const sandbox = vm.createContext(context);
    script.runInContext(sandbox);
    alpineInitListeners.forEach((listener) => listener());

    assert.equal(
        typeof shellFactory,
        "function",
        "expected Alpine.data('postEditorShell') to be registered",
    );
    return shellFactory();
}

function createController(overrides = {}) {
    return {
        publishing: false,
        loading: false,
        autoSaving: false,
        autoSavePending: false,
        autoSaveError: "",
        lastAutoSavedAt: null,
        postId: "42",
        postData: {
            status: "published",
        },
        latestPublication: null,
        contentMetrics: null,
        getLatestPublication() {
            return null;
        },
        async publish() {
            return true;
        },
        ...overrides,
    };
}

test("publishArticle keeps publishing flag during pre-publish refresh", async () => {
    const shell = createPostEditorShell();
    let publishingDuringInternalRefresh = null;

    shell.controller = createController({
        async publish() {
            shell.refreshStatus();
            publishingDuringInternalRefresh = shell.publishing;
            return true;
        },
    });

    await shell.publishArticle();

    assert.equal(
        publishingDuringInternalRefresh,
        true,
        "refreshStatus should not clear local publishing state while publish flow is running",
    );
});

test("refreshStatus still reflects controller publishing state", () => {
    const shell = createPostEditorShell();
    shell.controller = createController({
        publishing: true,
    });

    shell.refreshStatus();

    assert.equal(shell.publishing, true);
});

test("openMetadataPanel pins and keeps quick actions expanded", () => {
    const shell = createPostEditorShell();
    shell.quickActionsExpanded = true;
    shell.quickActionsTimer = 123;

    shell.openMetadataPanel();

    assert.equal(shell.panelOpen, true);
    assert.equal(shell.quickActionsPinned, true);
    assert.equal(shell.quickActionsExpanded, true);
    assert.equal(shell.quickActionsTimer, null);
});

test("closePanel keeps quick actions visible while pinned", () => {
    const shell = createPostEditorShell();
    shell.panelOpen = true;
    shell.quickActionsPinned = true;
    shell.quickActionsExpanded = true;

    shell.closePanel();

    assert.equal(shell.panelOpen, false);
    assert.equal(shell.quickActionsPinned, true);
    assert.equal(shell.quickActionsExpanded, true);
});

test("outside click does not collapse pinned quick actions", () => {
    const shell = createPostEditorShell();
    shell.quickActionsPinned = true;
    shell.quickActionsExpanded = true;

    shell.handleQuickActionsOutside();

    assert.equal(shell.quickActionsExpanded, true);
});

test("manual collapse clears pinned state", () => {
    const shell = createPostEditorShell();
    shell.quickActionsPinned = true;
    shell.quickActionsExpanded = true;

    shell.collapseQuickActions();

    assert.equal(shell.quickActionsPinned, false);
    assert.equal(shell.quickActionsExpanded, false);
});
