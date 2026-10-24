import { afterEach, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import WuxiaPreviewHub from '../src/components/WuxiaPreviewHub.vue'

const wrappers: ReturnType<typeof mount>[] = []
afterEach(() => {
  wrappers.forEach((w) => w.unmount())
  wrappers.length = 0
  window.history.replaceState(null, '', '/wuxia-preview.html')
})
it('keeps all previews in one hub and restores the selected tab from the URL', async () => {
  window.history.replaceState(null, '', '/wuxia-preview.html#professions')
  const wrapper = mount(WuxiaPreviewHub, {
    global: {
      stubs: {
        WuxiaWeaponGallery: true,
        WuxiaMartialGallery: true,
        UModal: true,
      },
    },
  })
  wrappers.push(wrapper)
  expect(wrapper.text()).toContain('裸身成长')
  const tabs = wrapper.findAll('nav[aria-label="预览内容"] button')
  expect(tabs.map((b) => b.text())).toEqual([
    '游戏界面',
    '装备图鉴',
    '武学图鉴',
    '职业与成长',
    '对战验证',
  ])
  await tabs.find((b) => b.text() === '对战验证')!.trigger('click')
  expect(window.location.hash).toBe('#balance')
  expect(wrapper.text()).toContain('六职业对战验证')
  await tabs.find((b) => b.text() === '装备图鉴')!.trigger('click')
  expect(window.location.hash).toBe('#equipment')
  expect(wrapper.find('wuxia-weapon-gallery-stub').exists()).toBe(true)
})
it('shows game-screen examples for all six candidate professions', () => {
  window.history.replaceState(null, '', '/wuxia-preview.html#screens')
  const wrapper = mount(WuxiaPreviewHub, {
    global: { stubs: { UModal: true } },
  })
  wrappers.push(wrapper)
  for (const name of ['问剑', '狂刀', '镇岳', '幽影', '千蛊', '弦歌']) {
    expect(wrapper.text()).toContain(name + ' · 全部配齐')
  }
})
