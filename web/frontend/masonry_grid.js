export function createMasonryGridController({
        containerSelector = '#post-grid',
        cardSelector = '[data-post-card]',
} = {}) {
        let rafId = null;
        let resizeTimer = null;

        const resolveContainer = () => document.querySelector(containerSelector);

        const measure = () => {
                const container = resolveContainer();
                if (!container) return;

                const cards = Array.from(container.querySelectorAll(cardSelector));
                if (!cards.length) return;

                const styles = window.getComputedStyle(container);
                const rowHeight = parseFloat(styles.gridAutoRows) || 8;
                const rowGap = parseFloat(styles.rowGap) || 0;

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
