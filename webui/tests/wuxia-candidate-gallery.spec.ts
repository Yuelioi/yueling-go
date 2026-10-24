import { expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import WuxiaWeaponGallery from '../src/components/WuxiaWeaponGallery.vue'
import WuxiaMartialGallery from '../src/components/WuxiaMartialGallery.vue'

it('shows all six weapon families and universal new attributes', async () => {
  const w = mount(WuxiaWeaponGallery, { global: { stubs: { UModal: true } } })
  for (const [name, item] of [
    ['幽影', '铁匕'],
    ['千蛊', '陶蛊盅'],
    ['弦歌', '桐木琴'],
  ]) {
    await w
      .findAll('nav[aria-label="兵器职业"] button')
      .find((b) => b.text() === name)!
      .trigger('click')
    expect(w.text()).toContain(item)
  }
  await w
    .findAll('nav[aria-label="装备分类"] button')
    .find((b) => b.text() === '通用饰品')!
    .trigger('click')
  for (const label of ['连击', '暴伤', '附毒', '吸血'])
    expect(w.text()).toContain(label)
  expect(w.findAll('button.weapon-card')).toHaveLength(30)
  expect(w.findAll('nav[aria-label="品级导航"] a')).toHaveLength(0)
  w.unmount()
})
it('shows five tiers for every new profession without broken artwork', async () => {
  const w = mount(WuxiaMartialGallery, { global: { stubs: { UModal: true } } })
  for (const name of ['幽影', '千蛊', '弦歌']) {
    await w
      .findAll('nav[aria-label="武学分类"] button')
      .find((b) => b.text() === name)!
      .trigger('click')
    expect(w.findAll('button[aria-label^="查看"]')).toHaveLength(5)
    expect(w.findAll('img')).toHaveLength(5)
    expect(w.find('img').attributes('src')).toContain('/martial-art/')
  }
  w.unmount()
})
it('makes all-profession coverage and per-tier totals explicit', () => {
  const w = mount(WuxiaWeaponGallery, { global: { stubs: { UModal: true } } })
  expect(w.text()).toContain('六职业 · 33 件兵器')
  expect(w.findAll('#quality-1 button.weapon-card')).toHaveLength(6)
  expect(w.find('#quality-1').text()).toContain('6 件')
  expect(w.findAll('[aria-label="各职业品级覆盖"] tbody tr')).toHaveLength(6)
  w.unmount()
})

it('shows all mind effects and martial mechanics without opening a modal', async () => {
  const w = mount(WuxiaMartialGallery, { global: { stubs: { UModal: true } } })
  await w.findAll('nav[aria-label="武学分类"] button').find(b => b.text() === '心法')!.trigger('click')
  expect(w.findAll('[data-testid="skill-mechanics"]')).toHaveLength(5)
  for (const text of ['气血 +25%', '攻击 +20%', '防御 +30%', '回复 5%', '伤害降低 20%', '穿透 15%']) expect(w.text()).toContain(text)
  await w.findAll('nav[aria-label="武学分类"] button').find(b => b.text() === '镇岳')!.trigger('click')
  expect(w.findAll('[data-testid="skill-mechanics"]')).toHaveLength(5)
  expect(w.text()).toContain('概率')
  expect(w.text()).not.toMatch(/最迟第|每第|每场最多|再次施展前|次自身出手起/)
  w.unmount()
})
