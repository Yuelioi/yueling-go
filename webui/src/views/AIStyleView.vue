<script setup lang="ts">
import { useRequestScope } from '../composables/useRequestScope'
import { useGroupsStore } from '../stores/groups.ts'
import { computed, onMounted, ref, watch } from 'vue'
import { aiApi, type GroupInfo } from '../api'
import { useGroupFilter } from '../composables/useGroupFilter'
import GroupAdministrationTable from '../components/GroupAdministrationTable.vue'
import GroupScopeSelect from '../components/GroupScopeSelect.vue'
import MetricCard from '../components/MetricCard.vue'
import MoreActions from '../components/MoreActions.vue'
import PageHeader from '../components/PageHeader.vue'

const groups = ref<GroupInfo[]>([])
const selectedGroupID = useGroupFilter()
const stylePrompt = ref('')
const savedPrompt = ref('')
const defaultPrompt = ref('')
const maxChars = ref(4000)
const custom = ref(false)
const loading = ref(false)
const loadingStyle = ref(false)
const saving = ref(false)
const resetting = ref(false)
const error = ref('')
const success = ref('')
const requests = useRequestScope()

const styleScopes = computed<GroupInfo[]>(() => [
  { group_id: 0, group_name: '全局默认风格' },
  ...groups.value,
])
const selectedGroup = computed(() =>
  styleScopes.value.find((group) => group.group_id === selectedGroupID.value),
)
const isDefaultScope = computed(() => selectedGroupID.value === 0)
const charCount = computed(() => Array.from(stylePrompt.value).length)
const dirty = computed(() => stylePrompt.value !== savedPrompt.value)
const modeLabel = computed(() => {
  if (isDefaultScope.value) return custom.value ? '自定义默认' : '内置默认'
  return custom.value ? '群独立风格' : '继承默认'
})

function applyStyleResponse(
  res: Awaited<ReturnType<typeof aiApi.groupAIStyle>>,
) {
  stylePrompt.value = res.style_prompt
  savedPrompt.value = res.style_prompt
  defaultPrompt.value = res.default_style_prompt
  maxChars.value = res.max_chars
  custom.value = res.custom
}

