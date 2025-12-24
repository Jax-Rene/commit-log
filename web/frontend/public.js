import Alpine from 'alpinejs';
import htmx from 'htmx.org';
import { createViewerTocController } from './toc_controller.js';
import { createMasonryGridController } from './masonry_grid.js';

import '../static/css/input.css';

globalThis.htmx = htmx;
globalThis.Alpine = Alpine;

Alpine.start();

let postTocController = null;
let masonryController = null;

function ensurePostToc() {
        if (!document.querySelector('[data-toc-card]')) {
                return;
        }
        if (postTocController) {
                postTocController.refresh();
                return;
        }
        const controller = createViewerTocController();
        if (controller) {
                postTocController = controller;
        }
}

function initMilkdownViewer() {
        const markdownNode = document.getElementById('post-markdown-data');
        const mount = document.querySelector('[data-milkdown-viewer]');
        if (!markdownNode || !mount) {
                ensurePostToc();
                return;
        }

        let markdown = '';
        const raw = markdownNode.textContent || '';
        if (raw.trim()) {
                try {
                        markdown = JSON.parse(raw);
                } catch (error) {
                        console.warn('[milkdown] 无法解析文章内容', error);
                        markdown = '';
                }
        }

        if (typeof markdown !== 'string' || markdown.trim().length === 0) {
                return;
        }

        import('./milkdown_v2.js')
                .then(() => {
                        const viewerFactory = globalThis.MilkdownV2?.createReadOnlyViewer;
                        if (typeof viewerFactory !== 'function') {
                                console.warn('[milkdown] 预览渲染器尚未就绪');
                                return null;
                        }
                        return viewerFactory({ mount, markdown });
                })
                .then(result => {
                        if (!result) {
                                ensurePostToc();
                                return;
                        }
                        mount.classList.remove('hidden');
                        const fallback = document.querySelector('[data-post-fallback]');
                        if (fallback) {
                                fallback.classList.add('hidden');
                        }
                        ensurePostToc();
                })
                .catch(error => {
                        console.error('[milkdown] 加载阅读器失败', error);
                        ensurePostToc();
                });
}

function ensureMasonryGrid() {
        if (!document.getElementById('post-grid')) {
                return;
        }
        if (masonryController) {
                masonryController.refresh();
                return;
        }
        masonryController = createMasonryGridController();
}

function setupSearchSuggestions() {
        const input = document.querySelector('[data-search-suggest-input]');
        const panel = document.getElementById('search-suggestions');
        const loadingTemplate = document.getElementById('search-suggestions-loading');
        const errorTemplate = document.getElementById('search-suggestions-error');
        const container = input?.closest('form');

        if (!input || !panel || !loadingTemplate || !errorTemplate || !container) {
                return;
        }

        const renderTemplate = templateNode => {
                panel.innerHTML = templateNode?.innerHTML ?? '';
        };

        const clearSearch = () => {
                if (!input.value.trim() && !panel.innerHTML) {
                        return;
                }
                input.value = '';
                panel.innerHTML = '';
                input.dispatchEvent(new Event('input', { bubbles: true }));
                input.dispatchEvent(new Event('search', { bubbles: true }));
        };

        input.addEventListener('htmx:beforeRequest', () => {
                if (!input.value.trim()) {
                        panel.innerHTML = '';
                        return;
                }
                renderTemplate(loadingTemplate);
        });

        input.addEventListener('htmx:responseError', () => {
                renderTemplate(errorTemplate);
        });

        const clearIfEmpty = () => {
                if (!input.value.trim()) {
                        panel.innerHTML = '';
                }
        };

        input.addEventListener('input', clearIfEmpty);
        input.addEventListener('search', clearIfEmpty);
        document.addEventListener('click', event => {
                if (!container.contains(event.target)) {
                        clearSearch();
                }
        });
}

function bootPublic() {
        // 构建只读渲染与目录浮框
        initMilkdownViewer();
        ensurePostToc();
        ensureMasonryGrid();
        setupSearchSuggestions();
}

if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', bootPublic, { once: true });
} else {
        bootPublic();
}

document.addEventListener('htmx:afterSwap', ensureMasonryGrid);

window.addEventListener('pagehide', () => {
        if (postTocController) {
                postTocController.destroy();
                postTocController = null;
        }
        if (masonryController) {
                masonryController.destroy();
                masonryController = null;
        }
}, { once: true });
