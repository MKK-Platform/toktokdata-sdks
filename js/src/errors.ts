/** The JSON error envelope the API answers with on 4xx/5xx. Extra fields (such as `resetAt`) are kept. */
export interface ErrorEnvelope {
  code?: string;
  message?: string;
  requestId?: string;
  retryable?: boolean;
  /** On `credit_cap_exceeded` (402): when the workspace's credits reset. */
  resetAt?: string;
  [key: string]: unknown;
}

export interface TokTokDataErrorInit {
  status: number;
  code: string;
  message: string;
  requestId?: string | undefined;
  retryable: boolean;
  retryAfter?: number | undefined;
  details?: ErrorEnvelope | undefined;
  cause?: unknown;
}

/**
 * Every failure the client raises. `status` is the HTTP status, or `0` when no response arrived
 * (`code` is then `network_error` or `timeout`).
 */
export class TokTokDataError extends Error {
  /** HTTP status, `0` for network errors and timeouts. */
  readonly status: number;
  /** Machine-readable code from the envelope (`http_error` when the body was not the envelope). */
  readonly code: string;
  /** Request id to quote to support. */
  readonly requestId: string | undefined;
  /** True when retrying the same request later can succeed. */
  readonly retryable: boolean;
  /** Seconds from the `Retry-After` header, when present. */
  readonly retryAfter: number | undefined;
  /** The whole error envelope (may carry `resetAt`). */
  readonly details: ErrorEnvelope | undefined;

  constructor(init: TokTokDataErrorInit) {
    super(init.message, init.cause !== undefined ? { cause: init.cause } : undefined);
    this.name = "TokTokDataError";
    this.status = init.status;
    this.code = init.code;
    this.requestId = init.requestId;
    this.retryable = init.retryable;
    this.retryAfter = init.retryAfter;
    this.details = init.details;
  }

  /** 429: too many requests (per second, minute or day). */
  get isRateLimited(): boolean {
    return this.status === 429;
  }

  /** 402: the workspace's credit wallet cannot cover the call (`credit_cap_exceeded`). */
  get isPaymentRequired(): boolean {
    return this.status === 402;
  }

  /** 403: the key lacks the scope (`insufficient_scope`) or the plan lacks the module (`module_not_entitled`). */
  get isForbidden(): boolean {
    return this.status === 403;
  }

  /** 401: missing or unknown API key. */
  get isUnauthorized(): boolean {
    return this.status === 401;
  }

  /** 404: the resource does not exist or is not visible to this workspace. */
  get isNotFound(): boolean {
    return this.status === 404;
  }
}

/**
 * A 202-aware method kept getting `202` ("still collecting") until its wait budget ran out.
 * Ask again after `retryAfterSeconds`; the 202 answers were not billed beyond a cached read.
 */
export class PendingTimeoutError extends TokTokDataError {
  /** `resolving` or `saturated`, from the last 202 body. */
  readonly pendingStatus: string | undefined;
  readonly jobId: string | undefined;
  /** Suggested wait before asking again, in seconds. */
  readonly retryAfterSeconds: number;
  /** How long the client waited, in milliseconds. */
  readonly waitedMs: number;
  /** The last 202 body. */
  readonly body: unknown;

  constructor(init: {
    path: string;
    body: unknown;
    retryAfterSeconds: number;
    waitedMs: number;
  }) {
    const body = (init.body ?? {}) as Record<string, unknown>;
    const pendingStatus = typeof body.status === "string" ? body.status : undefined;
    const jobId = typeof body.jobId === "string" ? body.jobId : undefined;
    super({
      status: 202,
      code: "pending_timeout",
      message: `${init.path} is still ${pendingStatus ?? "pending"} after ${Math.round(
        init.waitedMs / 1000,
      )} s; ask again in ${init.retryAfterSeconds} s.`,
      retryable: true,
      retryAfter: init.retryAfterSeconds,
    });
    this.name = "PendingTimeoutError";
    this.pendingStatus = pendingStatus;
    this.jobId = jobId;
    this.retryAfterSeconds = init.retryAfterSeconds;
    this.waitedMs = init.waitedMs;
    this.body = init.body;
  }
}
