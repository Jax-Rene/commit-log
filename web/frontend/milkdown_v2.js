import { Crepe } from '@milkdown/crepe';
import { editorViewCtx } from '@milkdown/kit/core';
import { listenerCtx } from '@milkdown/kit/plugin/listener';
import { cursor } from '@milkdown/plugin-cursor';
import { upload, uploadConfig } from '@milkdown/plugin-upload';
import { replaceAll } from '@milkdown/kit/utils';
import commonStyleUrl from '@milkdown/crepe/theme/common/style.css?url';
import nordStyleUrl from '@milkdown/crepe/theme/nord.css?url';

const styleUrls = [commonStyleUrl, nordStyleUrl];

const INLINE_AI_CHAT_ICON = `
  <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24">
    <path fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" d="M12 3l1.9 4.6L18.8 9l-3.4 3 1 4.9L12 14.8 7.6 17.9l1-4.9L5.2 9l4.9-.4L12 3z" />
  </svg>
`;

let inlineAIToolbarHandler = null;

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

function normalizePreviewFeatures(features) {
        const featureFlags = typeof features === 'object' && features !== null ? { ...features } : {};
        const Feature = Crepe.Feature || {};
        const disabledDefaults = {};
        if (Feature.BlockEdit) {
                disabledDefaults[Feature.BlockEdit] = false;
        }
        if (Feature.Toolbar) {
                disabledDefaults[Feature.Toolbar] = false;
        }
        if (Feature.Placeholder) {
                disabledDefaults[Feature.Placeholder] = false;
        }
        return { ...disabledDefaults, ...featureFlags };
}

function applyMarkdownToInstance(instance, markdown, { flush = false, silent = false } = {}) {
        if (!instance || typeof instance.editor?.action !== 'function') {
                return false;
        }
        const normalized = typeof markdown === 'string' ? markdown : '';
        try {
                instance.editor.action(replaceAll(normalized, flush));
                return true;
        } catch (error) {
                if (!silent) {
                        console.warn('[milkdown] Markdown 更新失败', error);
                }
                return false;
        }
}

function ensureSetMarkdown(instance, options = {}) {
        const { flush = false, silent = false } = options;
        if (!instance || typeof instance.editor?.action !== 'function') {
                return instance;
        }
        const existing = instance.setMarkdown;
        if (typeof existing === 'function' && existing.__commitLogPatched__) {
                return instance;
        }
        const setter = async (markdown, override = {}) => {
                const shouldFlush = typeof override.flush === 'boolean' ? override.flush : flush;
                const shouldSilence = typeof override.silent === 'boolean' ? override.silent : silent;
                return applyMarkdownToInstance(instance, markdown, {
                        flush: shouldFlush,
                        silent: shouldSilence,
                });
        };
        setter.__commitLogPatched__ = true;
        Object.defineProperty(instance, 'setMarkdown', {
                configurable: true,
                enumerable: false,
                writable: true,
                value: setter,
        });
        return instance;
}

async function createReadOnlyViewer({ mount, markdown = '', features, featureConfigs } = {}) {
        if (!mount) {
                throw new Error('[milkdown] 预览需要有效的挂载节点');
        }
        ensureStyles();
        const normalized = typeof markdown === 'string' ? markdown : '';
        const preview = new Crepe({
                root: mount,
                defaultValue: normalized.trim() ? normalized : '# ',
                features: normalizePreviewFeatures(features),
                featureConfigs,
        });
        ensureSetMarkdown(preview, { flush: true, silent: true });
        if (typeof preview.setReadonly === 'function') {
                preview.setReadonly(true);
        }
        mount.innerHTML = '';
        await preview.create();
        if (typeof preview.setMarkdown === 'function') {
                try {
                        const result = preview.setMarkdown(normalized);
                        if (result && typeof result.then === 'function') {
                                await result;
                        }
                } catch (error) {
                        console.warn('[milkdown] 预览内容设置失败', error);
                }
        }
        return preview;
}

