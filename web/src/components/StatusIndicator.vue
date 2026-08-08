<script setup lang="ts">
import { computed } from "vue";
import { statusLabels } from "../lib/format";
import type { NodeStatus } from "../types";

const props = defineProps<{ status: NodeStatus }>();
const label = computed(() => statusLabels[props.status] ?? props.status);
</script>

<template>
  <span class="status-indicator" :class="`status-indicator--${status}`">
    <span class="status-indicator__dot" aria-hidden="true" />
    <span>{{ label }}</span>
  </span>
</template>
