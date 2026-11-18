import { pathSegment, type Transport, type WaitOptions } from "./http.js";
import { paginate, type PaginateOptions } from "./pagination.js";
import type {
  AlertRule,
  AlertRuleCreateInput,
  AlertRuleInput,
  ApiResponse,
  BoardKey,
  CreatorData,
  ExportJob,
  FollowPageData,
  JsonObject,
  LiveComments,
  LivePerformanceData,
  LiveSessionsData,
  OfficialBoardData,
  PendingResponse,
  RankingHistoryData,
  Usage,
  VideoCommentsData,
} from "./types.js";
import { PendingTimeoutError } from "./errors.js";
import { sleep, parseRetryAfter } from "./http.js";

/** A 202-aware method's result: with `wait: false` the 202 body may come back instead. */
export type Waited<T, W extends boolean> = [W] extends [false] ? T | PendingResponse : T;

/** True when a 202-aware method returned the "not ready yet" body (only with `wait: false`). */
export function isPending(value: unknown): value is PendingResponse {
  if (typeof value !== "object" || value === null || "data" in value) return false;
  const status = (value as { status?: unknown }).status;
  return status === "resolving" || status === "saturated";
}

const handle = (uid: string) => pathSegment(uid, { stripAt: true });
const id = (value: string) => pathSegment(value);

// ---------------------------------------------------------------- rankings

export interface OfficialParams {
  /** ISO 3166-1 alpha-2 region (default `VN`). */
  region?: string;
  /** `current` (running period, default) or `history` (last completed one). */
  period?: "current" | "history";
  /** 1–200 (default 200). */
  limit?: number;
  /** Only creators live right now. */
  liveOnly?: boolean;
  /** League tier filter, `A1` … `D5`. */
  leagueTier?: string;
}

export interface HistoryParams {
  region?: string;
  cursor?: string | null;
  /** Periods per page, 1–24 (default 24). */
  limit?: number;
  leagueTier?: string;
}

export interface DayBarsParams {
  region?: string;
  /** `daily` (default) or a league board. */
  board?: BoardKey;
  /** Days back, up to 90 (default 30). */
  days?: number;
}

export class Rankings {
  constructor(private readonly t: Transport) {}

  /** `GET /v1/rankings/official`: one official board for one region. Scope `rank:read`. */
  official(board: BoardKey, params: OfficialParams = {}): Promise<ApiResponse<OfficialBoardData>> {
    return this.t.request("GET", "/v1/rankings/official", { query: { board, ...params } });
  }

  /** `GET /v1/rankings/regions`: regions that currently have observed boards. */
  regions(): Promise<ApiResponse<{ regions: JsonObject[]; [key: string]: unknown }>> {
    return this.t.request("GET", "/v1/rankings/regions");
  }

  /** `GET /v1/rankings/history`: completed periods of a board, newest first (paged). */
  history(board: BoardKey, params: HistoryParams = {}): Promise<ApiResponse<RankingHistoryData>> {
    return this.t.request("GET", "/v1/rankings/history", { query: { board, ...params } });
  }

  /** Iterates over every page of {@link history}. */
  iterHistory(board: BoardKey, params: HistoryParams = {}, options?: PaginateOptions) {
    return paginate((p: HistoryParams) => this.history(board, p), params, options);
  }

  /** `GET /v1/rankings/day-bars`: per board day, how many creators and the score bar to make it. */
  dayBars(params: DayBarsParams = {}): Promise<ApiResponse> {
    return this.t.request("GET", "/v1/rankings/day-bars", { query: { ...params } });
  }

  /** `GET /v1/rankings/masked/{ref}`: 30-day LIVE totals of a masked board row. */
  masked(ref: string): Promise<ApiResponse> {
    return this.t.request("GET", `/v1/rankings/masked/${id(ref)}`);
  }
}

// ---------------------------------------------------------------- creators

export interface CursorParams {
  cursor?: string | null;
  limit?: number;
}

export class Creators {
  constructor(private readonly t: Transport) {}

  /** `GET /v1/creators`: search stored creators by handle or display name. */
  search(params: { q?: string; limit?: number } = {}): Promise<ApiResponse<{ items?: JsonObject[]; [key: string]: unknown }>> {
    return this.t.request("GET", "/v1/creators", { query: { ...params } });
  }

