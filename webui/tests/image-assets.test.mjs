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

test('changing sort resets pagination and forwards order to the server', async () => {
 const calls=[]
 const state=useImageAssets({imageAssets:async(...args)=>{calls.push(args);return result([])}})
 state.category.value='表情';state.page.value=3;state.selected.value=['old']
 await state.changeSort('modified_asc')
 assert.deepEqual(calls,[['表情','',1,'modified_asc']])
 assert.equal(state.page.value,1);assert.deepEqual(state.selected.value,[])
})
test('rename uses the selected asset then refreshes IDs and selection', async () => {
 const calls=[]
 const state=useImageAssets({imageAssets:async()=>result([{id:'new',name:'新名.png'}]),renameImageAsset:async(...args)=>{calls.push(args)}})
 state.category.value='表情';state.rows.value=[{id:'old',name:'旧名.png'}];state.selected.value=['old']
 assert.equal(await state.rename(state.rows.value[0],' 新名 '),true)
 assert.deepEqual(calls,[['表情','old','新名']]);assert.deepEqual(state.selected.value,[])
 assert.equal(state.rows.value[0].id,'new');assert.match(state.notice.value,/已重命名/)
})
test('failed rename keeps the original asset and exposes the failure',async()=>{
 const state=useImageAssets({renameImageAsset:async()=>{throw new Error('目标文件冲突')}})
 state.category.value='表情';state.rows.value=[{id:'old',name:'旧名.png'}]
 assert.equal(await state.rename(state.rows.value[0],'新名'),false)
 assert.equal(state.rows.value[0].id,'old');assert.equal(state.error.value,'目标文件冲突')
 assert.equal(state.renaming.value,false)
})
test('rename completing after navigation does not reload or overwrite the new view',async()=>{
 const pending=deferred();let reads=0
 const state=useImageAssets({imageAssets:async()=>{reads++;return result([{id:'new',name:'new'}])},renameImageAsset:()=>pending.promise})
 state.category.value='old';state.rows.value=[{id:'old',name:'old'}]
 const rename=state.rename(state.rows.value[0],'renamed');await state.changeCategory('new')
 pending.resolve({ok:true});await rename
 assert.equal(reads,1);assert.equal(state.rows.value[0].id,'new');assert.equal(state.notice.value,'')
})
