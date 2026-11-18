/**
 * Light response types. The API only grows compatibly, so every object keeps an index signature:
 * fields not listed here are still there at runtime, typed as `unknown`.
 */

export type JsonObject = Record<string, unknown>;

/** How fresh and complete the data is, and where it comes from. */
export interface Provenance {
  /** When the data was observed (never when it was served); `null` when nothing was observed yet. */
  observedAt: string | null;
  freshness: "near_realtime" | "historical" | "stale" | (string & {});
  source: string;
  /** `official`: TikTok's own number. `derived`: observed, counted or computed by toktokdata.live. */
  sourceKind: "official" | "derived" | (string & {});
  /** `observed`, `complete`, `partial`, `stale` or `not-covered`. */
  coverageStatus: string;
  methodologyVersion: string;
  window: { from?: string | null; to?: string | null; label?: string; [key: string]: unknown } | null;
  [key: string]: unknown;
}

/** The standard read envelope: `{ data, provenance }`. */
export interface ApiResponse<T = JsonObject> {
  data: T;
  provenance: Provenance;
  [key: string]: unknown;
}

/** Body of a `202` "not ready yet" answer. */
export interface PendingResponse {
  status: "resolving" | "saturated" | (string & {});
  jobId?: string;
  retryAfterSeconds?: number;
  [key: string]: unknown;
}

/** A page of a cursor list: `data.nextCursor` is `null` on the last page. */
export interface CursorPageData {
  nextCursor: string | null;
  hasMore?: boolean;
  [key: string]: unknown;
}

export type CursorPage<T extends CursorPageData = CursorPageData> = ApiResponse<T>;

export type OfficialBoardKey =
  | "daily"
  | "hourly"
  | "popular_live"
  | "gaming_daily"
  | "gaming_weekly"
  | "shopping_daily"
  | "shopping_weekly"
  | `league_${"a" | "b" | "c" | "d"}${1 | 2 | 3 | 4 | 5}`;

/** A board key; known keys autocomplete, any string is accepted. */
export type BoardKey = OfficialBoardKey | (string & {});

export interface BoardEntry {
  rank: number;
  score: number | null;
  creatorUid: string | null;
  uniqueId: string | null;
  nickname: string | null;
  avatarUrl: string | null;
  isLive: boolean | null;
  roomId: string | null;
  movement: string | null;
  previousRank: number | null;
  /** True when the plan does not unmask this row: no identity, see `unmaskTier` and `maskedRef`. */
  masked: boolean;
  unmaskTier: string | null;
  maskedRef?: string | null;
  sparkline?: Array<number | null> | null;
  [key: string]: unknown;
}

export interface BoardCoverage {
  completeBoards: number;
  expectedBoards: number;
  ratio: number;
  threshold: number;
  [key: string]: unknown;
}

export interface OfficialBoardData {
  board: {
    kind: string;
    board: string;
    region: string;
    periodKey: string | null;
    resetAt: string | null;
    /** Read `publishable` and `coverage` before showing entries. */
    publishable: boolean;
    coverage: BoardCoverage | null;
    entries: BoardEntry[];
    [key: string]: unknown;
  };
  [key: string]: unknown;
}

export interface RankingHistoryData extends CursorPageData {
  board: string;
  region: string;
  periods: Array<{
    periodKey: string;
    observedAt: string | null;
    coverageStatus: string;
    publishable: boolean;
    entries: BoardEntry[];
    [key: string]: unknown;
  }>;
}

export interface Creator {
  tiktokUserId: string | null;
  uniqueId: string;
  displayName: string | null;
  region: string | null;
  signature: string | null;
  avatarUrl: string | null;
  followerCount: number | null;
  followingCount: number | null;
  verified: boolean | null;
  [key: string]: unknown;
}

export interface CreatorData {
  creator: Creator;
  [key: string]: unknown;
}

export interface FollowPageData extends CursorPageData {
  tiktokUserId: string | null;
  uniqueId: string;
  direction: "followers" | "following" | (string & {});
  entries: JsonObject[];
  isPrivate: boolean;
}

