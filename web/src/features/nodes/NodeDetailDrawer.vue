<script setup lang="ts">
import { KeyRound, Pencil } from "@lucide/vue";
import { NButton, NDrawer, NDrawerContent, NIcon } from "naive-ui";
import MetricBar from "../../components/MetricBar.vue";
import StatusIndicator from "../../components/StatusIndicator.vue";
import {
  adapterLabels,
  formatBytes,
  formatPercent,
  formatRate,
  formatUptime,
  percent,
  relativeTime,
} from "../../lib/format";
import type { NodeRecord } from "../../types";

defineProps<{ show: boolean; node: NodeRecord | null }>();
const emit = defineEmits<{
  "update:show": [show: boolean];
  edit: [node: NodeRecord];
  enroll: [node: NodeRecord];
}>();
</script>

<template>
  <n-drawer
    :show="show"
    width="min(480px, 100vw)"
    placement="right"
    @update:show="emit('update:show', $event)"
  >
    <n-drawer-content v-if="node" :title="node.name" closable>
      <div class="detail-status-line">
        <status-indicator :status="node.status" />
        <span>{{ adapterLabels[node.adapter_type] }}</span>
      </div>
      <p v-if="node.status_reason" class="detail-reason">{{ node.status_reason }}</p>

      <section class="detail-section">
        <h2>资源</h2>
        <div class="detail-metrics">
          <metric-bar label="CPU" :value="node.cpu_percent" :display="formatPercent(node.cpu_percent)" />
          <metric-bar
            label="内存"
            :value="percent(node.memory_used_bytes, node.memory_total_bytes)"
            :display="`${formatBytes(node.memory_used_bytes)} / ${formatBytes(node.memory_total_bytes)}`"
          />
          <metric-bar
            label="磁盘"
            :value="percent(node.disk_used_bytes, node.disk_total_bytes)"
            :display="`${formatBytes(node.disk_used_bytes)} / ${formatBytes(node.disk_total_bytes)}`"
          />
        </div>
        <dl class="detail-list detail-list--two">
          <div><dt>下行速率</dt><dd>{{ formatRate(node.network_rx_bps) }}</dd></div>
          <div><dt>上行速率</dt><dd>{{ formatRate(node.network_tx_bps) }}</dd></div>
          <div><dt>系统负载</dt><dd>{{ node.load_1.toFixed(2) }} / {{ node.load_5.toFixed(2) }} / {{ node.load_15.toFixed(2) }}</dd></div>
          <div><dt>运行时间</dt><dd>{{ formatUptime(node.uptime_seconds) }}</dd></div>
        </dl>
      </section>

      <section class="detail-section">
        <h2>运行环境</h2>
        <dl class="detail-list">
          <div><dt>服务商</dt><dd>{{ node.provider || "-" }}</dd></div>
          <div><dt>地区</dt><dd>{{ node.region || "-" }}</dd></div>
          <div><dt>系统</dt><dd>{{ [node.os_name, node.os_version, node.architecture].filter(Boolean).join(" ") || "尚未上报" }}</dd></div>
          <div><dt>代理核心</dt><dd>{{ [node.core_name, node.core_version].filter(Boolean).join(" ") || "尚未上报" }}</dd></div>
          <div><dt>核心服务</dt><dd>{{ node.agent_installation_id ? (node.core_running ? "运行中" : "未运行") : "尚未上报" }}</dd></div>
          <div><dt>Agent</dt><dd>{{ node.agent_version || "尚未注册" }}</dd></div>
        </dl>
      </section>

      <section class="detail-section">
        <h2>同步</h2>
        <dl class="detail-list">
          <div><dt>最后上报</dt><dd>{{ relativeTime(node.last_seen_at) }}</dd></div>
          <div><dt>配置版本</dt><dd>{{ node.applied_version }} / {{ node.desired_version }}</dd></div>
          <div><dt>最后应用</dt><dd>{{ relativeTime(node.last_applied_at) }}</dd></div>
        </dl>
      </section>

      <template #footer>
        <div class="drawer-actions">
          <n-button @click="emit('edit', node)">
            <template #icon><n-icon><pencil /></n-icon></template>
            编辑
          </n-button>
          <n-button type="primary" @click="emit('enroll', node)">
            <template #icon><n-icon><key-round /></n-icon></template>
            注册 Agent
          </n-button>
        </div>
      </template>
    </n-drawer-content>
  </n-drawer>
</template>
