import { Editor, rootCtx, defaultValueCtx } from '@milkdown/core';
import { commonmark } from '@milkdown/preset-commonmark';
import { nord } from '@milkdown/theme-nord';
import milkdownTheme from '@milkdown/theme-nord/style.css?raw';

function getInitialMarkdown() {
	if (typeof window !== 'undefined' && window.__MILKDOWN_V2__) {
		const { content } = window.__MILKDOWN_V2__;
		if (typeof content === 'string') {
			return content;
		}
	}
	return '# 欢迎使用 Milkdown 编辑器\n';
}

async function initialize() {
	const mount = document.getElementById('milkdown-app');
	if (!mount) {
		console.warn('[milkdown] 未找到挂载节点 #milkdown-app');
		return;
	}

	if (!document.getElementById('milkdown-theme-style')) {
		const style = document.createElement('style');
		style.id = 'milkdown-theme-style';
		style.textContent = milkdownTheme;
		document.head.appendChild(style);
	}

	const initial = getInitialMarkdown();

	try {
		const instance = await Editor.make()
			.config((ctx) => {
				ctx.set(rootCtx, mount);
				ctx.set(defaultValueCtx, initial);
			})
			.use(nord)
			.use(commonmark)
			.create();

		if (typeof window !== 'undefined') {
			window.MilkdownV2 = { instance };
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
