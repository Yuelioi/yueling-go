import { expect, it } from 'vitest'
import { defineComponent, h, onMounted } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import FeatureWorkspace from '../src/components/FeatureWorkspace.vue'
it('loads each tab on first visit and preserves its mounted draft when switching back', async () => {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { render: () => null } }],
  })
  await router.push('/')
  await router.isReady()
  const mounted: string[] = []
  const panel = (name: string) =>
    defineComponent({
      setup() {
        onMounted(() => mounted.push(name))
        return () => h('input', { value: name })
      },
    })
  const Tabs = defineComponent({
    props: ['items', 'modelValue'],
    emits: ['update:modelValue'],
    setup(props, { slots, emit }) {
      return () =>
        h(
          'div',
          props.items.flatMap((item: { label: string; value: string }) => [
            h(
              'button',
              { onClick: () => emit('update:modelValue', item.value) },
              item.label,
            ),
            slots.content?.({ item }),
          ]),
        )
    },
  })
  const wrapper = mount(FeatureWorkspace, {
    props: {
      title: 'Fixture',
      description: '',
      icon: '',
      tabs: [
        { label: 'First', value: 'first', icon: '' },
        { label: 'Second', value: 'second', icon: '' },
      ],
    },
    slots: { first: panel('first'), second: panel('second') },
    global: { plugins: [router], stubs: { PageHeader: true, UTabs: Tabs } },
  })
  await flushPromises()
  expect(mounted).toEqual(['first'])
  await wrapper.find('input').setValue('draft')
  await wrapper.findAll('button')[1]!.trigger('click')
  await flushPromises()
  expect(mounted).toEqual(['first', 'second'])
  await wrapper.findAll('button')[0]!.trigger('click')
  await flushPromises()
  expect(mounted).toEqual(['first', 'second'])
  expect(wrapper.find('input').element.value).toBe('draft')
  wrapper.unmount()
})
