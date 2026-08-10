<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import {
  Bell,
  ClipboardList,
  LayoutDashboard,
  LogOut,
  Plus,
  RefreshCw,
  Server,
  UsersRound,
} from "@lucide/vue";
import { NAlert, NBadge, NButton, NIcon, NSpin, NTooltip, useDialog, useMessage } from "naive-ui";
import { api, APIError } from "../../api";
import BrandMark from "../../components/BrandMark.vue";
import ColorModePicker from "../../components/ColorModePicker.vue";
import { issueCount } from "../../lib/format";
import type { AlertRecord, NodeInput, NodeRecord, Session } from "../../types";
import AlertDrawer from "../alerts/AlertDrawer.vue";
import EnrollmentDialog from "./EnrollmentDialog.vue";
import FleetOverview from "./FleetOverview.vue";
import NodeDetailPage from "./NodeDetailPage.vue";
import NodeFormModal from "./NodeFormModal.vue";
import NodeTable from "./NodeTable.vue";
import UserDashboard from "../users/UserDashboard.vue";
import OperationsDashboard from "../operations/OperationsDashboard.vue";

const props = defineProps<{ session: Session }>();
const emit = defineEmits<{ logout: []; "session-expired": [] }>();

const message = useMessage();
const dialog = useDialog();
const nodes = ref<NodeRecord[]>([]);
const loading = ref(true);
const refreshing = ref(false);
const loadError = ref("");
const formOpen = ref(false);
const saving = ref(false);
const editingNode = ref<NodeRecord | null>(null);
const detailNodeID = ref<string | null>(null);
const enrollmentNode = ref<NodeRecord | null>(null);
type DashboardView = "overview" | "nodes" | "users" | "operations" | "node-detail";
const activeView = ref<DashboardView>("overview");
const detailReturnView = ref<"overview" | "nodes" | "users" | "operations">("overview");
const operationsNodeFilter = ref("");
const alerts = ref<AlertRecord[]>([]);
const alertsOpen = ref(false);
const alertsLoading = ref(false);
const alertWorking = ref("");

const onlineCount = computed(() => nodes.value.filter((node) => node.status === "online").length);
const pendingCount = computed(() => nodes.value.filter((node) => node.status === "pending").length);
const issues = computed(() => issueCount(nodes.value));
const detailNode = computed(() => nodes.value.find((node) => node.id === detailNodeID.value) ?? null);

function handleAPIError(error: unknown, fallback: string) {
  if (error instanceof APIError && error.status === 401) {
    emit("session-expired");
    return;
  }
  message.error(error instanceof APIError ? error.message : fallback);
}

async function loadNodes(silent = false) {
  if (refreshing.value) return;
  if (!silent) loading.value = nodes.value.length === 0;
  refreshing.value = true;
  loadError.value = "";
  try {
    nodes.value = await api.listNodes();
  } catch (error) {
    if (error instanceof APIError && error.status === 401) {
      emit("session-expired");
      return;
    }
    loadError.value = error instanceof APIError ? error.message : "节点列表加载失败。";
  } finally {
    loading.value = false;
    refreshing.value = false;
  }
}

async function loadAlerts(silent = false) {
  if (!silent) alertsLoading.value = true;
  try {
    alerts.value = await api.listAlerts("active");
  } catch (error) {
    if (error instanceof APIError && error.status === 401) {
      emit("session-expired");
    } else if (!silent) {
      handleAPIError(error, "告警加载失败。");
    }
  } finally {
    alertsLoading.value = false;
  }
}

async function acknowledgeAlert(alert: AlertRecord) {
  alertWorking.value = alert.id;
  try {
    await api.acknowledgeAlert(alert.id);
    await loadAlerts(true);
  } catch (error) {
    handleAPIError(error, "告警确认失败。");
  } finally {
    alertWorking.value = "";
  }
}

function selectAlertNode(nodeId: string) {
	openNode(nodeId, "overview");
	alertsOpen.value = false;
}

function openNodeFromUsers(nodeId: string) {
	openNode(nodeId, "users");
}

function openNode(nodeId: string, from: "overview" | "nodes" | "users" | "operations" = "nodes") {
	detailNodeID.value = nodeId;
	detailReturnView.value = from;
	activeView.value = "node-detail";
}

function openOperations(nodeId = "") {
	operationsNodeFilter.value = nodeId;
	activeView.value = "operations";
}

