<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import {
  ArrowDown,
  ArrowLeft,
  ArrowUp,
  Globe2,
  KeyRound,
  MapPin,
  Pencil,
  RefreshCw,
} from "@lucide/vue";
import { NButton, NIcon, NSpin, NTooltip } from "naive-ui";
import { api, APIError } from "../../api";
import MetricChart, { type ChartSeries } from "../../components/MetricChart.vue";
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
} from "../../lib/format";
import type { MetricRange, NodeMetricSeries, NodeRecord } from "../../types";
import HostTelemetryPanel from "./HostTelemetryPanel.vue";
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
const telemetryPanel = ref<InstanceType<typeof HostTelemetryPanel> | null>(null);
const ranges: { value: MetricRange; label: string }[] = [
  { value: "1h", label: "1 小时" },
  { value: "6h", label: "6 小时" },
  { value: "24h", label: "24 小时" },
  { value: "7d", label: "7 天" },
  { value: "30d", label: "30 天" },
];
const selectedRangeLabel = computed(() => ranges.find((item) => item.value === metricRange.value)?.label ?? "24 小时");
const memoryPercent = computed(() => percent(props.node.memory_used_bytes, props.node.memory_total_bytes));
const diskPercent = computed(() => percent(props.node.disk_used_bytes, props.node.disk_total_bytes));

function meterWidth(value: number) {
  return `${Math.max(0, Math.min(100, value))}%`;
}

