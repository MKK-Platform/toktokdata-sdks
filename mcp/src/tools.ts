import type { TokTokData } from "@toktokdata.live/sdk";
import { z } from "zod";

export type Credits = "cached-read" | "free";

export interface RunContext {
  /** Wait budget for 202 "still collecting" answers, in ms. */
  maxWaitMs: number;
}

/** One read-only MCP tool, mapped onto one public GET operation of the API. */
export interface ToolDef {
  name: string;
  title: string;
  /** What the tool returns (scope and credit hints are appended). */
  summary: string;
  /** The API operation, `METHOD /path` as in openapi.json. */
  operation: string;
  /** Scope the API key needs (`x-scope`). */
  scope: string;
  /** Credit class (`x-credits`). */
  credits: Credits;
  /** True when the route may answer 202 and the tool waits for the data. */
  waits?: boolean;
  input: z.ZodRawShape;
  // Args are validated against `input` by the MCP SDK before `run` is called.
  run(client: TokTokData, args: any, ctx: RunContext): Promise<unknown>;
}

// ------------------------------------------------------------- shared params

const uid = z
  .string()
  .min(1)
  .describe("Creator handle without `@` (e.g. `linh.live`) or numeric TikTok user id.");
const region = (fallback: string) =>
  z.string().min(2).optional().describe(`ISO 3166-1 alpha-2 region code (default \`${fallback}\`).`);
const cursor = z.string().optional().describe("Opaque cursor from `nextCursor` of the previous page.");
const limit = (max: number, fallback?: number, what = "Maximum rows") =>
  z
    .number()
    .int()
    .min(1)
    .max(max)
    .optional()
    .describe(`${what}, 1–${max}${fallback !== undefined ? ` (default ${fallback})` : ""}.`);
const freeLimit = (what: string) => z.number().int().min(1).optional().describe(`${what}.`);
const boardKey = z
  .string()
  .min(1)
  .describe(
    "Board key: `daily`, `hourly`, `popular_live`, `gaming_daily`, `gaming_weekly`, `shopping_daily`, " +
      "`shopping_weekly` (shopping boards need the `shop:read` scope), or a league board `league_a1` … `league_d5`.",
  );
const leagueTier = z.string().optional().describe("League tier filter, `A1` … `D5`.");
const window = z.enum(["7d", "30d"]).optional().describe("`7d` (default) or `30d`.");
const period = z
  .string()
  .regex(/^\d{4}-\d{2}-01$/, "Use YYYY-MM-01")
  .optional()
  .describe("Billing period start, `YYYY-MM-01`; defaults to the current one.");
const sessionId = z.string().min(1).describe("Session id from `live_sessions`.");

const wait = (ctx: RunContext) => ({ maxWaitMs: ctx.maxWaitMs });

// ------------------------------------------------------------- tools

