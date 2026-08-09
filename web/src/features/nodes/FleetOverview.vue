<script setup lang="ts">
import { computed } from "vue";
import {
  AlertTriangle,
  ArrowDown,
  ArrowUp,
  Clock3,
  Plus,
  RefreshCw,
  Server,
} from "@lucide/vue";
import { NButton, NIcon, NSpin, NTooltip } from "naive-ui";
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
import type { AlertRecord, NodeRecord } from "../../types";

const props = defineProps<{
  nodes: NodeRecord[];
  alerts: AlertRecord[];
  loading: boolean;
  refreshing: boolean;
  error: string;
}>();
const emit = defineEmits<{
  select: [node: NodeRecord];
  refresh: [];
  create: [];
  alerts: [];
}>();

const online = computed(() => props.nodes.filter((node) => node.status === "online").length);
const currentRX = computed(() => props.nodes.reduce((total, node) => total + node.network_rx_bps, 0));
const currentTX = computed(() => props.nodes.reduce((total, node) => total + node.network_tx_bps, 0));
const oldestUptime = computed(() => Math.max(0, ...props.nodes.map((node) => node.uptime_seconds)));
</script>

<template>
  <main class="workspace overview-workspace">
    <div class="page-heading">
      <div>
        <h1>总览</h1>
        <p>{{ online }} / {{ nodes.length }} 台主机在线</p>
      </div>
      <div class="page-heading__actions">
        <n-tooltip trigger="hover">
          <template #trigger>
            <n-button circle secondary aria-label="刷新总览" :loading="refreshing" @click="emit('refresh')">
              <template #icon><n-icon><refresh-cw /></n-icon></template>
            </n-button>
          </template>
          刷新
        </n-tooltip>
        <n-button type="primary" @click="emit('create')">
          <template #icon><n-icon><plus /></n-icon></template>
          添加节点
        </n-button>
      </div>
    </div>

    <section class="overview-summary" aria-label="集群状态">
      <div><server :size="20" /><span>主机</span><strong>{{ nodes.length }}</strong></div>
      <div class="overview-summary--healthy"><i /><span>在线</span><strong>{{ online }}</strong></div>
      <button type="button" :class="{ 'overview-summary--danger': alerts.length > 0 }" @click="emit('alerts')">
        <alert-triangle :size="20" /><span>当前告警</span><strong>{{ alerts.length }}</strong>
      </button>
      <div class="overview-summary__network">
        <span><arrow-down :size="15" />{{ formatRate(currentRX) }}</span>
        <span><arrow-up :size="15" />{{ formatRate(currentTX) }}</span>
      </div>
      <div><clock3 :size="20" /><span>最长运行</span><strong class="overview-summary__uptime">{{ formatUptime(oldestUptime) }}</strong></div>
    </section>

    <div v-if="error" class="overview-error">
      <span>{{ error }}</span><n-button text type="error" @click="emit('refresh')">重新加载</n-button>
    </div>
    <div v-if="loading" class="overview-loading"><n-spin :size="28" /></div>
    <section v-else-if="nodes.length" class="host-grid" aria-label="主机状态">
      <article
        v-for="node in nodes"
        :key="node.id"
        class="host-panel"
        role="button"
        tabindex="0"
        @click="emit('select', node)"
        @keydown.enter="emit('select', node)"
      >
        <header>
          <div>
            <status-indicator :status="node.status" :show-label="false" />
            <strong>{{ node.name }}</strong>
          </div>
          <span>{{ node.region || node.provider || "未设置地区" }}</span>
        </header>
        <div class="host-panel__meta">
          <span>{{ adapterLabels[node.adapter_type] }}</span>
          <span :class="{ 'host-panel__core-down': !node.core_running }">
            {{ node.core_running ? "核心运行中" : "核心未运行" }}
          </span>
          <span>{{ formatUptime(node.uptime_seconds) }}</span>
        </div>
        <div class="host-panel__metrics">
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
        <dl class="host-panel__facts">
          <div><dt>负载</dt><dd>{{ node.load_1.toFixed(2) }} / {{ node.load_5.toFixed(2) }} / {{ node.load_15.toFixed(2) }}</dd></div>
          <div><dt><arrow-down :size="14" />下行</dt><dd>{{ formatRate(node.network_rx_bps) }}</dd></div>
          <div><dt><arrow-up :size="14" />上行</dt><dd>{{ formatRate(node.network_tx_bps) }}</dd></div>
          <div><dt>心跳</dt><dd>{{ relativeTime(node.last_seen_at) }}</dd></div>
        </dl>
      </article>
    </section>
    <div v-else class="overview-empty">
      <server :size="30" /><strong>尚未添加节点</strong>
      <n-button type="primary" size="small" @click="emit('create')">添加节点</n-button>
    </div>

    <section v-if="alerts.length" class="overview-alerts">
      <div class="section-heading"><h2>最近告警</h2><n-button text type="primary" @click="emit('alerts')">查看全部</n-button></div>
      <button v-for="alert in alerts.slice(0, 4)" :key="alert.id" type="button" @click="emit('alerts')">
        <span :class="`alert-severity alert-severity--${alert.severity}`" />
        <strong>{{ alert.node_name }}</strong><span>{{ alert.message }}</span><time>{{ relativeTime(alert.last_seen_at) }}</time>
      </button>
    </section>
  </main>
</template>
