export function createMasonryGridController({
        containerSelector = '#post-grid',
        cardSelector = '[data-post-card]',
} = {}) {
        const DEFAULT_ROW_HEIGHT = 8;
        let rafId = null;
        let resizeTimer = null;

        const resolveContainer = () => document.querySelector(containerSelector);

        const resolveSizing = container => {
                const styles = window.getComputedStyle(container);
                const parsedRowHeight = Number.parseFloat(styles.gridAutoRows);
                const rowHeightIsValid = Number.isFinite(parsedRowHeight) && parsedRowHeight > 0;
                const rowHeight = rowHeightIsValid ? parsedRowHeight : DEFAULT_ROW_HEIGHT;
                if (!rowHeightIsValid) {
                        // 防止未设置网格行高时内容重叠，兜底指定一个像素行高
                        container.style.gridAutoRows = `${rowHeight}px`;
                }
                const parsedRowGap = Number.parseFloat(styles.rowGap);
                const rowGap = Number.isFinite(parsedRowGap) && parsedRowGap >= 0 ? parsedRowGap : 0;
                return { rowHeight, rowGap };
        };

        const measure = () => {
                const container = resolveContainer();
                if (!container) return;

                const cards = Array.from(container.querySelectorAll(cardSelector));
                if (!cards.length) return;

                const { rowHeight, rowGap } = resolveSizing(container);

                cards.forEach(card => {
                        const rect = card.getBoundingClientRect?.() || {};
                        const height = typeof rect.height === 'number' ? rect.height : 0;
                        const span = Math.max(1, Math.ceil((height + rowGap) / (rowHeight + rowGap)));
                        card.style.gridRowEnd = `span ${span}`;
                        card.style.gridColumn = '';

                        card.querySelectorAll('img').forEach(img => {
                                if (img.dataset.masonryBound) return;
                                img.dataset.masonryBound = 'true';
                                if (img.complete) return;
                                img.addEventListener('load', schedule);
                                img.addEventListener('error', schedule);
                        });
                });
        };

        const schedule = () => {
                if (rafId) {
                        window.cancelAnimationFrame(rafId);
                }
                rafId = window.requestAnimationFrame(measure);
        };

        const onResize = () => {
                if (resizeTimer) {
                        clearTimeout(resizeTimer);
                }
                resizeTimer = setTimeout(schedule, 120);
        };

        const onAfterSwap = () => {
                schedule();
        };

        schedule();
        window.addEventListener('resize', onResize);
        document.addEventListener('htmx:afterSwap', onAfterSwap);

        return {
                refresh: schedule,
                destroy() {
                        if (rafId) {
                                cancelAnimationFrame(rafId);
                                rafId = null;
                        }
                        if (resizeTimer) {
                                clearTimeout(resizeTimer);
                                resizeTimer = null;
                        }
                        window.removeEventListener('resize', onResize);
                        document.removeEventListener('htmx:afterSwap', onAfterSwap);
                },
        };
}
