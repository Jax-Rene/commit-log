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
			window.MilkdownV2 = {
				crepe,
				editor: crepe.editor,
				getMarkdown: crepe.getMarkdown,
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
