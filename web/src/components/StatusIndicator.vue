<script setup lang="ts">
import { computed } from "vue";
import { statusLabels } from "../lib/format";
import type { NodeStatus } from "../types";

const props = withDefaults(defineProps<{ status: NodeStatus; showLabel?: boolean }>(), { showLabel: true });
const label = computed(() => statusLabels[props.status] ?? props.status);
</script>

<template>
  <span class="status-indicator" :class="`status-indicator--${status}`">
    <span class="status-indicator__dot" aria-hidden="true" />
    <span v-if="showLabel">{{ label }}</span>
  </span>
</template>
