export interface PaginateOptions {
  /** Stop after this many pages (default: no limit). */
  maxPages?: number;
}

/** Reads `data.nextCursor` of a page; `null` when there is no next page. */
export function nextCursorOf(page: unknown): string | null {
  if (typeof page !== "object" || page === null) return null;
  const data = (page as { data?: unknown }).data;
  if (typeof data !== "object" || data === null) return null;
  const cursor = (data as { nextCursor?: unknown }).nextCursor;
  return typeof cursor === "string" && cursor !== "" ? cursor : null;
}

/**
 * Walks a cursor list page by page: calls `fetchPage(params)`, yields the whole page, then asks
 * again with `cursor` set to the page's `data.nextCursor` until it is `null`.
 *
 * ```ts
 * for await (const page of paginate((p) => client.creators.followers("linh.live", p), { limit: 100 })) {
 *   console.log(page.data.entries.length);
 * }
 * ```
 */
export async function* paginate<P extends object, R>(
  fetchPage: (params: P) => Promise<R>,
  params?: P,
  options: PaginateOptions = {},
): AsyncGenerator<R, void, undefined> {
  let current = { ...(params ?? {}) } as P;
  const seen = new Set<string>();
  for (let pages = 0; options.maxPages === undefined || pages < options.maxPages; pages++) {
    const page = await fetchPage(current);
    yield page;
    const cursor = nextCursorOf(page);
    if (cursor === null || seen.has(cursor)) return;
    seen.add(cursor);
    current = { ...current, cursor } as P;
  }
}
