import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

export function summaryItems(notes) {
  const escape = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;', '<':'&lt;', '>':'&gt;', '"':'&quot;', "'":'&#39;'}[c]));
  return notes.flat().map(note => {
    const card = note.noteCard;
    const id = String(card.noteId || note.id).split('?')[0];
    const link = new URL('https://www.xiaohongshu.com/explore/' + encodeURIComponent(id));
    const token = note.xsecToken || card.xsecToken;
    if (token) {
      link.searchParams.set('xsec_token', token);
      link.searchParams.set('xsec_source', 'pc_user');
    }
    const cover = card.cover?.urlDefault || card.cover?.url || card.cover?.infoList?.at(-1)?.url;
    const image = cover && /^https?:\/\//.test(cover) ? `<img src="${escape(cover)}"><br>` : '';
    const timestamp = typeof card.time === 'number' && card.time > 0 ? new Date(card.time) : null;
    const pubDate = timestamp && !Number.isNaN(timestamp.getTime()) ? timestamp.toISOString() : undefined;
    return { pubDate, title: card.displayTitle || '小红书笔记', link: link.href,
      guid: 'https://www.xiaohongshu.com/explore/' + encodeURIComponent(id),
      description: image + escape(card.displayTitle), author: card.user?.nickname || card.user?.nickName || '' };
  });
}

export async function renderNotes(notes, render, prefix, livePhoto) {
  const result = [];
  let summaryOnly = false;
  for (const note of notes.flat()) {
    if (!summaryOnly) {
      try {
        result.push(...await render([[note]], prefix, livePhoto));
        continue;
      } catch {
        // Stop detail requests after a failure to avoid a burst against the source.
        summaryOnly = true;
      }
    }
    result.push(...summaryItems([[note]]));
  }
  return result;
}

export function normalizeUser(user) {
  const userPageData = user?.userPageData?._rawValue || user?.userPageData;
  const notes = user?.notes?._rawValue || user?.notes;
  if (!userPageData?.basicInfo || !Array.isArray(notes)) {
    throw new Error('小红书登录抓取未取得用户资料，请检查服务器登录状态');
  }
  return { ...user, userPageData, notes };
}

export function patch(source) {
  const replacements = [
    ['let e=await o(f),t=', 'let e=xiaohongshuNormalizeUser(await o(f)),t='],
    ['t=await a(e.notes,`https://www.xiaohongshu.com/explore`,u)', 't=await xiaohongshuRenderNotes(e.notes,a,`https://www.xiaohongshu.com/explore`,u)'],
    ['catch{return await d(f,s)}', 'catch(error){throw error}'],
  ];
  for (const [before, after] of replacements) {
    if (source.split(before).length !== 2) throw new Error('RSSHub route changed; refusing to patch an unknown version');
    source = source.replace(before, after);
  }
  return source + '\n' + summaryItems.toString().replace('function summaryItems', 'function xiaohongshuSummaryItems') + '\n' + normalizeUser.toString().replace('function normalizeUser', 'function xiaohongshuNormalizeUser') + '\n' + renderNotes.toString().replace('function renderNotes', 'function xiaohongshuRenderNotes').replaceAll('summaryItems(', 'xiaohongshuSummaryItems(') + '\n';
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const dir = process.argv[2] || '/app/dist';
  const files = fs.readdirSync(dir).filter(file => /^xiaohongshu-user-.*\.mjs$/.test(file));
  if (files.length !== 1) throw new Error('Expected exactly one Xiaohongshu user route');
  const file = path.join(dir, files[0]);
  fs.writeFileSync(file, patch(fs.readFileSync(file, 'utf8')));
  console.log('Applied Xiaohongshu note summary fallback');
}
