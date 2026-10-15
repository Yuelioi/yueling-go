<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type AIModelProfile, type AIConnection, type AIFeature } from '../api'

const profiles = defineModel<AIModelProfile[]>({ required: true })
const connections = defineModel<AIConnection[]>('connections', { required: true })
const bindings = defineModel<Record<string, string>>('bindings', { required: true })
const conversation = defineModel<string>('conversation', { required: true })
const vision = defineModel<string>('vision', { required: true })
const props = defineProps<{ disabled?: boolean; unsaved?: boolean }>()
const features = ref<AIFeature[]>([])
const error = ref('')
const testing = ref('')
const results = ref<Record<string, string>>({})
const auth = ref<Record<string, string>>({})
const replacing = ref<Record<string, boolean>>({})
const busyAuth = ref('')
const caps = [
  {value:'chat',label:'聊天'}, {value:'vision',label:'读图'},
  {value:'image_generation',label:'生成图片'}, {value:'image_edit',label:'编辑图片'},
  {value:'tools',label:'工具调用'}, {value:'structured_output',label:'结构化输出'},
]
const inputClass = 'w-full rounded-md border border-default bg-default px-3 py-2 text-sm outline-none focus:border-primary disabled:opacity-50'
const connectionOptions = computed(() => [{label:'选择连接',value:''}, ...(connections.value ?? []).map(c=>({label:c.name,value:c.id}))])
function capabilities(p: AIModelProfile) { return p.capabilities ?? ['chat','tools',...(p.vision ? ['vision'] : [])] }
function supports(p: AIModelProfile, required: string[]) { return !p.disabled && required.every(cap=>capabilities(p).includes(cap)) }
function options(required: string[], inherit = false) {
 return [
  ...(inherit ? [{label:'继承默认配置',value:''},{label:'关闭此功能',value:'off'}] : [{label:'不启用',value:''}]),
  ...profiles.value.filter(p=>supports(p,required)).map(p=>({label:p.name || '未命名模型',value:p.id})),
 ]
}
function setCapability(p: AIModelProfile, capability: string, checked: boolean) {
 const next=capabilities(p).filter(c=>c!==capability)
 if (checked) next.push(capability)
 p.capabilities=next; p.vision=next.includes('vision')
}
function isCodex(p: AIModelProfile) { return connections.value?.find(c=>c.id===p.connection_id)?.kind==='codex' }
function addConnection() {
 connections.value ??= []
 connections.value.push({id:crypto.randomUUID(),name:'新连接',kind:'openai',base_url:'',api_key:'',key_configured:false})
}
function changeKind(c: AIConnection) {
 if (c.kind === 'codex') { c.base_url=''; c.api_key=''; c.clear_key=true }
}
function removeConnection(id: string) {
 const used=profiles.value.filter(p=>p.connection_id===id)
 if(used.length) {error.value=`请先修改这些模型的连接：${used.map(p=>p.name).join('、')}`;return}
 connections.value=connections.value.filter(c=>c.id!==id)
}
function removeProfile(id: string) {
 const used=features.value.filter(f=>bindings.value?.[f.id]===id).map(f=>f.name)
 if(conversation.value===id) used.push('默认对话')
 if(vision.value===id) used.push('默认视觉')
 if(used.length){error.value=`请先换绑或关闭：${used.join('、')}`;return}
 profiles.value=profiles.value.filter(p=>p.id!==id)
}
async function test(p: AIModelProfile, cap: string) {
 testing.value=p.id+cap;error.value=''
 try { const r=await api.testAIProfile(p.id,cap);results.value[p.id+cap]=`${r.status==='verified'?'已验证':'测试失败'}：${r.message}（${r.duration_ms}ms）` }
 catch(e){results.value[p.id+cap]=e instanceof Error?e.message:'测试失败'}
 finally{testing.value=''}
}
async function status(c: AIConnection) {
 try { const r=await api.codexStatus(c.id);auth.value[c.id]=r.imported?`已导入 ${r.account_id ?? ''}；请测试模型确认可用`:'尚未导入登录态' }
 catch(e){auth.value[c.id]=e instanceof Error?e.message:'读取失败'}
}
async function importAuth(c: AIConnection, event: Event) {
 const input=event.target as HTMLInputElement;const file=input.files?.[0]
 if(!file)return
 busyAuth.value=c.id;error.value=''
 try {
  if(file.size>65536)throw new Error('登录文件不能超过64KB')
  const r=await api.importCodex(c.id,await file.text(),!!replacing.value[c.id])
  auth.value[c.id]=`已导入 ${r.account_id ?? ''}；服务器将维护后续刷新`
 }catch(e){error.value=e instanceof Error?e.message:'导入失败'}
 finally{busyAuth.value='';input.value=''}
}
watch([profiles,connections],()=>{results.value={}},{deep:true})
onMounted(async()=>{try{features.value=(await api.aiFeatures()).features}catch(e){error.value=e instanceof Error?e.message:'功能列表加载失败'}})
</script>

