<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { PluginEntry, AIModelProfile, ImageEntry } from '../api'
import { useRuntimeConfiguration } from '../composables/useRuntimeConfiguration'
import ImageEntriesEditor from './ImageEntriesEditor.vue'
import AIProfilesEditor from './AIProfilesEditor.vue'

const props = withDefaults(
  defineProps<{ sections: readonly string[]; showBootstrap?: boolean }>(),
  { showBootstrap: false },
)

const {
  configuration,
  selected,
  values,
  cleared,
  plugins,
  loading,
  saving,
  error,
  success,
  active,
  dirty,
  load,
  save,
  pluginEnabled,
  setPlugin,
  addProfile,
} = useRuntimeConfiguration(props.sections)
const categories = computed(
  () =>
    configuration.value?.sections
      .filter((s) => props.sections.includes(s.id))
      .map((s) => ({
        label: s.title,
        value: s.id,
        disabled: saving.value || loading.value,
      })) ?? [],
)
const icons: Record<string, string> = {
  ai: 'i-tabler-sparkles',
  bot: 'i-tabler-robot',
  plugins: 'i-tabler-components',
  tools: 'i-tabler-tool',
  image: 'i-tabler-photo',
  pack: 'i-tabler-package',
  feed: 'i-tabler-rss',
  bilibili: 'i-tabler-brand-bilibili',
  napcat: 'i-tabler-plug',
  http_api: 'i-tabler-api',
  webui: 'i-tabler-browser',
  database: 'i-tabler-database',
}
const pluginQuery = ref('')
const pluginGroups = computed(() => {
  const grouped = new Map<string, PluginEntry[]>()
  const q = pluginQuery.value.trim().toLowerCase()
  for (const p of plugins.value) {
    if (q && !`${p.name} ${p.group} ${p.desc}`.toLowerCase().includes(q))
      continue
    const group = p.group || '其他'
    grouped.set(group, [...(grouped.get(group) ?? []), p])
  }
  return [...grouped].map(([title, items]) => ({ title, items }))
})
const enabledCount = computed(
  () => plugins.value.filter((p) => pluginEnabled(p.id)).length,
)
const fieldGroups = computed(() => {
  const definitions: Record<
    string,
    { title: string; description: string; paths: string[] }[]
  > = {
    ai: [
      {
        title: '输出限制',
        description: '设置模型输出长度和群聊回复长度。',
        paths: ['ai.max_tokens', 'ai.reply_max_chars', 'ai.reasoning_effort'],
      },
      {
        title: '上下文',
        description: '控制聊天和总结使用的消息数量。',
        paths: ['ai.context.chat_history', 'ai.context.summary'],
      },
      {
        title: '调用限流',
        description: '限制每分钟的 AI 调用次数，0 表示不限制。',
        paths: ['ai.ratelimit.user_per_min', 'ai.ratelimit.group_per_min'],
      },
      {
        title: '关系与好感度',
        description: '设置初始关系、分数范围和静默阈值。',
        paths: [
          'ai.affinity.enabled',
          'ai.affinity.initial',
          'ai.affinity.block_below',
          'ai.affinity.min',
          'ai.affinity.max',
        ],
      },
    ],
    tools: [
      {
        title: '天气服务',
        description: '连接和风天气。',
        paths: ['tools.qweather_host', 'tools.qweather_key'],
      },
      {
        title: '搜索服务',
        description: '为 AI 联网搜索配置访问凭据。',
        paths: ['tools.tavily_key'],
      },
      {
        title: '网络与表情包',
        description: '代理地址和表情包服务。',
        paths: ['tools.proxy', 'tools.meme_server'],
      },
    ],
    image: [
      {
        title: '图片转换',
        description: '控制保存图片的格式、大小和质量。',
        paths: [
          'image.convert',
          'image.convert_min_kb',
          'image.convert_quality',
        ],
      },
      {
        title: '素材类目',
        description: '自定义素材目录与调用命令。',
        paths: ['image.entry'],
      },
    ],
  }
  const fields = (active.value?.fields ?? []).filter(
    (f) =>
      !['ai.profiles', 'ai.conversation_profile', 'ai.vision_profile'].includes(
        f.path,
      ),
  )
  const used = new Set<string>()
  const groups = (definitions[selected.value] ?? [])
    .map((group) => ({
      ...group,
      fields: group.paths.flatMap((path) => {
        const field = fields.find((f) => f.path === path)
        if (!field) return []
        used.add(path)
        return [field]
      }),
    }))
    .filter((group) => group.fields.length)
  const rest = fields.filter((f) => !used.has(f.path))
  if (rest.length)
    groups.push({
      title: groups.length ? '其他设置' : '',
      description: '',
      paths: [],
      fields: rest,
    })
  return groups
})
onMounted(load)
</script>

