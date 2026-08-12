<script setup lang="ts">
import { computed } from "vue";
import {
  AlertTriangle,
  ArrowDown,
  ArrowUp,
  ChevronRight,
  Clock3,
  Plus,
  RefreshCw,
  Server,
} from "@lucide/vue";
import { NButton, NIcon, NSpin, NTooltip } from "naive-ui";
import MetricBar from "../../components/MetricBar.vue";
import NodeSignalRail from "../../components/NodeSignalRail.vue";
import StatusIndicator from "../../components/StatusIndicator.vue";
import {
  adapterLabels,
  formatBytes,
  formatPercent,
  formatRate,
  formatUptime,
  percent,
  relativeTime,
  statusLabels,
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
const fleetState = computed(() => {
  if (props.nodes.length === 0) return "等待首台主机接入";
  if (online.value === props.nodes.length && props.alerts.length === 0) return "全部节点响应正常";
  const unavailable = props.nodes.length - online.value;
  return unavailable > 0 ? `${unavailable} 台节点未在线` : `${props.alerts.length} 项告警待处理`;
});

function endpointLabel(node: NodeRecord) {
  if (!node.public_host) return "端点未设置";
  const host = node.public_host.includes(":") && !node.public_host.startsWith("[")
    ? `[${node.public_host}]`
    : node.public_host;
  return `${host}:${node.public_port}`;
}
</script>

<template>
  <main class="workspace overview-workspace">
    <div class="page-heading">
      <div>
        <h1>运行总览</h1>
        <p>{{ online }} / {{ nodes.length }} 台主机在线 · {{ alerts.length ? `${alerts.length} 项告警` : "当前无告警" }}</p>
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

    <section class="fleet-register" aria-label="集群状态">
      <div class="fleet-register__primary">
        <span>节点可用度</span>
        <div><strong>{{ online }}</strong><b>/ {{ nodes.length }}</b></div>
        <p>{{ fleetState }}</p>
      </div>
      <button
        type="button"
        class="fleet-register__cell fleet-register__cell--alert"
        :class="{ 'is-danger': alerts.length > 0 }"
        @click="emit('alerts')"
      >
        <span><alert-triangle :size="14" aria-hidden="true" />待处理告警</span>
        <strong>{{ alerts.length }}</strong>
        <small>{{ alerts.length ? "查看告警记录" : "当前无需处理" }}</small>
      </button>
      <div class="fleet-register__cell fleet-register__cell--network">
        <span>实时带宽</span>
        <strong><arrow-down :size="14" aria-hidden="true" />{{ formatRate(currentRX) }}</strong>
        <small><arrow-up :size="13" aria-hidden="true" />{{ formatRate(currentTX) }}</small>
      </div>
      <div class="fleet-register__cell fleet-register__cell--uptime">
        <span><clock3 :size="14" aria-hidden="true" />最长运行</span>
        <strong>{{ formatUptime(oldestUptime) }}</strong>
        <small>{{ online }} 台主机正在响应</small>
      </div>
    </section>

    <div v-if="error" class="overview-error">
      <span>{{ error }}</span><n-button text type="error" @click="emit('refresh')">重新加载</n-button>
    </div>
    <div v-if="loading" class="overview-loading"><n-spin :size="28" /></div>
    <div v-if="!loading && nodes.length" class="overview-section-heading">
      <div><span>主机矩阵</span><h2>逐台运行状态</h2></div>
      <small>{{ nodes.length }} 台主机 · 按最近心跳更新</small>
    </div>
    <section v-if="!loading && nodes.length" class="host-grid" aria-label="主机状态">
      <article
        v-for="node in nodes"
        :key="node.id"
        class="host-panel"
        :class="`host-panel--${node.status}`"
        :aria-labelledby="`host-${node.id}-name`"
      >
        <header>
          <div class="host-panel__identity">
            <span>
              <h3 :id="`host-${node.id}-name`">{{ node.name }}</h3>
              <small>{{ [node.provider, node.region].filter(Boolean).join(" · ") || "未设置位置" }}</small>
            </span>
          </div>
          <div class="host-panel__state">
            <status-indicator :status="node.status" />
            <button
              type="button"
              class="host-panel__open"
              :aria-label="`查看 ${node.name} 详情，当前状态${statusLabels[node.status]}`"
              @click="emit('select', node)"
            >
              <span>详情</span>
              <chevron-right :size="15" aria-hidden="true" />
            </button>
          </div>
        </header>
        <dl class="host-panel__meta">
          <div><dt>协议</dt><dd>{{ adapterLabels[node.adapter_type] }}</dd></div>
          <div><dt>公网端点</dt><dd>{{ endpointLabel(node) }}</dd></div>
          <div><dt>最近心跳</dt><dd>{{ relativeTime(node.last_seen_at) }}</dd></div>
        </dl>
        <node-signal-rail :node="node" compact />
        <div class="host-panel__metrics">
          <metric-bar label="CPU" :value="node.cpu_percent" :display="formatPercent(node.cpu_percent)" />
          <metric-bar
            label="内存"
            :value="percent(node.memory_used_bytes, node.memory_total_bytes)"
            :display="formatPercent(percent(node.memory_used_bytes, node.memory_total_bytes))"
          />
          <metric-bar
            label="磁盘"
            :value="percent(node.disk_used_bytes, node.disk_total_bytes)"
            :display="formatPercent(percent(node.disk_used_bytes, node.disk_total_bytes))"
          />
        </div>
        <dl class="host-panel__facts">
          <div><dt>负载</dt><dd>{{ node.load_1.toFixed(2) }} / {{ node.load_5.toFixed(2) }} / {{ node.load_15.toFixed(2) }}</dd></div>
          <div><dt><arrow-down :size="14" />下行</dt><dd>{{ formatRate(node.network_rx_bps) }}</dd></div>
          <div><dt><arrow-up :size="14" />上行</dt><dd>{{ formatRate(node.network_tx_bps) }}</dd></div>
          <div><dt>在线</dt><dd>{{ node.adapter_type === "sing_box_vless_reality" ? "暂不支持" : `${node.online_connections} 个连接` }}</dd></div>
        </dl>
      </article>
    </section>
    <div v-else-if="!loading" class="overview-empty">
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
