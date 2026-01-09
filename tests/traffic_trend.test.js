const test = require("node:test");
const assert = require("node:assert/strict");

const {
  computeDotPoints,
  computeTooltipPosition,
  computeHoverIndex,
  computeXAxisTicks,
  computeYAxisTicks,
} = require("../web/static/js/traffic_trend");

test("computeDotPoints builds dot coordinates with chart scale", () => {
  const chart = { width: 640, height: 200, padding: 20 };
  const pv = [0, 50];
  const uv = [10, 20];

  const result = computeDotPoints(pv, uv, chart);

  assert.equal(result.maxValue, 50);
  assert.equal(result.dots.length, 2);
  assert.deepEqual(result.dots[0], {
    index: 0,
    x: 20,
    pv: 0,
    uv: 10,
    pvY: 180,
    uvY: 148,
  });
  assert.deepEqual(result.dots[1], {
    index: 1,
    x: 620,
    pv: 50,
    uv: 20,
    pvY: 20,
    uvY: 116,
  });
});

test("computeDotPoints handles empty series", () => {
  const chart = { width: 640, height: 200, padding: 20 };
  const result = computeDotPoints([], [], chart);

  assert.equal(result.maxValue, 1);
  assert.equal(result.dots.length, 0);
});

test("computeTooltipPosition clamps within container", () => {
  const point = { x: 20, pvY: 20, uvY: 148 };
  const chart = { width: 640, height: 200 };
  const container = { width: 320, height: 100 };
  const tooltip = { width: 180, height: 72 };

  const position = computeTooltipPosition(point, chart, container, tooltip);

  assert.deepEqual(position, { left: 90, top: 84 });
});

test("computeHoverIndex resolves nearest point with scale", () => {
  const chart = { width: 640, height: 200, padding: 20 };

  assert.equal(computeHoverIndex(20, 24, chart, { width: 640 }), 0);
  assert.equal(computeHoverIndex(620, 24, chart, { width: 640 }), 23);
  assert.equal(computeHoverIndex(320, 24, chart, { width: 640 }), 12);
  assert.equal(computeHoverIndex(160, 24, chart, { width: 320 }), 12);
});

test("computeHoverIndex handles single or empty series", () => {
  const chart = { width: 640, height: 200, padding: 20 };

  assert.equal(computeHoverIndex(10, 1, chart, { width: 640 }), 0);
  assert.equal(computeHoverIndex(10, 0, chart, { width: 640 }), null);
});

test("computeXAxisTicks picks daily steps for long ranges", () => {
  const chart = { width: 640, height: 200, padding: 20 };
  const ticks = computeXAxisTicks(168, chart);

  assert.equal(ticks[0].index, 0);
  assert.equal(ticks[1].index, 24);
  assert.equal(ticks[ticks.length - 1].index, 167);
  assert.ok(Math.abs(ticks[0].x - 20) < 1e-6);
  assert.ok(Math.abs(ticks[ticks.length - 1].x - 620) < 1e-6);
});

test("computeXAxisTicks emits each point when count is small", () => {
  const chart = { width: 640, height: 200, padding: 20 };
  const ticks = computeXAxisTicks(6, chart);

  assert.deepEqual(
    ticks.map((tick) => tick.index),
    [0, 1, 2, 3, 4, 5],
  );
});

test("computeYAxisTicks builds max/mid/zero ticks", () => {
  const chart = { width: 640, height: 200, padding: 20 };
  const ticks = computeYAxisTicks(120, chart, 120);

  assert.deepEqual(ticks, [
    { value: 120, y: 20 },
    { value: 60, y: 100 },
    { value: 0, y: 180 },
  ]);
});

test("computeYAxisTicks handles zero max", () => {
  const chart = { width: 640, height: 200, padding: 20 };
  const ticks = computeYAxisTicks(0, chart, 1);

  assert.deepEqual(ticks, [{ value: 0, y: 180 }]);
});