<template>
  <div class="min-w-0 space-y-5">
    <UAlert
      color="primary"
      variant="soft"
      icon="i-tabler-info-circle"
      description="标记为「重启生效」的设置在下次启动后生效，其余设置立即生效。"
    />
    <UAlert
      v-if="configuration?.restart_pending"
      color="warning"
      variant="soft"
      description="配置已保存，但本次进程尚未全部应用。重启后刷新页面确认生效。"
      role="status"
    />
    <UAlert
      v-if="error"
      color="error"
      variant="soft"
      :description="error"
      role="alert"
    />
    <UAlert
      v-if="success"
      color="success"
      variant="soft"
      :description="success"
      role="status"
    />
    <USkeleton
      v-if="loading && !configuration"
      class="h-96 rounded-xl"
      aria-busy="true"
    />
    <UButton
      v-if="!configuration && !loading"
      variant="soft"
      icon="i-tabler-refresh"
      @click="load"
      >重新加载</UButton
    >
    <template v-if="configuration">
      <template v-if="categories.length > 1">
        <UFormField class="sm:hidden" label="设置分类"
          ><USelect
            v-model="selected"
            :items="categories"
            :disabled="saving || loading"
            class="w-full"
            aria-label="设置分类"
        /></UFormField>
        <UTabs
          :model-value="selected"
          :items="categories.map((c) => ({ ...c, icon: icons[c.value] }))"
          :content="false"
          variant="pill"
          class="hidden sm:flex"
          :ui="{ list: 'w-fit max-w-full' }"
          @update:model-value="selected = String($event)"
        />
      </template>
      <UCard
        v-if="active"
        id="configuration-panel"
        as="form"
        class="@container min-w-0"
        :ui="{ body: 'p-0 sm:p-0' }"
        @submit.prevent="save"
      >
        <template #header>
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2 class="text-base font-semibold text-highlighted">
                {{ active.title }}
              </h2>
              <p class="mt-1 text-xs text-muted">
                {{
                  selected === 'plugins'
                    ? `${enabledCount} / ${plugins.length} 个插件已开启`
                    : '当前设置独立保存，修改会写入数据库。'
                }}
              </p>
            </div>
            <div class="flex items-center gap-2">
              <UButton
                type="button"
                v-if="selected === 'ai'"
                icon="i-tabler-plus"
                color="primary"
                variant="soft"
                :disabled="
                  saving ||
                  loading ||
                  (values['ai.profiles']?.length ?? 0) >= 32
                "
                aria-label="添加模型 Profile"
                @click="addProfile"
              />
              <UBadge
                :color="dirty ? 'warning' : 'neutral'"
                variant="soft"
                :icon="dirty ? 'i-tabler-pencil' : 'i-tabler-check'"
                >{{ dirty ? '未保存' : '已保存' }}</UBadge
              >
            </div>
          </div>
        </template>
        <fieldset :disabled="saving || loading" class="min-w-0">
          <div v-if="selected === 'plugins'" class="space-y-6 p-5">
            <UInput
              v-model="pluginQuery"
              class="w-full"
              icon="i-tabler-search"
              placeholder="搜索插件名称或功能"
              aria-label="搜索插件"
            />
            <section
              v-for="group in pluginGroups"
              :key="group.title"
              class="space-y-3"
            >
              <div class="flex items-center gap-2">
                <h3 class="text-sm font-semibold text-highlighted">
                  {{ group.title }}
                </h3>
                <UBadge color="neutral" variant="soft" size="xs">{{
                  group.items.length
                }}</UBadge>
              </div>
              <div class="divide-y divide-default rounded-lg ring ring-default">
                <div
                  v-for="plugin in group.items"
                  :key="plugin.id"
                  class="flex items-start justify-between gap-4 p-4"
                >
                  <div class="min-w-0">
                    <p class="text-sm font-medium text-highlighted">
                      {{ plugin.name }}
                    </p>
                    <p class="mt-1 text-sm leading-6 text-muted">
                      {{ plugin.desc }}
                    </p>
                  </div>
                  <USwitch
                    class="mt-0.5 shrink-0"
                    :model-value="pluginEnabled(plugin.id)"
                    :disabled="saving || loading"
                    :aria-label="`${plugin.name}全局开关`"
                    @update:model-value="setPlugin(plugin.id, $event)"
                  />
                </div>
              </div>
            </section>
            <p
              v-if="!pluginGroups.length"
              class="py-8 text-center text-sm text-muted"
            >
              没有匹配的插件
            </p>
          </div>
          <div v-else class="divide-y divide-default">
            <AIProfilesEditor
              v-if="selected === 'ai'"
              v-model="values['ai.profiles'] as AIModelProfile[]"
              v-model:conversation="values['ai.conversation_profile']"
              v-model:vision="values['ai.vision_profile']"
              :disabled="saving || loading"
            />
            <section
              v-for="(group, index) in fieldGroups"
              :key="index"
              class="min-w-0 space-y-4 p-5"
            >
              <div v-if="group.title">
                <h3 class="text-sm font-semibold text-highlighted">
                  {{ group.title }}
                </h3>
                <p class="mt-1 text-xs leading-5 text-muted">
                  {{ group.description }}
                </p>
              </div>
              <div class="grid min-w-0 gap-5 @min-[36rem]:grid-cols-2">
                <UFormField
                  v-for="f in group.fields"
                  :key="f.path"
                  :label="f.label"
                  class="min-w-0"
                  :class="
                    f.kind === 'image_entries' ? '@min-[36rem]:col-span-2' : ''
                  "
                >
                  <template #label
                    ><span class="inline-flex flex-wrap items-center gap-2"
                      >{{ f.label
                      }}<UBadge
                        v-if="f.reload"
                        color="neutral"
                        variant="soft"
                        size="xs"
                        >重启生效</UBadge
                      ></span
                    ></template
                  >
                  <USwitch
                    v-if="f.kind === 'boolean'"
                    v-model="values[f.path]"
                    :disabled="saving || loading"
                    :aria-label="f.label"
                  />
                  <ImageEntriesEditor
                    v-else-if="f.kind === 'image_entries'"
                    v-model="values[f.path] as ImageEntry[]"
                  />
                  <template v-else-if="f.secret">
                    <UInput
                      v-model="values[f.path]"
                      type="password"
                      autocomplete="new-password"
                      :disabled="cleared[f.path] || saving || loading"
                      :placeholder="
                        f.configured ? '已设置，留空保留' : '尚未设置'
                      "
                      class="w-full"
                      :aria-label="f.label"
                    />
                    <UCheckbox
                      v-if="f.configured"
                      v-model="cleared[f.path]"
                      class="mt-2"
                      :label="`清除已保存的${f.label}`"
                    />
                  </template>
                  <UInput
                    v-else
                    v-model="values[f.path]"
                    :type="f.kind === 'number' ? 'number' : 'text'"
                    class="w-full"
                    :aria-label="f.label"
                  />
                </UFormField>
              </div>
            </section>
          </div>
        </fieldset>
        <template #footer
          ><div
            class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"
          >
            <p class="text-xs text-muted">
              {{
                dirty ? '修改尚未应用，请保存当前分类。' : '当前设置已同步。'
              }}
            </p>
            <UButton
              type="submit"
              icon="i-tabler-device-floppy"
              :loading="saving"
              :disabled="!dirty || loading"
              >保存{{ active.title }}</UButton
            >
          </div></template
        >
      </UCard>
      <UCollapsible v-if="showBootstrap">
        <UButton
          color="neutral"
          variant="ghost"
          icon="i-tabler-server"
          trailing-icon="i-tabler-chevron-down"
          >查看启动信息</UButton
        >
        <template #content
          ><UCard
            class="mt-3"
            :ui="{ body: 'space-y-2 text-xs leading-6 text-muted' }"
            ><p>{{ configuration.bootstrap.note }}</p>
            <p class="break-all">
              数据库连接：{{
                configuration.bootstrap.database_dsn_configured
                  ? '已配置'
                  : '未配置'
              }}
              · 数据目录：{{ configuration.bootstrap.data_dir }}
            </p></UCard
          ></template
        >
      </UCollapsible>
    </template>
  </div>
</template>
