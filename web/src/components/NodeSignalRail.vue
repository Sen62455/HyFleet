<script setup lang="ts">
import { computed } from "vue";
import { statusLabels } from "../lib/format";
import type { NodeRecord } from "../types";

const props = withDefaults(defineProps<{ node: NodeRecord; compact?: boolean }>(), { compact: false });

type SignalState = "ok" | "warn" | "down" | "idle";

const stages = computed(() => {
  const agentState: SignalState = props.node.status === "disabled"
    ? "idle"
    : props.node.status === "online"
      ? "ok"
      : props.node.status === "pending" || props.node.status === "stale"
        ? "warn"
        : "down";
  const configurationApplied = props.node.desired_version > 0
    && props.node.applied_version >= props.node.desired_version;

  return [
    {
      label: "Agent",
      value: props.node.agent_version || statusLabels[props.node.status] || "未注册",
      state: agentState,
    },
    {
      label: "核心",
      value: props.node.core_running ? (props.node.core_name || "运行中") : "未运行",
      state: (props.node.core_running ? "ok" : "down") as SignalState,
    },
    {
      label: "配置",
      value: props.node.desired_version <= 0
        ? "尚未下发"
        : configurationApplied
          ? `已同步 v${props.node.applied_version}`
          : `等待 v${props.node.desired_version}`,
      state: (props.node.desired_version <= 0 ? "idle" : configurationApplied ? "ok" : "warn") as SignalState,
    },
  ];
});
</script>

<template>
  <div
    class="node-signal-rail"
    :class="{ 'node-signal-rail--compact': compact }"
    role="list"
    aria-label="节点运行链路"
  >
    <div
      v-for="stage in stages"
      :key="stage.label"
      class="node-signal-rail__stage"
      :class="`node-signal-rail__stage--${stage.state}`"
      role="listitem"
    >
      <i aria-hidden="true" />
      <span>
        <strong>{{ stage.label }}</strong>
        <small>{{ stage.value }}</small>
      </span>
    </div>
  </div>
</template>