  /** `GET /v1/creators/{uid}`: stored profile; waits while it is being looked up (202). */
  get<W extends boolean = true>(uid: string, options: WaitOptions<W> = {}): Promise<Waited<ApiResponse<CreatorData>, W>> {
    return this.t.getWaiting(`/v1/creators/${handle(uid)}`, undefined, options);
  }

  /** `GET /v1/creators/{uid}/analysis`: engagement report computed from the stored videos. */
  analysis(uid: string): Promise<ApiResponse> {
    return this.t.request("GET", `/v1/creators/${handle(uid)}/analysis`);
  }

  /** `GET /v1/creators/{uid}/changes`: handle, name, bio and verification changes. */
  changes(uid: string, params: { limit?: number } = {}): Promise<ApiResponse> {
    return this.t.request("GET", `/v1/creators/${handle(uid)}/changes`, { query: { ...params } });
  }

  /** `GET /v1/creators/{uid}/followers` (paged, 202 aware). */
  followers<W extends boolean = true>(
    uid: string,
    params: CursorParams & WaitOptions<W> = {},
  ): Promise<Waited<ApiResponse<FollowPageData>, W>> {
    const { wait, maxWaitMs, ...query } = params;
    return this.t.getWaiting(`/v1/creators/${handle(uid)}/followers`, query, { wait, maxWaitMs });
  }

  /** Iterates over every page of {@link followers}. */
  iterFollowers(uid: string, params: CursorParams & { maxWaitMs?: number } = {}, options?: PaginateOptions) {
    return paginate((p: CursorParams & { maxWaitMs?: number }) => this.followers(uid, p), params, options);
  }

  /** `GET /v1/creators/{uid}/following` (paged, 202 aware). */
  following<W extends boolean = true>(
    uid: string,
    params: CursorParams & WaitOptions<W> = {},
  ): Promise<Waited<ApiResponse<FollowPageData>, W>> {
    const { wait, maxWaitMs, ...query } = params;
    return this.t.getWaiting(`/v1/creators/${handle(uid)}/following`, query, { wait, maxWaitMs });
  }

  /** Iterates over every page of {@link following}. */
  iterFollowing(uid: string, params: CursorParams & { maxWaitMs?: number } = {}, options?: PaginateOptions) {
    return paginate((p: CursorParams & { maxWaitMs?: number }) => this.following(uid, p), params, options);
  }

  /** `GET /v1/creators/{uid}/gifters`: top gifters of the creator's rooms (Pro plan, `gifter:read`). */
  gifters(uid: string, params: { window?: "7d" | "30d"; limit?: number } = {}): Promise<ApiResponse> {
    return this.t.request("GET", `/v1/creators/${handle(uid)}/gifters`, { query: { ...params } });
  }

  /** `GET /v1/creators/{uid}/insights`: handle history, daily diamonds, league snapshots. */
  insights(uid: string): Promise<ApiResponse> {
    return this.t.request("GET", `/v1/creators/${handle(uid)}/insights`);
  }

  /** `GET /v1/creators/{uid}/videos`: stored videos, newest first (`content:read`). */
  videos(uid: string): Promise<ApiResponse> {
    return this.t.request("GET", `/v1/creators/${handle(uid)}/videos`);
  }
}

// ---------------------------------------------------------------- agencies

export interface AgencyListParams {
  q?: string;
  /** Region bucket (`VN`, `US+`, `MENA` ...). */
  region?: string;
  publicOnly?: boolean;
  sort?: "size" | "revenue" | "recent" | "name";
  limit?: number;
  offset?: number;
}

export class Agencies {
  constructor(private readonly t: Transport) {}

  /** `GET /v1/agencies`: TikTok LIVE agencies directory. */
  list(params: AgencyListParams = {}): Promise<ApiResponse> {
    return this.t.request("GET", "/v1/agencies", { query: { ...params } });
  }

  /** `GET /v1/agencies/{agency_id}`. */
  get(agencyId: string): Promise<ApiResponse> {
    return this.t.request("GET", `/v1/agencies/${id(agencyId)}`);
  }
}