async function loadStyle(groupID: number) {
  const currentRequest = requests.begin('load')
  loadingStyle.value = true
  error.value = ''
  success.value = ''
  try {
    const res = await aiApi.groupAIStyle(groupID, {
      signal: currentRequest.signal,
    })
    if (currentRequest() && selectedGroupID.value === groupID) {
      applyStyleResponse(res)
    }
  } catch (err) {
    if (currentRequest()) {
      error.value = err instanceof Error ? err.message : '风格设置加载失败'
    }
  } finally {
    if (currentRequest()) loadingStyle.value = false
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await useGroupsStore().loadManaged()
    groups.value = res.groups
    if (selectedGroupID.value !== null) {
      await loadStyle(selectedGroupID.value)
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '群聊加载失败'
  } finally {
    loading.value = false
  }
}

async function save() {
  if (selectedGroupID.value === null) return
  if (charCount.value > maxChars.value) {
    error.value = `提示词不能超过 ${maxChars.value} 个字符`
    return
  }
  const groupID = selectedGroupID.value
  saving.value = true
  error.value = ''
  success.value = ''
  try {
    const res = await aiApi.setGroupAIStyle(groupID, stylePrompt.value)
    if (selectedGroupID.value !== groupID) return
    applyStyleResponse(res)
    success.value = isDefaultScope.value
      ? res.custom
        ? '全局默认风格已保存'
        : '已恢复程序内置默认'
      : res.custom
        ? '本群独立风格已保存'
        : '本群已改为继承全局默认'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value = false
  }
}

async function reset() {
  if (selectedGroupID.value === null) return
  const groupID = selectedGroupID.value
  resetting.value = true
  error.value = ''
  success.value = ''
  try {
    const res = await aiApi.resetGroupAIStyle(groupID)
    if (selectedGroupID.value !== groupID) return
    applyStyleResponse(res)
    success.value = isDefaultScope.value
      ? '已恢复程序内置默认'
      : '本群已改为继承全局默认'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '恢复默认失败'
  } finally {
    resetting.value = false
  }
}

watch(selectedGroupID, (groupID) => {
  requests.begin('load')
  loadingStyle.value = false
  if (groupID !== null) void loadStyle(groupID)
})

onMounted(load)
</script>

<template>
  <section class="min-w-0 space-y-6">
    <PageHeader
      title="AI 对话风格"
      description="维护一个全局默认提示词，需要时再为特定群设置覆盖。修改后无需重启 Bot。"
      icon="i-tabler-sparkles"
    >
      <UButton
        color="neutral"
        variant="soft"
        icon="i-tabler-refresh"
        :loading="loading"
        @click="load"
        >刷新群聊</UButton
      >
    </PageHeader>

    <div
      v-if="selectedGroupID !== null"
      class="grid grid-cols-3 gap-3 sm:gap-4"
    >
      <MetricCard
        label="可覆盖群聊"
        :value="groups.length"
        detail="未覆盖时继承全局默认"
        icon="i-tabler-users-group"
        tone="violet"
      />
      <MetricCard
        label="当前模式"
        :value="modeLabel"
        :detail="isDefaultScope ? '默认作用于所有未覆盖群' : '仅影响所选群聊'"
        icon="i-tabler-adjustments"
        tone="cyan"
      />
      <MetricCard
        label="提示词长度"
        :value="`${charCount}/${maxChars}`"
        detail="已输入 / 字数上限"
        icon="i-tabler-text-size"
        tone="amber"
      />
    </div>

    <UAlert
      v-if="error"
      color="error"
      variant="soft"
      icon="i-tabler-alert-circle"
      :description="error"
    />
    <UAlert
      v-if="success"
      color="success"
      variant="soft"
      icon="i-tabler-circle-check"
      :description="success"
    />

    <div class="space-y-4">
      <GroupScopeSelect
        allow-all
        :disabled="saving || resetting"
        v-model="selectedGroupID"
        :groups="styleScopes"
        title="风格作用域"
        description="默认风格或指定群覆盖"
        zero-label="未覆盖群聊继承"
      />

      <GroupAdministrationTable
        v-if="selectedGroupID === null"
        feature="style"
        include-default
        @select="selectedGroupID = $event"
      />

      <div v-else class="space-y-4">
        <section
          class="rounded-2xl border border-default bg-default overflow-hidden"
        >
          <div
            class="flex flex-col items-stretch justify-between gap-4 border-b border-default px-5 py-4 sm:flex-row sm:items-center"
          >
            <div class="min-w-0">
              <div class="truncate font-medium text-highlighted">
                {{
                  selectedGroup?.group_name ||
                  (selectedGroupID === null
                    ? '全部范围'
                    : `群 ${selectedGroupID}`)
                }}
              </div>
              <div class="text-xs text-muted">
                {{
                  selectedGroupID === null
                    ? '选择作用域后可编辑'
                    : isDefaultScope
                      ? '所有没有独立配置的群都会继承'
                      : `群号 ${selectedGroupID} · 覆盖全局默认`
                }}
              </div>
            </div>
            <UBadge :color="custom ? 'primary' : 'neutral'" variant="subtle">
              {{ loadingStyle ? '读取中' : modeLabel }}
            </UBadge>
          </div>

          <div class="space-y-5 p-5">
            <UFormField
              :label="isDefaultScope ? '全局默认提示词' : '群专属提示词'"
              :description="
                isDefaultScope
                  ? '控制所有未单独配置群聊的人格、语气、称呼和表达习惯。'
                  : '只描述本群需要覆盖默认值的部分；权限、安全与工具规则不受影响。'
              "
            >
              <UTextarea
                v-model="stylePrompt"
                class="w-full"
                :ui="{ root: 'w-full' }"
                :rows="8"
                autoresize
                :maxlength="maxChars"
                :disabled="selectedGroupID === null || loadingStyle"
                :placeholder="defaultPrompt"
              />
            </UFormField>

            <div
              class="flex flex-wrap items-center justify-between gap-3 text-xs"
            >
              <span class="text-muted">
                {{
                  isDefaultScope
                    ? '留空保存将恢复程序内置默认。'
                    : '留空保存将删除本群覆盖并继承全局默认。'
                }}
              </span>
              <span
                :class="charCount > maxChars ? 'text-rose-300' : 'text-muted'"
                >{{ charCount }} / {{ maxChars }}</span
              >
            </div>

            <div
              class="flex flex-wrap items-center justify-between gap-3 border-t border-default pt-4"
            >
              <span class="text-xs text-muted"
                >普通问答和主动插话都会读取最终生效的风格。</span
              >
              <div class="flex items-center gap-2">
                <MoreActions
                  v-if="custom"
                  label="风格更多操作"
                  show-label
                  :disabled="resetting"
                  :items="[
                    {
                      label: isDefaultScope ? '恢复内置默认' : '继承全局默认',
                      description: isDefaultScope
                        ? '移除自定义默认提示词'
                        : '移除本群独立覆盖',
                      icon: 'i-tabler-restore',
                      color: 'warning',
                      loading: resetting,
                      onSelect: reset,
                    },
                  ]"
                />
                <UButton
                  icon="i-tabler-device-floppy"
                  :loading="saving"
                  :disabled="
                    selectedGroupID === null ||
                    loadingStyle ||
                    !dirty ||
                    charCount > maxChars
                  "
                  @click="save"
                >
                  保存风格
                </UButton>
              </div>
            </div>
          </div>
        </section>

        <section
          class="rounded-2xl border border-default bg-default overflow-hidden"
        >
          <div
            class="flex flex-col items-stretch justify-between gap-4 border-b border-default px-5 py-4 sm:flex-row sm:items-center"
          >
            <div>
              <div class="text-sm font-semibold text-highlighted">继承规则</div>
            </div>
            <UIcon name="i-tabler-hierarchy-2" class="size-4 text-violet-300" />
          </div>
          <div class="grid gap-3 p-4 sm:grid-cols-3">
            <div class="rounded-lg border border-default bg-muted p-3">
              <strong class="text-xs text-highlighted">全局默认</strong>
              <p class="mt-1 text-xs leading-5 text-muted">
                作为所有群的基础人格与表达习惯。
              </p>
            </div>
            <div class="rounded-lg border border-default bg-muted p-3">
              <strong class="text-xs text-highlighted">群级覆盖</strong>
              <p class="mt-1 text-xs leading-5 text-muted">
                只在需要不同语气或称呼的群里设置。
              </p>
            </div>
            <div class="rounded-lg border border-default bg-muted p-3">
              <strong class="text-xs text-highlighted">安全规则</strong>
              <p class="mt-1 text-xs leading-5 text-muted">
                风格提示词不能修改权限、真实性和工具边界。
              </p>
            </div>
          </div>
        </section>
      </div>
    </div>
  </section>
</template>
