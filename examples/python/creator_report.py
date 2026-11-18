"""Creator report: profile, content analysis and recent LIVE sessions.

Usage: python creator_report.py [HANDLE]     e.g. python creator_report.py tiktok
Scopes: creator:read (profile, analysis), live:read (sessions)
"""

import sys

from toktokdata import PendingTimeoutError, TokTokData, TokTokDataError

from common import exit_with_error

MAX_SESSIONS = 30


def fmt(n: int | float | None) -> str:
    return "-" if n is None else f"{n:,}"


def main() -> None:
    handle = (sys.argv[1] if len(sys.argv) > 1 else "tiktok").lstrip("@")
    client = TokTokData()

    try:
        # A creator seen for the first time answers 202 "resolving"; get() waits and asks again.
        creator = client.creators.get(handle)["data"]["creator"]
        analysis = client.creators.analysis(handle)["data"]
    except (TokTokDataError, PendingTimeoutError) as err:
        exit_with_error(err)

    verified = " [verified]" if creator["verified"] else ""
    print(f"{creator['displayName'] or creator['uniqueId']} (@{creator['uniqueId']}){verified}")
    print(
        f"  region {creator['region'] or '?'}, {fmt(creator['followerCount'])} followers, "
        f"{fmt(creator['awemeCount'])} videos, {fmt(creator['totalFavorited'])} likes"
    )
    print(f"  last refreshed {creator['lastRefreshedAt'] or '-'}")

    sample = analysis["sample"]
    print(f"\nContent ({sample['videosAnalyzed']} videos over {sample['spanDays']} days)")
    if eng := analysis.get("engagement"):
        print(
            f"  plays avg {fmt(eng['avgPlays'])}, median {fmt(eng['medianPlays'])}, "
            f"engagement {eng['engagementRate']:.1%}"
        )
    if cadence := analysis.get("cadence"):
        print(f"  {cadence['postsPerWeek']} posts per week")
    for limit in analysis.get("limits") or []:
        print(f"  note: {limit['message']}")

    print(f"\nRecent LIVE sessions (up to {MAX_SESSIONS})")
    shown = 0
    try:
        # The generator follows data.nextCursor page by page until it is null.
        for page in client.live.iter_sessions(creator_unique_id=creator["uniqueId"], limit=20):
            for s in page["data"]["items"]:
                minutes = "-" if s["durationSeconds"] is None else round(s["durationSeconds"] / 60)
                print(
                    f"  {s['startedAt']}  {minutes:>4} min  peak {fmt(s['peakViewerCount']):>7} viewers  "
                    f"{fmt(s['totalGiftDiamonds']):>9} diamonds"
                )
                shown += 1
                if shown >= MAX_SESSIONS:
                    break
            if shown >= MAX_SESSIONS:
                break
    except TokTokDataError as err:
        exit_with_error(err)
    if shown == 0:
        print("  no recorded sessions")


if __name__ == "__main__":
    main()