// ---------------------------------------------------------------- insights

export class Insights {
  constructor(private readonly t: Transport) {}

  /** `GET /v1/insights/best-time`: diamonds per weekday and UTC hour of a region. */
  bestTime(params: { region?: string } = {}): Promise<ApiResponse> {
    return this.t.request("GET", "/v1/insights/best-time", { query: { ...params } });
  }

  /** `GET /v1/insights/presence`: per hour, how many tracked ranked creators were live. */
  presence(params: { region?: string; hours?: number } = {}): Promise<ApiResponse> {
    return this.t.request("GET", "/v1/insights/presence", { query: { ...params } });
  }
}

// ---------------------------------------------------------------- videos

export interface VideoCommentsParams extends CursorParams {
  /** Return the replies of this comment. */
  parentCommentId?: string;
}

export class Videos {
  constructor(private readonly t: Transport) {}

  /** `GET /v1/videos/{video_id}`: one stored video with its counters. */
  get(videoId: string): Promise<ApiResponse> {
    return this.t.request("GET", `/v1/videos/${id(videoId)}`);
  }

  /** `GET /v1/videos/{video_id}/comments` (paged, 202 aware). */
  comments<W extends boolean = true>(
    videoId: string,
    params: VideoCommentsParams & WaitOptions<W> = {},
  ): Promise<Waited<ApiResponse<VideoCommentsData>, W>> {
    const { wait, maxWaitMs, ...query } = params;
    return this.t.getWaiting(`/v1/videos/${id(videoId)}/comments`, query, { wait, maxWaitMs });
  }

  /** Iterates over every page of {@link comments}. */
  iterComments(videoId: string, params: VideoCommentsParams & { maxWaitMs?: number } = {}, options?: PaginateOptions) {
    return paginate((p: VideoCommentsParams & { maxWaitMs?: number }) => this.comments(videoId, p), params, options);
  }
}

// ---------------------------------------------------------------- trends, gifts, gifters

export class Trends {
  constructor(private readonly t: Transport) {}

  /** `GET /v1/trends`: trending hashtags and sounds of a region (default `US`). */
  list(params: { region?: string; type?: "hashtag" | "sound" } = {}): Promise<ApiResponse> {
    return this.t.request("GET", "/v1/trends", { query: { ...params } });
  }
}

export interface GiftListParams {
  q?: string;
  minCoins?: number;
  maxCoins?: number;
  region?: string;
  sort?: "coins" | "name" | "recent";
  limit?: number;
}

export class Gifts {
  constructor(private readonly t: Transport) {}

  /** `GET /v1/gifts`: TikTok gifts seen in recorded rooms (`live:read`). */
  list(params: GiftListParams = {}): Promise<ApiResponse> {
    return this.t.request("GET", "/v1/gifts", { query: { ...params } });
  }
}

export interface GifterListParams {
  window?: "7d" | "30d";
  segment?: "whale" | "rising" | "regular" | "one_time" | "occasional";
  region?: string;
  limit?: number;
}

export class Gifters {
  constructor(private readonly t: Transport) {}

  /** `GET /v1/gifters`: who gifts in recorded rooms (Pro plan, `gifter:read`). */
  list(params: GifterListParams = {}): Promise<ApiResponse> {
    return this.t.request("GET", "/v1/gifters", { query: { ...params } });
  }
}

// ---------------------------------------------------------------- live

export interface SessionsParams extends CursorParams {
  /** Only this creator's sessions (handle without `@`). */
  creatorUniqueId?: string;
}

export class Live {
  constructor(private readonly t: Transport) {}

  /** `GET /v1/live/sessions`: recorded sessions of followed creators, newest first (paged). */
  sessions(params: SessionsParams = {}): Promise<ApiResponse<LiveSessionsData>> {
    return this.t.request("GET", "/v1/live/sessions", { query: { ...params } });
  }

  /** Iterates over every page of {@link sessions}. */
  iterSessions(params: SessionsParams = {}, options?: PaginateOptions) {
    return paginate((p: SessionsParams) => this.sessions(p), params, options);
  }