if (typeof window !== 'undefined') {
        window.MilkdownV2 = window.MilkdownV2 || {};
        if (typeof window.MilkdownV2.createReadOnlyViewer !== 'function') {
                window.MilkdownV2.createReadOnlyViewer = createReadOnlyViewer;
        }
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

function cloneValue(value) {
        if (typeof structuredClone === 'function') {
                try {
                        return structuredClone(value);
                } catch (error) {
                        console.warn('[milkdown] 复制数据失败，将使用回退方案', error);
                }
        }
        try {
                return JSON.parse(JSON.stringify(value));
        } catch (error) {
                console.warn('[milkdown] JSON 克隆数据失败，将返回原始引用', error);
        }
        return value;
}

function calculateContentMetrics(markdown) {
        const source = coerceString(markdown, '');
        if (!source.trim()) {
                return { words: 0, characters: 0, paragraphs: 0 };
        }

        const withoutCode = source
                .replace(/```[\s\S]*?```/g, ' ')
                .replace(/`[^`]*`/g, ' ');

        const withoutLinks = withoutCode
                .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
                .replace(/\[[^\]]*\]\([^)]*\)/g, '$1 ');

        const normalized = withoutLinks
                .replace(/[#>*_~`\-]+/g, ' ')
                .replace(/\d+\./g, ' ')
                .replace(/&[a-z]+;/gi, ' ')
                .replace(/\r/g, '\n');

        const paragraphCount = normalized
                .split(/\n{2,}/)
                .map(block => block.trim())
                .filter(Boolean).length;

        const collapsed = normalized
                .replace(/\n+/g, ' ')
                .replace(/[\u200B-\u200D\uFEFF]/g, '')
                .replace(/[^\p{Letter}\p{Number}\p{Mark}\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}]+/gu, ' ')
                .trim();

        if (!collapsed) {
                return { words: 0, characters: 0, paragraphs: paragraphCount };
        }

        const segments = collapsed.split(/\s+/).filter(Boolean);
        let words = 0;
        let characters = 0;

        for (const segment of segments) {
                const cjkMatches = segment.match(/[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}]/gu);
                if (cjkMatches && cjkMatches.length > 0) {
                        words += cjkMatches.length;
                        characters += cjkMatches.length;
                }

                const remaining = segment.replace(/[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}]/gu, '');
                const normalizedRemaining = remaining.replace(/\s+/g, '');
                if (normalizedRemaining.length > 0) {
                        words += 1;
                        characters += normalizedRemaining.length;
                } else if (!cjkMatches) {
                        words += 1;
                        characters += segment.length;
                }
        }

        return {
                words,
                characters,
                paragraphs: paragraphCount,
        };
}

function clamp(value, min, max) {
        const numeric = Number(value);
        if (!Number.isFinite(numeric)) {
                return min;
        }
        if (numeric < min) {
                return min;
        }
        if (numeric > max) {
                return max;
        }
        return numeric;
}

function normalizeSelectionContent(value) {
        if (typeof value !== 'string') {
                return '';
        }
        return value.replace(/\r\n/g, '\n').replace(/\u00a0/g, ' ');
}

function readInlineSelection(editor, controller) {
        if (!editor || typeof editor.action !== 'function') {
                return null;
        }
        let info = null;
        editor.action(ctx => {
                const view = ctx.get(editorViewCtx);
                if (!view) {
                        return;
                }
                const { state } = view;
                const selection = state.selection;
                if (!selection || selection.empty) {
                        return;
                }
                const { from, to } = selection;
                if (from === to) {
                        return;
                }
                const raw = state.doc.textBetween(from, to, '\n', '\n');
                const normalized = normalizeSelectionContent(raw);
                if (!normalized.trim()) {
                        return;
                }

                let coords = null;
                try {
                        const start = view.coordsAtPos(from);
                        const end = view.coordsAtPos(Math.max(from, to - 1));
                        const left = Math.min(start.left, end.left);
                        const right = Math.max(start.right, end.right);
                        const top = Math.min(start.top, end.top);
                        const bottom = Math.max(start.bottom, end.bottom);
                        coords = {
                                left,
                                right,
                                top,
                                bottom,
                                width: right - left,
                                height: bottom - top,
                        };
                } catch (error) {
                        coords = null;
                }

                let contextText = normalized;
                try {
                        const range = selection.$from?.blockRange(selection.$to);
                        if (range) {
                                const block = normalizeSelectionContent(
                                        state.doc.textBetween(range.start, range.end, '\n', '\n'),
                                );
                                if (block.trim()) {
                                        contextText = block;
                                }
                        }
                } catch (error) {
                        // ignore block range issues
                }

                let revision = 0;
                if (controller && typeof controller === 'object') {
                        const rawRevision = controller.autoSaveRevision;
                        if (Number.isFinite(rawRevision)) {
                                revision = Number(rawRevision);
                        } else if (typeof rawRevision === 'string') {
                                const parsed = Number(rawRevision);
                                if (Number.isFinite(parsed)) {
                                        revision = parsed;
                                }
                        }
                }

                info = {
                        from,
                        to,
                        text: raw,
                        normalizedText: normalized,
                        context: contextText,
                        coords,
                        revision,
                        docSize: state.doc.content.size,
                };
        });
        return info;
}

