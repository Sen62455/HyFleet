<script setup lang="ts">
import { computed, ref, watch } from "vue";
import {
  ChevronDown,
  ChevronUp,
  History,
  KeyRound,
  Link2,
  LogOut,
  Pencil,
  Plus,
  RotateCw,
  Save,
  Server,
  Trash2,
  Wifi,
} from "@lucide/vue";
import {
  NButton,
  NDrawer,
  NDrawerContent,
  NIcon,
  NInputNumber,
  NProgress,
  NSelect,
  NSpin,
  NSwitch,
  NTag,
  NTooltip,
} from "naive-ui";
import { formatBytes, formatDateTime, quotaPercent, relativeTime } from "../../lib/format";
import type { NodeRecord, SubscriptionTokenRecord, UserAssignment, UserRecord } from "../../types";

const props = defineProps<{
  show: boolean;
  user: UserRecord | null;
  assignableNodes: NodeRecord[];
  subscriptionTokens: SubscriptionTokenRecord[];
  subscriptionLoading: boolean;
  working: string;
}>();
const emit = defineEmits<{
  "update:show": [show: boolean];
  edit: [user: UserRecord];
  "toggle-user": [user: UserRecord, enabled: boolean];
  assign: [user: UserRecord, nodeId: string, trafficLimitBytes: number];
  "toggle-assignment": [user: UserRecord, assignment: UserAssignment, enabled: boolean];
  unassign: [user: UserRecord, assignment: UserAssignment];
  reveal: [user: UserRecord, assignment: UserAssignment];
  "update-assignment-limit": [user: UserRecord, assignment: UserAssignment, trafficLimitBytes: number];
  "kick-user": [user: UserRecord];
  "kick-assignment": [user: UserRecord, assignment: UserAssignment];
  "create-subscription": [user: UserRecord];
  "rotate-subscription": [user: UserRecord, token: SubscriptionTokenRecord];
  "revoke-subscription": [user: UserRecord, token: SubscriptionTokenRecord];
  "rotate-assignment-credential": [user: UserRecord, assignment: UserAssignment];
  "rotate-user-credentials": [user: UserRecord];
  "open-node": [nodeId: string];
}>();

const selectedNodeID = ref<string | null>(null);
const selectedLimitGiB = ref(0);
const assignmentLimits = ref<Record<string, number>>({});
const subscriptionHistoryOpen = ref(false);
const assignedNodeIDs = computed(() => new Set(props.user?.assignments.map((item) => item.node_id) ?? []));
const availableOptions = computed(() =>
  props.assignableNodes
    .filter((node) => !assignedNodeIDs.value.has(node.id))
    .map((node) => ({
      label: `${node.name}${node.adapter_type === "s_ui" ? " · S-UI" : ""}`,
      value: node.id,
      disabled: !node.enabled,
    })),
);
const activeSubscriptionTokens = computed(() =>
  props.subscriptionTokens.filter((token) => token.status === "active"),
);
const inactiveSubscriptionTokens = computed(() =>
  props.subscriptionTokens.filter((token) => token.status !== "active"),
);
const displayedSubscriptionTokens = computed(() =>
  subscriptionHistoryOpen.value
    ? [...activeSubscriptionTokens.value, ...inactiveSubscriptionTokens.value]
    : activeSubscriptionTokens.value,
);
const managedAssignments = computed(
  () => props.user?.assignments.filter((assignment) => assignment.management_mode === "managed") ?? [],
);
const kickableAssignments = computed(
  () => managedAssignments.value.filter((assignment) => assignment.node_adapter === "native_hysteria2"),
);

watch(
  () => [props.show, props.user?.id] as const,
  () => {
    selectedNodeID.value = null;
    selectedLimitGiB.value = 0;
    subscriptionHistoryOpen.value = false;
    assignmentLimits.value = Object.fromEntries(
      (props.user?.assignments ?? []).map((assignment) => [
        assignment.id,
        assignment.traffic_limit_bytes / 1024 ** 3,
      ]),
    );
  },
);

