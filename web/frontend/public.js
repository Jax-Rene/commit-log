import Alpine from 'alpinejs';
import htmx from 'htmx.org';

import '../static/css/input.css';

globalThis.htmx = htmx;
globalThis.Alpine = Alpine;

Alpine.start();

function initMilkdownViewer() {
        const markdownNode = document.getElementById('post-markdown-data');
        const mount = document.querySelector('[data-milkdown-viewer]');
        if (!markdownNode || !mount) {
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
                                return;
                        }
                        mount.classList.remove('hidden');
                        const fallback = document.querySelector('[data-post-fallback]');
                        if (fallback) {
                                fallback.classList.add('hidden');
                        }
                })
                .catch(error => {
                        console.error('[milkdown] 加载阅读器失败', error);
                });
}

if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initMilkdownViewer, { once: true });
} else {
        initMilkdownViewer();
}