function openCreate() {
  editingNode.value = null;
  formOpen.value = true;
}

function openEdit(node: NodeRecord) {
  editingNode.value = node;
  formOpen.value = true;
}

async function saveNode(input: Required<NodeInput>) {
  saving.value = true;
  try {
    const saved = editingNode.value
      ? await api.updateNode(editingNode.value.id, input)
      : await api.createNode(input);
    formOpen.value = false;
    editingNode.value = null;
    await loadNodes(true);
		openNode(saved.id, "nodes");
    message.success(saved.desired_version === 1 ? "节点已添加" : "节点已更新");
  } catch (error) {
    handleAPIError(error, "节点保存失败。 ");
  } finally {
    saving.value = false;
  }
}

function archiveNode(node: NodeRecord) {
  dialog.warning({
    title: "归档节点",
    content: `确认归档“${node.name}”？该节点将从控制台列表中移除。`,
    positiveText: "归档",
    negativeText: "取消",
    positiveButtonProps: { type: "error" },
    async onPositiveClick() {
      try {
        await api.archiveNode(node.id);
				if (detailNodeID.value === node.id) {
					detailNodeID.value = null;
					activeView.value = "nodes";
				}
        await loadNodes(true);
        message.success("节点已归档");
      } catch (error) {
        handleAPIError(error, "节点归档失败。 ");
        return false;
      }
      return true;
    },
  });
}

function handleAction(action: "edit" | "enroll" | "archive", node: NodeRecord) {
  if (action === "edit") openEdit(node);
  if (action === "enroll") enrollmentNode.value = node;
  if (action === "archive") archiveNode(node);
}

let refreshTimer: number | undefined;
onMounted(() => {
  loadNodes();
  loadAlerts(true);
  refreshTimer = window.setInterval(() => {
    if (document.visibilityState === "visible") {
      loadNodes(true);
      loadAlerts(true);
    }
  }, 15_000);
});
onBeforeUnmount(() => window.clearInterval(refreshTimer));
</script>

