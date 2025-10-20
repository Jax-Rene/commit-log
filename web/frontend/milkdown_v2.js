import { Crepe } from '@milkdown/crepe';
import commonStyleUrl from '@milkdown/crepe/theme/common/style.css?url';
import nordStyleUrl from '@milkdown/crepe/theme/nord.css?url';

const styleUrls = [commonStyleUrl, nordStyleUrl];

function ensureStyles() {
	styleUrls.forEach((href, index) => {
		if (!href) {
			return;
		}
		const id = `milkdown-crepe-style-${index}`;
		if (document.getElementById(id)) {
			return;
		}
		const link = document.createElement('link');
		link.id = id;
		link.rel = 'stylesheet';
		link.href = href;
		document.head.appendChild(link);
	});
}

function getInitialMarkdown() {
	if (typeof window !== 'undefined' && window.__MILKDOWN_V2__) {
		const { content } = window.__MILKDOWN_V2__;
		if (typeof content === 'string') {
			return content.trim().length > 0 ? content : '# ';
		}
	}
	return '# ';
}

const DEFAULT_CHANGE_POLL_INTERVAL = 1000;
const DEFAULT_AUTOSAVE_INTERVAL = 30000;

function createToast() {
        if (window.AdminUI && typeof window.AdminUI.toast === 'function') {
                return window.AdminUI.toast;
        }
        return ({ type, message }) => {
                const prefix = type ? `[${type}]` : '[info]';
                console.log(prefix, message);
        };
}

function coerceNumber(value, fallback = 0) {
        const parsed = typeof value === 'string' ? Number(value) : value;
        return Number.isFinite(parsed) ? parsed : fallback;
}

function coerceString(value, fallback = '') {
        if (typeof value === 'string') {
                return value;
        }
        if (typeof value === 'number') {
                return String(value);
        }
        return fallback;
}

function pickProperty(source, candidates, fallback) {
        if (!source || typeof source !== 'object') {
                return fallback;
        }
        for (const key of candidates) {
                if (Object.prototype.hasOwnProperty.call(source, key)) {
                        const value = source[key];
                        if (value !== undefined && value !== null) {
                                return value;
                        }
                }
        }
        return fallback;
}

class PostDraftController {
        constructor(crepe, initialData = {}) {
                this.crepe = crepe;
                this.editor = crepe?.editor ?? null;
                this.getMarkdown = typeof crepe?.getMarkdown === 'function' ? () => crepe.getMarkdown() : () => '';
                this.toast = createToast();
                this.initialData = initialData || {};
                this.postData = this.normalizePost(initialData.post);
                this.latestPublication = initialData.latestPublication || null;
                this.postId = this.resolvePostId(this.postData);
                this.loading = false;
                this.autoSaving = false;
                this.autoSavePending = false;
                this.autoSaveError = '';
                this.autoSaveRevision = 0;
                this.lastAutoSavedAt = null;
                this.autoSaveIntervalId = null;
                this.changeMonitorId = null;
                this.visibilityHandler = null;
                this.unloadHandler = null;
                this.boundKeyHandler = this.handleKeydown.bind(this);
                this.pendingAutoSaveFlush = false;

                const initialContent = this.safeGetInitialContent();
                this.currentContent = initialContent;
                this.lastSavedContent = initialContent;
        }

