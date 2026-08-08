<script setup lang="ts">
import { h } from "vue";
import { Archive, MoreHorizontal, Pencil, Settings2, UserRound, Wifi } from "@lucide/vue";
import { NButton, NDropdown, NIcon, NTag, NTooltip, type DropdownOption } from "naive-ui";
import { formatBytes, formatDateTime } from "../../lib/format";
import type { UserRecord } from "../../types";

defineProps<{ users: UserRecord[] }>();
const emit = defineEmits<{
  select: [user: UserRecord];
  action: [action: "edit" | "manage" | "archive", user: UserRecord];
}>();

function icon(component: typeof Pencil) {
  return () => h(NIcon, null, { default: () => h(component, { size: 16 }) });
}

const options: DropdownOption[] = [
  { label: "编辑", key: "edit", icon: icon(Pencil) },
  { label: "管理节点", key: "manage", icon: icon(Settings2) },
  { type: "divider", key: "divider" },
  { label: "归档", key: "archive", icon: icon(Archive) },
];

const statusLabels = { active: "启用", disabled: "已停用", expired: "已到期" } as const;
const statusTypes = { active: "success", disabled: "default", expired: "error" } as const;

function choose(key: string | number, user: UserRecord) {
  emit("action", key as "edit" | "manage" | "archive", user);
}

function syncState(user: UserRecord): { label: string; state: string } {
  if (user.assignments.length === 0) return { label: "未分配", state: "idle" };
  if (user.assignments.some((assignment) => assignment.state === "failed")) {
    return { label: "同步失败", state: "error" };
  }
  if (user.assignments.some((assignment) => assignment.state !== "applied" || assignment.applied_version < assignment.desired_version)) {
    return { label: "等待同步", state: "pending" };
  }
  return { label: "已同步", state: "applied" };
}
</script>

<template>
  <div class="user-table-wrap user-table-wrap--desktop">
    <table class="user-table">
      <thead>
        <tr>
          <th>用户</th>
          <th>状态</th>
          <th>节点</th>
          <th>流量</th>
          <th>到期时间</th>
          <th>配置同步</th>
          <th><span class="sr-only">操作</span></th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="user in users"
          :key="user.id"
          tabindex="0"
          @click="emit('select', user)"
          @keydown.enter="emit('select', user)"
        >
          <td>
            <div class="user-identity">
              <span class="user-identity__icon"><user-round :size="18" aria-hidden="true" /></span>
              <span>
                <strong>{{ user.display_name || user.username }}</strong>
                <small>@{{ user.username }}</small>
              </span>
            </div>
          </td>
          <td>
            <n-tag :type="statusTypes[user.status]" size="small" :bordered="false">
              {{ statusLabels[user.status] }}
            </n-tag>
            <span class="table-secondary online-inline" :class="{ 'online-inline--active': user.online_connections > 0 }">
              <wifi :size="12" aria-hidden="true" />{{ user.online_connections }}
            </span>
          </td>
          <td>
            <div v-if="user.assignments.length" class="assignment-tags">
              <n-tag v-for="assignment in user.assignments.slice(0, 2)" :key="assignment.id" size="small">
                {{ assignment.node_name }}
              </n-tag>
              <span v-if="user.assignments.length > 2">+{{ user.assignments.length - 2 }}</span>
            </div>
            <span v-else class="table-muted">未分配</span>
          </td>
          <td>
            <span class="traffic-value" :class="{ 'traffic-value--limited': user.quota_state === 'limited' }">
              {{ formatBytes(user.traffic_used_bytes) }}
            </span>
            <span class="table-secondary">{{ user.traffic_limit_bytes ? `共 ${formatBytes(user.traffic_limit_bytes)}` : "不限额" }}</span>
          </td>
          <td><span class="user-expiry">{{ formatDateTime(user.expires_at) }}</span></td>
          <td>
            <span class="sync-state" :class="`sync-state--${syncState(user).state}`">
              <i aria-hidden="true" />{{ syncState(user).label }}
            </span>
          </td>
          <td class="action-cell" @click.stop @keydown.stop>
            <n-dropdown trigger="click" :options="options" @select="choose($event, user)">
              <n-tooltip trigger="hover">
                <template #trigger>
                  <n-button quaternary circle :aria-label="`${user.username} 操作`">
                    <template #icon><n-icon><more-horizontal /></n-icon></template>
                  </n-button>
                </template>
                操作
              </n-tooltip>
            </n-dropdown>
          </td>
        </tr>
      </tbody>
    </table>
  </div>

  <div class="user-mobile-list">
    <article v-for="user in users" :key="user.id" class="user-mobile-item" @click="emit('select', user)">
      <header>
        <div class="user-identity">
          <span class="user-identity__icon"><user-round :size="18" aria-hidden="true" /></span>
          <span>
            <strong>{{ user.display_name || user.username }}</strong>
            <small>@{{ user.username }}</small>
          </span>
        </div>
        <div @click.stop>
          <n-dropdown trigger="click" :options="options" @select="choose($event, user)">
            <n-button quaternary circle :aria-label="`${user.username} 操作`">
              <template #icon><n-icon><more-horizontal /></n-icon></template>
            </n-button>
          </n-dropdown>
        </div>
      </header>
      <div class="user-mobile-item__status">
        <n-tag :type="statusTypes[user.status]" size="small" :bordered="false">
          {{ statusLabels[user.status] }}
        </n-tag>
        <span>{{ user.assignments.length }} 个节点</span>
        <span class="online-inline" :class="{ 'online-inline--active': user.online_connections > 0 }">
          <wifi :size="12" aria-hidden="true" />{{ user.online_connections }}
        </span>
      </div>
      <footer>
        <span>{{ formatBytes(user.traffic_used_bytes) }} / {{ user.traffic_limit_bytes ? formatBytes(user.traffic_limit_bytes) : "不限额" }}</span>
        <span>{{ formatDateTime(user.expires_at) }}</span>
        <span class="sync-state" :class="`sync-state--${syncState(user).state}`">
          <i aria-hidden="true" />{{ syncState(user).label }}
        </span>
      </footer>
    </article>
  </div>
</template>