<template>
  <div class="app-shell">
    <header class="topbar">
      <div class="topbar__inner">
        <brand-mark compact />
        <nav class="topbar__nav" aria-label="主导航">
          <button
            type="button"
            class="topbar__nav-item"
            :class="{ 'topbar__nav-item--active': activeView === 'overview' }"
            :aria-current="activeView === 'overview' ? 'page' : undefined"
            aria-label="总览"
            title="总览"
            @click="activeView = 'overview'"
          >
            <layout-dashboard :size="16" aria-hidden="true" />
            <span>总览</span>
          </button>
          <button
            type="button"
            class="topbar__nav-item"
            :class="{ 'topbar__nav-item--active': activeView === 'nodes' || activeView === 'node-detail' }"
            :aria-current="activeView === 'nodes' || activeView === 'node-detail' ? 'page' : undefined"
            aria-label="节点"
            title="节点"
            @click="activeView = 'nodes'"
          >
            <server :size="16" aria-hidden="true" />
            <span>节点</span>
          </button>
          <button
            type="button"
            class="topbar__nav-item"
            :class="{ 'topbar__nav-item--active': activeView === 'users' }"
            :aria-current="activeView === 'users' ? 'page' : undefined"
            aria-label="用户"
            title="用户"
            @click="activeView = 'users'"
          >
            <users-round :size="16" aria-hidden="true" />
            <span>用户</span>
          </button>
          <button
            type="button"
            class="topbar__nav-item"
            :class="{ 'topbar__nav-item--active': activeView === 'operations' }"
            :aria-current="activeView === 'operations' ? 'page' : undefined"
            aria-label="操作记录"
            title="操作记录"
            @click="openOperations()"
          >
            <clipboard-list :size="16" aria-hidden="true" />
            <span>操作记录</span>
          </button>
        </nav>
        <div class="topbar__account">
          <span class="topbar__username">{{ props.session.admin.username }}</span>
          <color-mode-picker />
          <n-tooltip trigger="hover">
            <template #trigger>
              <n-badge :value="alerts.length" :max="99" :show="alerts.length > 0">
                <n-button
                  quaternary
                  circle
                  aria-label="查看告警"
                  @click="alertsOpen = true; loadAlerts()"
                >
                  <template #icon><n-icon><bell /></n-icon></template>
                </n-button>
              </n-badge>
            </template>
            告警
          </n-tooltip>
          <n-tooltip trigger="hover">
            <template #trigger>
              <n-button quaternary circle aria-label="退出登录" @click="emit('logout')">
                <template #icon><n-icon><log-out /></n-icon></template>
              </n-button>
            </template>
            退出登录
          </n-tooltip>
        </div>
      </div>
    </header>

    <fleet-overview
      v-if="activeView === 'overview'"
      :nodes="nodes"
      :alerts="alerts"
      :loading="loading"
      :refreshing="refreshing"
      :error="loadError"
      @select="openNode($event.id, 'overview')"
      @refresh="loadNodes()"
      @create="openCreate"
      @alerts="alertsOpen = true; loadAlerts()"
    />

    <main v-else-if="activeView === 'nodes'" id="nodes" class="workspace">
      <div class="page-heading">
        <div>
          <h1>节点</h1>
          <p>主机状态与 Agent 连接</p>
        </div>
        <div class="page-heading__actions">
          <n-tooltip trigger="hover">
            <template #trigger>
              <n-button circle secondary aria-label="刷新节点" :loading="refreshing" @click="loadNodes()">
                <template #icon><n-icon><refresh-cw /></n-icon></template>
              </n-button>
            </template>
            刷新
          </n-tooltip>
          <n-button type="primary" @click="openCreate">
            <template #icon><n-icon><plus /></n-icon></template>
            添加节点
          </n-button>
        </div>
      </div>

      <section class="fleet-summary" aria-label="节点摘要">
        <div class="fleet-summary__item">
          <span>全部节点</span>
          <strong>{{ nodes.length }}</strong>
        </div>
        <div class="fleet-summary__item fleet-summary__item--healthy">
          <span>在线</span>
          <strong>{{ onlineCount }}</strong>
        </div>
        <div class="fleet-summary__item fleet-summary__item--warning">
          <span>待连接</span>
          <strong>{{ pendingCount }}</strong>
        </div>
        <div class="fleet-summary__item fleet-summary__item--danger">
          <span>需关注</span>
          <strong>{{ issues }}</strong>
        </div>
      </section>

      <n-alert v-if="loadError" type="error" :show-icon="false" class="workspace-alert">
        <div class="alert-row">
          <span>{{ loadError }}</span>
          <n-button text type="error" @click="loadNodes()">重新加载</n-button>
        </div>
      </n-alert>

      <section class="node-surface" aria-label="节点列表">
        <div v-if="loading" class="surface-state"><n-spin :size="28" /></div>
        <div v-else-if="nodes.length === 0" class="surface-state surface-state--empty">
          <server :size="28" :stroke-width="1.7" aria-hidden="true" />
          <strong>尚未添加节点</strong>
          <n-button type="primary" size="small" @click="openCreate">
            <template #icon><n-icon><plus /></n-icon></template>
            添加节点
          </n-button>
        </div>
        <node-table
          v-else
          :nodes="nodes"
          @select="openNode($event.id, 'nodes')"
          @action="handleAction"
        />
      </section>
    </main>

    <user-dashboard
      v-else-if="activeView === 'users'"
      :nodes="nodes"
      @nodes-changed="loadNodes(true)"
      @open-node="openNodeFromUsers"
      @session-expired="emit('session-expired')"
    />

    <operations-dashboard
      v-else-if="activeView === 'operations'"
      :nodes="nodes"
      :initial-node-id="operationsNodeFilter"
      @select-node="openNode($event, 'operations')"
      @session-expired="emit('session-expired')"
    />

    <node-detail-page
      v-else-if="activeView === 'node-detail' && detailNode"
      :node="detailNode"
      @back="activeView = detailReturnView"
      @edit="openEdit"
      @enroll="enrollmentNode = $event"
      @operations="openOperations"
      @changed="loadNodes(true)"
      @session-expired="emit('session-expired')"
    />

    <node-form-modal
      v-model:show="formOpen"
      :node="editingNode"
      :saving="saving"
      @submit="saveNode"
    />
    <enrollment-dialog
      :show="enrollmentNode !== null"
      :node="enrollmentNode"
      @update:show="!$event && (enrollmentNode = null)"
      @session-expired="emit('session-expired')"
    />
    <alert-drawer
      v-model:show="alertsOpen"
      :alerts="alerts"
      :loading="alertsLoading"
      :working="alertWorking"
      @refresh="loadAlerts()"
      @acknowledge="acknowledgeAlert"
      @select-node="selectAlertNode"
    />
  </div>
</template>