  /** `GET /v1/live/sessions/{session_id}`: one session with gift and gifter rollups. */
  session(sessionId: string): Promise<ApiResponse> {
    return this.t.request("GET", `/v1/live/sessions/${id(sessionId)}`);
  }

  /** `GET /v1/live/sessions/{session_id}/timeline`: the session minute by minute. */
  timeline(sessionId: string): Promise<ApiResponse> {
    return this.t.request("GET", `/v1/live/sessions/${id(sessionId)}/timeline`);
  }

  /** `GET /v1/live/sessions/compare`: 2 to 4 sessions side by side. */
  compare(ids: readonly string[]): Promise<ApiResponse> {
    if (ids.length < 2 || ids.length > 4) {
      return Promise.reject(new RangeError(`compare() takes 2 to 4 session ids, got ${ids.length}.`));
    }
    return this.t.request("GET", "/v1/live/sessions/compare", { query: { ids } });
  }

  /** `GET /v1/live/creators/{uid}/performance`: one row per recorded session (paged). */
  performance(uid: string, params: CursorParams = {}): Promise<ApiResponse<LivePerformanceData>> {
    return this.t.request("GET", `/v1/live/creators/${handle(uid)}/performance`, { query: { ...params } });
  }

  /** Iterates over every page of {@link performance}. */
  iterPerformance(uid: string, params: CursorParams = {}, options?: PaginateOptions) {
    return paginate((p: CursorParams) => this.performance(uid, p), params, options);
  }

  /** `POST /v1/live/creators/{uid}/refresh`: queue a live-status check (202 `{queued, jobId}`). */
  refresh(uid: string): Promise<{ queued: boolean; jobId?: string; [key: string]: unknown }> {
    return this.t.request("POST", `/v1/live/creators/${handle(uid)}/refresh`);
  }

  /** `GET /v1/live/creators/{uid}/playback`: signed video URL of the current room (`live:stream`, 202 aware). */
  playback<W extends boolean = true>(uid: string, options: WaitOptions<W> = {}): Promise<Waited<ApiResponse, W>> {
    return this.t.getWaiting(`/v1/live/creators/${handle(uid)}/playback`, undefined, options);
  }

  /** `GET /v1/live/creators/{uid}/comments`: latest LIVE chat; pass the previous `cursor` as `after`. Free. */
  comments(uid: string, params: { after?: string | null } = {}): Promise<LiveComments> {
    return this.t.request("GET", `/v1/live/creators/${handle(uid)}/comments`, { query: { ...params } });
  }
}

// ---------------------------------------------------------------- alerts

export class Alerts {
  constructor(private readonly t: Transport) {}

  /** `GET /v1/alerts/rules`. Scope `webhook:manage`. */
  listRules(): Promise<{ rules?: AlertRule[]; [key: string]: unknown }> {
    return this.t.request("GET", "/v1/alerts/rules");
  }

  /** `POST /v1/alerts/rules`. A webhook channel's signing secret comes back once, in `webhookSecrets`. */
  createRule(body: AlertRuleCreateInput): Promise<{ rule: AlertRule; webhookSecrets: JsonObject[]; [key: string]: unknown }> {
    return this.t.request("POST", "/v1/alerts/rules", { body });
  }

  /** `PATCH /v1/alerts/rules/{rule_id}`: send only the fields to change; `channels` replaces the list. */
  updateRule(ruleId: string, body: AlertRuleInput): Promise<{ rule: AlertRule; [key: string]: unknown }> {
    return this.t.request("PATCH", `/v1/alerts/rules/${id(ruleId)}`, { body });
  }

  /** `DELETE /v1/alerts/rules/{rule_id}`. */
  async deleteRule(ruleId: string): Promise<void> {
    await this.t.request("DELETE", `/v1/alerts/rules/${id(ruleId)}`);
  }

  /** `POST /v1/alerts/rules/{rule_id}/test`: deliver a synthetic event to every enabled channel now. */
  testRule(ruleId: string): Promise<JsonObject> {
    return this.t.request("POST", `/v1/alerts/rules/${id(ruleId)}/test`);
  }

