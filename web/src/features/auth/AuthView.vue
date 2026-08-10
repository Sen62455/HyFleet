<script setup lang="ts">
import { computed, reactive } from "vue";
import { LockKeyhole, RefreshCw } from "@lucide/vue";
import {
  NAlert,
  NButton,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  type FormInst,
  type FormRules,
} from "naive-ui";
import { ref } from "vue";
import BrandMark from "../../components/BrandMark.vue";
import ColorModePicker from "../../components/ColorModePicker.vue";

const props = defineProps<{
  mode: "setup" | "login";
  bootstrapConfigured: boolean;
  loading: boolean;
  errorMessage: string;
}>();

const emit = defineEmits<{
  submit: [payload: { username: string; password: string; bootstrapToken?: string }];
  retry: [];
}>();

const formRef = ref<FormInst | null>(null);
const form = reactive({
  bootstrapToken: "",
  username: "",
  password: "",
  confirmation: "",
});

const isSetup = computed(() => props.mode === "setup");
const rules = computed<FormRules>(() => ({
  bootstrapToken: isSetup.value ? [{ required: true, message: "请输入初始化令牌", trigger: "blur" }] : [],
  username: [
    { required: true, message: "请输入用户名", trigger: "blur" },
    { min: 3, max: 32, message: "用户名长度为 3 到 32 个字符", trigger: "blur" },
  ],
  password: [
    { required: true, message: "请输入密码", trigger: "blur" },
    ...(isSetup.value ? [{ min: 12, max: 128, message: "密码至少需要 12 个字符", trigger: "blur" }] : []),
  ],
  confirmation: isSetup.value
    ? [
        { required: true, message: "请再次输入密码", trigger: "blur" },
        {
          validator: (_rule, value: string) => value === form.password,
          message: "两次输入的密码不一致",
          trigger: ["input", "blur"],
        },
      ]
    : [],
}));

async function submit() {
  try {
    await formRef.value?.validate();
  } catch {
    return;
  }
  emit("submit", {
    username: form.username.trim(),
    password: form.password,
    bootstrapToken: isSetup.value ? form.bootstrapToken : undefined,
  });
}
</script>

<template>
  <main class="auth-screen">
    <header class="auth-screen__header">
      <brand-mark />
      <color-mode-picker />
    </header>
    <section class="auth-panel" :aria-labelledby="isSetup ? 'setup-title' : 'login-title'">
      <div class="auth-panel__icon" aria-hidden="true">
        <lock-keyhole :size="22" />
      </div>
      <h1 :id="isSetup ? 'setup-title' : 'login-title'">
        {{ isSetup ? "初始化控制台" : "登录控制台" }}
      </h1>
      <p>{{ isSetup ? "创建此实例的唯一管理员账户" : "使用管理员账户继续" }}</p>

      <n-alert v-if="isSetup && !bootstrapConfigured" type="warning" :show-icon="false" class="auth-panel__alert">
        服务器尚未配置 <code>HYFLEET_BOOTSTRAP_TOKEN</code>，设置后重启服务。
      </n-alert>
      <n-alert v-if="errorMessage" type="error" :show-icon="false" class="auth-panel__alert">
        <div class="alert-row">
          <span>{{ errorMessage }}</span>
          <n-button quaternary size="small" aria-label="重试连接" @click="emit('retry')">
            <template #icon><n-icon><refresh-cw /></n-icon></template>
          </n-button>
        </div>
      </n-alert>

      <n-form ref="formRef" :model="form" :rules="rules" label-placement="top" @submit.prevent="submit">
        <n-form-item v-if="isSetup" label="初始化令牌" path="bootstrapToken">
          <n-input
            v-model:value="form.bootstrapToken"
            type="password"
            show-password-on="click"
            :input-props="{ autocomplete: 'one-time-code' }"
            placeholder="请输入初始化令牌"
          />
        </n-form-item>
        <n-form-item label="用户名" path="username">
          <n-input
            v-model:value="form.username"
            :input-props="{ autocomplete: 'username' }"
            maxlength="32"
            placeholder="请输入用户名"
            @keyup.enter="submit"
          />
        </n-form-item>
        <n-form-item label="密码" path="password">
          <n-input
            v-model:value="form.password"
            type="password"
            show-password-on="click"
            :input-props="{ autocomplete: isSetup ? 'new-password' : 'current-password' }"
            maxlength="128"
            placeholder="请输入密码"
            @keyup.enter="!isSetup && submit()"
          />
        </n-form-item>
        <n-form-item v-if="isSetup" label="确认密码" path="confirmation">
          <n-input
            v-model:value="form.confirmation"
            type="password"
            show-password-on="click"
            :input-props="{ autocomplete: 'new-password' }"
            maxlength="128"
            placeholder="请再次输入密码"
            @keyup.enter="submit"
          />
        </n-form-item>
        <n-button
          type="primary"
          block
          attr-type="submit"
          :loading="loading"
          :disabled="isSetup && !bootstrapConfigured"
        >
          {{ isSetup ? "创建管理员" : "登录" }}
        </n-button>
      </n-form>
    </section>
  </main>
</template>
