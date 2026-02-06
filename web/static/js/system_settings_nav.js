(function (root, factory) {
    if (typeof module === "object" && module.exports) {
        module.exports = factory();
    } else {
        root.SystemSettingsNav = factory();
    }
})(this, function () {
    function normalizeNavIndex(value) {
        if (value === null || value === undefined) {
            return null;
        }
        const parsed = Number(value);
        return Number.isFinite(parsed) ? parsed : null;
    }

    function countNavButtonType(navButtons, type, currentIndex) {
        if (!Array.isArray(navButtons)) {
            return 0;
        }
        const normalizedIndex = normalizeNavIndex(currentIndex);
        return navButtons.filter((item, idx) => {
            if (normalizedIndex !== null && idx === normalizedIndex) {
                return false;
            }
            return item && item.type === type;
        }).length;
    }

    function canUseNavButtonType(navButtons, type, currentIndex) {
        if (type === "custom") {
            return true;
        }
        return countNavButtonType(navButtons, type, currentIndex) === 0;
    }

    return {
        normalizeNavIndex,
        countNavButtonType,
        canUseNavButtonType,
    };
});
