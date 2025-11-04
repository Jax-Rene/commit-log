import Alpine from 'alpinejs';
import htmx from 'htmx.org';

import '../static/css/input.css';

globalThis.htmx = htmx;
globalThis.Alpine = Alpine;

Alpine.start();

let postTocCleanup = null;

// 枚举正文标题并渲染右侧目录卡片
function initPostToc() {
        const card = document.querySelector('[data-toc-card]');
        const list = card?.querySelector('[data-toc-list]');
        if (!card || !list) {
                return;
        }

        if (postTocCleanup) {
                postTocCleanup();
                postTocCleanup = null;
        }

        list.innerHTML = '';

        const containers = Array.from(document.querySelectorAll('[data-toc-content]')).filter(node => {
                const style = window.getComputedStyle(node);
                return style.display !== 'none' && !node.classList.contains('hidden');
        });
        const contentRoot = containers[0];

        if (!contentRoot) {
                card.hidden = true;
                return;
        }

        const headings = Array.from(contentRoot.querySelectorAll('h1, h2, h3')).filter(node => {
                const text = (node.textContent || '').trim();
                return text.length > 0;
        });

        if (headings.length === 0) {
                card.hidden = true;
                return;
        }

        card.hidden = false;

        const slugCounts = new Map();
        const headingItems = [];

        const ensureId = node => {
                if (node.id && node.id.trim().length > 0) {
                        return node.id;
                }
                const base = (node.textContent || '')
                        .toLowerCase()
                        .trim()
                        .replace(/[\s]+/g, '-')
                        .replace(/[^a-z0-9\-]/g, '');
                const safe = base || 'section';
                const count = slugCounts.get(safe) || 0;
                slugCounts.set(safe, count + 1);
                const id = count === 0 ? safe : `${safe}-${count}`;
                node.setAttribute('id', id);
                return id;
        };

        const makeLevel = tagName => {
                const level = Number.parseInt(tagName.replace('H', ''), 10);
                if (Number.isNaN(level)) {
                        return 3;
                }
                return Math.max(1, Math.min(level, 3));
        };

        const itemsMap = new Map();

        headings.forEach(heading => {
                const id = ensureId(heading);
                const text = (heading.textContent || '').trim();
                if (!text) {
                        return;
                }
                const level = makeLevel(heading.tagName);
                const li = document.createElement('li');
                li.className = 'toc-item';
                li.dataset.level = String(level);

                const link = document.createElement('a');
                link.className = 'toc-link';
                link.href = `#${encodeURIComponent(id)}`;
                link.textContent = '';
                link.title = text;

                const tick = document.createElement('span');
                tick.className = 'toc-tick';
                tick.setAttribute('aria-hidden', 'true');

                const textNode = document.createElement('span');
                textNode.className = 'toc-text';
                textNode.textContent = text;

                link.append(tick, textNode);
                li.appendChild(link);
                list.appendChild(li);

                itemsMap.set(id, { heading, li, link });
                headingItems.push({ heading, id });
        });

        if (headingItems.length === 0) {
                card.hidden = true;
                list.innerHTML = '';
                return;
        }

        let activeId = null;

        const setActive = id => {
                if (!id || !itemsMap.has(id) || activeId === id) {
                        return;
                }
                if (activeId && itemsMap.has(activeId)) {
                        const prev = itemsMap.get(activeId);
                        prev.li.classList.remove('is-active');
                        prev.link.removeAttribute('aria-current');
                }
                const next = itemsMap.get(id);
                next.li.classList.add('is-active');
                next.link.setAttribute('aria-current', 'true');
                activeId = id;
        };

        const updateActive = () => {
                const offset = 140;
                let candidate = headingItems[0];

                for (const item of headingItems) {
                        const rect = item.heading.getBoundingClientRect();
                        if (rect.top <= offset) {
                                candidate = item;
                        } else {
                                break;
                        }
                }

                if (candidate?.id) {
                        setActive(candidate.id);
                }
        };

        let ticking = false;

        const requestUpdate = () => {
                if (ticking) {
                        return;
                }
                ticking = true;
                window.requestAnimationFrame(() => {
                        updateActive();
                        ticking = false;
                });
        };

        const onScroll = () => {
                requestUpdate();
        };

        const onResize = () => {
                requestUpdate();
        };

        const onHashChange = () => {
                const hash = decodeURIComponent(window.location.hash || '').replace(/^#/, '');
                if (hash && itemsMap.has(hash)) {
                        setActive(hash);
                }
        };

        const onClick = event => {
                const target = event.target instanceof Element ? event.target.closest('.toc-link') : null;
                if (!target) {
                        return;
                }
                const href = target.getAttribute('href') || '';
                if (!href.startsWith('#')) {
                        return;
                }
                const id = decodeURIComponent(href.slice(1));
                if (itemsMap.has(id)) {
                        window.requestAnimationFrame(() => setActive(id));
                }
        };

        window.addEventListener('scroll', onScroll, { passive: true });
        window.addEventListener('resize', onResize);
        window.addEventListener('hashchange', onHashChange);
        list.addEventListener('click', onClick);

        if (window.location.hash) {
                onHashChange();
        } else {
                updateActive();
        }

        window.addEventListener('load', updateActive, { once: true });

        postTocCleanup = () => {
                window.removeEventListener('scroll', onScroll);
                window.removeEventListener('resize', onResize);
                window.removeEventListener('hashchange', onHashChange);
                list.removeEventListener('click', onClick);
        };
}

function initMilkdownViewer() {
        const markdownNode = document.getElementById('post-markdown-data');
        const mount = document.querySelector('[data-milkdown-viewer]');
        if (!markdownNode || !mount) {
                initPostToc();
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
                                initPostToc();
                                return;
                        }
                        mount.classList.remove('hidden');
                        const fallback = document.querySelector('[data-post-fallback]');
                        if (fallback) {
                                fallback.classList.add('hidden');
                        }
                        initPostToc();
                })
                .catch(error => {
                        console.error('[milkdown] 加载阅读器失败', error);
                        initPostToc();
                });
}

function bootPublic() {
        // 构建只读渲染与目录浮框
        initMilkdownViewer();
        initPostToc();
}

if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', bootPublic, { once: true });
} else {
        bootPublic();
}
