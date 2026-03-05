#!/usr/bin/env python3
"""Report weekly queue status and throughput estimate."""

import argparse
import json
import math
import sys
from pathlib import Path
from typing import Dict, List


ROOT = Path(__file__).resolve().parent.parent
STATE_PATH = ROOT / "automation" / "state.json"
TRACKED_STATUS = ["pending", "in_progress", "blocked", "done", "skipped"]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Summarize automation/state.json by week and status, and estimate "
            "remaining weekly throughput required to finish pending chunks."
        )
    )
    parser.add_argument(
        "--state-file",
        default=str(STATE_PATH),
        help="Path to state.json (default: automation/state.json).",
    )
    return parser.parse_args()


def fail(message: str) -> None:
    print(f"error: {message}", file=sys.stderr)
    raise SystemExit(1)


def load_state(path: Path) -> Dict[str, object]:
    if not path.exists():
        fail(f"state file does not exist: {path}")
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        fail(f"cannot read state file: {exc}")
    if not isinstance(data, dict) or not isinstance(data.get("chunks"), list):
        fail("state file must be an object with a 'chunks' list")
    return data


def week_order(chunks: List[Dict[str, object]]) -> List[str]:
    ordered: List[str] = []
    seen = set()
    for chunk in chunks:
        week = str(chunk.get("week", "(none)"))
        if week not in seen:
            seen.add(week)
            ordered.append(week)
    return ordered


def main() -> None:
    args = parse_args()
    state = load_state(Path(args.state_file))
    chunks: List[Dict[str, object]] = list(state.get("chunks", []))

    if not chunks:
        print("No chunks found in state file.")
        return

    weeks = week_order(chunks)
    week_stats: Dict[str, Dict[str, int]] = {
        week: {status: 0 for status in TRACKED_STATUS} for week in weeks
    }
    unknown_status_count = 0

    for chunk in chunks:
        week = str(chunk.get("week", "(none)"))
        status = str(chunk.get("status", "pending"))
        if status in week_stats[week]:
            week_stats[week][status] += 1
        else:
            unknown_status_count += 1

    print("Week       total  pending  in_prog  blocked  done  skipped")
    print("---------  -----  -------  -------  -------  ----  -------")

    total_counts = {status: 0 for status in TRACKED_STATUS}
    for week in weeks:
        counts = week_stats[week]
        total = sum(counts.values())
        for status in TRACKED_STATUS:
            total_counts[status] += counts[status]
        print(
            f"{week:<9}  {total:<5}  {counts['pending']:<7}  {counts['in_progress']:<7}  "
            f"{counts['blocked']:<7}  {counts['done']:<4}  {counts['skipped']:<7}"
        )

    print("---------  -----  -------  -------  -------  ----  -------")
    grand_total = sum(total_counts.values())
    print(
        f"TOTAL      {grand_total:<5}  {total_counts['pending']:<7}  {total_counts['in_progress']:<7}  "
        f"{total_counts['blocked']:<7}  {total_counts['done']:<4}  {total_counts['skipped']:<7}"
    )

    if unknown_status_count:
        print(f"Unknown status chunks: {unknown_status_count}")

    remaining_total = (
        total_counts["pending"] + total_counts["in_progress"] + total_counts["blocked"]
    )
    remaining_weeks = 0
    for week in weeks:
        counts = week_stats[week]
        if counts["pending"] + counts["in_progress"] + counts["blocked"] > 0:
            remaining_weeks += 1

    print("\nThroughput estimate:")
    if remaining_total == 0:
        print("- Queue is complete. No remaining chunks.")
        return

    if remaining_weeks == 0:
        print(f"- Remaining chunks: {remaining_total}")
        print("- Remaining weeks: 0")
        print("- Required throughput: n/a")
        return

    required = remaining_total / float(remaining_weeks)
    print(f"- Remaining chunks: {remaining_total}")
    print(f"- Remaining weeks with work: {remaining_weeks}")
    print(
        f"- Required throughput to finish within remaining weeks: {required:.2f} chunks/week"
    )

    completed = total_counts["done"] + total_counts["skipped"]
    completed_weeks = 0
    for week in weeks:
        counts = week_stats[week]
        if counts["done"] + counts["skipped"] > 0:
            completed_weeks += 1
    if completed > 0 and completed_weeks > 0:
        observed = completed / float(completed_weeks)
        eta_weeks = int(math.ceil(remaining_total / observed))
        print(f"- Observed throughput: {observed:.2f} chunks/week")
        print(f"- ETA at observed throughput: {eta_weeks} week(s)")
    else:
        print("- Observed throughput: n/a (no completed chunks yet)")


if __name__ == "__main__":
    main()
