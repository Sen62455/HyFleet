<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import {
  ArrowDown,
  ArrowLeft,
  ArrowUp,
  Cpu,
  HardDrive,
  KeyRound,
  MemoryStick,
  Pencil,
  RefreshCw,
  Wifi,
} from "@lucide/vue";
import { NButton, NIcon, NSpin, NTag, NTooltip } from "naive-ui";
import { api, APIError } from "../../api";
import MetricBar from "../../components/MetricBar.vue";
import MetricChart, { type ChartSeries } from "../../components/MetricChart.vue";
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
import type { MetricRange, NodeMetricSeries, NodeRecord } from "../../types";
import NodeOperationsPanel from "./NodeOperationsPanel.vue";
import SUIAdapterPanel from "./SUIAdapterPanel.vue";

const props = defineProps<{ node: NodeRecord }>();
const emit = defineEmits<{
  back: [];
  edit: [node: NodeRecord];
  enroll: [node: NodeRecord];
  changed: [];
  operations: [nodeId: string];
  "session-expired": [];
}>();

const metricRange = ref<MetricRange>("24h");
const metrics = ref<NodeMetricSeries>({ range: "24h", step_seconds: 60, samples: [] });
const metricsLoading = ref(false);
const metricsError = ref("");
const ranges: { value: MetricRange; label: string }[] = [
  { value: "1h", label: "1 小时" },
  { value: "6h", label: "6 小时" },
  { value: "24h", label: "24 小时" },
  { value: "7d", label: "7 天" },
  { value: "30d", label: "30 天" },
];

const labels = computed(() => metrics.value.samples.map((sample) => sample.bucket_at));
const cpuSeries = computed<ChartSeries[]>(() => [{
  name: "CPU", color: "#147d64", values: metrics.value.samples.map((sample) => sample.cpu_percent),
}]);
const memorySeries = computed<ChartSeries[]>(() => [
  {
    name: "内存", color: "#147d64",
    values: metrics.value.samples.map((sample) => percent(sample.memory_used_bytes, sample.memory_total_bytes)),
  },
  {
    name: "Swap", color: "#d97706",
    values: metrics.value.samples.map((sample) => percent(sample.swap_used_bytes, sample.swap_total_bytes)),
  },
]);
const networkSeries = computed<ChartSeries[]>(() => [
  { name: "下行", color: "#2563a6", values: metrics.value.samples.map((sample) => sample.network_rx_bps) },
  { name: "上行", color: "#147d64", values: metrics.value.samples.map((sample) => sample.network_tx_bps) },
]);
const diskSeries = computed<ChartSeries[]>(() => [
  { name: "读取", color: "#2563a6", values: metrics.value.samples.map((sample) => sample.disk_read_bytes_per_second) },
  { name: "写入", color: "#d97706", values: metrics.value.samples.map((sample) => sample.disk_write_bytes_per_second) },
]);

async function loadMetrics(silent = false) {
  if (!silent) metricsLoading.value = true;
  metricsError.value = "";
  try {
    metrics.value = await api.getNodeMetrics(props.node.id, metricRange.value);
  } catch (error) {
    if (error instanceof APIError && error.status === 401) {
      emit("session-expired");
      return;
    }
    metricsError.value = error instanceof APIError ? error.message : "历史指标加载失败。";
  } finally {
    metricsLoading.value = false;
  }
}

function endpointLabel(node: NodeRecord) {
  if (!node.public_host) return "未配置";
  const host = node.public_host.includes(":") ? `[${node.public_host}]` : node.public_host;
  return `${host}:${node.public_port}`;
}

let refreshTimer: number | undefined;
onMounted(() => {
  void loadMetrics();
  refreshTimer = window.setInterval(() => {
    if (document.visibilityState === "visible") void loadMetrics(true);
  }, 30_000);
});
onBeforeUnmount(() => window.clearInterval(refreshTimer));
watch(metricRange, () => void loadMetrics());
watch(() => props.node.id, () => void loadMetrics());
</script>

