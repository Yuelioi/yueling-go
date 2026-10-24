<script setup lang="ts">
import catalog from '../../../services/wuxia/internal/content/six_professions.json'
const tiers = [
  '凡品 / 入门',
  '精制 / 精修',
  '上品 / 上乘',
  '名器 / 绝学',
  '传世',
]
const rows = catalog.professions.map((p) => ({
  name: p.name,
  counts: tiers.map((_, index) => ({
    weapons: catalog.weapons.filter(
      (w) => w.kind === p.weapon_kind && w.tier === index + 1,
    ).length,
    martial: catalog.skills.filter(
      (s) =>
        s.kind === 'martial' &&
        s.weapon_kind === p.weapon_kind &&
        s.tier === index + 1,
    ).length,
  })),
}))
</script>

<template>
  <details
    class="rounded-xl border border-stone-700/60 bg-stone-900/40 px-4 py-3"
  >
    <summary class="cursor-pointer text-sm text-amber-100">
      六职业品级覆盖 · 查看兵器与武学数量
    </summary>
    <div class="mt-4 overflow-x-auto">
      <table
        aria-label="各职业品级覆盖"
        class="w-full min-w-[540px] text-center text-sm"
      >
        <thead>
          <tr class="text-stone-400">
            <th class="p-2 text-left">兵器 / 武技</th>
            <th v-for="tier in tiers" :key="tier" class="p-2">{{ tier }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="row in rows"
            :key="row.name"
            class="border-t border-stone-700/50"
          >
            <th class="p-2 text-left font-normal text-stone-200">
              {{ row.name }}
            </th>
            <td
              v-for="(count, i) in row.counts"
              :key="i"
              class="p-2 text-stone-300"
            >
              {{ count.weapons }} / {{ count.martial }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="mt-3 text-xs leading-6 text-stone-400">
      六职业每个品级都有兵器和武技。问剑、狂刀、镇岳各保留一件额外精制兵器，问剑额外保留万剑归宗。通用护具、饰品每品各展示六件，心法每品一门，全部职业共享。
    </p>
  </details>
</template>
