(function (root, factory) {
  if (typeof module === "object" && module.exports) {
    module.exports = factory();
  } else {
    root.CreationHeatmap = factory();
  }
})(this, function () {
  const dateKeyPattern = /^(\d{4})-(\d{2})-(\d{2})$/;

  function toNumber(value) {
    const num = Number(value);
    return Number.isFinite(num) ? num : 0;
  }

  function parseDateKey(raw) {
    const value = String(raw || "").trim();
    const match = dateKeyPattern.exec(value);
    if (!match) {
      return null;
    }

    const month = Number(match[2]);
    const day = Number(match[3]);
    if (month < 1 || month > 12 || day < 1 || day > 31) {
      return null;
    }

    return value;
  }

  function normalizePoints(points) {
    const entries = Array.isArray(points) ? points : [];
    const normalized = [];

    entries.forEach((entry) => {
      const date = parseDateKey(entry && entry.Date);
      if (!date) {
        return;
      }

      const count = Math.max(0, Math.round(toNumber(entry && entry.Count)));
      const titles = Array.isArray(entry && entry.Titles)
        ? entry.Titles
            .map((title) => String(title || "").trim())
            .filter((title) => title.length > 0)
        : [];

      normalized.push({ date, count, titles });
    });

    normalized.sort((a, b) => a.date.localeCompare(b.date));
    return normalized;
  }

  function computeIntensity(count, maxCount) {
    const safeCount = Math.max(0, Math.round(toNumber(count)));
    const safeMax = Math.max(0, Math.round(toNumber(maxCount)));
    if (safeCount <= 0 || safeMax <= 0) {
      return 0;
    }

    const ratio = safeCount / safeMax;
    if (ratio >= 0.75) {
      return 4;
    }
    if (ratio >= 0.5) {
      return 3;
    }
    if (ratio >= 0.25) {
      return 2;
    }
    return 1;
  }

  function computeStreaks(points) {
    const entries = Array.isArray(points) ? points : [];
    let longest = 0;
    let current = 0;

    entries.forEach((point) => {
      if ((point && point.count) > 0) {
        current += 1;
        if (current > longest) {
          longest = current;
        }
        return;
      }
      current = 0;
    });

    let tail = 0;
    for (let index = entries.length - 1; index >= 0; index -= 1) {
      if ((entries[index] && entries[index].count) > 0) {
        tail += 1;
        continue;
      }
      break;
    }

    return { longestStreak: longest, currentStreak: tail };
  }

  function mondayFirstWeekday(dateKey) {
    const day = new Date(`${dateKey}T00:00:00Z`).getUTCDay();
    return (day + 6) % 7;
  }

  function buildDisplayCells(points, maxCount) {
    const entries = Array.isArray(points) ? points : [];
    if (entries.length === 0) {
      return [];
    }

    const leading = mondayFirstWeekday(entries[0].date);
    const cells = [];

    for (let index = 0; index < leading; index += 1) {
      cells.push({ empty: true });
    }

    entries.forEach((point, index) => {
      cells.push({
        empty: false,
        index,
        date: point.date,
        count: point.count,
        titles: point.titles,
        level: computeIntensity(point.count, maxCount),
      });
    });

    const trailing = (7 - (cells.length % 7)) % 7;
    for (let index = 0; index < trailing; index += 1) {
      cells.push({ empty: true });
    }

    return cells;
  }

  function summarizeHeatmap(points) {
    const normalized = normalizePoints(points);
    const maxCount = normalized.reduce(
      (max, point) => Math.max(max, point.count),
      0,
    );
    const totalCount = normalized.reduce((sum, point) => sum + point.count, 0);
    const activeDays = normalized.filter((point) => point.count > 0).length;
    const streaks = computeStreaks(normalized);
    const displayCells = buildDisplayCells(normalized, maxCount);
    const weeks = displayCells.length > 0 ? Math.ceil(displayCells.length / 7) : 0;

    return {
      points: normalized,
      displayCells,
      weeks,
      maxCount,
      totalCount,
      activeDays,
      longestStreak: streaks.longestStreak,
      currentStreak: streaks.currentStreak,
    };
  }

  return {
    normalizePoints,
    computeIntensity,
    computeStreaks,
    summarizeHeatmap,
  };
});
