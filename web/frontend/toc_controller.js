const DEFAULTS = {
        cardSelector: '[data-toc-card]',
        listSelector: '[data-toc-list]',
        contentSelector: '[data-toc-content]',
        headingSelector: 'h1, h2, h3',
        ensureContentRoot: host => host,
        showCard: card => {
                card.hidden = false;
                card.classList.remove('hidden');
        },
        hideCard: card => {
                card.hidden = true;
        },
        matchMediaQuery: null,
        scrollContainer: null,
        baseline: () => 140,
        nearEndThreshold: () => Math.max(32, window.innerHeight * 0.05),
        lockTolerance: 240,
        enableScrollLock: false,
        enableHashSync: false,
        handleListClicks: true,
        listenToHashChange: false,
        triggerUpdateOnLoad: false,
        watchContentMutations: false,
        observeHostMutations: false,
        headingFilter: heading => Boolean((heading.textContent || '').trim()),
        itemClass: 'toc-item',
        linkClass: 'toc-link',
        tickClass: 'toc-tick',
        textClass: 'toc-text',
        datasetTarget: 'tocTarget',
        encodeHash: true,
        onStateChange: null,
        enableTestAPI: false,
};

const requestFrame = callback => {
        if (typeof window === 'undefined') {
                callback();
                return 0;
        }
        if (typeof window.requestAnimationFrame === 'function') {
                return window.requestAnimationFrame(callback);
        }
        return window.setTimeout(callback, 16);
};

const cancelFrame = id => {
        if (typeof window === 'undefined') {
                return;
        }
        if (typeof window.cancelAnimationFrame === 'function') {
                window.cancelAnimationFrame(id);
        } else {
                window.clearTimeout(id);
        }
};

function resolveElement(source, selector, context = document) {
        if (source instanceof Element) {
                return source;
        }
        if (typeof source === 'string') {
                return (context || document).querySelector(source);
        }
        if (selector) {
                return (context || document).querySelector(selector);
        }
        return null;
}

function defaultBaselineResolver(options) {
        if (options?.scrollContainer) {
                const rect = options.scrollContainer.getBoundingClientRect();
                return rect.top + 120;
        }
        return 140;
}

function defaultNearEndResolver(options) {
        if (options?.scrollContainer) {
                return Math.max(32, options.scrollContainer.clientHeight * 0.05);
        }
        if (typeof window !== 'undefined') {
                return Math.max(32, window.innerHeight * 0.05);
        }
        return 48;
}