function assign() {
  if (!props.user || !selectedNodeID.value) return;
  emit("assign", props.user, selectedNodeID.value, Math.round(selectedLimitGiB.value * 1024 ** 3));
  selectedNodeID.value = null;
  selectedLimitGiB.value = 0;
}

function saveAssignmentLimit(assignment: UserAssignment) {
  if (!props.user) return;
  const value = assignmentLimits.value[assignment.id] ?? 0;
  emit("update-assignment-limit", props.user, assignment, Math.round(value * 1024 ** 3));
}

function assignmentState(assignment: UserAssignment) {
  if (assignment.state === "failed") return { label: "失败", type: "error" as const };
  if (assignment.state === "applied" && assignment.applied_version >= assignment.desired_version) {
    return { label: "已同步", type: "success" as const };
  }
  return { label: "等待同步", type: "warning" as const };
}

function subscriptionState(token: SubscriptionTokenRecord) {
  if (token.status === "revoked") return { label: "已撤销", type: "default" as const };
  if (token.status === "expired") return { label: "已到期", type: "error" as const };
  return { label: "有效", type: "success" as const };
}

function subscriptionEligibility(assignment: UserAssignment) {
  if (assignment.subscription_eligible) return { label: "已纳入订阅", type: "success" as const };
  const labels: Record<string, string> = {
    read_only_requires_adoption: "接管后纳入订阅",
    adapter_not_supported: "适配器不支持订阅",
    node_disabled: "节点已停用",
    node_not_ready: "节点尚未就绪",
    endpoint_missing: "缺少公网端点",
    assignment_disabled: "分配已停用",
    assignment_quota_limited: "节点额度已用尽",
    assignment_not_applied: "等待配置同步",
    credential_not_applied: "凭据尚未应用",
  };
  return { label: labels[assignment.subscription_reason] ?? "未纳入订阅", type: "warning" as const };
}

function formatLabel(format: string) {
  return { uri: "URI", base64: "Base64", clash: "Clash", "sing-box": "sing-box" }[format] ?? format;
}
</script>

