<script setup lang="ts">
import { computed, ref } from 'vue'
import catalog from '../../../services/wuxia/internal/content/six_professions.json'
import artManifest from '../data/wuxia-art.json'
import summary from '../../public/wuxia-concepts/balance-summary.json'
defineProps<{ balance?: boolean }>()
const selected = ref(catalog.professions[0]!.id)
const profession = computed(() =>
  catalog.professions.find((p) => p.id === selected.value)!,
)
const weapons = computed(() =>
  catalog.weapons
    .filter((w) => w.kind === profession.value.weapon_kind)
    .sort((a, b) => a.tier - b.tier),
)
const skills = computed(() =>
  catalog.skills
    .filter(
      (s) =>
        s.kind === 'martial' && s.weapon_kind === profession.value.weapon_kind,
    )
    .sort((a, b) => a.tier - b.tier),
)
const labels: Record<string, string> = {
  hp: '气血',
  attack: '攻击',
  defense: '防御',
  speed: '身法',
  critical: '暴击',
  critical_damage: '暴伤',
  dodge: '闪避',
  guard: '格挡',
  pierce: '穿透',
  combo: '连击',
  poison: '附毒',
  life_steal: '吸血',
}
const qualities = ['凡品', '精制', '上品', '名器', '传世']
const percent = (n: number) => `${+(n * 100).toFixed(2)}%`
const bonuses = (t: object) =>
  Object.entries(t)
    .filter(([, v]) => v)
    .map(([k, v]) => `${labels[k]} +${percent(v)}`)
    .join(' · ')
const stats = (t: object) =>
  Object.entries(t)
    .filter(([, v]) => v)
    .map(([k, v]) => `${labels[k]} +${v}`)
    .join(' · ')
const martialArt: Record<string, string> = artManifest.martial
</script>

