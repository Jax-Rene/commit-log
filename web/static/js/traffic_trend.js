(function (root, factory) {
  if (typeof module === "object" && module.exports) {
    module.exports = factory();
  } else {
    root.TrafficTrend = factory();
  }
})(this, function () {
  const defaultChart = { width: 640, height: 200, padding: 20 };

  function clamp(value, min, max) {
    return Math.min(Math.max(value, min), max);
  }

  function toNumber(value) {
    const num = Number(value);
    return Number.isFinite(num) ? num : 0;
  }

  function normalizeChart(chart) {
    if (!chart) {
      return { ...defaultChart };
    }
    return {
      width: toNumber(chart.width) || defaultChart.width,
      height: toNumber(chart.height) || defaultChart.height,
      padding:
        toNumber(chart.padding) || chart.padding === 0
          ? toNumber(chart.padding)
          : defaultChart.padding,
    };
  }

  function valueToY(value, maxValue, chart) {
    const safeMax = maxValue > 0 ? maxValue : 1;
    const available = chart.height - chart.padding * 2;
    return chart.height - chart.padding - (available * value) / safeMax;
  }

  function computeDotPoints(pv, uv, chart) {
    const safePv = Array.isArray(pv) ? pv.map(toNumber) : [];
    const safeUv = Array.isArray(uv) ? uv.map(toNumber) : [];
    const count = Math.max(safePv.length, safeUv.length);
    const normalizedChart = normalizeChart(chart);
    const maxValue = Math.max(1, ...safePv, ...safeUv);

    if (count === 0) {
      return { maxValue, dots: [] };
    }

    const step =
      count > 1
        ? (normalizedChart.width - normalizedChart.padding * 2) / (count - 1)
        : 0;

    const dots = [];
    for (let index = 0; index < count; index += 1) {
      const pvValue = safePv[index] || 0;
      const uvValue = safeUv[index] || 0;
      const x = normalizedChart.padding + step * index;
      dots.push({
        index,
        x,
        pv: pvValue,
        uv: uvValue,
        pvY: valueToY(pvValue, maxValue, normalizedChart),
        uvY: valueToY(uvValue, maxValue, normalizedChart),
      });
    }

    return { maxValue, dots };
  }

  function computeTooltipPosition(point, chart, container, tooltip) {
    const normalizedChart = normalizeChart(chart);
    const width = toNumber(container && container.width);
    const height = toNumber(container && container.height);
    const tooltipWidth = toNumber(tooltip && tooltip.width) || 180;
    const tooltipHeight = toNumber(tooltip && tooltip.height) || 72;

    const containerWidth = width > 0 ? width : normalizedChart.width;
    const containerHeight = height > 0 ? height : normalizedChart.height;
    const scaleX = containerWidth / normalizedChart.width;
    const scaleY = containerHeight / normalizedChart.height;

    const anchorX = toNumber(point && point.x) * scaleX;
    const anchorY =
      Math.min(toNumber(point && point.pvY), toNumber(point && point.uvY)) *
      scaleY;

    const left = clamp(
      anchorX,
      tooltipWidth / 2,
      containerWidth - tooltipWidth / 2,
    );
    const top = clamp(anchorY, tooltipHeight + 12, containerHeight);

    return { left, top };
  }

  function computeHoverIndex(offsetX, count, chart, container) {
    const total = Math.floor(toNumber(count));
    if (total <= 0) {
      return null;
    }
    if (total === 1) {
      return 0;
    }

    const normalizedChart = normalizeChart(chart);
    const width = toNumber(container && container.width);
    const containerWidth = width > 0 ? width : normalizedChart.width;
    if (containerWidth <= 0 || normalizedChart.width <= 0) {
      return 0;
    }

    const scaleX = containerWidth / normalizedChart.width;
    const chartX = toNumber(offsetX) / scaleX;
    const step =
      (normalizedChart.width - normalizedChart.padding * 2) / (total - 1);
    if (step <= 0) {
      return 0;
    }

    const rawIndex = Math.round((chartX - normalizedChart.padding) / step);
    return clamp(rawIndex, 0, total - 1);
  }

  function resolveXAxisStep(total) {
    if (total <= 12) {
      return 1;
    }
    if (total <= 48) {
      return Math.ceil(total / 6);
    }
    return 24;
  }

  function computeXAxisTicks(count, chart) {
    const total = Math.floor(toNumber(count));
    if (total <= 0) {
      return [];
    }

    const normalizedChart = normalizeChart(chart);
    const step = resolveXAxisStep(total);
    const indices = new Set([0]);
    for (let index = 0; index < total; index += step) {
      indices.add(index);
    }
    indices.add(total - 1);

    const span = normalizedChart.width - normalizedChart.padding * 2;
    const divisor = total > 1 ? total - 1 : 1;
    return Array.from(indices)
      .sort((a, b) => a - b)
      .map((index) => ({
        index,
        x: normalizedChart.padding + (span * index) / divisor,
      }));
  }

  function computeYAxisTicks(maxValue, chart, scaleMax) {
    const normalizedChart = normalizeChart(chart);
    const rawMax = Math.max(0, Math.round(toNumber(maxValue)));
    const safeScale = Math.max(1, toNumber(scaleMax) || rawMax);

    if (rawMax <= 0) {
      return [
        {
          value: 0,
          y: normalizedChart.height - normalizedChart.padding,
        },
      ];
    }

    const mid = Math.round(rawMax / 2);
    const values = [];
    [rawMax, mid, 0].forEach((value) => {
      if (!values.includes(value)) {
        values.push(value);
      }
    });

    return values
      .sort((a, b) => b - a)
      .map((value) => ({
        value,
        y: valueToY(value, safeScale, normalizedChart),
      }));
  }

  return {
    computeDotPoints,
    computeTooltipPosition,
    computeHoverIndex,
    computeXAxisTicks,
    computeYAxisTicks,
  };
});
