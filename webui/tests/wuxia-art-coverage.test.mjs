import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync, existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
const read = (path) =>
  JSON.parse(readFileSync(new URL(path, import.meta.url), 'utf8'))
const catalog = read(
  '../../services/wuxia/internal/content/six_professions.json',
)
const art = read('../src/data/wuxia-art.json')
test('candidate weapons and skills all have real project artwork and QQ copies', () => {
  for (const weapon of catalog.weapons) {
    assert.ok(weapon.image, `${weapon.name}: missing artwork`)
    assert.equal(art.weapons[weapon.id], weapon.image)
    assert.ok(
      existsSync(
        fileURLToPath(new URL('../public' + weapon.image, import.meta.url)),
      ),
      weapon.image,
    )
  }
  for (const skill of catalog.skills) {
    const path = art.martial[skill.id]
    assert.ok(path, `${skill.name}: missing artwork`)
    assert.ok(
      existsSync(fileURLToPath(new URL('../public' + path, import.meta.url))),
      path,
    )
    assert.ok(
      existsSync(
        fileURLToPath(
          new URL(
            '../../services/wuxia/cards/assets/martial/' + skill.id + '.webp',
            import.meta.url,
          ),
        ),
      ),
      skill.id,
    )
  }
})

test('game cards have the same universal equipment art as the gallery', () => {
  const manifest = read(
    '../../services/wuxia/cards/assets/equipment/manifest.json',
  )
  for (const item of [
    ...catalog.weapons,
    ...catalog.items.filter(
      (i) => ['armor', 'charm'].includes(i.kind) && i.image,
    ),
  ]) {
    assert.ok(manifest[item.name], item.name)
    assert.ok(
      existsSync(
        fileURLToPath(
          new URL(
            '../../services/wuxia/cards/assets/equipment/' +
              manifest[item.name],
            import.meta.url,
          ),
        ),
      ),
      item.name,
    )
  }
  for (const p of catalog.professions) {
    for (const kind of ['profile', 'empty', 'adventure', 'draw', 'duel']) {
      assert.ok(
        existsSync(
          fileURLToPath(
            new URL(
              '../public/wuxia-preview/candidate-' +
                kind +
                '-' +
                p.id +
                '.webp',
              import.meta.url,
            ),
          ),
        ),
        kind + ' ' + p.name,
      )
    }
  }
})
