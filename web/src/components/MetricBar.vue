<script setup lang="ts">
import { computed } from "vue";

const props = defineProps<{
  label: string;
  value: number;
  display: string;
}>();

const normalized = computed(() => Math.max(0, Math.min(100, props.value)));
const severity = computed(() => (normalized.value >= 90 ? "critical" : normalized.value >= 75 ? "warning" : "normal"));
</script>

<template>
  <div class="metric-bar">
    <div class="metric-bar__label">
      <span>{{ label }}</span>
      <span>{{ display }}</span>
    </div>
    <div class="metric-bar__track" aria-hidden="true">
      <span :class="`metric-bar__fill metric-bar__fill--${severity}`" :style="{ width: `${normalized}%` }" />
    </div>
  </div>
</template>
