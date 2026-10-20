import type { NuxtUIOptions } from '@nuxt/ui/vite'

const fieldBase =
  'min-h-10 rounded-lg bg-muted text-highlighted ring-default placeholder:text-dimmed transition-shadow'
const menuContent =
  'rounded-xl bg-elevated p-1.5 shadow-xl shadow-black/20 ring-default'
const dialogSlots = {
  overlay: 'bg-black/65 backdrop-blur-sm',
  content:
    'rounded-2xl bg-elevated text-default shadow-2xl shadow-black/40 ring-accented divide-default',
  header: 'px-5 py-5 sm:px-6',
  body: 'p-5 sm:p-6',
  footer: 'justify-end bg-muted/40 px-5 py-4 sm:px-6',
  title: 'text-base font-semibold text-highlighted',
  description: 'mt-1.5 text-sm leading-6 text-muted',
}

export const uiTheme = {
  colors: { primary: 'violet', neutral: 'zinc' },
  button: {
    slots: { base: 'rounded-lg font-medium transition-colors' },
    variants: {
      size: {
        xs: { base: 'min-h-7 px-2 py-1 text-xs gap-1' },
        sm: { base: 'min-h-8 px-2.5 py-1.5 text-xs gap-1.5' },
        md: { base: 'min-h-10 px-3 py-2 text-sm gap-2' },
        lg: { base: 'min-h-11 px-4 py-2.5 text-sm gap-2' },
        xl: { base: 'min-h-12 px-4 py-3 text-base gap-2' },
      },
    },
  },
  card: {
    slots: {
      root: 'rounded-2xl bg-default shadow-sm shadow-black/10',
      header: 'p-5 sm:p-6',
      body: 'p-5 sm:p-6',
      footer: 'p-5 sm:p-6',
    },
  },
  input: { slots: { base: fieldBase } },
  inputNumber: { slots: { base: fieldBase } },
  textarea: {
    slots: {
      base: 'rounded-lg bg-muted text-highlighted ring-default placeholder:text-dimmed leading-6',
    },
  },
  select: {
    slots: {
      base: fieldBase,
      content: menuContent,
      item: 'min-h-9 rounded-lg',
    },
  },
  selectMenu: {
    slots: {
      base: fieldBase,
      content: menuContent,
      input: 'border-b border-default',
      item: 'min-h-10 rounded-lg',
      itemDescription: 'mt-0.5 text-xs text-muted',
    },
  },
  dropdownMenu: { slots: { content: menuContent, item: 'min-h-9 rounded-lg' } },
  formField: {
    slots: {
      label: 'text-sm font-medium text-highlighted',
      description: 'mt-1 text-xs leading-5 text-muted',
      container: 'mt-2',
    },
  },
  modal: { slots: dialogSlots },
  slideover: {
    slots: {
      ...dialogSlots,
      content: 'bg-elevated text-default ring-accented divide-default',
    },
  },
  alert: { slots: { root: 'rounded-xl', description: 'text-sm leading-6' } },
  tabs: {
    compoundVariants: [
      {
        orientation: 'horizontal',
        variant: 'link',
        class: { indicator: 'bottom-0' },
      },
    ],
    slots: {
      list: 'max-w-full overflow-y-hidden',
      trigger: 'shrink-0 whitespace-nowrap rounded-lg',
      content: 'focus-visible:outline-offset-4',
    },
  },
  navigationMenu: {
    slots: {
      link: 'min-h-10 rounded-xl px-3 py-2.5',
      label: 'px-3 text-[11px] font-medium tracking-wider text-dimmed',
    },
  },
} satisfies NonNullable<NuxtUIOptions['ui']>
