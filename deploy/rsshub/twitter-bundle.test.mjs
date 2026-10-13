import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import { patchTwitter } from './twitter-patch.mjs';

const bundle = process.env.RSSHUB_TEST_BUNDLE || new URL('./twitter-parser.fixture.mjs', import.meta.url);
test('deployed parser recovers Day 8 from a conversation without leaking other authors or replies', async () => {
  const source = fs.readFileSync(bundle, 'utf8');
  const parser = source.slice(source.indexOf('function R('),source.indexOf('const z=',source.indexOf('function R(')));
  const member = code => code.slice(code.indexOf('getUserTweets:'),code.indexOf(',getUserTweetsAndReplies:'));
  const tweet = (id, author='author', reply) => ({entryId:'tweet-'+id, item:{itemContent:{tweet_results:{result:{rest_id:id, legacy:{full_text:id,in_reply_to_status_id_str:reply}, core:{user_results:{result:{rest_id:author}}}}}}}});
  const direct = tweet('latest'); direct.content=direct.item;
  const root = tweet('2109812216212099456');
  const entries = [direct,{entryId:'profile-conversation-1',content:{items:[root,tweet('reply','author',root.entryId),tweet('foreign','other'),root]}}];
  const run = async code => {
    const helper = code.slice(code.indexOf('\nfunction yuelingTimelinePosts'));
    const script = 'const V=(e,t,k,f)=>f(e,t),I=async()=>entries,L=(legacy,tweet)=>{legacy.user_id_str=tweet.core.user_results.result.rest_id};'+parser+';const H={'+member(code)+'};'+(code.includes('function yuelingTimelinePosts')?helper:'')+';H.getUserTweets("author",{})';
    return Array.from(await vm.runInNewContext(script,{entries}),x=>x.id_str);
  };
  assert.deepEqual(await run(source),['latest']);
  const patched = patchTwitter(source);
  assert.deepEqual(await run(patched),['latest','2109812216212099456']);
  assert.throws(()=>patchTwitter(patched),/unknown version/);
});
