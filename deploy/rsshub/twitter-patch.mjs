import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

export function timelinePosts(entries, parse, userID) {
  const seen = new Set();
  return parse(entries, ['profile-conversation-'], userID).filter(item => {
    if (item.in_reply_to_status_id_str || item.in_reply_to_status_id || !item.id_str || seen.has(item.id_str)) return false;
    seen.add(item.id_str);
    return true;
  });
}

export function patchTwitter(source) {
  const before = 'getUserTweets:(e,t)=>V(e,t,`getUserTweets`,async(e,t={})=>R(await I(`UserTweets`,e,{...t,count:20,includePromotedContent:!0,withQuickPromoteEligibilityTweetFields:!0,withVoice:!0,withV2Timeline:!0})))';
  const after = 'getUserTweets:(e,t)=>V(e,t,`getUserTweetsThreadsV1`,async(e,t={})=>yuelingTimelinePosts(await I(`UserTweets`,e,{...t,count:20,includePromotedContent:!0,withQuickPromoteEligibilityTweetFields:!0,withVoice:!0,withV2Timeline:!0}),R,e))';
  if (source.split(before).length !== 2) throw new Error('RSSHub Twitter implementation changed; refusing to patch an unknown version');
  return source.replace(before, after) + '\n' + timelinePosts.toString().replace('function timelinePosts', 'function yuelingTimelinePosts') + '\n';
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const dir = process.argv[2] || '/app/dist';
  const files = fs.readdirSync(dir).filter(file => /^twitter-api-.*\.mjs$/.test(file));
  const targets = files.filter(file => fs.readFileSync(path.join(dir, file), 'utf8').includes('getUserTweets:(e,t)=>V('));
  if (targets.length !== 1) throw new Error('Expected exactly one Twitter Web API implementation');
  const file = path.join(dir, targets[0]);
  fs.writeFileSync(file, patchTwitter(fs.readFileSync(file, 'utf8')));
  console.log('Applied Twitter conversation root extraction');
}