function applyInlineAIResult(editor, payload = {}) {
        if (!editor || typeof editor.action !== 'function') {
                throw new Error('编辑器尚未就绪');
        }
        const { from, to, expected, replacement } = payload;
        let applied = false;
        let failure = null;
        editor.action(ctx => {
                const view = ctx.get(editorViewCtx);
                if (!view) {
                        failure = new Error('编辑器尚未就绪');
                        return;
                }
                const { state } = view;
                const docSize = state.doc.content.size;
                const start = clamp(from, 0, docSize);
                const end = clamp(to, 0, docSize);
                if (start === end) {
                        failure = new Error('无法定位原始选区，请重新选择后再试');
                        return;
                }
                const current = normalizeSelectionContent(state.doc.textBetween(start, end, '\n', '\n'));
                const expectedNormalized = normalizeSelectionContent(expected || '');
                if (expectedNormalized.trim() && current.trim() && expectedNormalized.trim() !== current.trim()) {
                        failure = new Error('选中内容已发生变化，请重新选择后再试');
                        return;
                }
                const text = typeof replacement === 'string' ? replacement : '';
                const tr = state.tr.insertText(text, start, end);
                view.dispatch(tr.scrollIntoView());
                applied = true;
        });
        if (failure) {
                throw failure;
        }
        return applied;
}

function setupInlineAI(crepe, toast, getController) {
        const editor = crepe?.editor;
        if (!editor || typeof editor.action !== 'function') {
                inlineAIToolbarHandler = null;
                return null;
        }

        let latestInfo = null;
        const notify =
                typeof toast === 'function'
                        ? toast
                        : ({ type, message }) => {
                                  const prefix = type === 'error' ? '[error]' : '[warn]';
                                  console.warn(`${prefix} ${message}`);
                          };

        const hide = () => {
                if (typeof window === 'undefined' || typeof window.getSelection !== 'function') {
                        return;
                }
                try {
                        const selection = window.getSelection();
                        if (selection && typeof selection.removeAllRanges === 'function') {
                                selection.removeAllRanges();
                        }
                } catch (error) {
                        console.warn('[milkdown] 清理选区失败', error);
                }
        };

        const dispatchInlineAI = () => {
                const controller = typeof getController === 'function' ? getController() : null;
                const info = readInlineSelection(editor, controller);
                latestInfo = info;
                if (!info || !info.normalizedText || !info.normalizedText.trim()) {
                        notify({ type: 'warning', message: '请选择需要改写的段落后再试' });
                        hide();
                        return;
                }
                window.dispatchEvent(new CustomEvent('post-editor:inline-ai', { detail: { selection: info } }));
                hide();
        };

        inlineAIToolbarHandler = () => {
                try {
                        dispatchInlineAI();
                } catch (error) {
                        console.warn('[milkdown] AI Chat 工具触发失败', error);
                }
        };

        const destroy = () => {
                inlineAIToolbarHandler = null;
        };

        return {
                hide,
                destroy,
                getLatestSelection: () => latestInfo,
                readSelection: () => {
                        const controller = typeof getController === 'function' ? getController() : null;
                        const info = readInlineSelection(editor, controller);
                        latestInfo = info;
                        return info;
                },
                applyChange: options => applyInlineAIResult(editor, options),
        };
}

function usePlugins(editor, plugins) {
        if (!editor || !plugins) {
                return;
        }
        plugins.forEach(plugin => {
                if (!plugin) {
                        return;
                }
                if (Array.isArray(plugin)) {
                        usePlugins(editor, plugin);
                        return;
                }
                if (typeof editor.use === 'function') {
                        editor.use(plugin);
                }
        });
}

