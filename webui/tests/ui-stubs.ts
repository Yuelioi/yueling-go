import { defineComponent, h } from 'vue'
const button = defineComponent({
  props: ['disabled', 'loading', 'type'],
  setup(props, { slots, attrs }) {
    return () =>
      h(
        'button',
        {
          ...attrs,
          disabled: props.disabled || props.loading,
          type: props.type || 'button',
        },
        slots.default?.(),
      )
  },
})
const input = defineComponent({
  props: ['modelValue', 'disabled', 'type'],
  emits: ['update:modelValue'],
  setup(props, { emit, attrs }) {
    return () =>
      h('input', {
        ...attrs,
        value: props.modelValue,
        disabled: props.disabled,
        type: props.type || 'text',
        onInput: (event: Event) =>
          emit('update:modelValue', (event.target as HTMLInputElement).value),
      })
  },
})
const select = defineComponent({
  props: ['modelValue', 'items', 'disabled'],
  emits: ['update:modelValue'],
  setup(props, { emit, attrs }) {
    return () =>
      h(
        'select',
        {
          ...attrs,
          value: props.modelValue,
          disabled: props.disabled,
          onChange: (event: Event) =>
            emit(
              'update:modelValue',
              (event.target as HTMLSelectElement).value,
            ),
        },
        props.items?.map(
          (item: { label: string; value: string; disabled?: boolean }) =>
            h(
              'option',
              { value: item.value, disabled: item.disabled },
              item.label,
            ),
        ),
      )
  },
})
const dialog = defineComponent({
  props: ['open', 'title'],
  setup(props, { slots }) {
    return () =>
      props.open
        ? h('section', { role: 'dialog' }, [
            h('h2', props.title),
            slots.body?.(),
            slots.footer?.(),
          ])
        : null
  },
})
const field = defineComponent({
  props: ['label'],
  setup(props, { slots }) {
    return () => h('label', [props.label, slots.default?.()])
  },
})
const collapsible = defineComponent({
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  },
})
export const uiStubs = {
  UButton: button,
  UInput: input,
  USelect: select,
  UFormField: field,
  USlideover: dialog,
  UModal: dialog,
  UCollapsible: collapsible,
  UAlert: true,
  UIcon: true,
  UCheckbox: true,
  USwitch: true,
}