export function createTocController(rawOptions = {}) {
        if (typeof document === 'undefined') {
                return null;
        }
        const options = { ...DEFAULTS, ...rawOptions };
        const card = resolveElement(options.card, options.cardSelector);
        const listContext = card || document;
        const list = resolveElement(options.list, options.listSelector, listContext);
        const contentHost = resolveElement(options.contentHost, options.contentSelector);
        if (!card || !list || !contentHost) {
                return null;
        }

        const scrollContainer = options.scrollContainer || null;
        const scrollRoot = scrollContainer || window;
        const slugCounts = new Map();
        const itemsMap = new Map();
        let headingsData = [];
        let activeId = null;
        let mutationObserver = null;
        let hostObserver = null;
        let rebuildFrame = null;
        let updateFrame = null;
        let suppressAutoUpdateUntil = 0;
        let lockedTargetId = null;
        let lockScrollOffset = null;

        const getNow = () => {
                if (typeof performance !== 'undefined' && typeof performance.now === 'function') {
                        return performance.now();
                }
                return Date.now();
        };

        const mq = options.matchMediaQuery && typeof window !== 'undefined' && typeof window.matchMedia === 'function'
                ? window.matchMedia(options.matchMediaQuery)
                : null;
        const isLargeScreen = () => !mq || mq.matches;

        const showCard = () => {
                if (!isLargeScreen()) {
                        hideCard();
                        return;
                }
                options.showCard(card);
        };
        const hideCard = () => {
                options.hideCard(card);
        };
        const ensureContentRoot = () => options.ensureContentRoot(contentHost);
        const getScrollMetrics = () => {
                if (scrollContainer) {
                        return {
                                max: scrollContainer.scrollHeight,
                                current: scrollContainer.scrollTop + scrollContainer.clientHeight,
                        };
                }
                const doc = document.scrollingElement || document.documentElement || document.body;
                const max = doc.scrollHeight;
                const current = (typeof window.scrollY === 'number' ? window.scrollY : doc.scrollTop) + window.innerHeight;
                return { max, current };
        };
        const getScrollOffset = () => {
                if (scrollContainer) {
                        return scrollContainer.scrollTop;
                }
                const doc = document.scrollingElement || document.documentElement || document.body;
                return typeof window.scrollY === 'number' ? window.scrollY : doc.scrollTop;
        };
        const resolveBaseline = () => {
                if (typeof options.baseline === 'function') {
                        return options.baseline({ scrollContainer, card, contentHost });
                }
                if (options.baseline != null) {
                        if (scrollContainer) {
                                const rect = scrollContainer.getBoundingClientRect();
                                return rect.top + Number(options.baseline);
                        }
                        return Number(options.baseline);
                }
                return defaultBaselineResolver({ scrollContainer });
        };
        const resolveNearEndThreshold = () => {
                if (typeof options.nearEndThreshold === 'function') {
                        return options.nearEndThreshold({ scrollContainer });
                }
                if (options.nearEndThreshold != null) {
                                return Number(options.nearEndThreshold);
                }
                return defaultNearEndResolver({ scrollContainer });
        };
        const isNearScrollEnd = () => {
                const { max, current } = getScrollMetrics();
                const threshold = resolveNearEndThreshold();
                if (typeof options.onStateChange === 'function') {
                        options.onStateChange({ type: 'metrics', max, current, threshold });
                }
                return max - current <= threshold;
        };
        const headingFilter = options.headingFilter || DEFAULTS.headingFilter;
        const slugify = text => {
                const base = text
                        .toLowerCase()
                        .trim()
                        .replace(/[\s]+/g, '-')
                        .replace(/[^a-z0-9\-]/g, '');
                const safe = base || 'section';
                const count = slugCounts.get(safe) || 0;
                slugCounts.set(safe, count + 1);
                return count === 0 ? safe : `${safe}-${count}`;
        };
        const ensureId = heading => {
                const existing = (heading.getAttribute('id') || '').trim();
                if (existing) {
                        return existing;
                }
                const text = (heading.textContent || '').trim();
                const id = slugify(text);
                heading.setAttribute('id', id);
                return id;
        };
        const parseLevel = heading => {
                        const raw = Number.parseInt(heading.tagName.replace('H', ''), 10);
                        if (Number.isNaN(raw)) {
                                return 3;
                        }
                        return Math.max(1, Math.min(raw, 3));
                };
        const resetActive = () => {
                if (activeId && itemsMap.has(activeId)) {
                        const prev = itemsMap.get(activeId);
                        prev.li.classList.remove('is-active');
                        prev.link.removeAttribute('aria-current');
                }
                activeId = null;
        };
        const setActive = id => {
                if (activeId === id) {
                        return;
                }
                if (activeId && itemsMap.has(activeId)) {
                        const prev = itemsMap.get(activeId);
                        prev.li.classList.remove('is-active');
                        prev.link.removeAttribute('aria-current');
                }
                if (!id || !itemsMap.has(id)) {
                        activeId = null;
                        return;
                }
                const next = itemsMap.get(id);
                next.li.classList.add('is-active');
                next.link.setAttribute('aria-current', 'true');
                activeId = id;
        };
        const releaseLockIfNeeded = () => {
                if (!options.enableScrollLock || !lockedTargetId) {
                        return;
                }
                if (!itemsMap.has(lockedTargetId)) {
                        lockedTargetId = null;
                        lockScrollOffset = null;
                        return;
                }
                if (lockScrollOffset == null) {
                        lockScrollOffset = getScrollOffset();
                        return;
                }
                const delta = Math.abs(getScrollOffset() - lockScrollOffset);
                if (delta > options.lockTolerance) {
                        lockedTargetId = null;
                        lockScrollOffset = null;
                }
        };
        const updateActive = () => {
                if (!headingsData.length) {
                        resetActive();
                        return;
                }
                const containerRect = scrollContainer ? scrollContainer.getBoundingClientRect() : null;
                const baseline = containerRect ? containerRect.top + 120 : resolveBaseline();
                releaseLockIfNeeded();
                if (options.enableScrollLock && lockedTargetId && itemsMap.has(lockedTargetId)) {
                        setActive(lockedTargetId);
                        return;
                }
                const nearScrollEnd = isNearScrollEnd();
                let before = null;
                let after = null;
                for (const item of headingsData) {
                        const rectTop = item.heading.getBoundingClientRect().top;
                        const diff = rectTop - baseline;
                        if (diff <= 0) {
                                const distance = Math.abs(diff);
                                if (!before || distance <= before.distance) {
                                        before = { item, distance };
                                }
                        } else {
                                after = { item, distance: diff };
                                break;
                        }
                }
                const lastItem = headingsData[headingsData.length - 1];
                const forceLast = nearScrollEnd && (!lockedTargetId || lockedTargetId === lastItem.id);
                let finalCandidate = null;
                if (forceLast) {
                        finalCandidate = lastItem;
                } else if (before && after) {
                        finalCandidate = before.distance <= after.distance ? before.item : after.item;
                } else if (before) {
                        finalCandidate = before.item;
                } else if (after) {
                        finalCandidate = after.item;
                } else {
                        finalCandidate = headingsData[headingsData.length - 1];
                }
                if (typeof options.onStateChange === 'function') {
                        options.onStateChange({
                                type: 'update',
                                nearScrollEnd,
                                lockedId: lockedTargetId,
                                candidate: finalCandidate?.id || null,
                        });
                }
                if (finalCandidate) {
                        setActive(finalCandidate.id);
                        if (typeof options.onStateChange === 'function') {
                                options.onStateChange({ type: 'active', id: finalCandidate.id });
                        }
                }
        };
        const requestUpdate = () => {
                if (updateFrame) {
                        return;
                }
                updateFrame = requestFrame(() => {
                        updateFrame = null;
                        if (suppressAutoUpdateUntil > getNow()) {
                                requestUpdate();
                                return;
                        }
                        updateActive();
                });
        };
        const buildList = root => {
                const headings = Array.from(root.querySelectorAll(options.headingSelector)).filter(headingFilter);
                slugCounts.clear();
                itemsMap.clear();
                headingsData = [];
                list.innerHTML = '';
                if (!headings.length) {
                        hideCard();
                        resetActive();
                        return;
                }
                showCard();
                headings.forEach(heading => {
                        const id = ensureId(heading);
                        const text = (heading.textContent || '').trim();
                        if (!text) {
                                return;
                        }
                        const level = parseLevel(heading);
                        const li = document.createElement('li');
                        li.className = options.itemClass;
                        li.dataset.level = String(level);
                        const link = document.createElement('a');
                        link.className = options.linkClass;
                        link.dataset[options.datasetTarget] = id;
                        const hrefId = options.encodeHash ? encodeURIComponent(id) : id;
                        link.href = `#${hrefId}`;
                        link.title = text;
                        const tick = document.createElement('span');
                        tick.className = options.tickClass;
                        tick.setAttribute('aria-hidden', 'true');
                        const textNode = document.createElement('span');
                        textNode.className = options.textClass;
                        textNode.textContent = text;
                        link.append(tick, textNode);
                        li.appendChild(link);
                        list.appendChild(li);
                        const record = { heading, id, li, link };
                        itemsMap.set(id, record);
                        headingsData.push(record);
                });
                requestUpdate();
        };
        const rebuild = () => {
                if (!isLargeScreen()) {
                        hideCard();
                        return;
                }
                const root = ensureContentRoot();
                if (!root) {
                        hideCard();
                        list.innerHTML = '';
                        headingsData = [];
                        resetActive();
                        return;
                }
                buildList(root);
                if (options.watchContentMutations) {
                        if (mutationObserver) {
                                mutationObserver.disconnect();
                        }
                        mutationObserver = new MutationObserver(requestRebuild);
                        mutationObserver.observe(root, { childList: true, subtree: true, characterData: true });
                }
        };
        const requestRebuild = () => {
                if (rebuildFrame) {
                        return;
                }
                rebuildFrame = requestFrame(() => {
                        rebuildFrame = null;
                        rebuild();
                });
        };
        const onScroll = () => {
                if (typeof options.onStateChange === 'function') {
                        options.onStateChange({ type: 'scroll-event' });
                }
                requestUpdate();
        };
        const onResize = () => requestRebuild();
        const onMediaChange = () => requestRebuild();
        const onHashChange = () => {
                if (!options.listenToHashChange) {
                        return;
                }
                const hash = decodeURIComponent(window.location.hash || '').replace(/^#/, '');
                if (hash && itemsMap.has(hash)) {
                        setActive(hash);
                }
        };
        const onClick = event => {
                if (!options.handleListClicks) {
                        return;
                }
                const link = event.target instanceof Element ? event.target.closest(`.${options.linkClass}`) : null;
                if (!link) {
                        return;
                }
                const targetId = link.dataset[options.datasetTarget] || '';
                if (!targetId || !itemsMap.has(targetId)) {
                        return;
                }
                if (options.enableScrollLock) {
                        event.preventDefault();
                }
                const entry = itemsMap.get(targetId);
                if (!entry) {
                        return;
                }
                const heading = entry.heading;
                if (options.enableScrollLock) {
                        lockedTargetId = targetId;
                        lockScrollOffset = getScrollOffset();
                        try {
                                heading.scrollIntoView({ behavior: 'smooth', block: 'start', inline: 'nearest' });
                        } catch (error) {
                                const targetTop = getScrollOffset() + heading.getBoundingClientRect().top - 120;
                                window.scrollTo({ top: Math.max(0, targetTop), behavior: 'smooth' });
                        }
                        suppressAutoUpdateUntil = getNow() + 360;
                        requestFrame(() => {
                                setActive(targetId);
                                window.setTimeout(() => {
                                        suppressAutoUpdateUntil = 0;
                                        requestUpdate();
                                }, 420);
                        });
                } else {
                        requestFrame(() => setActive(targetId));
                }
                if (options.enableHashSync) {
                        if (typeof window?.history?.replaceState === 'function') {
                                const url = new URL(window.location.href);
                                url.hash = targetId;
                                window.history.replaceState(null, '', url.toString());
                        } else {
                                window.location.hash = targetId;
                        }
                }
        };

        if (options.handleListClicks) {
                list.addEventListener('click', onClick);
        }
        if (scrollRoot && typeof scrollRoot.addEventListener === 'function') {
                scrollRoot.addEventListener('scroll', onScroll, { passive: true });
        }
        if (scrollContainer && scrollRoot !== window) {
                window.addEventListener('scroll', onScroll, { passive: true });
        }
        window.addEventListener('resize', onResize);
        if (mq) {
                if (typeof mq.addEventListener === 'function') {
                        mq.addEventListener('change', onMediaChange);
                } else if (typeof mq.addListener === 'function') {
                        mq.addListener(onMediaChange);
                }
        }
        if (options.listenToHashChange) {
                        window.addEventListener('hashchange', onHashChange);
        }
        if (options.observeHostMutations) {
                hostObserver = new MutationObserver(requestRebuild);
                hostObserver.observe(contentHost, { childList: true, subtree: false });
        }
        if (options.triggerUpdateOnLoad) {
                window.addEventListener('load', () => requestUpdate(), { once: true });
        }

        requestRebuild();

        const controller = {
                refresh: () => requestRebuild(),
                destroy: () => {
                        if (options.handleListClicks) {
                                list.removeEventListener('click', onClick);
                        }
                        if (scrollRoot && typeof scrollRoot.removeEventListener === 'function') {
                                scrollRoot.removeEventListener('scroll', onScroll);
                        }
                        if (scrollContainer && scrollRoot !== window) {
                                window.removeEventListener('scroll', onScroll);
                        }
                        window.removeEventListener('resize', onResize);
                        if (mq) {
                                if (typeof mq.removeEventListener === 'function') {
                                        mq.removeEventListener('change', onMediaChange);
                                } else if (typeof mq.removeListener === 'function') {
                                        mq.removeListener(onMediaChange);
                                }
                        }
                        if (options.listenToHashChange) {
                                window.removeEventListener('hashchange', onHashChange);
                        }
                        if (hostObserver) {
                                hostObserver.disconnect();
                        }
                        if (mutationObserver) {
                                mutationObserver.disconnect();
                        }
                        if (rebuildFrame) {
                                cancelFrame(rebuildFrame);
                        }
                        if (updateFrame) {
                                cancelFrame(updateFrame);
                        }
                        hideCard();
                        list.innerHTML = '';
                        headingsData = [];
                        itemsMap.clear();
                        lockedTargetId = null;
                        lockScrollOffset = null;
                        resetActive();
                },
        };

        if (options.enableTestAPI) {
                controller.__test = {
                        runUpdate: () => {
                                suppressAutoUpdateUntil = 0;
                                updateActive();
                        },
                        getState: () => ({
                                activeId,
                                lockedId: lockedTargetId,
                                headings: headingsData.map(item => item.id),
                        }),
                };
        }

        return controller;
}

function resolveScrollableContainer(selector) {
        if (typeof document === 'undefined') {
                return null;
        }
        const candidate = selector ? document.querySelector(selector) : null;
        if (!candidate) {
                return null;
        }
        const style = window.getComputedStyle(candidate);
        const overflowY = (style?.overflowY || candidate.style?.overflowY || '').toString();
        const overflow = (style?.overflow || candidate.style?.overflow || '').toString();
        const mayScroll = /auto|scroll/i.test(`${overflowY} ${overflow}`);
        const hasScrollableContent = candidate.scrollHeight - candidate.clientHeight > 1;
        if (!mayScroll || !hasScrollableContent) {
                return null;
        }
        return candidate;
}

export function createEditorTocController(overrides = {}) {
        const scrollContainer = overrides.scrollContainer ?? resolveScrollableContainer('[data-editor-scroll]');
        return createTocController({
                cardSelector: '[data-editor-toc-card]',
                listSelector: '[data-toc-list]',
                contentSelector: '[data-editor-toc-content]',
                ensureContentRoot: host => host.querySelector('.ProseMirror') || host,
                showCard: card => card.classList.remove('hidden'),
                hideCard: card => card.classList.add('hidden'),
                matchMediaQuery: '(min-width: 1280px)',
                scrollContainer,
                baseline: () => {
                        if (scrollContainer) {
                                const rect = scrollContainer.getBoundingClientRect();
                                return rect.top + 120;
                        }
                        return 120;
                },
                nearEndThreshold: () => {
                        if (scrollContainer) {
                                return Math.max(32, scrollContainer.clientHeight * 0.05);
                        }
                        return Math.max(32, window.innerHeight * 0.05);
                },
                enableScrollLock: true,
                enableHashSync: true,
                handleListClicks: true,
                listenToHashChange: false,
                triggerUpdateOnLoad: false,
                watchContentMutations: true,
                observeHostMutations: true,
                ...overrides,
        });
}

export function createViewerTocController(overrides = {}) {
        return createTocController({
                cardSelector: '[data-toc-card]',
                listSelector: '[data-toc-list]',
                contentSelector: '[data-toc-content]',
                showCard: card => {
                        card.hidden = false;
                },
                hideCard: card => {
                        card.hidden = true;
                },
                baseline: () => 140,
                nearEndThreshold: () => Math.max(32, window.innerHeight * 0.08),
                enableScrollLock: false,
                enableHashSync: true,
                handleListClicks: true,
                listenToHashChange: true,
                triggerUpdateOnLoad: true,
                watchContentMutations: false,
                observeHostMutations: false,
                ...overrides,
        });
}