        normalizePost(post) {
                const source = post && typeof post === 'object' ? { ...post } : {};
                if (source.Title === undefined && typeof source.title === 'string') {
                        source.Title = source.title;
                }
                if (source.Summary === undefined && typeof source.summary === 'string') {
                        source.Summary = source.summary;
                }
                if (source.Content === undefined && typeof source.content === 'string') {
                        source.Content = source.content;
                }
                if (source.CoverURL === undefined && typeof source.cover_url === 'string') {
                        source.CoverURL = source.cover_url;
                }
                if (source.CoverWidth === undefined && source.cover_width !== undefined) {
                        source.CoverWidth = source.cover_width;
                }
                if (source.CoverHeight === undefined && source.cover_height !== undefined) {
                        source.CoverHeight = source.cover_height;
                }
                if (!Array.isArray(source.Tags) && Array.isArray(source.tags)) {
                        source.Tags = source.tags;
                }
                if (source.Status === undefined && typeof source.status === 'string') {
                        source.Status = source.status;
                }
                if (source.UserID === undefined && source.user_id !== undefined) {
                        source.UserID = source.user_id;
                }

                return {
                        Title: '',
                        Summary: '',
                        Content: '',
                        Status: 'draft',
                        Tags: [],
                        CoverURL: '',
                        CoverWidth: 0,
                        CoverHeight: 0,
                        ...source,
                };
        }

        resolvePostId(post) {
                if (!post || typeof post !== 'object') {
                        return 'new';
                }
                const id = pickProperty(post, ['ID', 'id', 'Id'], null);
                if (id === null || id === undefined) {
                        return 'new';
                }
                const stringified = coerceString(id, '').trim();
                return stringified ? stringified : 'new';
        }

        safeGetInitialContent() {
                const fromInitial = typeof this.initialData.content === 'string' ? this.initialData.content : '';
                if (fromInitial) {
                        return fromInitial;
                }
                const fromPost = pickProperty(this.postData, ['Content', 'content'], '');
                return typeof fromPost === 'string' ? fromPost : '';
        }

        safeGetMarkdown() {
                try {
                        const value = this.getMarkdown();
                        return typeof value === 'string' ? value : '';
                } catch (error) {
                        console.error('[milkdown] 获取 Markdown 内容失败', error);
                        return '';
                }
        }

        init() {
                this.startChangeMonitor();
                this.bindKeyboardShortcuts();
                this.initAutoSaveLifecycle();
        }

        dispose() {
                this.stopChangeMonitor();
                this.stopAutoSaveTimer();
                if (this.visibilityHandler) {
                        document.removeEventListener('visibilitychange', this.visibilityHandler);
                        this.visibilityHandler = null;
                }
                if (this.unloadHandler) {
                        window.removeEventListener('pagehide', this.unloadHandler);
                        window.removeEventListener('beforeunload', this.unloadHandler);
                        this.unloadHandler = null;
                }
                if (this.boundKeyHandler) {
                        window.removeEventListener('keydown', this.boundKeyHandler);
                }
        }

        startChangeMonitor() {
                this.stopChangeMonitor();
                this.changeMonitorId = window.setInterval(() => {
                        const markdown = this.safeGetMarkdown();
                        if (markdown === this.currentContent) {
                                return;
                        }
                        const isInitialChange = this.currentContent === this.lastSavedContent && markdown === this.lastSavedContent;
                        this.currentContent = markdown;
                        if (isInitialChange || this.currentContent === this.lastSavedContent) {
                                this.autoSavePending = false;
                                return;
                        }
                        this.markDirty();
                }, DEFAULT_CHANGE_POLL_INTERVAL);
        }

        stopChangeMonitor() {
                if (this.changeMonitorId) {
                        window.clearInterval(this.changeMonitorId);
                        this.changeMonitorId = null;
                }
        }

        bindKeyboardShortcuts() {
                if (this.boundKeyHandler) {
                        window.addEventListener('keydown', this.boundKeyHandler);
                }
        }

        handleKeydown(event) {
                if (!event || !(event.ctrlKey || event.metaKey)) {
                        return;
                }
                const key = typeof event.key === 'string' ? event.key.toLowerCase() : '';
                if (key !== 's') {
                        return;
                }
                event.preventDefault();
                if (this.loading || this.autoSaving) {
                        return;
                }
                void this.saveDraft({
                        redirectOnCreate: false,
                        silent: false,
                        notifyOnSilent: true,
                        useLoadingState: false,
                });
        }