<template>
  <n-drawer
    :show="show"
    width="min(520px, 100vw)"
    placement="right"
    @update:show="emit('update:show', $event)"
  >
    <n-drawer-content v-if="user" :title="user.display_name || user.username" closable>
      <div class="user-detail-heading">
        <div>
          <span>@{{ user.username }}</span>
          <n-tag v-if="user.status === 'expired'" type="error" size="small" :bordered="false">已到期</n-tag>
          <n-tag v-else-if="!user.enabled" size="small" :bordered="false">已停用</n-tag>
          <n-tag v-else type="success" size="small" :bordered="false">启用</n-tag>
        </div>
        <n-switch
          :value="user.enabled"
          :loading="working === `user:${user.id}`"
          aria-label="启用用户"
          @update:value="emit('toggle-user', user, $event)"
        />
      </div>

      <section class="detail-section">
        <h2>账户</h2>
        <dl class="detail-list">
          <div><dt>到期时间</dt><dd>{{ formatDateTime(user.expires_at) }}</dd></div>
          <div><dt>创建时间</dt><dd>{{ formatDateTime(user.created_at, false) }}</dd></div>
          <div><dt>最后更新</dt><dd>{{ relativeTime(user.updated_at) }}</dd></div>
        </dl>
        <p v-if="user.notes" class="user-notes">{{ user.notes }}</p>
      </section>

      <section class="detail-section">
        <div class="detail-section__heading">
          <h2>统一订阅</h2>
          <n-button size="small" secondary @click="emit('create-subscription', user)">
            <template #icon><n-icon><plus /></n-icon></template>
            创建 Token
          </n-button>
        </div>
        <div v-if="subscriptionLoading" class="subscription-loading"><n-spin :size="20" /></div>
        <div v-else-if="displayedSubscriptionTokens.length" class="subscription-token-list">
          <article v-for="token in displayedSubscriptionTokens" :key="token.id" class="subscription-token-item">
            <header>
              <div>
                <strong>{{ token.name }}</strong>
                <span>{{ token.token_prefix }}••••</span>
              </div>
              <n-tag :type="subscriptionState(token).type" size="small" :bordered="false">
                {{ subscriptionState(token).label }}
              </n-tag>
            </header>
            <div class="subscription-token-meta">
              <span>{{ token.allowed_formats.map(formatLabel).join(" · ") }}</span>
              <span>使用 {{ relativeTime(token.last_used_at) }}</span>
              <span>到期 {{ formatDateTime(token.expires_at) }}</span>
            </div>
            <footer>
              <span>{{ formatDateTime(token.created_at, false) }}</span>
              <div class="assignment-actions">
                <n-tooltip v-if="token.status === 'active'" trigger="hover">
                  <template #trigger>
                    <n-button
                      circle
                      quaternary
                      :loading="working === `subscription-rotate:${token.id}`"
                      :aria-label="`轮换 ${token.name} Token`"
                      @click="emit('rotate-subscription', user, token)"
                    >
                      <template #icon><n-icon><rotate-cw /></n-icon></template>
                    </n-button>
                  </template>
                  轮换 Token
                </n-tooltip>
                <n-tooltip v-if="token.status === 'active'" trigger="hover">
                  <template #trigger>
                    <n-button
                      circle
                      quaternary
                      type="error"
                      :loading="working === `subscription-revoke:${token.id}`"
                      :aria-label="`撤销 ${token.name} Token`"
                      @click="emit('revoke-subscription', user, token)"
                    >
                      <template #icon><n-icon><trash2 /></n-icon></template>
                    </n-button>
                  </template>
                  撤销 Token
                </n-tooltip>
              </div>
            </footer>
          </article>
        </div>
        <div v-else class="subscription-empty">
          <link2 :size="18" aria-hidden="true" />
          <span>暂无有效订阅 Token</span>
        </div>
        <div v-if="!subscriptionLoading && inactiveSubscriptionTokens.length" class="subscription-history-toggle">
          <n-button
            text
            size="small"
            :aria-expanded="subscriptionHistoryOpen"
            :aria-label="subscriptionHistoryOpen ? '收起历史 Token' : '展开历史 Token'"
            @click="subscriptionHistoryOpen = !subscriptionHistoryOpen"
          >
            <template #icon><n-icon><history /></n-icon></template>
            历史 Token {{ inactiveSubscriptionTokens.length }}
            <n-icon class="subscription-history-chevron">
              <chevron-up v-if="subscriptionHistoryOpen" />
              <chevron-down v-else />
            </n-icon>
          </n-button>
        </div>
        <div class="subscription-count">{{ activeSubscriptionTokens.length }} 个有效 Token</div>
      </section>

      <section class="detail-section">
        <div class="detail-section__heading">
          <h2>流量与在线</h2>
          <n-tag v-if="user.quota_state === 'limited'" type="error" size="small" :bordered="false">
            额度用尽
          </n-tag>
          <span v-else>{{ user.online_connections }} 台设备</span>
        </div>
        <n-progress
          v-if="user.traffic_limit_bytes > 0"
          type="line"
          :percentage="quotaPercent(user.traffic_used_bytes, user.traffic_limit_bytes)"
          :show-indicator="false"
          :status="user.quota_state === 'limited' ? 'error' : 'success'"
          processing
        />
        <dl class="detail-list detail-list--two traffic-summary">
          <div><dt>已用 / 额度</dt><dd>{{ formatBytes(user.traffic_used_bytes) }} / {{ user.traffic_limit_bytes ? formatBytes(user.traffic_limit_bytes) : '不限额' }}</dd></div>
          <div><dt>在线节点</dt><dd>{{ user.online_nodes }} / {{ user.assignments.length }}</dd></div>
          <div><dt>上传</dt><dd>{{ formatBytes(user.traffic_upload_bytes) }}</dd></div>
          <div><dt>下载</dt><dd>{{ formatBytes(user.traffic_download_bytes) }}</dd></div>
        </dl>
      </section>

      <section class="detail-section">
        <div class="detail-section__heading">
          <h2>节点分配</h2>
          <span>{{ user.assignments.length }}</span>
        </div>
        <div class="assignment-add">
          <n-select
            v-model:value="selectedNodeID"
            filterable
            clearable
            :options="availableOptions"
            :disabled="availableOptions.length === 0"
            placeholder="选择 Hysteria2 节点"
          />
          <n-input-number
            v-model:value="selectedLimitGiB"
            :min="0"
            :max="8388607"
            :precision="2"
            size="small"
            placeholder="额度 GiB"
          />
          <n-button
            type="primary"
            :disabled="!selectedNodeID"
            :loading="working === `assign:${user.id}`"
            @click="assign"
          >
            <template #icon><n-icon><plus /></n-icon></template>
            分配
          </n-button>
        </div>

        <div v-if="user.assignments.length" class="assignment-list">
          <article v-for="assignment in user.assignments" :key="assignment.id" class="assignment-item">
            <header>
              <div>
                <strong>{{ assignment.node_name }}</strong>
                <span>v{{ assignment.applied_version }} / {{ assignment.desired_version }}</span>
              </div>
              <div class="assignment-tags">
                <n-tag
                  v-if="assignment.management_mode === 'read_only'"
                  type="warning"
                  size="small"
                  :bordered="false"
                >
                  只读导入
                </n-tag>
                <n-tag :type="assignmentState(assignment).type" size="small" :bordered="false">
                  {{ assignmentState(assignment).label }}
                </n-tag>
                <n-tag :type="subscriptionEligibility(assignment).type" size="small" :bordered="false">
                  {{ subscriptionEligibility(assignment).label }}
                </n-tag>
              </div>
            </header>
            <p v-if="assignment.last_error_message" class="assignment-error">
              {{ assignment.last_error_message }}
            </p>
            <div
              v-if="assignment.subscription_reason === 'read_only_requires_adoption'"
              class="assignment-adoption"
            >
              <span>当前远端凭据保持只读，额度和统一订阅尚未接管。</span>
              <n-button text type="primary" @click="emit('open-node', assignment.node_id)">
                <template #icon><n-icon><server /></n-icon></template>
                打开节点
              </n-button>
            </div>
            <div class="assignment-usage">
              <div>
                <span>{{ formatBytes(assignment.traffic_used_bytes) }}</span>
                <span v-if="assignment.traffic_limit_bytes"> / {{ formatBytes(assignment.traffic_limit_bytes) }}</span>
                <span v-else> / 不限额</span>
              </div>
              <span :class="{ 'assignment-online--active': assignment.online_connections > 0 }">
                <wifi :size="13" aria-hidden="true" />{{ assignment.online_connections }}
              </span>
            </div>
            <n-progress
              v-if="assignment.traffic_limit_bytes > 0"
              type="line"
              :percentage="quotaPercent(assignment.traffic_used_bytes, assignment.traffic_limit_bytes)"
              :show-indicator="false"
              :status="assignment.quota_state === 'limited' ? 'error' : 'success'"
            />
            <div v-if="assignment.management_mode === 'managed'" class="assignment-limit-row">
              <n-input-number
                v-model:value="assignmentLimits[assignment.id]"
                :min="0"
                :max="8388607"
                :precision="2"
                size="small"
              >
                <template #suffix>GiB</template>
              </n-input-number>
              <n-tooltip trigger="hover">
                <template #trigger>
                  <n-button
                    circle
                    secondary
                    size="small"
                    :loading="working === `limit:${assignment.id}`"
                    :aria-label="`保存 ${assignment.node_name} 流量额度`"
                    @click="saveAssignmentLimit(assignment)"
                  >
                    <template #icon><n-icon><save /></n-icon></template>
                  </n-button>
                </template>
                保存节点额度
              </n-tooltip>
            </div>
            <footer>
              <span>
                {{ assignment.management_mode === "read_only" ? "远端凭据不受管" : assignment.credential_fingerprint }}
              </span>
              <div class="assignment-actions">
                <n-switch
                  v-if="assignment.management_mode === 'managed'"
                  size="small"
                  :value="assignment.enabled"
                  :loading="working === `toggle:${assignment.id}`"
                  :aria-label="`启用 ${assignment.node_name} 分配`"
                  @update:value="emit('toggle-assignment', user, assignment, $event)"
                />
                <n-tooltip
                  v-if="assignment.management_mode === 'managed' && assignment.node_adapter === 'native_hysteria2'"
                  trigger="hover"
                >
                  <template #trigger>
                    <n-button
                      circle
                      quaternary
                      :loading="working === `kick:${assignment.id}`"
                      :aria-label="`将用户从 ${assignment.node_name} 踢下线`"
                      @click="emit('kick-assignment', user, assignment)"
                    >
                      <template #icon><n-icon><log-out /></n-icon></template>
                    </n-button>
                  </template>
                  踢下线
                </n-tooltip>
                <n-tooltip v-if="assignment.management_mode === 'managed'" trigger="hover">
                  <template #trigger>
                    <n-button
                      circle
                      quaternary
                      :loading="working === `reveal:${assignment.id}`"
                      :aria-label="`查看 ${assignment.node_name} 凭据`"
                      @click="emit('reveal', user, assignment)"
                    >
                      <template #icon><n-icon><key-round /></n-icon></template>
                    </n-button>
                  </template>
                  查看凭据
                </n-tooltip>
                <n-tooltip v-if="assignment.management_mode === 'managed'" trigger="hover">
                  <template #trigger>
                    <n-button
                      circle
                      quaternary
                      :disabled="assignment.state !== 'applied' || assignment.applied_version < assignment.desired_version"
                      :loading="working === `credential-rotate:${assignment.id}`"
                      :aria-label="`轮换 ${assignment.node_name} 凭据`"
                      @click="emit('rotate-assignment-credential', user, assignment)"
                    >
                      <template #icon><n-icon><rotate-cw /></n-icon></template>
                    </n-button>
                  </template>
                  轮换凭据
                </n-tooltip>
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <n-button
                      circle
                      quaternary
                      type="error"
                      :aria-label="`取消 ${assignment.node_name} 分配`"
                      @click="emit('unassign', user, assignment)"
                    >
                      <template #icon><n-icon><trash2 /></n-icon></template>
                    </n-button>
                  </template>
                  取消分配
                </n-tooltip>
              </div>
            </footer>
          </article>
        </div>
        <div v-else class="assignment-empty">尚未分配节点</div>
      </section>

      <template #footer>
        <div class="drawer-actions">
          <n-button @click="emit('edit', user)">
            <template #icon><n-icon><pencil /></n-icon></template>
            编辑用户
          </n-button>
          <n-button
            secondary
            :disabled="kickableAssignments.length === 0"
            :loading="working === `kick:${user.id}`"
            @click="emit('kick-user', user)"
          >
            <template #icon><n-icon><log-out /></n-icon></template>
            全部踢下线
          </n-button>
          <n-button
            secondary
            :disabled="managedAssignments.length === 0"
            :loading="working === `credential-rotate:${user.id}`"
            @click="emit('rotate-user-credentials', user)"
          >
            <template #icon><n-icon><rotate-cw /></n-icon></template>
            全部轮换
          </n-button>
        </div>
      </template>
    </n-drawer-content>
  </n-drawer>
</template>