async function uploadImageViaAPI(file) {
        const formData = new FormData();
        formData.append('image', file);

        let response;
        try {
                response = await fetch('/admin/api/upload/image', {
                        method: 'POST',
                        body: formData,
                });
        } catch (error) {
                const message = error?.message || '网络异常，图片上传失败';
                throw new Error(message);
        }

        let payload = null;
        try {
                payload = await response.json();
        } catch (error) {
                console.error('[milkdown] 图片上传响应解析失败', error);
        }

        if (!response.ok || !payload || payload.success !== 1) {
                const message =
                        payload?.error ||
                        payload?.message ||
                        `图片上传失败（${response?.status ?? '未知状态'}）`;
                throw new Error(message);
        }

        const url = payload?.data?.url || payload?.data?.filePath;
        if (!url) {
                throw new Error('图片上传成功但未返回访问地址');
        }
        return url;
}

function applyMilkdownPlugins(editor, toast) {
        if (!editor || typeof editor.config !== 'function') {
                return;
        }

        editor.config(ctx => {
                ctx.update(uploadConfig.key, prev => {
                        const fallbackUploader =
                                prev && typeof prev.uploader === 'function' ? prev.uploader : null;
                        return {
                                ...prev,
                                enableHtmlFileUploader: true,
                                uploader: async (files, schema) => {
                                        const imageNode = schema?.nodes?.image;
                                        const items = files ? Array.from(files).filter(file => file && file.type && file.type.startsWith('image/')) : [];

                                        if (!items.length || !imageNode) {
                                                return fallbackUploader ? fallbackUploader(files, schema) : [];
                                        }

                                        const createdNodes = [];
                                        for (const file of items) {
                                                try {
                                                        const url = await uploadImageViaAPI(file);
                                                        const node = imageNode.createAndFill({
                                                                src: url,
                                                                alt: file.name || '',
                                                        });
                                                        if (node) {
                                                                createdNodes.push(node);
                                                        }
                                                } catch (error) {
                                                        const message = error?.message || '图片上传失败';
                                                        if (typeof toast === 'function') {
                                                                toast({ type: 'error', message });
                                                        } else {
                                                                console.error('[milkdown] 图片上传失败', error);
                                                        }
                                                }
                                        }

                                        if (!createdNodes.length) {
                                                return fallbackUploader ? fallbackUploader(files, schema) : [];
                                        }

                                        return createdNodes;
                                },
                        };
                });
        });

        usePlugins(editor, [upload, cursor]);
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
                this.eventTarget = typeof window !== 'undefined' ? window : null;
                this.loading = false;
                this.autoSaving = false;
                this.autoSavePending = false;
                this.autoSaveError = '';
                this.autoSaveRevision = 0;
                this.lastAutoSavedAt = null;
                this.publishing = false;
                this.autoSaveIntervalId = null;
                this.changeMonitorId = null;
                this.visibilityHandler = null;
                this.unloadHandler = null;
                this.boundKeyHandler = this.handleKeydown.bind(this);
                this.pendingAutoSaveFlush = false;
                this.contentMetrics = { words: 0, characters: 0, paragraphs: 0 };

                const initialContent = this.safeGetInitialContent();
                this.currentContent = initialContent;
                this.lastSavedContent = initialContent;
                this.updateContentMetrics(initialContent);
        }

        getPostSnapshot() {
                if (!this.postData || typeof this.postData !== 'object') {
                        return {};
                }
                return cloneValue(this.postData) || {};
        }

        emitPostEvent(type, detail = {}) {
                if (!this.eventTarget || typeof this.eventTarget.dispatchEvent !== 'function' || !type) {
                        return;
                }
                const payload = {
                        controller: this,
                        post: this.getPostSnapshot(),
                        ...detail,
                };
                try {
                        this.eventTarget.dispatchEvent(new CustomEvent(`post-editor:${type}`, { detail: payload }));
                } catch (error) {
                        console.warn('[milkdown] 分发事件失败', type, error);
                }
        }

        notifyPostChange(reason = '') {
                const detail = reason ? { reason } : {};
                this.emitPostEvent('post-updated', detail);
        }

        updateContentMetrics(markdown) {
                const metrics = calculateContentMetrics(markdown);
                this.contentMetrics = metrics;
                this.emitPostEvent('metrics', { metrics });
                return metrics;
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
                        this.updateContentMetrics(markdown);
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
                this.emitPostEvent('dirty', { revision: this.autoSaveRevision });
        }

        setTitle(value) {
                const normalized = coerceString(value, '').trim();
                if (this.postData.Title === normalized) {
                        return this.postData.Title;
                }
                this.postData.Title = normalized;
                this.postData.title = normalized;
                this.markDirty();
                this.notifyPostChange('title');
                return normalized;
        }

        setSummary(value) {
                const normalized = coerceString(value, '');
                if (this.postData.Summary === normalized) {
                        return this.postData.Summary;
                }
                this.postData.Summary = normalized;
                this.postData.summary = normalized;
                this.markDirty();
                this.notifyPostChange('summary');
                return normalized;
        }

        setCover(info = {}) {
                const nextUrl = coerceString(pickProperty(info, ['url', 'CoverURL', 'cover_url'], ''), '').trim();
                const nextWidth = coerceNumber(pickProperty(info, ['width', 'CoverWidth', 'cover_width'], 0), 0);
                const nextHeight = coerceNumber(pickProperty(info, ['height', 'CoverHeight', 'cover_height'], 0), 0);

                const sameAsBefore =
                        this.postData.CoverURL === nextUrl &&
                        this.postData.CoverWidth === nextWidth &&
                        this.postData.CoverHeight === nextHeight;
                if (sameAsBefore) {
                        return {
                                url: this.postData.CoverURL,
                                width: this.postData.CoverWidth,
                                height: this.postData.CoverHeight,
                        };
                }

                this.postData.CoverURL = nextUrl;
                this.postData.cover_url = nextUrl;
                this.postData.CoverWidth = nextWidth;
                this.postData.cover_width = nextWidth;
                this.postData.CoverHeight = nextHeight;
                this.postData.cover_height = nextHeight;
                this.markDirty();
                this.notifyPostChange('cover');
                return {
                        url: nextUrl,
                        width: nextWidth,
                        height: nextHeight,
                };
        }

        clearCover() {
                return this.setCover({ url: '', width: 0, height: 0 });
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
                this.notifyPostChange('server');
        }

        setLatestPublication(publication) {
                if (!publication || typeof publication !== 'object') {
                        this.latestPublication = null;
                        return this.latestPublication;
                }
                this.latestPublication = cloneValue(publication) || publication;
                return this.latestPublication;
        }

        getLatestPublication() {
                return cloneValue(this.latestPublication);
        }

        async publish(options = {}) {
                const { silent = false } = options;
                if (this.publishing) {
                        return false;
                }

                const shouldSaveDraft = this.postId === 'new' || this.autoSavePending;
                if (shouldSaveDraft) {
                        const saved = await this.saveDraft({
                                redirectOnCreate: false,
                                silent: true,
                                notifyOnSilent: false,
                                useLoadingState: false,
                        });
                        if (!saved) {
                                if (!silent) {
                                        this.toast({ message: '请先完善内容并保存草稿后再发布', type: 'warning' });
                                }
                                return false;
                        }
                }

                if (this.postId === 'new') {
                        if (!silent) {
                                this.toast({ message: '文章尚未保存，无法发布', type: 'warning' });
                        }
                        return false;
                }

                this.publishing = true;
                try {
                        const response = await fetch(`/admin/api/posts/${this.postId}/publish`, {
                                method: 'POST',
                        });
                        let data = {};
                        try {
                                data = await response.json();
                        } catch (error) {
                                data = {};
                        }
                        if (!response.ok) {
                                const message = data?.error || data?.message || '发布失败，请稍后重试';
                                if (!silent) {
                                        this.toast({ message, type: 'error' });
                                }
                                return false;
                        }

                        if (data?.post) {
                                this.updatePostData(data.post);
                        }
                        if (data?.publication) {
                                this.setLatestPublication(data.publication);
                        }

                        const shouldNotify = !silent;
                        if (shouldNotify && Array.isArray(data?.notices)) {
                                data.notices.forEach(message => {
                                        this.toast({ message, type: 'info' });
                                });
                        }
                        if (shouldNotify && Array.isArray(data?.warnings)) {
                                data.warnings.forEach(message => {
                                        this.toast({ message, type: 'warning' });
                                });
                        }
                        if (shouldNotify && data?.message) {
                                this.toast({ message: data.message, type: 'success' });
                        }

                        this.lastSavedContent = this.currentContent;
                        this.autoSavePending = false;
                        this.autoSaveError = '';
                        this.lastAutoSavedAt = new Date();

                        this.notifyPostChange('publish');
                        if (this.latestPublication) {
                                this.emitPostEvent('publication', { publication: this.getLatestPublication() });
                        }
                        return true;
                } catch (error) {
                        const message = error?.message || '发布失败，请稍后重试';
                        if (!silent) {
                                this.toast({ message, type: 'error' });
                        }
                        return false;
                } finally {
                        this.publishing = false;
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

                        this.notifyPostChange('save');

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
                const toast = createToast();
                inlineAIToolbarHandler = null;

                const toolbarKey = Crepe?.Feature?.Toolbar ?? 'toolbar';
                const featureConfigs = {
                        [toolbarKey]: {
                                buildToolbar(builder) {
                                        try {
                                                if (!builder || typeof builder.getGroup !== 'function') {
                                                        return;
                                                }
                                                const group = builder.getGroup('function');
                                                if (!group || typeof group.addItem !== 'function') {
                                                        return;
                                                }
                                                const items = Array.isArray(group.group?.items) ? group.group.items : [];
                                                if (items.some(item => item && item.key === 'inline-ai-chat')) {
                                                        return;
                                                }
                                                group.addItem('inline-ai-chat', {
                                                        icon: INLINE_AI_CHAT_ICON,
                                                        active: () => false,
                                                        onRun: () => {
                                                                if (typeof inlineAIToolbarHandler === 'function') {
                                                                        inlineAIToolbarHandler();
                                                                } else {
                                                                        console.warn('[milkdown] AI Chat 功能尚未就绪');
                                                                }
                                                        },
                                                });
                                        } catch (error) {
                                                console.warn('[milkdown] 注册 AI Chat 工具失败', error);
                                        }
                                },
                        },
                };

                const crepe = new Crepe({
                        root: mount,
                        defaultValue: initial,
                        featureConfigs,
                });
                applyMilkdownPlugins(crepe.editor, toast);

                await crepe.create();
                ensureSetMarkdown(crepe);

                if (typeof window !== 'undefined') {
                        const initialData = typeof window.__MILKDOWN_V2__ === 'object' ? window.__MILKDOWN_V2__ : {};
                        const controller = new PostDraftController(crepe, initialData);
                        controller.init();

                        const inlineAI = setupInlineAI(crepe, toast, () => controller);

                        crepe.editor.action(ctx => {
                                try {
                                        const listener = ctx.get(listenerCtx);
                                        if (!listener) {
                                                return;
                                        }
                                        listener.markdownUpdated((_ctx, markdown) => {
                                                if (typeof markdown !== 'string') {
                                                        return;
                                                }
                                                if (markdown === controller.currentContent) {
                                                        return;
                                                }
                                                controller.currentContent = markdown;
                                                controller.postData.Content = markdown;
                                                controller.updateContentMetrics(markdown);
                                                if (markdown !== controller.lastSavedContent) {
                                                        controller.markDirty();
                                                }
                                        });
                                        listener.blur(() => {
                                                if (typeof controller.autoSaveIfNeeded === 'function') {
                                                        void controller.autoSaveIfNeeded();
                                                }
                                        });
                                } catch (error) {
                                        console.error('[milkdown] 监听器初始化失败', error);
                                }
                        });

                        window.MilkdownV2 = {
                                crepe,
                                editor: crepe.editor,
                        getMarkdown: crepe.getMarkdown,
                        controller,
                        createReadOnlyViewer,
                        saveDraft: controller.saveDraft.bind(controller),
                        publish: controller.publish.bind(controller),
                        inlineAI: inlineAI
                                ? {
                                        hideToolbar: inlineAI.hide,
                                        getSelection: inlineAI.readSelection,
                                        applyChange: options => inlineAI.applyChange(options),
                                        }
                                        : null,
                        };

                        controller.emitPostEvent('ready');
                        controller.notifyPostChange('init');
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
