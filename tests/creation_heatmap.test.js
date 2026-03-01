const test = require("node:test");
const assert = require("node:assert/strict");

const {
  normalizePoints,
  computeIntensity,
  summarizeHeatmap,
} = require("../web/static/js/creation_heatmap");

test("normalizePoints sorts dates and sanitizes payload", () => {
  const points = normalizePoints([
    {
      Date: "2026-03-03",
      Count: "2.4",
      Titles: ["  标题 A  ", "", " ", null, "标题 B"],
    },
    {
      Date: "2026-03-01",
      Count: -3,
      Titles: ["标题 C"],
    },
    {
      Date: "",
      Count: 8,
      Titles: ["无效日期"],
    },
  ]);

  assert.deepEqual(points, [
    { date: "2026-03-01", count: 0, titles: ["标题 C"] },
    { date: "2026-03-03", count: 2, titles: ["标题 A", "标题 B"] },
  ]);
});

test("computeIntensity maps count into 0-4 levels", () => {
  assert.equal(computeIntensity(0, 5), 0);
  assert.equal(computeIntensity(1, 5), 1);
  assert.equal(computeIntensity(2, 5), 2);
  assert.equal(computeIntensity(3, 5), 3);
  assert.equal(computeIntensity(4, 5), 4);
  assert.equal(computeIntensity(8, 5), 4);
});

test("summarizeHeatmap builds display cells and streak stats", () => {
  const summary = summarizeHeatmap([
    { Date: "2026-03-01", Count: 2, Titles: ["第一篇", "第二篇"] },
    { Date: "2026-03-02", Count: 1, Titles: ["第三篇"] },
    { Date: "2026-03-03", Count: 0, Titles: [] },
    { Date: "2026-03-04", Count: 3, Titles: ["第四篇"] },
  ]);

  assert.equal(summary.totalCount, 6);
  assert.equal(summary.activeDays, 3);
  assert.equal(summary.longestStreak, 2);
  assert.equal(summary.currentStreak, 1);
  assert.equal(summary.maxCount, 3);

  assert.equal(summary.weeks, 2);
  assert.equal(summary.displayCells.length, 14);
  assert.equal(summary.displayCells.slice(0, 6).every((cell) => cell.empty), true);

  assert.deepEqual(summary.displayCells[6], {
    empty: false,
    index: 0,
    date: "2026-03-01",
    count: 2,
    titles: ["第一篇", "第二篇"],
    level: 3,
  });
  assert.deepEqual(summary.displayCells[9], {
    empty: false,
    index: 3,
    date: "2026-03-04",
    count: 3,
    titles: ["第四篇"],
    level: 4,
  });
});
