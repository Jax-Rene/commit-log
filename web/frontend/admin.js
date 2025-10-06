import Alpine from 'alpinejs';
import htmx from 'htmx.org';
import flatpickr from 'flatpickr';
import { Mandarin } from 'flatpickr/dist/l10n/zh.js';
import Cropper from 'cropperjs';
import hljs from 'highlight.js/lib/common';
import { marked } from 'marked';
import { nord } from '@milkdown/theme-nord';
import { Editor, defaultValueCtx, rootCtx } from '@milkdown/kit/core';
import { commonmark } from '@milkdown/kit/preset/commonmark';
import { gfm } from '@milkdown/kit/preset/gfm';
import { listener, listenerCtx } from '@milkdown/kit/plugin/listener';
import { history } from '@milkdown/kit/plugin/history';
import { clipboard } from '@milkdown/kit/plugin/clipboard';
import { indent } from '@milkdown/kit/plugin/indent';
import { block } from '@milkdown/kit/plugin/block';
import { trailing } from '@milkdown/kit/plugin/trailing';
import { cursor } from '@milkdown/kit/plugin/cursor';
import { upload, uploadConfig } from '@milkdown/kit/plugin/upload';
import { getMarkdown, getHTML, replaceAll, insert } from '@milkdown/kit/utils';

import 'flatpickr/dist/flatpickr.min.css';
import 'flatpickr/dist/themes/airbnb.css';
import 'cropperjs/dist/cropper.min.css';
import 'highlight.js/styles/github.css';
import '@milkdown/kit/prose/view/style/prosemirror.css';
import '@milkdown/kit/prose/tables/style/tables.css';
import '@milkdown/kit/prose/gapcursor/style/gapcursor.css';
import '../static/css/editor.css';
import '../static/css/input.css';

globalThis.htmx = htmx;
globalThis.Alpine = Alpine;
globalThis.flatpickr = flatpickr;
globalThis.Cropper = Cropper;
globalThis.hljs = hljs;

flatpickr.localize(Mandarin);

marked.setOptions({ gfm: true, breaks: true });

const toElement = element => {
        if (!element) return null;
        return typeof element === 'string' ? document.querySelector(element) : element;
};

const ensureArray = value => (Array.isArray(value) ? value : value ? [value] : []);

const createMilkdownEditor = async ({
        element,
        initialValue = '',
        onChange,
        uploadImage,
}) => {
        const root = toElement(element);
        if (!root) {
                throw new Error('Milkdown root element not found');
        }

        if (root.__milkdownInstance) {
                return root.__milkdownInstance;
        }

        if (root.__milkdownInstancePromise) {
                return root.__milkdownInstancePromise;
        }

        // 清理旧的 Milkdown DOM，避免重复挂载导致多个编辑实例
        if (root.hasChildNodes()) {
                root.innerHTML = '';
        }

        const creationPromise = (async () => {
                const editor = Editor.make()
                .config(ctx => {
                        ctx.set(rootCtx, root);
                        ctx.set(defaultValueCtx, initialValue || '');

                        const listenerCtxValue = ctx.get(listenerCtx);
                        listenerCtxValue.markdownUpdated((_, markdown) => {
                                if (typeof onChange === 'function') {
                                        onChange(markdown);
                                }
                        });

                        if (typeof uploadImage === 'function') {
                                ctx.update(uploadConfig.key, prev => ({
                                        ...prev,
                                        uploader: async (files, schema) => {
                                                const fileList = Array.from(files || []);
                                                const nodes = [];
                                                for (const file of fileList) {
                                                        try {
                                                                const uploadResult = await uploadImage(file);
                                                                const imageUrl = typeof uploadResult === 'string' ? uploadResult : uploadResult?.url;
                                                                if (!imageUrl) continue;
                                                                const altText = typeof uploadResult === 'object' && uploadResult ? (uploadResult.alt || uploadResult.name || '') : '';
                                                                const node = schema.nodes.image?.createAndFill({ src: imageUrl, alt: altText || file.name || '' });
                                                                if (node) {
                                                                        nodes.push(node);
                                                                }
                                                        } catch (error) {
                                                                console.error('图片上传失败', error);
                                                        }
                                                }
                                                if (!nodes.length) {
                                                        const paragraph = schema.nodes.paragraph?.createAndFill();
                                                        return paragraph || schema.topNodeType.createAndFill();
                                                }
                                                return nodes.length === 1 ? nodes[0] : nodes;
                                        },
                                }));
                        }
                })
                .use(nord)
                .use(commonmark)
                .use(gfm)
                .use(listener)
                .use(history)
                .use(clipboard)
                .use(indent)
                .use(block)
                .use(trailing)
                .use(cursor);

                if (typeof uploadImage === 'function') {
                        editor.use(upload);
                }

                const instance = await editor.create();

                const api = {
                        instance,
                        getMarkdown: () => instance.action(getMarkdown()),
                        getHTML: () => instance.action(getHTML()),
                        setMarkdown: markdown => instance.action(replaceAll(markdown || '', true)),
                        insertMarkdown: markdown => {
                                const chunks = ensureArray(markdown).filter(Boolean);
                                if (!chunks.length) return;
                                chunks.forEach(chunk => {
                                        instance.action(insert(chunk));
                                });
                        },
                        destroy: () => {
                                instance.destroy();
                                root.__milkdownInstance = null;
                                root.__milkdownInstancePromise = null;
                                delete root.dataset.milkdownMounted;
                        },
                };
                root.__milkdownInstance = api;
                root.dataset.milkdownMounted = 'true';
                return api;
        })();

        root.__milkdownInstancePromise = creationPromise
                .then(api => {
                        root.__milkdownInstancePromise = null;
                        return api;
                })
                .catch(error => {
                        root.__milkdownInstancePromise = null;
                        throw error;
                });

        return creationPromise;
};

globalThis.AdminMilkdown = {
        createEditor: createMilkdownEditor,
        renderMarkdown: markdown => marked.parse(markdown || ''),
};

Alpine.start();