        initAutoSaveLifecycle() {
                this.startAutoSaveTimer();
                if (!this.visibilityHandler) {
                        this.visibilityHandler = () => {
                                if (document.visibilityState === 'hidden') {
                                        void this.autoSaveIfNeeded();
                                }
                        };
                        document.addEventListener('visibilitychange', this.visibilityHandler);
                }
                if (!this.unloadHandler) {
                        this.unloadHandler = () => {
                                if (this.pendingAutoSaveFlush) {
                                        return;
                                }
                                this.pendingAutoSaveFlush = true;
                                void this.autoSaveIfNeeded().finally(() => {
                                        this.pendingAutoSaveFlush = false;
                                });
                        };
                        window.addEventListener('pagehide', this.unloadHandler);
                        window.addEventListener('beforeunload', this.unloadHandler);
                }
        }

        startAutoSaveTimer() {
                this.stopAutoSaveTimer();
                this.autoSaveIntervalId = window.setInterval(() => {
                        void this.autoSaveIfNeeded();
                }, DEFAULT_AUTOSAVE_INTERVAL);
        }

        stopAutoSaveTimer() {
                if (this.autoSaveIntervalId) {
                        window.clearInterval(this.autoSaveIntervalId);
                        this.autoSaveIntervalId = null;
                }
        }

        markDirty() {
                this.autoSaveRevision += 1;
                this.autoSavePending = true;
                this.autoSaveError = '';
        }

        buildPayload(content) {
                const post = this.postData || {};
                const title = coerceString(pickProperty(post, ['Title', 'title'], ''), '');
                const summary = coerceString(pickProperty(post, ['Summary', 'summary'], ''), '');
                const coverUrl = coerceString(pickProperty(post, ['CoverURL', 'cover_url'], ''), '');
                const coverWidth = coerceNumber(pickProperty(post, ['CoverWidth', 'cover_width'], 0), 0);
                const coverHeight = coerceNumber(pickProperty(post, ['CoverHeight', 'cover_height'], 0), 0);
                const tags = Array.isArray(post.Tags) ? post.Tags : Array.isArray(post.tags) ? post.tags : [];
                const tagIds = tags
                        .map(tag => {
                                if (!tag || typeof tag !== 'object') {
                                        return null;
                                }
                                const identifier = pickProperty(tag, ['ID', 'id', 'Id'], null);
                                if (identifier === null || identifier === undefined) {
                                        return null;
                                }
                                const numeric = Number(identifier);
                                return Number.isFinite(numeric) ? numeric : null;
                        })
                        .filter(id => id !== null);

                return {
                        title,
                        summary,
                        content,
                        tag_ids: tagIds,
                        cover_url: coverUrl,
                        cover_width: coverWidth,
                        cover_height: coverHeight,
                };
        }

        updatePostData(nextPost) {
                if (!nextPost || typeof nextPost !== 'object') {
                        return;
                }
                const merged = { ...this.postData, ...nextPost };
                this.postData = this.normalizePost(merged);
                const resolvedId = this.resolvePostId(this.postData);
                if (resolvedId !== 'new') {
                        this.postId = resolvedId;
                }
        }

