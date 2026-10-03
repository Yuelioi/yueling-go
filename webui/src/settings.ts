export const settingsAreas = {
  ai: { path: '/ai', tab: 'settings', sections: ['ai'] },
  feeds: { path: '/feeds', tab: 'settings', sections: ['feed'] },
  plugins: { path: '/plugins', tab: 'settings', sections: ['plugins'] },
  bilibili: { path: '/bilibili', tab: 'account', sections: ['bilibili'] },
  images: { path: '/images', sections: ['image', 'pack'] },
  tools: { path: '/tools', sections: ['tools'] },
  connections: { path: '/connections', sections: ['napcat', 'http_api'] },
  system: { path: '/system', sections: ['bot', 'webui', 'database'] },
} as const

export function configurationTarget(section: unknown) {
  const area =
    Object.values(settingsAreas).find((area) =>
      area.sections.some((id) => id === section),
    ) ?? settingsAreas.system
  return { path: area.path, query: 'tab' in area ? { tab: area.tab } : {} }
}
