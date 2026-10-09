import test from 'node:test'
import assert from 'node:assert/strict'
import { useImageAssets } from '../src/composables/useImageAssets.ts'
const deferred = () => { let resolve; const promise = new Promise(r => { resolve = r }); return { promise, resolve } }
const result = assets => ({ assets, total: assets.length, page: 1, page_size: 20 })
test('switching category discards old gallery results and selection', async () => {
 const first = deferred(), second = deferred()
 const state = useImageAssets({ imageAssets: category => category === 'a' ? first.promise : second.promise })
 state.selected.value = ['old']
 const old = state.changeCategory('a')
 const recent = state.changeCategory('b')
 second.resolve(result([{id:'b',name:'B'}])); await recent
 first.resolve(result([{id:'a',name:'A'}])); await old
 assert.equal(state.rows.value[0].id,'b'); assert.deepEqual(state.selected.value,[])
})
test('external sources never request previews/list or delete', async () => {
 let calls=0
 const state=useImageAssets({imageAssets:async()=>{calls++;return result([])},deleteImageAsset:async()=>{calls++}})
 state.categories.value=[{name:'外站',external:true}]
 await state.changeCategory('外站');await state.remove([{id:'x',name:'x'}])
 assert.equal(calls,0)
})
test('partial deletion reports actual results and never switches target category',async()=>{
 const calls=[]
 const state=useImageAssets({imageAssets:async()=>result([{id:'b',name:'B'}]),deleteImageAsset:async(category,id)=>{calls.push([category,id]);if(id==='b')throw new Error('失败')}})
 state.category.value='a';state.rows.value=[{id:'a',name:'A'},{id:'b',name:'B'}]
 await state.remove([...state.rows.value])
 assert.deepEqual(calls,[['a','a'],['a','b']]);assert.match(state.notice.value,/回收站 1 张/);assert.match(state.notice.value,/1 张删除失败/)
})
test('deletion completing after navigation does not reload or overwrite new category',async()=>{
 const pending=deferred();let reads=0
 const state=useImageAssets({imageAssets:async()=>{reads++;return result([{id:'new',name:'new'}])},deleteImageAsset:()=>pending.promise})
 state.category.value='old';state.rows.value=[{id:'old',name:'old'}]
 const deletion=state.remove([...state.rows.value]);await state.changeCategory('new')
 pending.resolve({ok:true});await deletion
 assert.equal(reads,1);assert.equal(state.rows.value[0].id,'new');assert.equal(state.notice.value,'')
})
