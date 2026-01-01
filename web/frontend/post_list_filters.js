export function initPostListDatePickers({
        root = typeof document === 'undefined' ? null : document,
        flatpickrInstance = typeof globalThis === 'undefined' ? null : globalThis.flatpickr,
        dateOptions = { dateFormat: 'Y-m-d', allowInput: true },
        startInputId = 'start_date',
        endInputId = 'end_date',
        startTriggerSelector = '[data-date-trigger="start"]',
        endTriggerSelector = '[data-date-trigger="end"]',
} = {}) {
        if (!root || !flatpickrInstance) {
                return null;
        }

        const startInput = root.getElementById
                ? root.getElementById(startInputId)
                : root.querySelector?.(`#${startInputId}`) || null;
        const endInput = root.getElementById
                ? root.getElementById(endInputId)
                : root.querySelector?.(`#${endInputId}`) || null;

        if (!startInput && !endInput) {
                return null;
        }

        const startTrigger = root.querySelector
                ? root.querySelector(startTriggerSelector)
                : null;
        const endTrigger = root.querySelector
                ? root.querySelector(endTriggerSelector)
                : null;

        let startInstance;
        let endInstance;

        if (startInput) {
                startInstance = flatpickrInstance(startInput, {
                        ...dateOptions,
                        onChange(selectedDates) {
                                if (selectedDates.length && endInstance) {
                                        endInstance.set('minDate', selectedDates[0]);
                                }
                        },
                });
                startInput.addEventListener('focus', () => startInstance.open());
                startInput.addEventListener('click', () => startInstance.open());
                if (startTrigger) {
                        startTrigger.addEventListener('click', () => startInstance.open());
                }
        }

        if (endInput) {
                endInstance = flatpickrInstance(endInput, {
                        ...dateOptions,
                        onChange(selectedDates) {
                                if (selectedDates.length && startInstance) {
                                        startInstance.set('maxDate', selectedDates[0]);
                                }
                        },
                });
                endInput.addEventListener('focus', () => endInstance.open());
                endInput.addEventListener('click', () => endInstance.open());
                if (endTrigger) {
                        endTrigger.addEventListener('click', () => endInstance.open());
                }
        }

        return { startInstance, endInstance };
}