  /** `GET /v1/alerts/fires`: every time a rule matched, newest first. */
  fires(params: { ruleId?: string; since?: string; limit?: number } = {}): Promise<JsonObject> {
    return this.t.request("GET", "/v1/alerts/fires", { query: { ...params } });
  }

  /** `GET /v1/alerts/deliveries`: one row per rule, channel and event. */
  deliveries(
    params: { ruleId?: string; status?: "delivered" | "failed" | "capped"; since?: string; limit?: number } = {},
  ): Promise<JsonObject> {
    return this.t.request("GET", "/v1/alerts/deliveries", { query: { ...params } });
  }

  /** `GET /v1/alerts/stats`: activity over the last 24 hours. */
  stats(): Promise<JsonObject> {
    return this.t.request("GET", "/v1/alerts/stats");
  }
}

// ---------------------------------------------------------------- exports

export interface ExportCreateInput {
  dataset: "live_sessions" | "rankings" | "watchlist" | "recruit_pipeline" | "gifters";
  format: "csv" | "xlsx";
  board?: BoardKey;
  region?: string;
  leagueTier?: string;
  /** 1–366 (default 30). */
  rangeDays?: number;
}

export class Exports {
  constructor(private readonly t: Transport) {}

  /** `GET /v1/exports`: exports of the workspace, newest first. Free. */
  list(): Promise<JsonObject> {
    return this.t.request("GET", "/v1/exports");
  }

  /** `POST /v1/exports`: `status: ready` (201) up to 1,000 rows, else `pending` (202): see {@link waitUntilReady}. */
  create(body: ExportCreateInput): Promise<ExportJob> {
    return this.t.request("POST", "/v1/exports", { body });
  }

  /** `GET /v1/exports/{job_id}`. */
  get(jobId: string): Promise<ExportJob> {
    return this.t.request("GET", `/v1/exports/${id(jobId)}`);
  }

  /** `GET /v1/exports/{job_id}/download`: the file, as text (default, CSV) or bytes (`as: "bytes"`, Excel). */
  download(jobId: string, options?: { as?: "text" }): Promise<string>;
  download(jobId: string, options: { as: "bytes" }): Promise<Uint8Array>;
  download(jobId: string, options: { as?: "text" | "bytes" } = {}): Promise<string | Uint8Array> {
    return this.t.download(`/v1/exports/${id(jobId)}/download`, options.as ?? "text");
  }

  /**
   * Polls {@link get} every `intervalMs` (default 2 s, or the `Retry-After` hint) until `status` is
   * `ready` or `failed`. Throws {@link PendingTimeoutError} after `timeoutMs` (default 300 s).
   */
  async waitUntilReady(jobId: string, options: { timeoutMs?: number; intervalMs?: number } = {}): Promise<ExportJob> {
    const timeoutMs = options.timeoutMs ?? 300_000;
    const intervalMs = options.intervalMs ?? 2_000;
    const started = Date.now();
    const path = `/v1/exports/${id(jobId)}`;
    for (;;) {
      const result = await this.t.send("GET", path);
      const job = result.body as ExportJob;
      if (job?.status === "ready" || job?.status === "failed") return job;
      const hinted = parseRetryAfter(result.headers.get("retry-after"));
      const delay = hinted !== undefined ? Math.max(1, Math.min(hinted, 60)) * 1000 : intervalMs;
      const waited = Date.now() - started;
      if (waited + delay > timeoutMs) {
        throw new PendingTimeoutError({ path, body: job, retryAfterSeconds: Math.ceil(delay / 1000), waitedMs: waited });
      }
      await sleep(delay);
    }
  }
}

// ---------------------------------------------------------------- account

export class Account {
  constructor(private readonly t: Transport) {}

  /** `GET /v1/entitlements`: plan, modules, rate limits and credit wallet. Free, scope `keys:manage`. */
  entitlements(params: { period?: string } = {}): Promise<ApiResponse> {
    return this.t.request("GET", "/v1/entitlements", { query: { ...params } });
  }

  /** `GET /v1/usage`: credits spent per operation in a billing period. Free, scope `keys:manage`. */
  usage(params: { period?: string; limit?: number } = {}): Promise<Usage> {
    return this.t.request("GET", "/v1/usage", { query: { ...params } });
  }
}
