<script setup lang="ts">
import { computed, ref } from "vue";

export interface ChartSeries {
  name: string;
  color: string;
  values: number[];
}

const props = withDefaults(defineProps<{
  series: ChartSeries[];
  labels: string[];
  valueFormatter?: (value: number) => string;
  emptyLabel?: string;
}>(), {
  valueFormatter: (value: number) => value.toFixed(1),
  emptyLabel: "暂无历史数据",
});

const hoverIndex = ref<number | null>(null);
const width = 640;
const height = 190;
const padding = { top: 14, right: 14, bottom: 28, left: 42 };
const plotWidth = width - padding.left - padding.right;
const plotHeight = height - padding.top - padding.bottom;

const pointCount = computed(() => Math.max(0, ...props.series.map((item) => item.values.length)));
const maximum = computed(() => {
  const values = props.series.flatMap((item) => item.values).filter(Number.isFinite);
  const found = Math.max(0, ...values);
  return found > 0 ? found * 1.08 : 1;
});
const lines = computed(() => props.series.map((item) => ({
  ...item,
  points: item.values.map((value, index) => `${x(index)},${y(value)}`).join(" "),
})));
const ticks = computed(() => [1, 0.75, 0.5, 0.25, 0].map((ratio) => ({
  ratio,
  value: maximum.value * ratio,
  y: padding.top + plotHeight * (1 - ratio),
})));

function x(index: number) {
  return padding.left + (pointCount.value <= 1 ? 0 : (index / (pointCount.value - 1)) * plotWidth);
}

function y(value: number) {
  return padding.top + plotHeight - (Math.max(0, value) / maximum.value) * plotHeight;
}

function move(event: MouseEvent) {
  if (pointCount.value === 0) return;
  const bounds = (event.currentTarget as SVGElement).getBoundingClientRect();
  const relative = ((event.clientX - bounds.left) / bounds.width) * width;
  const ratio = Math.max(0, Math.min(1, (relative - padding.left) / plotWidth));
  hoverIndex.value = Math.round(ratio * (pointCount.value - 1));
}

function shortTime(value: string | undefined) {
  if (!value) return "";
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) return "";
  return new Intl.DateTimeFormat("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false }).format(date);
}
</script>

<template>
  <div class="metric-chart" @mouseleave="hoverIndex = null">
    <div v-if="pointCount === 0" class="metric-chart__empty">{{ emptyLabel }}</div>
    <svg
      v-else
      class="metric-chart__canvas"
      :viewBox="`0 0 ${width} ${height}`"
      role="img"
      aria-label="历史指标折线图"
      @mousemove="move"
    >
      <g v-for="tick in ticks" :key="tick.ratio">
        <line :x1="padding.left" :x2="width - padding.right" :y1="tick.y" :y2="tick.y" class="metric-chart__grid" />
        <text :x="padding.left - 7" :y="tick.y + 4" text-anchor="end" class="metric-chart__axis-label">
          {{ valueFormatter(tick.value) }}
        </text>
      </g>
      <polyline
        v-for="line in lines"
        :key="line.name"
        :points="line.points"
        :stroke="line.color"
        class="metric-chart__line"
      />
      <g v-if="hoverIndex !== null">
        <line :x1="x(hoverIndex)" :x2="x(hoverIndex)" :y1="padding.top" :y2="padding.top + plotHeight" class="metric-chart__cursor" />
        <circle
          v-for="line in lines"
          :key="line.name"
          :cx="x(hoverIndex)"
          :cy="y(line.values[hoverIndex] ?? 0)"
          r="3.5"
          :fill="line.color"
          class="metric-chart__point"
        />
      </g>
      <text :x="padding.left" :y="height - 5" class="metric-chart__time">{{ shortTime(labels[0]) }}</text>
      <text :x="width - padding.right" :y="height - 5" text-anchor="end" class="metric-chart__time">
        {{ shortTime(labels[labels.length - 1]) }}
      </text>
    </svg>
    <div v-if="hoverIndex !== null" class="metric-chart__tooltip">
      <strong>{{ shortTime(labels[hoverIndex]) }}</strong>
      <span v-for="line in lines" :key="line.name">
        <i :style="{ background: line.color }" />{{ line.name }} {{ valueFormatter(line.values[hoverIndex] ?? 0) }}
      </span>
    </div>
  </div>
</template>