<template>
  <section v-if="balance" class="space-y-6">
    <h2 class="font-serif text-2xl text-stone-100">六职业对战验证</h2>
    <p class="leading-8 text-stone-300">{{ summary.summary }}</p>
    <div class="grid gap-4 sm:grid-cols-3">
      <article
        v-for="p in summary.professions"
        :key="p.name"
        class="rounded-xl border border-stone-700 bg-stone-900/40 p-6"
      >
        <h3 class="text-stone-300">{{ p.name }}</h3>
        <p class="mt-3 text-3xl text-amber-100">{{ percent(p.win_rate) }}</p>
        <p class="mt-2 text-xs text-stone-400">各对手等权平均胜率</p>
      </article>
    </div>
    <p class="text-sm leading-7 text-stone-400">{{ summary.scope }}</p>
  </section>
  <section v-else class="space-y-6">
    <p class="text-sm leading-7 text-stone-400">
      六职业候选设计，尚未替换正式玩法。查看成长、兵器与武技；兵器与武技原画已补齐，尚未开放正式职业。
    </p>
    <nav class="flex flex-wrap gap-3" aria-label="职业选择">
      <button
        v-for="p in catalog.professions"
        :key="p.id"
        :aria-pressed="selected === p.id"
        class="rounded-lg border px-6 py-3 text-sm"
        :class="
          selected === p.id
            ? 'border-amber-200/50 bg-amber-200/10 text-amber-100'
            : 'border-stone-700 text-stone-400'
        "
        @click="selected = p.id"
      >
        {{ p.name }}
      </button>
    </nav>
    <header>
      <h2 class="font-serif text-2xl text-stone-100">
        {{ profession.name }} · {{ profession.weapon_kind }}
      </h2>
      <p class="mt-3 text-stone-400">{{ profession.description }}</p>
      <p class="mt-3 text-amber-100/80">{{ bonuses(profession.traits) }}</p>
    </header>
    <div class="overflow-x-auto rounded-xl border border-stone-700">
      <table class="w-full whitespace-nowrap text-left text-sm">
        <thead>
          <tr>
            <th class="px-5 py-4">裸身成长</th>
            <th v-for="(_, k) in profession.base" :key="k" class="px-5 py-4">
              {{ labels[k] }}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr class="border-t border-stone-700">
            <td class="px-5 py-4">每级增加</td>
            <td
              v-for="(n, k) in profession.per_level"
              :key="k"
              class="px-5 py-4"
            >
              +{{ n }}
            </td>
          </tr>
          <tr
            v-for="level in [1, 5, 10]"
            :key="level"
            class="border-t border-stone-700"
          >
            <td class="px-5 py-4">{{ level }}级</td>
            <td v-for="(n, k) in profession.base" :key="k" class="px-5 py-4">
              {{ n + (level - 1) * profession.per_level[k] }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="text-xs text-stone-400">
      不含练武与配装；概率词条为额外加成，暴伤基础175%。
    </p>
    <h2 class="font-serif text-2xl text-stone-100">兵器</h2>
    <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      <article
        v-for="w in weapons"
        :key="w.id"
        class="space-y-3 rounded-xl border border-stone-700 bg-stone-900/40 p-6"
      >
        <img
          v-if="w.image"
          :src="w.image"
          :alt="w.name"
          class="mx-auto aspect-square w-full max-w-64 object-contain"
          loading="lazy"
        />
        <p class="text-xs text-amber-200/70">{{ qualities[w.tier - 1] }}</p>
        <h3 class="font-serif text-xl">{{ w.name }}</h3>
        <p class="whitespace-pre-line text-sm leading-7 text-stone-300">
          {{ w.description }}
        </p>
        <p class="text-sm text-stone-400">
          {{
            [`攻击 +${w.attack}`, stats(w.stats), bonuses(w.traits)]
              .filter(Boolean)
              .join(' · ')
          }}
        </p>
        <p class="text-xs text-stone-400">{{ w.affinity }} · 无人物等级门槛</p>
      </article>
    </div>
    <h2 class="font-serif text-2xl text-stone-100">职业武技</h2>
    <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      <article
        v-for="s in skills"
        :key="s.id"
        class="space-y-3 rounded-xl border border-stone-700 bg-stone-900/40 p-6"
      >
        <img
          v-if="martialArt[s.id]"
          :src="martialArt[s.id]"
          :alt="s.name"
          class="mx-auto aspect-square w-full max-w-64 object-contain"
          loading="lazy"
        />
        <div
          v-if="martialArt[s.id]"
          class="flex items-center gap-4 border-t border-stone-700 pt-4"
        >
          <img
            v-for="size in [48, 72]"
            :key="size"
            :src="martialArt[s.id]"
            :alt="`${s.name} ${size}像素`"
            :width="size"
            :height="size"
            class="object-contain"
          />
          <span class="text-xs text-stone-400"
            >48 / 72 px<br />同一原画缩放</span
          >
        </div>
        <p class="text-xs text-amber-200/70">
          {{ ['入门', '精修', '上乘', '绝学', '传世'][s.tier - 1] }}
        </p>
        <h3 class="font-serif text-xl">{{ s.name }}</h3>
        <p class="whitespace-pre-line text-sm leading-7 text-stone-300">
          {{ s.description }}
        </p>
        <p class="text-sm leading-7 text-stone-400">
          <template v-if="s.ready_turn"
            >第{{ s.ready_turn }}次自身出手起，</template
          ><template v-if="s.every">每第{{ s.every }}次自身行动施展</template
          ><template v-else
            >每次自身行动{{ percent(s.chance || 0) }}概率施展</template
          >；{{ s.hits }}段 × {{ percent(s.damage || 0)
          }}<template v-if="s.pierce"
            >，忽略{{ percent(s.pierce) }}防御</template
          >。<template v-if="s.limit">每场最多{{ s.limit }}次。</template
          ><template v-else-if="s.cooldown"
            >再次施展前普攻{{ s.cooldown }}次。</template
          ><template v-if="s.guarantee_turn"
            >仍存活且轮到行动时，最迟第{{ s.guarantee_turn }}次首发。</template
          >
        </p>
        <p class="text-xs text-stone-400">
          {{ s.affinity }}兵器：武技增伤10%，追加普攻不继承。
        </p>
      </article>
    </div>
  </section>
</template>
