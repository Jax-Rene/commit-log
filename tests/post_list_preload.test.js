const test = require("node:test");
const assert = require("node:assert/strict");
const { JSDOM } = require("jsdom");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

function loadPostListScript(context) {
  const htmlPath = path.resolve(
    __dirname,
    "../web/template/admin/post_list.html",
  );
  const html = fs.readFileSync(htmlPath, "utf8");
  const match = html.match(/<script>([\s\S]*?)<\/script>/);
  if (!match) {
    throw new Error("post_list script block not found");
  }
  vm.runInContext(match[1], context);
}

test("post list preloads next page without reaching bottom", async () => {
  const dom = new JSDOM(
    `<!doctype html><html><body>
                <div data-post-list data-current-page="1" data-total-pages="3" data-query="&foo=bar"></div>
                <div data-preload-status class="hidden"></div>
                <div data-preload-sentinel></div>
                <nav data-post-pagination></nav>
                </body></html>`,
    { url: "https://example.com/admin/posts", runScripts: "outside-only" },
  );
  const { window } = dom;
  const { document } = window;

  // Stubs required globals used inside the script
  global.window = window;
  global.document = window.document;
  global.navigator = window.navigator;
  global.flatpickr = { localize: () => {}, l10ns: { zh: {} } };
  window.flatpickr = global.flatpickr;
  window.AdminUI = { confirm: () => Promise.resolve(true), toast: () => {} };

  class FakeObserver {
    observe() {}
    disconnect() {}
  }
  global.IntersectionObserver = FakeObserver;
  window.IntersectionObserver = FakeObserver;

  const sentinel = document.querySelector("[data-preload-sentinel]");
  sentinel.getBoundingClientRect = () => ({
    top: 4000,
    bottom: 4001,
    left: 0,
    right: 1,
    width: 1,
    height: 1,
  });

  const requests = [];
  global.fetch = async (url) => {
    requests.push(url);
    return {
      ok: true,
      text: async () =>
        '<div data-post-list><article id="new-article"></article></div><nav data-post-pagination></nav>',
    };
  };
  window.fetch = global.fetch;

  try {
    loadPostListScript(dom.getInternalVMContext());
    // allow warmup timer to fire
    await new Promise((resolve) => setTimeout(resolve, 300));

    assert.equal(requests.length, 1);
    assert.equal(requests[0], "/admin/posts?page=2&foo=bar");

    const list = document.querySelector("[data-post-list]");
    assert.ok(
      list.querySelector("#new-article"),
      "preloaded article should be appended",
    );
  } finally {
    dom.window.close();
    delete global.window;
    delete global.document;
    delete global.navigator;
    delete global.fetch;
    delete global.flatpickr;
    delete global.IntersectionObserver;
  }
});