export const TOOLS: ToolDef[] = [
  // Rankings
  {
    name: "rankings_official",
    title: "Official TikTok LIVE board",
    summary:
      "One official TikTok LIVE leaderboard for one region, as TikTok publishes it: rank, score, creator, live status, " +
      "movement and a 7-period sparkline. Check `publishable` and `coverage` before trusting entries; rows beyond the " +
      "plan's unmask tier come back `masked: true` without identity.",
    operation: "GET /v1/rankings/official",
    scope: "rank:read",
    credits: "cached-read",
    input: {
      board: boardKey,
      region: region("VN"),
      period: z
        .enum(["current", "history"])
        .optional()
        .describe("`current` is the running period (default), `history` the last completed one."),
      limit: limit(200, 200, "Maximum entries"),
      liveOnly: z.boolean().optional().describe("Only creators live right now."),
      leagueTier,
    },
    run: (c, { board, ...a }) => c.rankings.official(board, a),
  },
  {
    name: "rankings_regions",
    title: "Covered ranking regions",
    summary: "Regions that currently have observed boards, with a display label, coverage and the boards offered.",
    operation: "GET /v1/rankings/regions",
    scope: "rank:read",
    credits: "cached-read",
    input: {},
    run: (c) => c.rankings.regions(),
  },
  {
    name: "rankings_history",
    title: "Past periods of a board",
    summary:
      "Completed periods of one board, newest first, within the plan's history window (one page; pass `nextCursor` " +
      "as `cursor` for the next).",
    operation: "GET /v1/rankings/history",
    scope: "rank:read",
    credits: "cached-read",
    input: { board: boardKey, region: region("VN"), cursor, limit: limit(24, 24, "Periods per page"), leagueTier },
    run: (c, { board, ...a }) => c.rankings.history(board, a),
  },
  {
    name: "rankings_day_bars",
    title: "Daily bar of a board",
    summary:
      "For each board day of the last `days` days: how many creators the board listed, the lowest score on it (the " +
      "bar to make the board) and the top score. `daily` and the league boards only.",
    operation: "GET /v1/rankings/day-bars",
    scope: "rank:read",
    credits: "cached-read",
    input: {
      region: region("VN"),
      board: z.string().optional().describe("`daily` (default) or a league board `league_a1` … `league_d5`."),
      days: z.number().int().min(1).max(90).optional().describe("Days back, 1–90 (default 30)."),
    },
    run: (c, a) => c.rankings.dayBars(a),
  },

  // Creators
  {
    name: "creators_search",
    title: "Search creators",
    summary:
      "Stored creators whose handle or display name contains `q` (case-insensitive), most-followed first. An empty " +
      "`q` browses the stored set.",
    operation: "GET /v1/creators",
    scope: "creator:read",
    credits: "cached-read",
    input: {
      q: z.string().optional().describe("Handle (with or without `@`) or part of the display name."),
      limit: limit(100, 20, "Maximum results"),
    },
    run: (c, a) => c.creators.search(a),
  },
  {
    name: "creators_get",
    title: "Creator profile",
    summary:
      "The public profile of a creator (followers, likes, videos, bio, verification, region) and its recent counter " +
      "history. A creator not stored yet is looked up on TikTok first.",
    operation: "GET /v1/creators/{uid}",
    scope: "creator:read",
    credits: "cached-read",
    waits: true,
    input: { uid },
    run: (c, { uid: id }, ctx) => c.creators.get(id, wait(ctx)),
  },
  {
    name: "creators_analysis",
    title: "Creator video analysis",
    summary:
      "Engagement report computed from the creator's stored videos: totals, medians, posting cadence, plus up to " +
      "five coded findings (`read`) and what the report cannot cover (`limits`).",
    operation: "GET /v1/creators/{uid}/analysis",
    scope: "creator:read",
    credits: "cached-read",
    input: { uid },
    run: (c, { uid: id }) => c.creators.analysis(id),
  },
  {
    name: "creators_changes",
    title: "Creator profile changes",
    summary: "Handle, name, bio and verification changes observed over time, newest first.",
    operation: "GET /v1/creators/{uid}/changes",
    scope: "creator:read",
    credits: "cached-read",
    input: { uid, limit: limit(200, 50, "Maximum changes") },
    run: (c, { uid: id, ...a }) => c.creators.changes(id, a),
  },
  {
    name: "creators_followers",
    title: "Creator followers",
    summary: "One page of the creator's stored followers (pass `nextCursor` as `cursor` for the next page).",
    operation: "GET /v1/creators/{uid}/followers",
    scope: "creator:read",
    credits: "cached-read",
    waits: true,
    input: { uid, cursor, limit: freeLimit("Entries per page") },
    run: (c, { uid: id, ...a }, ctx) => c.creators.followers(id, { ...a, ...wait(ctx) }),
  },
  {
    name: "creators_following",
    title: "Accounts a creator follows",
    summary:
      "One page of the accounts the creator follows (stored follow graph). Private accounts return `isPrivate: true` " +
      "and no entries.",
    operation: "GET /v1/creators/{uid}/following",
    scope: "creator:read",
    credits: "cached-read",
    waits: true,
    input: { uid, cursor, limit: freeLimit("Entries per page") },
    run: (c, { uid: id, ...a }, ctx) => c.creators.following(id, { ...a, ...wait(ctx) }),
  },
  {
    name: "creators_gifters",
    title: "Creator's top gifters",
    summary:
      "Top gifters of the creator's LIVE rooms over the last 7 or 30 days, by diamonds: gift count, sessions, last " +
      "gift. Coverage is always partial (recorded sessions only). Needs the Pro plan or above.",
    operation: "GET /v1/creators/{uid}/gifters",
    scope: "gifter:read",
    credits: "cached-read",
    input: { uid, window, limit: limit(100, 20) },
    run: (c, { uid: id, ...a }) => c.creators.gifters(id, a),
  },
  {
    name: "creators_insights",
    title: "Creator insights",
    summary:
      "Handle history, daily diamonds of the last 30 days, daily league snapshots and the creator's LIVE room.",
    operation: "GET /v1/creators/{uid}/insights",
    scope: "creator:read",
    credits: "cached-read",
    input: { uid },
    run: (c, { uid: id }) => c.creators.insights(id),
  },
  {
    name: "creators_videos",
    title: "Creator videos",
    summary: "Videos stored for the creator, newest first, with play, like and comment counts.",
    operation: "GET /v1/creators/{uid}/videos",
    scope: "content:read",
    credits: "cached-read",
    input: { uid },
    run: (c, { uid: id }) => c.creators.videos(id),
  },

  // Agencies
  {
    name: "agencies_list",
    title: "TikTok LIVE agencies",
    summary:
      "TikTok LIVE agencies from their public agency profiles, searchable by name or handle and region, sorted by size, " +
      "revenue band, newest or name. Bands are TikTok's own ranges.",
    operation: "GET /v1/agencies",
    scope: "creator:read",
    credits: "cached-read",
    input: {
      q: z.string().optional().describe("Part of the agency name, or its official handle."),
      region: z.string().optional().describe("Region bucket (e.g. `VN`, `US+`, `MENA`)."),
      publicOnly: z.boolean().optional().describe("Only agencies with a public profile."),
      sort: z.enum(["size", "revenue", "recent", "name"]).optional().describe("Sort order (default `size`)."),
      limit: limit(200, 50),
      offset: z.number().int().min(0).optional().describe("Rows to skip (default 0)."),
    },
    run: (c, a) => c.agencies.list(a),
  },
  {
    name: "agencies_get",
    title: "TikTok LIVE agency",
    summary: "One agency of the directory, as `agencies_list` lists it.",
    operation: "GET /v1/agencies/{agency_id}",
    scope: "creator:read",
    credits: "cached-read",
    input: { agency_id: z.string().min(1).describe("TikTok agency id.") },
    run: (c, { agency_id }) => c.agencies.get(agency_id),
  },

  // Insights
  {
    name: "insights_best_time",
    title: "Best time to go LIVE",
    summary:
      "Diamonds and gifter-hours of a region's recorded LIVE sessions per weekday (0 = Monday) and UTC hour. `cells` " +
      "is empty while nothing has been recorded for the region.",
    operation: "GET /v1/insights/best-time",
    scope: "creator:read",
    credits: "cached-read",
    input: { region: region("VN") },
    run: (c, a) => c.insights.bestTime(a),
  },
  {
    name: "insights_presence",
    title: "Hourly LIVE presence",
    summary: "Per hour, how many of the region's tracked ranked creators were live, oldest first.",
    operation: "GET /v1/insights/presence",
    scope: "creator:read",
    credits: "cached-read",
    input: {
      region: region("VN"),
      hours: z.number().int().min(1).max(168).optional().describe("How many hours back, 1–168 (default 48)."),
    },
    run: (c, a) => c.insights.presence(a),
  },

  // Videos
  {
    name: "videos_get",
    title: "Video",
    summary: "One stored TikTok video with its counters.",
    operation: "GET /v1/videos/{video_id}",
    scope: "content:read",
    credits: "cached-read",
    input: { video_id: z.string().min(1).describe("TikTok video id.") },
    run: (c, { video_id }) => c.videos.get(video_id),
  },
  {
    name: "videos_comments",
    title: "Video comments",
    summary:
      "One page of a video's stored comments; pass `parentCommentId` for the replies of one comment and `nextCursor` " +
      "as `cursor` for the next page.",
    operation: "GET /v1/videos/{video_id}/comments",
    scope: "content:read",
    credits: "cached-read",
    waits: true,
    input: {
      video_id: z.string().min(1).describe("TikTok video id."),
      parentCommentId: z.string().optional().describe("Return the replies of this comment."),
      cursor,
      limit: freeLimit("Comments per page"),
    },
    run: (c, { video_id, ...a }, ctx) => c.videos.comments(video_id, { ...a, ...wait(ctx) }),
  },

  // Trends, gifts, gifters
  {
    name: "trends_list",
    title: "Trends of a region",
    summary: "Trending hashtags and sounds of a region, by rank.",
    operation: "GET /v1/trends",
    scope: "trend:read",
    credits: "cached-read",
    input: {
      region: region("US"),
      type: z.enum(["hashtag", "sound"]).optional().describe("Only one kind of trend."),
    },
    run: (c, a) => c.trends.list(a),
  },
  {
    name: "gifts_list",
    title: "TikTok gifts",
    summary:
      "TikTok gifts seen in the LIVE rooms toktokdata.live recorded: coins of one gift, regions, first and last seen. " +
      "Coverage is partial; synced daily.",
    operation: "GET /v1/gifts",
    scope: "live:read",
    credits: "cached-read",
    input: {
      q: z.string().optional().describe("Part of the gift name."),
      minCoins: z.number().int().min(0).optional().describe("Minimum coins of one gift."),
      maxCoins: z.number().int().min(0).optional().describe("Maximum coins of one gift."),
      region: z.string().optional().describe("Only gifts seen in this region (e.g. `VN`)."),
      sort: z.enum(["coins", "name", "recent"]).optional().describe("`coins` (most first, default), `name` or `recent`."),
      limit: limit(500, 100),
    },
    run: (c, a) => c.gifts.list(a),
  },
  {
    name: "gifters_list",
    title: "Gifters",
    summary:
      "Who gifts in the recorded LIVE rooms over 7 or 30 days, by diamonds, with a segment (`whale`, `rising`, " +
      "`regular`, `one_time`, `occasional`). Coverage is partial. Needs the Pro plan or above.",
    operation: "GET /v1/gifters",
    scope: "gifter:read",
    credits: "cached-read",
    input: {
      window,
      segment: z
        .enum(["whale", "rising", "regular", "one_time", "occasional"])
        .optional()
        .describe("Only one segment."),
      region: z.string().optional().describe("Region of the creators gifted (e.g. `VN`)."),
      limit: limit(500, 50),
    },
    run: (c, a) => c.gifters.list(a),
  },

  // Live
  {
    name: "live_sessions",
    title: "Recorded LIVE sessions",
    summary:
      "One page of LIVE sessions recorded for the workspace's watchlist and tracked rooms, newest first (a running " +
      "session has `endedAt: null`). Pass `nextCursor` as `cursor` for the next page.",
    operation: "GET /v1/live/sessions",
    scope: "live:read",
    credits: "cached-read",
    input: {
      creatorUniqueId: z.string().optional().describe("Only this creator's sessions (handle without `@`)."),
      cursor,
      limit: limit(100, 50, "Sessions per page"),
    },
    run: (c, a) => c.live.sessions(a),
  },
  {
    name: "live_session",
    title: "LIVE session",
    summary: "One recorded LIVE session with gift and gifter rollups. Sessions the workspace does not follow are 404.",
    operation: "GET /v1/live/sessions/{session_id}",
    scope: "live:read",
    credits: "cached-read",
    input: { session_id: sessionId },
    run: (c, { session_id }) => c.live.session(session_id),
  },
  {
    name: "live_session_timeline",
    title: "LIVE session minute by minute",
    summary:
      "A session's per-minute series from its start: peak viewers, chats, likes, gifts, diamonds, new gifters and " +
      "treasure boxes, with `offsetMinutes`.",
    operation: "GET /v1/live/sessions/{session_id}/timeline",
    scope: "live:read",
    credits: "cached-read",
    input: { session_id: sessionId },
    run: (c, { session_id }) => c.live.timeline(session_id),
  },
  {
    name: "live_sessions_compare",
    title: "Compare LIVE sessions",
    summary: "Two to four sessions side by side, each with totals and minutes aligned on `offsetMinutes`.",
    operation: "GET /v1/live/sessions/compare",
    scope: "live:read",
    credits: "cached-read",
    input: {
      ids: z.array(z.string().min(1)).min(2).max(4).describe("2 to 4 session ids from `live_sessions`."),
    },
    run: (c, { ids }) => c.live.compare(ids),
  },
  {
    name: "live_performance",
    title: "Creator LIVE performance",
    summary:
      "One row per recorded session of a creator: duration, peak viewers, gifts, diamonds, distinct gifters. Only for " +
      "creators on the workspace's watchlist or tracked rooms (otherwise `items` is empty and `followed` false).",
    operation: "GET /v1/live/creators/{uid}/performance",
    scope: "live:read",
    credits: "cached-read",
    input: { uid, cursor, limit: limit(100, 50, "Rows per page") },
    run: (c, { uid: id, ...a }) => c.live.performance(id, a),
  },
  {
    name: "live_comments",
    title: "Recent LIVE chat",
    summary:
      "The latest chat comments of a creator's LIVE room, oldest first (at most 100). Pass the previous answer's " +
      "`cursor` as `after` for newer comments only. Only for rooms the workspace follows (`status: tracked`).",
    operation: "GET /v1/live/creators/{uid}/comments",
    scope: "live:read",
    credits: "free",
    input: {
      uid: z.string().min(1).describe("Creator handle (e.g. `linh.live`); a leading `@` is ignored."),
      after: z.string().optional().describe("`cursor` of the previous answer: only newer comments."),
    },
    run: (c, { uid: id, ...a }) => c.live.comments(id, a),
  },

  // Account
  {
    name: "account_usage",
    title: "Credit usage",
    summary: "Credits spent per operation in a billing period, and the remaining balance.",
    operation: "GET /v1/usage",
    scope: "keys:manage",
    credits: "free",
    input: { period, limit: freeLimit("Maximum rows") },
    run: (c, a) => c.account.usage(a),
  },
  {
    name: "account_entitlements",
    title: "Plan and limits",
    summary:
      "The workspace's plan, modules, rate limits and credit wallet (`credits.remaining`, `credits.resetAt`) for a " +
      "billing period.",
    operation: "GET /v1/entitlements",
    scope: "keys:manage",
    credits: "free",
    input: { period },
    run: (c, a) => c.account.entitlements(a),
  },
];

/** The description an MCP client shows: summary, scope, cost and waiting behaviour. */
export function describeTool(tool: ToolDef, maxWaitSeconds: number): string {
  const cost =
    tool.credits === "free"
      ? "Free (no credits)."
      : "Costs credits from the workspace wallet (billed as a cached read).";
  const waits = tool.waits
    ? ` May wait up to ${maxWaitSeconds} s while toktokdata.live collects fresh data; if it is still collecting, the ` +
      "tool says when to retry."
    : "";
  return `${tool.summary} Needs API key scope \`${tool.scope}\`. ${cost}${waits} Returns the API answer as JSON, with \`provenance\` (freshness, coverage, source).`;
}
