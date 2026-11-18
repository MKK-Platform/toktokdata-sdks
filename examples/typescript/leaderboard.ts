// Top 10 of an official TikTok LIVE board for one region.
// Usage: npx tsx leaderboard.ts [REGION] [BOARD]     e.g. npx tsx leaderboard.ts VN hourly
// Scope: rank:read
import { TokTokData } from '@toktokdata.live/sdk';
import { exitWithError } from './common.js';

interface Entry {
  rank: number;
  uniqueId: string | null;
  nickname: string | null;
  score: number;
  isLive: boolean;
  masked: boolean;
  unmaskTier: string | null;
  movement: string | null;
}
interface OfficialBoard {
  data: {
    board: {
      board: string;
      region: string;
      periodKey: string;
      publishable: boolean;
      coverage: { ratio: number; threshold: number };
      entries: Entry[];
    };
  };
}

const region = process.argv[2] ?? 'VN';
const boardKey = process.argv[3] ?? 'hourly';

const client = new TokTokData(); // reads TOKTOKDATA_API_KEY

try {
  const res = (await client.rankings.official(boardKey, { region, limit: 10 })) as unknown as OfficialBoard;
  const { board } = res.data;

  console.log(`${board.board} board, ${board.region}, period ${board.periodKey}`);
  if (!board.publishable) {
    // Coverage below the gate: the order may still change, so label it as partial.
    const pct = (n: number) => `${Math.round(n * 100)}%`;
    console.log(`(partial: coverage ${pct(board.coverage.ratio)} of the ${pct(board.coverage.threshold)} gate)`);
  }
  for (const e of board.entries) {
    const who = e.masked ? `(masked, plan ${e.unmaskTier})` : `${e.nickname ?? ''} @${e.uniqueId}`;
    const live = e.isLive ? '  LIVE' : '';
    console.log(`${String(e.rank).padStart(3)}  ${who.padEnd(40)} ${e.score.toLocaleString('en-US').padStart(12)}${live}`);
  }

  const rl = client.lastRateLimit;
  if (rl) console.log(`\nrate limit: ${rl.remaining} of ${rl.limit} left, ${rl.dailyRemaining} left today`);
} catch (err) {
  exitWithError(err);
}