const labels = computed(() => metrics.value.samples.map((sample) => sample.bucket_at));
const cpuSeries = computed<ChartSeries[]>(() => [{
  name: "CPU", color: "var(--hf-chart-primary)", values: metrics.value.samples.map((sample) => sample.cpu_percent),
}]);
const memorySeries = computed<ChartSeries[]>(() => [
  {
    name: "内存", color: "var(--hf-chart-primary)",
    values: metrics.value.samples.map((sample) => percent(sample.memory_used_bytes, sample.memory_total_bytes)),
  },
  {
    name: "Swap", color: "var(--hf-chart-tertiary)",
    values: metrics.value.samples.map((sample) => percent(sample.swap_used_bytes, sample.swap_total_bytes)),
  },
]);
const networkSeries = computed<ChartSeries[]>(() => [
  { name: "下行", color: "var(--hf-chart-primary)", values: metrics.value.samples.map((sample) => sample.network_rx_bps) },
  { name: "上行", color: "var(--hf-chart-secondary)", values: metrics.value.samples.map((sample) => sample.network_tx_bps) },
]);
const diskSeries = computed<ChartSeries[]>(() => [
  { name: "读取", color: "var(--hf-chart-primary)", values: metrics.value.samples.map((sample) => sample.disk_read_bytes_per_second) },
  { name: "写入", color: "var(--hf-chart-tertiary)", values: metrics.value.samples.map((sample) => sample.disk_write_bytes_per_second) },
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

function refreshMonitoring() {
  void loadMetrics();
  void telemetryPanel.value?.refresh();
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
      <div class="node-detail-header__identity">
        <div class="node-detail-header__eyebrow">
          <status-indicator :status="node.status" />
          <span>{{ adapterLabels[node.adapter_type] }}</span>
        </div>
        <h1>{{ node.name }}</h1>
        <div class="node-detail-header__meta">
          <span><map-pin :size="14" aria-hidden="true" />{{ [node.provider, node.region].filter(Boolean).join(" · ") || "未设置位置" }}</span>
          <span><globe-2 :size="14" aria-hidden="true" />{{ endpointLabel(node) }}</span>
          <span>Agent {{ node.agent_version || "未注册" }}</span>
        </div>
      </div>
      <div class="node-detail-header__actions">
        <n-tooltip trigger="hover">
          <template #trigger>
            <n-button circle secondary aria-label="刷新监控" :loading="metricsLoading" @click="refreshMonitoring">
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
      <node-signal-rail :node="node" />
    </header>

    <nav class="node-detail-nav" aria-label="节点详情导航">
      <a href="#node-performance">性能轨迹</a>
      <a href="#node-runtime">进程与服务</a>
      <a href="#node-configuration">配置与运维</a>
    </nav>

    <section id="node-performance" class="node-performance-layout" aria-label="节点性能">
      <div class="monitor-workspace node-performance-main" aria-label="历史监控">
        <header class="monitor-workspace__heading">
          <div>
            <h2>性能轨迹</h2>
            <span>{{ selectedRangeLabel }}<template v-if="metrics.step_seconds > 60"> · {{ Math.round(metrics.step_seconds / 60) }} 分钟聚合</template></span>
          </div>
          <div class="monitor-toolbar">
            <div class="range-segment" aria-label="监控时间范围">
              <button
                v-for="item in ranges"
                :key="item.value"
                type="button"
                :class="{ active: metricRange === item.value }"
                :aria-pressed="metricRange === item.value"
                @click="metricRange = item.value"
              >{{ item.label }}</button>
            </div>
          </div>
        </header>
        <div v-if="metricsError" class="monitor-error">{{ metricsError }}</div>
        <div v-if="metricsLoading && metrics.samples.length === 0" class="monitor-loading"><n-spin :size="26" /></div>
        <section v-else class="monitor-grid monitor-ledger">
          <article class="monitor-panel monitor-panel--cpu">
            <header><h2>CPU 使用率</h2><span>{{ formatPercent(node.cpu_percent) }}</span></header>
            <metric-chart label="CPU 使用率" :series="cpuSeries" :labels="labels" :value-formatter="(value) => `${value.toFixed(0)}%`" />
          </article>
          <article class="monitor-panel monitor-panel--memory">
            <header><h2>内存与 Swap</h2><span>{{ formatBytes(node.memory_used_bytes) }} / {{ formatBytes(node.memory_total_bytes) }}</span></header>
            <metric-chart label="内存与 Swap" :series="memorySeries" :labels="labels" :value-formatter="(value) => `${value.toFixed(0)}%`" />
          </article>
          <article class="monitor-panel monitor-panel--network">
            <header><h2>网络速率</h2><span>{{ formatRate(node.network_rx_bps + node.network_tx_bps) }}</span></header>
            <metric-chart label="网络速率" :series="networkSeries" :labels="labels" :value-formatter="formatRate" />
          </article>
          <article class="monitor-panel monitor-panel--disk">
            <header><h2>磁盘 I/O</h2><span>{{ formatBytes(node.disk_read_bytes_per_second + node.disk_write_bytes_per_second) }}/s</span></header>
            <metric-chart label="磁盘 I/O" :series="diskSeries" :labels="labels" :value-formatter="(value) => `${formatBytes(value)}/s`" />
          </article>
        </section>
      </div>

      <aside class="node-resource-rail" aria-label="当前资源">
        <header class="node-resource-rail__heading">
          <div><h2>当前资源</h2><span>心跳 {{ relativeTime(node.last_seen_at) }}</span></div>
          <status-indicator :status="node.status" :show-label="false" />
        </header>
        <section class="host-kpis">
          <article class="host-kpi host-kpi--primary">
            <header><span>CPU 使用率</span><small>{{ node.cpu_cores || "-" }} 核</small></header>
            <strong>{{ formatPercent(node.cpu_percent) }}</strong>
            <div class="host-kpi__meter" aria-hidden="true"><i :style="{ width: meterWidth(node.cpu_percent) }" /></div>
            <small>1 分钟负载 {{ node.load_1.toFixed(2) }}</small>
          </article>
          <article class="host-kpi host-kpi--primary">
            <header><span>内存占用</span><small>{{ formatBytes(node.memory_total_bytes) }}</small></header>
            <strong>{{ formatPercent(memoryPercent) }}</strong>
            <div class="host-kpi__meter" aria-hidden="true"><i :style="{ width: meterWidth(memoryPercent) }" /></div>
            <small>{{ formatBytes(node.memory_used_bytes) }} · Swap {{ formatBytes(node.swap_used_bytes) }}</small>
          </article>
          <article class="host-kpi host-kpi--primary">
            <header><span>根分区</span><small>{{ formatBytes(node.disk_total_bytes) }}</small></header>
            <strong>{{ formatPercent(diskPercent) }}</strong>
            <div class="host-kpi__meter" aria-hidden="true"><i :style="{ width: meterWidth(diskPercent) }" /></div>
            <small>{{ formatBytes(node.disk_used_bytes) }} 已用</small>
          </article>
          <article class="host-kpi host-kpi--context">
            <header><span>当前网络</span><small>接收 / 发送</small></header>
            <strong><arrow-down :size="14" aria-hidden="true" />{{ formatRate(node.network_rx_bps) }}</strong>
            <small><arrow-up :size="13" aria-hidden="true" />{{ formatRate(node.network_tx_bps) }}</small>
          </article>
          <article class="host-kpi host-kpi--context">
            <header><span>运行时间</span><small>自上次启动</small></header>
            <strong>{{ formatUptime(node.uptime_seconds) }}</strong>
            <small>Agent {{ node.agent_version || "未注册" }}</small>
          </article>
        </section>
      </aside>
    </section>

    <host-telemetry-panel
      id="node-runtime"
      ref="telemetryPanel"
      :node-id="node.id"
      @session-expired="emit('session-expired')"
    />

    <section id="node-configuration" class="node-detail-lower">
      <div class="node-detail-column">
        <section class="detail-band">
          <h2>底层与累计指标</h2>
          <dl class="detail-list detail-list--two">
            <div><dt>CPU 核心</dt><dd>{{ node.cpu_cores || "-" }}</dd></div>
            <div><dt>负载（1 / 5 / 15 分钟）</dt><dd>{{ node.load_1.toFixed(2) }} / {{ node.load_5.toFixed(2) }} / {{ node.load_15.toFixed(2) }}</dd></div>
            <div><dt>磁盘读取 / 写入</dt><dd>{{ formatBytes(node.disk_read_bytes_per_second) }}/s / {{ formatBytes(node.disk_write_bytes_per_second) }}/s</dd></div>
            <div><dt>网卡接收 / 发送</dt><dd>{{ formatRate(node.network_rx_bps) }} / {{ formatRate(node.network_tx_bps) }}</dd></div>
            <div><dt>累计接收</dt><dd>{{ formatBytes(node.network_rx_bytes_total) }}</dd></div>
            <div><dt>累计发送</dt><dd>{{ formatBytes(node.network_tx_bytes_total) }}</dd></div>
          </dl>
        </section>
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
