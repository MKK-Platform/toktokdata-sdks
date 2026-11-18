// Creator report: profile, content analysis and recent LIVE sessions.
// Usage: npx tsx creator-report.ts [HANDLE]     e.g. npx tsx creator-report.ts tiktok
// Scopes: creator:read (profile, analysis), live:read (sessions)
import { TokTokData } from '@toktokdata.live/sdk';
import { exitWithError } from './common.js';

interface Profile {
  data: {
    creator: {
      uniqueId: string;
      displayName: string | null;
      region: string | null;
      verified: boolean;
      followerCount: number | null;
      followingCount: number | null;
      awemeCount: number | null;
      totalFavorited: number | null;
      lastRefreshedAt: string | null;
    };
  };
}
interface Analysis {
  data: {
    sample: { videosAnalyzed: number; spanDays: number };
    engagement: { avgPlays: number; medianPlays: number; engagementRate: number } | null;
    cadence: { postsPerWeek: number } | null;
    limits: { code: string; message: string }[];
  };
}
interface SessionsPage {
  data: {
    items: {
      id: string;
      startedAt: string;
      endedAt: string | null;
      durationSeconds: number | null;
      peakViewerCount: number | null;
      giftCount: number | null;
      totalGiftDiamonds: number | null;
    }[];
    nextCursor: string | null;
  };
}

const handle = (process.argv[2] ?? 'tiktok').replace(/^@/, '');
const MAX_SESSIONS = 30;
const fmt = (n: number | null | undefined) => (n == null ? '-' : n.toLocaleString('en-US'));

const client = new TokTokData();

try {
  // A creator seen for the first time answers 202 "resolving"; get() waits and asks again.
  const { creator } = ((await client.creators.get(handle)) as unknown as Profile).data;
  console.log(`${creator.displayName ?? creator.uniqueId} (@${creator.uniqueId})${creator.verified ? ' [verified]' : ''}`);
  console.log(`  region ${creator.region ?? '?'}, ${fmt(creator.followerCount)} followers, ${fmt(creator.awemeCount)} videos, ${fmt(creator.totalFavorited)} likes`);
  console.log(`  last refreshed ${creator.lastRefreshedAt ?? '-'}`);

  const analysis = ((await client.creators.analysis(handle)) as unknown as Analysis).data;
  console.log(`\nContent (${analysis.sample.videosAnalyzed} videos over ${analysis.sample.spanDays} days)`);
  if (analysis.engagement) {
    const e = analysis.engagement;
    console.log(`  plays avg ${fmt(e.avgPlays)}, median ${fmt(e.medianPlays)}, engagement ${(e.engagementRate * 100).toFixed(1)}%`);
  }
  if (analysis.cadence) console.log(`  ${analysis.cadence.postsPerWeek} posts per week`);
  for (const limit of analysis.limits) console.log(`  note: ${limit.message}`);

  console.log(`\nRecent LIVE sessions (up to ${MAX_SESSIONS})`);
  let shown = 0;
  // The iterator follows data.nextCursor page by page until it is null.
  pages: for await (const page of client.live.iterSessions({ creatorUniqueId: creator.uniqueId, limit: 20 })) {
    for (const s of (page as unknown as SessionsPage).data.items) {
      const minutes = s.durationSeconds == null ? '-' : Math.round(s.durationSeconds / 60);
      console.log(`  ${s.startedAt}  ${String(minutes).padStart(4)} min  peak ${fmt(s.peakViewerCount).padStart(7)} viewers  ${fmt(s.totalGiftDiamonds).padStart(9)} diamonds`);
      if (++shown >= MAX_SESSIONS) break pages;
    }
  }
  if (shown === 0) console.log('  no recorded sessions');
} catch (err) {
  exitWithError(err);
}
