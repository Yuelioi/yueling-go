<script setup lang="ts">
const groups = [
  {
    title: '出门走走',
    commands: [
      ['江湖', '一键建角，连续历练三次'],
      ['江湖历练', '完成今日剩余三次历练，也可发「历练」'],
      ['我的侠客', '查看角色、装备与武学'],
    ],
  },
  {
    title: '练功换装',
    commands: [
      ['江湖练武', '旧基础属性养成，每重 +1.5%；不是武技升级'],
      ['江湖转职 职业名', '切换职业，保留成长'],
      ['江湖武器 武器名', '切换已拥有的兵器'],
      ['江湖装备 物品名', '换上背包中的装备'],
      ['江湖学艺 秘籍名', '学习背包中的秘籍'],
      ['江湖武技 武技名', '选择出战武技，也可用「江湖武学」'],
      ['江湖心法 心法名', '选择修习心法'],
      ['江湖形象 编号或名称', '更换外观；填 0 则跟随职业'],
    ],
  },
  {
    title: '寻宝与行囊',
    commands: [
      ['江湖寻宝', '寻宝一次，也可用「江湖抽奖」'],
      ['江湖寻宝 5', '连续寻宝五次，可在次数前加奖池名'],
      ['江湖奖池', '查看当前可获得物品及概率'],
      ['江湖背包', '按品质查看收藏；重复物品按估价40%折银'],
    ],
  },
  {
    title: '会友与消遣',
    commands: [
      ['切磋 @群友', '每日首次主动切磋获100银两、20阅历，不论胜负'],
      ['江湖榜', '查看本群战力榜'],
      ['江湖赌场', '默认下注 100 银两；可填写其他赌金，最低 20'],
      ['江湖赌场帮助', '查看骰子规则与返还倍率'],
      ['江湖战报', '重看最近一次结果，不再次消耗或领奖'],
      ['江湖帮助', '查看完整玩法帮助'],
    ],
  },
]
</script>
<template>
  <section
    aria-label="游戏指令"
    class="mt-12 border-t border-stone-700/70 pt-8"
  >
    <div class="mb-6 space-y-2">
      <p class="text-xs tracking-[0.3em] text-amber-200/60">行走江湖</p>
      <h2 class="font-serif text-2xl text-stone-100">想做什么，群里说一声</h2>
      <p class="text-sm leading-7 text-stone-400">
        把下方指令直接发到群聊。换职业、武器、装备、武技、心法和形象时，也可以先不填名称，查看可选项。
      </p>
    </div>
    <div class="grid items-start gap-5 md:grid-cols-2">
      <section
        v-for="group in groups"
        :key="group.title"
        class="rounded-lg border border-stone-700/60 bg-[#14171b] p-5"
      >
        <h3 class="mb-4 font-serif text-lg text-amber-100">
          {{ group.title }}
        </h3>
        <dl class="space-y-4">
          <div
            v-for="[command, description] in group.commands"
            :key="command"
            class="space-y-1"
          >
            <dt class="text-sm font-medium text-stone-100 select-all">
              {{ command }}
            </dt>
            <dd class="text-xs leading-6 text-stone-400">{{ description }}</dd>
          </div>
        </dl>
      </section>
    </div>
  </section>
</template>
