const test = require("node:test");
const assert = require("node:assert/strict");

const {
  computeDotPoints,
  computeTooltipPosition,
  computeHoverIndex,
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