        async saveDraft(options = {}) {
                const {
                        redirectOnCreate = false,
                        silent = false,
                        notifyOnSilent = false,
                        useLoadingState = false,
                        contentOverride = null,
                } = options;

                if (this.loading && useLoadingState) {
                        return false;
                }

                const shouldToggleLoading = useLoadingState;
                if (shouldToggleLoading) {
                        this.loading = true;
                }

                const content = typeof contentOverride === 'string' ? contentOverride : this.safeGetMarkdown();
                this.currentContent = content;
                this.postData.Content = content;

                const hasMeaningfulContent = [
                        pickProperty(this.postData, ['Title', 'title'], ''),
                        pickProperty(this.postData, ['Summary', 'summary'], ''),
                        content,
                ].some(value => typeof value === 'string' && value.trim().length > 0);

                if (!hasMeaningfulContent && this.postId === 'new') {
                        if (shouldToggleLoading) {
                                this.loading = false;
                        }
                        return false;
                }

                const revisionAtStart = this.autoSaveRevision;
                const payload = this.buildPayload(content);

                let url = '/admin/api/posts';
                let method = 'POST';
                if (this.postId !== 'new') {
                        url = `/admin/api/posts/${this.postId}`;
                        method = 'PUT';
                }

                try {
                        const response = await fetch(url, {
                                method,
                                headers: { 'Content-Type': 'application/json' },
                                body: JSON.stringify(payload),
                        });
                        const data = await response.json();
                        if (!response.ok) {
                                const message = data.error || data.message || '保存失败，请稍后重试';
                                if (silent && !notifyOnSilent) {
                                        this.autoSaveError = message;
                                } else {
                                        this.toast({ message, type: 'error' });
                                }
                                return false;
                        }

                        if (data.post) {
                                this.updatePostData(data.post);
                        }

                        const shouldNotify = !silent || notifyOnSilent;
                        if (shouldNotify && Array.isArray(data.notices)) {
                                data.notices.forEach(message => {
                                        this.toast({ message, type: 'info' });
                                });
                        }
                        if (shouldNotify && Array.isArray(data.warnings)) {
                                data.warnings.forEach(message => {
                                        this.toast({ message, type: 'warning' });
                                });
                        }

                        if (!silent && data.message) {
                                this.toast({ message: data.message, type: 'success' });
                        }

                        if (this.autoSaveRevision === revisionAtStart) {
                                this.autoSavePending = false;
                        }
                        this.lastSavedContent = content;
                        this.lastAutoSavedAt = new Date();
                        this.autoSaveError = '';

                        if (this.postId !== 'new' && window.location && !window.location.pathname.includes('/edit')) {
                                const target = `/admin/posts/${this.postId}/edit`;
                                if (redirectOnCreate) {
                                        window.location.href = target;
                                } else {
                                        window.history.replaceState({}, '', target);
                                }
                        }

                        return true;
                } catch (error) {
                        const message = error?.message || '保存失败，请稍后重试';
                        if (silent && !notifyOnSilent) {
                                this.autoSaveError = message;
                        } else {
                                this.toast({ message, type: 'error' });
                        }
                        return false;
                } finally {
                        if (shouldToggleLoading) {
                                this.loading = false;
                        }
                }
        }

        async autoSaveIfNeeded() {
                if (this.autoSaving || this.loading) {
                        return false;
                }
                if (!this.autoSavePending) {
                        return false;
                }

                const content = this.safeGetMarkdown();
                this.currentContent = content;
                this.postData.Content = content;

                this.autoSaving = true;
                try {
                        const saved = await this.saveDraft({
                                redirectOnCreate: false,
                                silent: true,
                                notifyOnSilent: false,
                                useLoadingState: false,
                                contentOverride: content,
                        });
                        return saved;
                } finally {
                        this.autoSaving = false;
                }
        }
}

async function initialize() {
        const mount = document.getElementById('milkdown-app');
        if (!mount) {
                console.warn('[milkdown] 未找到挂载节点 #milkdown-app');
                return;
        }

	ensureStyles();

	const initial = getInitialMarkdown();

	try {
                const crepe = new Crepe({
                        root: mount,
                        defaultValue: initial,
                });

                await crepe.create();

                if (typeof window !== 'undefined') {
                        const initialData = typeof window.__MILKDOWN_V2__ === 'object' ? window.__MILKDOWN_V2__ : {};
                        const controller = new PostDraftController(crepe, initialData);
                        controller.init();

                        window.MilkdownV2 = {
                                crepe,
                                editor: crepe.editor,
                                getMarkdown: crepe.getMarkdown,
                                controller,
                                saveDraft: controller.saveDraft.bind(controller),
                        };
                }
        } catch (error) {
                console.error('[milkdown] 初始化失败', error);
        }
}

if (document.readyState === 'loading') {
	document.addEventListener('DOMContentLoaded', initialize, { once: true });
} else {
	initialize();
}
