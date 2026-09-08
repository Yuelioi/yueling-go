import { test } from 'node:test';
import assert from 'node:assert/strict';
import { patch, summaryItems } from './patch.mjs';
test('failed fulltext still produces stable, escaped note summaries', async () => {
  const notes = [[{ id:'abc', xsecToken:'token+example', noteCard:{displayTitle:'<title>',cover:{urlDefault:'https://example.com/image?a=1&b=2'},user:{nickname:'name'}} }]];
  const source = 'export async function render(input,a,u){const f=input,s=0,o=async value=>value,d=()=>{throw Error("anonymous fallback must not run")};try{let e=await o(f),t=await a(e.notes,`https://www.xiaohongshu.com/explore`,u);return t}catch{return await d(f,s)}}';
  const module = await import('data:text/javascript,' + encodeURIComponent(patch(source)));
  const result = await module.render({notes,userPageData:{basicInfo:{nickname:'name'}}}, async () => {throw new Error('missing note');}, false);
  await assert.rejects(module.render({notes:[],userPageData:{}}, async () => [], false), /登录/);
  assert.equal(result.length, 1);
  assert.equal(result[0].guid, 'https://www.xiaohongshu.com/explore/abc');
  assert.equal(new URL(result[0].link).searchParams.get('xsec_token'), 'token+example');
  assert.match(result[0].description, /&lt;title&gt;/);
  let attempts = 0;
  const multiple = await module.render({notes:[[...notes[0],...notes[0]]],userPageData:{basicInfo:{}}}, async () => {attempts++;throw Error('detail failed');}, false);
  assert.equal(attempts, 1);
  assert.equal(multiple.length, 2);
  assert.deepEqual(await module.render({notes,userPageData:{basicInfo:{nickname:'name'}}}, async () => ['fulltext'], false), ['fulltext']);
  assert.equal(summaryItems(notes)[0].guid, result[0].guid);
});
test('unknown upstream code fails closed', () => assert.throws(() => patch('changed'), /unknown version/));

test('empty login response remains an error instead of an empty successful feed', async () => {
  const { normalizeUser } = await import('./patch.mjs');
  assert.throws(() => normalizeUser({ userPageData: {}, notes: [] }), /登录/);
  assert.deepEqual(normalizeUser({userPageData:{_rawValue:{basicInfo:{nickname:'name'}}},notes:{_rawValue:[]}}).notes, []);
});

test('summary preserves actual publish time so a pinned old note cannot hide new notes', () => {
  const items = summaryItems([[{id:'pinned',noteCard:{displayTitle:'old pinned',time:1756493831000}},{id:'new',noteCard:{displayTitle:'new note',time:1788880465000}}]]);
  assert.equal(items[0].pubDate, new Date(1756493831000).toISOString());
  assert.equal(items[1].pubDate, new Date(1788880465000).toISOString());
  assert.equal([...items].sort((a,b)=>Date.parse(b.pubDate)-Date.parse(a.pubDate))[0].title,'new note');
  assert.equal(summaryItems([[{id:'undated',noteCard:{displayTitle:'unknown'}}]])[0].pubDate, undefined);
});
