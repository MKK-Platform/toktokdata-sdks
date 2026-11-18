"""Export an official board as CSV and save it to a file.

Usage: python export_csv.py [REGION] [BOARD] [OUT_FILE]     e.g. python export_csv.py VN daily
Scope: export
"""

import sys
from pathlib import Path

from toktokdata import TokTokData, TokTokDataError

from common import exit_with_error


def main() -> None:
    region = sys.argv[1] if len(sys.argv) > 1 else "VN"
    board = sys.argv[2] if len(sys.argv) > 2 else "daily"
    client = TokTokData()

    try:
        # 201 when the file is ready at once, 202 while it is being built.
        job = client.exports.create(dataset="rankings", board=board, region=region, format="csv")
        print(f"export {job['id']}: {job['status']}")
        if job["status"] not in ("ready", "failed"):
            job = client.exports.wait_until_ready(job["id"])
        if job["status"] == "failed":
            sys.exit(f"export failed: {job.get('error') or 'unknown error'}")
        content = client.exports.download(job["id"])
    except TokTokDataError as err:
        exit_with_error(err)

    out = Path(sys.argv[3] if len(sys.argv) > 3 else job.get("filename") or f"toktokdata-{board}-{region}.csv")
    if isinstance(content, str):
        out.write_text(content, encoding="utf-8")
    else:
        out.write_bytes(content)
    print(f"saved {job.get('rowCount') or '?'} rows to {out}")


if __name__ == "__main__":
    main()
