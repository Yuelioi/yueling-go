import { test } from 'node:test';
import assert from 'node:assert/strict';
import { timelinePosts, patchTwitter } from './twitter-patch.mjs';

test('conversation roots survive while replies and repeated posts are excluded', () => {
  const root = { id_str: '2109812216212099456', full_text: 'Day 8/ Reset' };
  const items = [ {id_str:'new'}, root, {id_str:'self-reply',in_reply_to_status_id_str:root.id_str}, root,
    {id_str:'other-reply',in_reply_to_status_id:123}, {id_str:'repost',retweeted_status:{id_str:'quoted'}}, {} ];
  const result = timelinePosts([], (_, prefixes, userID) => {
    assert.deepEqual(prefixes,['profile-conversation-']);
    assert.equal(userID,'author');
    return items;
  },'author');
  assert.deepEqual(result.map(x=>x.id_str),['new',root.id_str,'repost']);
  assert.deepEqual(timelinePosts([],()=>[],'author'),[]);
});

test('unknown or already patched bundles fail closed', () => {
  assert.throws(()=>patchTwitter('changed'),/unknown version/);
});
