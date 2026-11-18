"""Top 10 of an official TikTok LIVE board for one region.

Usage: python leaderboard.py [REGION] [BOARD]     e.g. python leaderboard.py VN hourly
Scope: rank:read
"""

import sys

from toktokdata import TokTokData, TokTokDataError

from common import exit_with_error


def main() -> None:
    region = sys.argv[1] if len(sys.argv) > 1 else "VN"
    board_key = sys.argv[2] if len(sys.argv) > 2 else "hourly"

    client = TokTokData()  # reads TOKTOKDATA_API_KEY
    try:
        res = client.rankings.official(board_key, region=region, limit=10)
    except TokTokDataError as err:
        exit_with_error(err)

    board = res["data"]["board"]
    print(f"{board['board']} board, {board['region']}, period {board['periodKey']}")
    if not board["publishable"]:
        # Coverage below the gate: the order may still change, so label it as partial.
        cov = board["coverage"]
        print(f"(partial: coverage {cov['ratio']:.0%} of the {cov['threshold']:.0%} gate)")
    for e in board["entries"]:
        who = f"(masked, plan {e['unmaskTier']})" if e["masked"] else f"{e['nickname'] or ''} @{e['uniqueId']}"
        live = "  LIVE" if e["isLive"] else ""
        print(f"{e['rank']:>3}  {who:<40} {e['score']:>12,}{live}")


if __name__ == "__main__":
    main()