<template>
  <main class="workspace node-detail-page">
    <div class="node-detail-page__breadcrumb">
      <n-button quaternary circle aria-label="返回节点" @click="emit('back')">
        <template #icon><n-icon><arrow-left /></n-icon></template>
      </n-button>
      <span>节点</span><b>/</b><strong>{{ node.name }}</strong>
    </div>
    <header class="node-detail-header">
      <div>
        <status-indicator :status="node.status" :show-label="false" />
        <h1>{{ node.name }}</h1>
        <status-indicator :status="node.status" />
        <span>{{ [node.provider, node.region].filter(Boolean).join(" · ") || "未设置位置" }}</span>
        <n-tag :type="node.core_running ? 'success' : 'error'" size="small" :bordered="false">
          {{ node.core_name || "代理核心" }} {{ node.core_running ? "运行中" : "未运行" }}
        </n-tag>
      </div>
      <div>
        <n-tooltip trigger="hover">
          <template #trigger>
            <n-button circle secondary aria-label="刷新监控" :loading="metricsLoading" @click="loadMetrics()">
              <template #icon><n-icon><refresh-cw /></n-icon></template>
            </n-button>
          </template>
          刷新监控
        </n-tooltip>
        <n-button secondary @click="emit('edit', node)">
          <template #icon><n-icon><pencil /></n-icon></template>编辑
        </n-button>
        <n-button type="primary" @click="emit('enroll', node)">
          <template #icon><n-icon><key-round /></n-icon></template>注册 Agent
        </n-button>
      </div>
    </header>

    <section class="host-kpis" aria-label="当前资源">
      <div><cpu :size="20" /><span>CPU</span><strong>{{ formatPercent(node.cpu_percent) }}</strong><small>{{ node.cpu_cores || "-" }} 核 · 负载 {{ node.load_1.toFixed(2) }}</small></div>
      <div><memory-stick :size="20" /><span>内存</span><strong>{{ formatBytes(node.memory_used_bytes) }}</strong><small>{{ formatBytes(node.memory_total_bytes) }} · Swap {{ formatBytes(node.swap_used_bytes) }}</small></div>
      <div><hard-drive :size="20" /><span>根分区</span><strong>{{ formatPercent(percent(node.disk_used_bytes, node.disk_total_bytes)) }}</strong><small>{{ formatBytes(node.disk_used_bytes) }} / {{ formatBytes(node.disk_total_bytes) }}</small></div>
      <div><wifi :size="20" /><span>当前网络</span><strong><arrow-down :size="15" />{{ formatRate(node.network_rx_bps) }}</strong><small><arrow-up :size="14" />{{ formatRate(node.network_tx_bps) }}</small></div>
      <div><span>运行时间</span><strong>{{ formatUptime(node.uptime_seconds) }}</strong><small>心跳 {{ relativeTime(node.last_seen_at) }}</small></div>
    </section>

    <div class="monitor-toolbar">
      <div class="range-segment" aria-label="监控时间范围">
        <button
          v-for="item in ranges"
          :key="item.value"
          type="button"
          :class="{ active: metricRange === item.value }"
          @click="metricRange = item.value"
        >{{ item.label }}</button>
      </div>
      <span v-if="metrics.step_seconds > 60">{{ Math.round(metrics.step_seconds / 60) }} 分钟聚合</span>
    </div>
    <div v-if="metricsError" class="monitor-error">{{ metricsError }}</div>
    <div v-if="metricsLoading && metrics.samples.length === 0" class="monitor-loading"><n-spin :size="26" /></div>
    <section v-else class="monitor-grid" aria-label="历史监控">
      <article class="monitor-panel">
        <header><h2>CPU 使用率</h2><span>{{ formatPercent(node.cpu_percent) }}</span></header>
        <metric-chart :series="cpuSeries" :labels="labels" :value-formatter="(value) => `${value.toFixed(0)}%`" />
      </article>
      <article class="monitor-panel">
        <header><h2>内存与 Swap</h2><span>{{ formatBytes(node.memory_used_bytes) }} / {{ formatBytes(node.memory_total_bytes) }}</span></header>
        <metric-chart :series="memorySeries" :labels="labels" :value-formatter="(value) => `${value.toFixed(0)}%`" />
      </article>
      <article class="monitor-panel">
        <header><h2>网络速率</h2><span>{{ formatRate(node.network_rx_bps + node.network_tx_bps) }}</span></header>
        <metric-chart :series="networkSeries" :labels="labels" :value-formatter="formatRate" />
      </article>
      <article class="monitor-panel">
        <header><h2>磁盘 I/O</h2><span>{{ formatBytes(node.disk_read_bytes_per_second + node.disk_write_bytes_per_second) }}/s</span></header>
        <metric-chart :series="diskSeries" :labels="labels" :value-formatter="(value) => `${formatBytes(value)}/s`" />
      </article>
    </section>

    <section class="node-detail-lower">
      <div class="node-detail-column">
        <section class="detail-band">
          <h2>系统信息</h2>
          <dl class="detail-list detail-list--two">
            <div><dt>主机名</dt><dd>{{ node.hostname || "尚未上报" }}</dd></div>
            <div><dt>操作系统</dt><dd>{{ [node.os_name, node.os_version].filter(Boolean).join(" ") || "尚未上报" }}</dd></div>
            <div><dt>架构</dt><dd>{{ node.architecture || "-" }}</dd></div>
            <div><dt>内核</dt><dd>{{ node.kernel_version || "尚未上报" }}</dd></div>
            <div><dt>Agent</dt><dd>{{ node.agent_version || "尚未注册" }}</dd></div>
            <div><dt>核心</dt><dd>{{ [node.core_name, node.core_version].filter(Boolean).join(" ") || "尚未上报" }}</dd></div>
            <div><dt>适配方式</dt><dd>{{ adapterLabels[node.adapter_type] }}</dd></div>
            <div><dt>订阅端点</dt><dd>{{ endpointLabel(node) }}</dd></div>
            <div><dt>证书固定</dt><dd>{{ node.tls_cert_fingerprint ? "已配置" : "未配置" }}</dd></div>
            <div><dt>公钥固定</dt><dd>{{ node.tls_public_key_sha256 ? "已配置" : "未配置" }}</dd></div>
          </dl>
        </section>
        <section class="detail-band">
          <h2>流量与在线</h2>
          <div class="detail-metrics"><metric-bar label="磁盘" :value="percent(node.disk_used_bytes, node.disk_total_bytes)" :display="`${formatBytes(node.disk_used_bytes)} / ${formatBytes(node.disk_total_bytes)}`" /></div>
          <dl class="detail-list detail-list--two">
            <div><dt>在线用户 / 连接</dt><dd>{{ node.online_users }} / {{ node.online_connections }}</dd></div>
            <div><dt>未识别用户</dt><dd>{{ node.online_unknown_users }}</dd></div>
            <div><dt>代理上传</dt><dd>{{ formatBytes(node.traffic_upload_bytes) }}</dd></div>
            <div><dt>代理下载</dt><dd>{{ formatBytes(node.traffic_download_bytes) }}</dd></div>
            <div><dt>网卡接收总量</dt><dd>{{ formatBytes(node.network_rx_bytes_total) }}</dd></div>
            <div><dt>网卡发送总量</dt><dd>{{ formatBytes(node.network_tx_bytes_total) }}</dd></div>
          </dl>
        </section>
      </div>
      <div class="node-detail-column">
        <node-operations-panel
          v-if="node.agent_installation_id"
          :node="node"
          compact
          @view-all="emit('operations', node.id)"
          @changed="emit('changed')"
          @session-expired="emit('session-expired')"
        />
        <s-u-i-adapter-panel
          v-if="node.adapter_type === 's_ui'"
          :node="node"
          @changed="emit('changed')"
          @session-expired="emit('session-expired')"
        />
      </div>
    </section>
  </main>
</template>