<template>
 <section class="min-w-0 space-y-6 p-5">
  <UAlert v-if="error" color="error" :description="error" @close="error=''" />
  <div class="space-y-3">
   <div class="flex items-center justify-between gap-3"><h3 class="font-semibold text-highlighted">连接账号</h3><UButton size="sm" variant="soft" :disabled="disabled" @click="addConnection">添加连接</UButton></div>
   <p class="text-xs text-muted">地址和凭据在这里配置，同一连接可供多个模型使用。</p>
   <details v-for="c in connections" :key="c.id" class="rounded-lg border border-default p-4" open>
    <summary class="cursor-pointer font-medium">{{c.name}}</summary>
    <div class="mt-4 grid gap-4 @min-[36rem]:grid-cols-2">
     <UFormField label="连接名称"><input v-model="c.name" :class="inputClass" :disabled="disabled" /></UFormField>
     <UFormField label="接入方式"><select v-model="c.kind" :class="inputClass" :disabled="disabled" @change="changeKind(c)"><option value="openai">OpenAI 兼容 API</option><option value="codex">Codex 订阅</option></select></UFormField>
     <template v-if="c.kind==='openai'">
      <UFormField label="API 地址"><input v-model="c.base_url" placeholder="https://api.example.com/v1" :class="inputClass" :disabled="disabled" /></UFormField>
      <UFormField :label="c.key_configured?'API Key（已设置，留空保留）':'API Key（本地服务可留空）'"><input v-model="c.api_key" type="password" autocomplete="new-password" :class="inputClass" :disabled="disabled" /></UFormField>
      <UCheckbox v-model="c.clear_key" label="清除已保存的 Key" :disabled="disabled" />
     </template>
     <div v-else class="space-y-3 @min-[36rem]:col-span-2">
      <p class="text-xs text-muted">先保存连接，再选择本机 Codex 的 auth.json 导入。文件只用于订阅认证。服务器需安装 Codex CLI。</p>
      <p v-if="c.api_key || c.key_configured || c.base_url" class="text-sm text-warning">保存后会清除该连接原有的 API 地址和 Key，改用订阅登录态。</p>
      <div class="flex flex-wrap items-center gap-3">
       <input type="file" accept=".json,application/json" class="max-w-full text-sm" :disabled="disabled || unsaved || !!busyAuth" @change="importAuth(c,$event)" />
       <UButton size="sm" variant="soft" :disabled="disabled || unsaved" @click="status(c)">检查导入状态</UButton>
      </div>
      <UCheckbox v-model="replacing[c.id]" label="替换已有登录态（先暂停该连接的请求）" :disabled="disabled" />
      <p v-if="auth[c.id]" class="break-all text-sm text-muted">{{auth[c.id]}}</p>
     </div>
    </div>
    <div class="mt-3 flex items-center justify-between gap-3"><code class="break-all text-xs text-muted">ID: {{c.id}}</code><UButton size="xs" color="error" variant="ghost" :disabled="disabled" @click="removeConnection(c.id)">删除连接</UButton></div>
   </details>
  </div>

  <div class="space-y-3">
   <h3 class="font-semibold text-highlighted">模型配置</h3>
   <p class="text-xs text-muted">勾选能力后保存，再逐项测试。测试会实际消耗对应账号额度，生图测试也会生成一张测试图片。</p>
   <details v-for="p in profiles" :key="p.id" class="rounded-lg border border-default p-4" open>
    <summary class="cursor-pointer font-medium">{{p.name}} <span v-if="p.disabled" class="text-muted">（已停用）</span></summary>
    <div class="mt-4 grid gap-4 @min-[36rem]:grid-cols-2">
     <UFormField label="配置名称"><input v-model="p.name" :class="inputClass" :disabled="disabled" /></UFormField>
     <UFormField label="所属连接"><select v-model="p.connection_id" :class="inputClass" :disabled="disabled"><option v-for="o in connectionOptions" :key="o.value" :value="o.value">{{o.label}}</option></select></UFormField>
     <UFormField label="模型 ID"><input v-model="p.model" :class="inputClass" :disabled="disabled" placeholder="填写该账号可用的模型 ID" /></UFormField>
     <UFormField v-if="capabilities(p).includes('chat')" label="推理强度"><select v-model="p.reasoning_effort" :class="inputClass" :disabled="disabled"><option value="">继承默认</option><option v-for="v in ['none','minimal','low','medium','high','xhigh']" :key="v" :value="v">{{v}}</option></select></UFormField>
     <UFormField v-if="!isCodex(p) && capabilities(p).includes('chat')" label="最大输出 Token（0 继承）"><input v-model.number="p.max_tokens" type="number" min="0" max="200000" :class="inputClass" :disabled="disabled" /></UFormField>
     <UFormField label="请求超时秒数（0 使用默认）"><input v-model.number="p.timeout_seconds" type="number" min="0" max="600" :class="inputClass" :disabled="disabled" /></UFormField>
    </div>
    <div class="mt-4 grid gap-3 @min-[36rem]:grid-cols-2">
     <div v-for="cap in caps" :key="cap.value" class="space-y-1">
      <div class="flex items-center justify-between gap-2">
       <UCheckbox :model-value="capabilities(p).includes(cap.value)" :label="cap.label" :disabled="disabled || (isCodex(p) && !capabilities(p).includes(cap.value) && ['image_generation','image_edit','structured_output'].includes(cap.value))" @update:model-value="setCapability(p,cap.value,$event===true)" />
       <UButton v-if="capabilities(p).includes(cap.value)" size="xs" variant="ghost" :loading="testing===p.id+cap.value" :disabled="disabled || unsaved || !!testing || p.disabled" @click="test(p,cap.value)">测试</UButton>
      </div>
      <p v-if="results[p.id+cap.value]" class="text-xs text-muted">{{results[p.id+cap.value]}}</p>
     </div>
    </div>
    <div class="mt-4 flex items-center justify-between gap-3"><UCheckbox v-model="p.disabled" label="停用此配置" :disabled="disabled" /><UButton size="xs" color="error" variant="ghost" :disabled="disabled" @click="removeProfile(p.id)">删除配置</UButton></div>
   </details>
  </div>

  <div class="space-y-4">
   <h3 class="font-semibold text-highlighted">功能模型分配</h3>
   <div class="grid gap-4 @min-[36rem]:grid-cols-2">
    <UFormField label="默认对话模型"><select v-model="conversation" :class="inputClass" :disabled="disabled"><option v-for="o in options(['chat','tools'])" :key="o.value" :value="o.value">{{o.label}}</option></select></UFormField>
    <UFormField label="默认视觉模型"><select v-model="vision" :class="inputClass" :disabled="disabled"><option v-for="o in options(['chat','vision'])" :key="o.value" :value="o.value">{{o.label}}</option></select></UFormField>
    <UFormField v-for="f in features" :key="f.id" :label="f.name"><select :value="bindings?.[f.id] ?? ''" :class="inputClass" :disabled="disabled" @change="bindings = {...bindings,[f.id]:($event.target as HTMLSelectElement).value}"><option v-for="o in options(f.capabilities,true)" :key="o.value" :value="o.value">{{o.label}}</option></select></UFormField>
   </div>
   <p class="text-xs text-muted">文本功能继承默认对话，读图继承默认视觉；生图和编辑图片需要单独选择配置。关闭后不会自动换用其他模型。</p>
   <p v-if="unsaved" class="text-xs text-warning">有未保存的修改。保存后可导入订阅登录态、测试能力。</p>
  </div>
 </section>
</template>