export interface LiveSessionSummary {
  id: string;
  roomKey: string;
  roomId: string | null;
  creatorUniqueId: string | null;
  startedAt: string;
  /** `null` while the session is still running. */
  endedAt: string | null;
  peakViewerCount: number | null;
  giftCount: number | null;
  totalGiftDiamonds: number | null;
  durationSeconds: number | null;
  [key: string]: unknown;
}

export interface LiveSessionsData extends CursorPageData {
  items: LiveSessionSummary[];
}

export interface LivePerformanceData extends CursorPageData {
  items: JsonObject[];
  followed: boolean;
}

export interface VideoCommentsData extends CursorPageData {
  videoId: string;
  parentCommentId: string | null;
  comments: Array<{
    commentId: string;
    text: string;
    diggCount: number | null;
    replyCount: number | null;
    createTime: string | null;
    authorUniqueId: string | null;
    [key: string]: unknown;
  }>;
}

/** `GET /v1/live/creators/{uid}/comments` is flat (no `data` wrapper). */
export interface LiveComments {
  status: "tracked" | "not_tracked" | (string & {});
  followed: boolean;
  roomId?: string | null;
  items: Array<{
    id: string;
    createdAt: string;
    comment: string;
    user: { uniqueId: string | null; nickname: string | null; avatarUrl: string | null; [key: string]: unknown };
    [key: string]: unknown;
  }>;
  /** Pass as `after` on the next poll. */
  cursor?: string | null;
  provenance?: Provenance;
  [key: string]: unknown;
}

/** `GET /v1/usage` is flat (no `data` wrapper). */
export interface Usage {
  period: string;
  balance: { remaining: number; spent: number; [key: string]: unknown };
  items: JsonObject[];
  provenance?: Provenance;
  [key: string]: unknown;
}

export interface ExportJob {
  id: string;
  dataset: string;
  format: "csv" | "xlsx" | (string & {});
  status: "pending" | "ready" | "failed" | (string & {});
  rowCount: number | null;
  filename: string | null;
  downloadUrl: string | null;
  error: string | null;
  createdAt: string;
  completedAt: string | null;
  [key: string]: unknown;
}

export interface AlertChannelInput {
  channelType: "webhook" | "email" | "in_app" | "telegram" | "discord" | "zalo";
  target?: string;
  enabled?: boolean;
}

export type AlertEventType =
  | "gift"
  | "envelope"
  | "goodyBag"
  | "live_start"
  | "live_end"
  | "host_quiet"
  | "league_change"
  | "diamond_target"
  | "follower_milestone"
  | "*";

export interface AlertRuleInput {
  name?: string;
  creatorUniqueId?: string | null;
  scope?: "creator" | "any" | "watchlist" | "gifter_watchlist";
  region?: string | null;
  eventTypes?: AlertEventType[];
  minDiamondCount?: number | null;
  minFollowerMilestone?: number | null;
  enabled?: boolean;
  /** Replaces the whole channel list on update. */
  channels?: AlertChannelInput[];
  [key: string]: unknown;
}

export interface AlertRuleCreateInput extends AlertRuleInput {
  name: string;
}

export interface AlertRule {
  id: string;
  name: string;
  creatorUniqueId: string | null;
  scope: string;
  region: string | null;
  eventTypes: string[];
  enabled: boolean;
  channels: Array<{
    id: string;
    channelType: string;
    target: string | null;
    enabled: boolean;
    hasSigningSecret: boolean;
    [key: string]: unknown;
  }>;
  [key: string]: unknown;
}

/** Rate-limit headers of the last response; a field is `undefined` when its header was absent. */
export interface RateLimitInfo {
  /** Requests per second allowed (`X-RateLimit-Limit`). */
  limit?: number;
  remaining?: number;
  minuteLimit?: number;
  minuteRemaining?: number;
  dailyLimit?: number;
  dailyRemaining?: number;
}
