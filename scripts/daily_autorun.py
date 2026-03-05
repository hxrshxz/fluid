#!/usr/bin/env python3
"""Daily unattended queue runner for Fluid automation/state.json."""

import argparse
import datetime as dt
import json
import math
import re
import subprocess
import sys
from pathlib import Path
from typing import Dict, List, Optional, Tuple


ROOT = Path(__file__).resolve().parent.parent
STATE_PATH = ROOT / "automation" / "state.json"
LOG_ROOT = ROOT / "automation" / "logs"
PAUSE_PATH = ROOT / "automation" / "PAUSE"
REVIEW_STATE_PATH = ROOT / "automation" / "review_state.json"
OUTCOME_TOKENS = (
    "IMPLEMENTED_NO_PR",
    "PR_READY_PENDING_CONFIRMATION",
    "BLOCKED",
)
WEEK_RANGE_RE = re.compile(
    r"^(Week\s+\d+|Buffer\s+Week)\s*\(\s*(\d{1,2})\s+([A-Za-z]{3})\s*[\u2013-]\s*(\d{1,2})\s+([A-Za-z]{3})\s*\)$"
)
PR_URL_RE = re.compile(r"https://github\.com/[\w.-]+/[\w.-]+/pull/\d+")
PR_NUMBER_RE = re.compile(r"/pull/(\d+)")
MONTHS = {
    "jan": 1,
    "feb": 2,
    "mar": 3,
    "apr": 4,
    "may": 5,
    "jun": 6,
    "jul": 7,
    "aug": 8,
    "sep": 9,
    "oct": 10,
    "nov": 11,
    "dec": 12,
}


def fail(message: str) -> None:
    print(f"error: {message}", file=sys.stderr)
    raise SystemExit(1)


def parse_args(argv: Optional[List[str]] = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Run daily unattended queue chunks from automation/state.json "
            "using opencode fluid-pr-agent."
        )
    )
    parser.add_argument(
        "--no-refresh",
        action="store_true",
        help="Skip queue refresh via python3 scripts/plan_week.py.",
    )
    parser.add_argument(
        "--max-chunks-per-day",
        type=int,
        default=None,
        help="Maximum chunks to process today (default: 2).",
    )
    parser.add_argument(
        "--auto-open-pr",
        action="store_true",
        help="Allow automation mode where PR-ready output can be finalized with PR URL.",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Show selection and planned actions without changing state or running commands.",
    )
    parser.add_argument(
        "--date",
        help="Override current date (YYYY-MM-DD) for schedule testing.",
    )
    parser.add_argument(
        "--ignore-pause",
        action="store_true",
        help="Run even if automation/PAUSE exists.",
    )
    parser.add_argument(
        "--disable-post-review",
        action="store_true",
        help="Disable automatic post-review triage sweep for known PR URLs.",
    )
    parser.add_argument(
        "--post-review-rounds",
        type=int,
        default=3,
        help="Rounds for post-review triage per PR (default: 3).",
    )
    parser.add_argument(
        "--post-review-max-triggers",
        type=int,
        default=5,
        help="Maximum post-review triggers per PR (default: 5).",
    )
    args = parser.parse_args(argv)
    if args.max_chunks_per_day is not None and args.max_chunks_per_day <= 0:
        fail("--max-chunks-per-day must be > 0")
    if args.post_review_rounds < 1 or args.post_review_rounds > 5:
        fail("--post-review-rounds must be between 1 and 5")
    if args.post_review_max_triggers < 1 or args.post_review_max_triggers > 20:
        fail("--post-review-max-triggers must be between 1 and 20")
    return args


def atomic_write_text(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temp_path = path.with_suffix(path.suffix + ".tmp")
    temp_path.write_text(content, encoding="utf-8")
    temp_path.replace(path)


def load_state() -> Dict[str, object]:
    if not STATE_PATH.exists():
        fail(f"missing state file: {STATE_PATH}. Run scripts/plan_week.py first.")
    try:
        data = json.loads(STATE_PATH.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        fail(f"failed to read {STATE_PATH}: {exc}")
    if not isinstance(data, dict) or not isinstance(data.get("chunks"), list):
        fail("state.json must contain an object with a 'chunks' list")
    return data


def save_state(state: Dict[str, object]) -> None:
    atomic_write_text(STATE_PATH, json.dumps(state, indent=2, sort_keys=True) + "\n")


def load_review_state() -> Dict[str, object]:
    if not REVIEW_STATE_PATH.exists():
        return {"version": 1, "prs": {}}
    try:
        data = json.loads(REVIEW_STATE_PATH.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        fail(f"failed to read {REVIEW_STATE_PATH}: {exc}")
    if not isinstance(data, dict):
        fail("review_state.json must be an object")
    prs = data.get("prs")
    if not isinstance(prs, dict):
        data["prs"] = {}
    return data


def save_review_state(state: Dict[str, object]) -> None:
    atomic_write_text(
        REVIEW_STATE_PATH, json.dumps(state, indent=2, sort_keys=True) + "\n"
    )


def chunk_sort_key(chunk: Dict[str, object]) -> Tuple[int, int, int, str]:
    return (
        int(chunk.get("global_order", 0)),
        int(chunk.get("week_index", 0)),
        int(chunk.get("order", 0)),
        str(chunk.get("id", "")),
    )


def parse_run_date(raw: Optional[str]) -> dt.date:
    if not raw:
        return dt.date.today()
    try:
        return dt.date.fromisoformat(raw)
    except ValueError:
        fail("--date must be in YYYY-MM-DD format")
    raise AssertionError("unreachable")


def parse_week_window(
    week_name: str, year_hint: int
) -> Optional[Tuple[dt.date, dt.date]]:
    match = WEEK_RANGE_RE.match(week_name.strip())
    if not match:
        return None

    start_day = int(match.group(2))
    start_mon = MONTHS.get(match.group(3).lower())
    end_day = int(match.group(4))
    end_mon = MONTHS.get(match.group(5).lower())
    if start_mon is None or end_mon is None:
        return None

    start_year = year_hint
    end_year = year_hint
    if end_mon < start_mon:
        end_year += 1

    try:
        start_date = dt.date(start_year, start_mon, start_day)
        end_date = dt.date(end_year, end_mon, end_day)
    except ValueError:
        return None
    if end_date < start_date:
        return None
    return (start_date, end_date)


def select_active_week(
    chunks: List[Dict[str, object]], run_date: dt.date
) -> Optional[Tuple[str, dt.date, dt.date]]:
    windows: Dict[str, Tuple[dt.date, dt.date]] = {}
    for chunk in chunks:
        week_name = str(chunk.get("week", "")).strip()
        if week_name in windows:
            continue
        window = parse_week_window(week_name, run_date.year)
        if window is None:
            continue
        windows[week_name] = window

    active: List[Tuple[str, dt.date, dt.date]] = []
    for week_name, (start_date, end_date) in windows.items():
        if start_date <= run_date <= end_date:
            active.append((week_name, start_date, end_date))

    if not active:
        return None

    active.sort(key=lambda item: item[1])
    return active[0]


def select_chunks_for_today(
    chunks: List[Dict[str, object]],
    active_week: Optional[Tuple[str, dt.date, dt.date]],
    run_date: dt.date,
    max_chunks_per_day: int,
    max_flag_explicitly_set: bool,
) -> Tuple[List[Dict[str, object]], str]:
    pending = [
        c for c in sorted(chunks, key=chunk_sort_key) if c.get("status") == "pending"
    ]
    if not pending:
        return ([], "no pending chunks")

    if active_week is not None:
        week_name, _start, end_date = active_week
        week_pending = [c for c in pending if str(c.get("week", "")) == week_name]
        if week_pending:
            days_left = max(1, (end_date - run_date).days + 1)
            target_per_day = int(math.ceil(len(week_pending) / float(days_left)))
            to_take = min(max_chunks_per_day, target_per_day)
            return (
                week_pending[:to_take],
                (
                    f"active week '{week_name}' pending={len(week_pending)} "
                    f"days_left={days_left} target_per_day={target_per_day}"
                ),
            )

    fallback_limit = max_chunks_per_day if max_flag_explicitly_set else 1
    chosen = pending[:fallback_limit]
    return (
        chosen,
        f"fallback to global pending order (limit={fallback_limit})",
    )


def run_refresh() -> None:
    command = ["python3", "scripts/plan_week.py"]
    proc = subprocess.run(
        command, text=True, capture_output=True, check=False, cwd=ROOT
    )
    if proc.returncode != 0:
        details = (proc.stderr or proc.stdout).strip() or "unknown error"
        fail(f"refresh failed: {details}")


def build_agent_message(chunk: Dict[str, object], auto_open_pr: bool) -> str:
    files = chunk.get("files", [])
    file_lines = "\n".join(f"- {item}" for item in files)
    policy_line = (
        "You may create/open a PR if needed and include its URL in final output."
        if auto_open_pr
        else "Do not open or create any pull request without explicit user confirmation."
    )
    return (
        "Execute this queue chunk with minimal deterministic changes.\n"
        f"Chunk ID: {chunk.get('id')}\n"
        f"Week: {chunk.get('week')}\n"
        f"Package: {chunk.get('package')}\n"
        "Files:\n"
        f"{file_lines}\n\n"
        "Final output requirements:\n"
        "- Print exactly one outcome token: IMPLEMENTED_NO_PR or "
        "PR_READY_PENDING_CONFIRMATION or BLOCKED\n"
        "- If PR_READY_PENDING_CONFIRMATION, include PR URL when available\n"
        f"- {policy_line}\n"
    )


def run_chunk(chunk: Dict[str, object], auto_open_pr: bool) -> Tuple[int, str]:
    message = build_agent_message(chunk, auto_open_pr)
    command = ["opencode", "run", "--agent", "fluid-pr-agent", message]
    proc = subprocess.run(
        command, text=True, capture_output=True, check=False, cwd=ROOT
    )
    merged_output = (proc.stdout or "") + ("\n" + proc.stderr if proc.stderr else "")
    return (proc.returncode, merged_output)


def parse_outcome_token(output: str) -> Optional[str]:
    for token in OUTCOME_TOKENS:
        if re.search(rf"\b{re.escape(token)}\b", output):
            return token
    return None


def parse_pr_url(output: str) -> str:
    match = PR_URL_RE.search(output)
    return match.group(0) if match else ""


def extract_pr_number(url: str) -> int:
    match = PR_NUMBER_RE.search(url)
    if not match:
        return 0
    try:
        return int(match.group(1))
    except ValueError:
        return 0


def build_post_review_message(pr_number: int, rounds: int) -> str:
    return (
        f"Run fluid-pr-agent in post-review mode for PR #{pr_number}. "
        f"Use post_review_rounds={rounds}. Fetch reviewer/bot comments, classify "
        "must-fix/optional/ignore, apply only valid in-scope must-fix items, re-run "
        "verification, and return Review Disposition + Final State. Do not create/open PR."
    )


def append_note(chunk: Dict[str, object], text: str) -> None:
    current = str(chunk.get("notes", "")).strip()
    if not current:
        chunk["notes"] = text
        return
    if text in current:
        chunk["notes"] = current
        return
    chunk["notes"] = f"{current}; {text}"


def write_chunk_log(run_date: dt.date, chunk_id: str, content: str) -> Path:
    day_dir = LOG_ROOT / run_date.isoformat()
    day_dir.mkdir(parents=True, exist_ok=True)
    safe_name = re.sub(r"[^A-Za-z0-9_.-]", "_", chunk_id)
    log_path = day_dir / f"{safe_name}.log"
    log_path.write_text(content, encoding="utf-8")
    return log_path


def require_repo_root_cwd() -> None:
    cwd = Path.cwd().resolve()
    if cwd != ROOT.resolve():
        fail(f"run from repo root: cd {ROOT} && python3 scripts/daily_autorun.py")


def read_pause_reason() -> str:
    if not PAUSE_PATH.exists():
        return ""
    try:
        return PAUSE_PATH.read_text(encoding="utf-8").strip()
    except OSError:
        return ""


def main() -> None:
    args = parse_args()
    require_repo_root_cwd()

    if PAUSE_PATH.exists() and not args.ignore_pause:
        print(f"paused: found {PAUSE_PATH.relative_to(ROOT)}")
        reason = read_pause_reason()
        if reason:
            print(f"pause_reason={reason}")
        print("No chunks executed.")
        return

    if PAUSE_PATH.exists() and args.ignore_pause:
        print("warning: pause switch exists but --ignore-pause is set")

    run_date = parse_run_date(args.date)
    max_chunks = args.max_chunks_per_day if args.max_chunks_per_day is not None else 2

    if args.dry_run:
        refresh_mode = "skip" if args.no_refresh else "would run"
        print(f"dry-run date={run_date.isoformat()} refresh={refresh_mode}")
    elif not args.no_refresh:
        run_refresh()

    state = load_state()
    chunks = list(state.get("chunks", []))
    if not chunks:
        fail("state has no chunks. Run scripts/plan_week.py first.")

    active_week = select_active_week(chunks, run_date)
    selected, reason = select_chunks_for_today(
        chunks,
        active_week,
        run_date,
        max_chunks,
        args.max_chunks_per_day is not None,
    )

    active_text = active_week[0] if active_week else "(none)"
    print(f"active_week={active_text}")
    print(f"selection={reason}")
    print(f"selected_chunks={len(selected)}")

    if not selected:
        if args.dry_run or args.disable_post_review:
            return

    review_state = load_review_state()

    if args.dry_run:
        for chunk in selected:
            print(f"- would run {chunk.get('id')}")
        if not args.disable_post_review:
            prs = sorted(
                {
                    extract_pr_number(str(c.get("pr_url", "")))
                    for c in chunks
                    if extract_pr_number(str(c.get("pr_url", ""))) > 0
                }
            )
            if prs:
                print(
                    f"- would run post-review triage for PRs: {', '.join(str(p) for p in prs)}"
                )
        return

    results: List[str] = []
    for chunk in selected:
        chunk_id = str(chunk.get("id", ""))
        chunk["status"] = "in_progress"
        save_state(state)

        return_code, output = run_chunk(chunk, args.auto_open_pr)
        log_path = write_chunk_log(run_date, chunk_id, output)

        outcome = parse_outcome_token(output)
        pr_url = parse_pr_url(output)
        if outcome == "BLOCKED":
            chunk["status"] = "blocked"
        elif outcome == "IMPLEMENTED_NO_PR":
            chunk["status"] = "done"
        elif outcome == "PR_READY_PENDING_CONFIRMATION":
            chunk["status"] = "done"
            if args.auto_open_pr and pr_url:
                chunk["pr_url"] = pr_url
            else:
                append_note(chunk, "pr_ready_pending_confirmation")
        else:
            chunk["status"] = "blocked"
            append_note(chunk, "missing_outcome_token")

        if return_code != 0:
            append_note(chunk, f"agent_exit_code={return_code}")

        save_state(state)
        results.append(
            (
                f"- {chunk_id}: status={chunk.get('status')} outcome={outcome or 'UNKNOWN'} "
                f"pr_url={'yes' if pr_url else 'no'} log={log_path.relative_to(ROOT)}"
            )
        )

    print("run_summary:")
    for line in results:
        print(line)

    if args.disable_post_review:
        print("post_review: disabled")
        return

    prs_to_review = sorted(
        {
            extract_pr_number(str(c.get("pr_url", "")))
            for c in chunks
            if extract_pr_number(str(c.get("pr_url", ""))) > 0
        }
    )
    if not prs_to_review:
        print("post_review: no PR URLs in state")
        return

    review_lines: List[str] = []
    prs_map = review_state.setdefault("prs", {})
    for pr_number in prs_to_review:
        key = str(pr_number)
        entry = prs_map.get(
            key,
            {
                "triggers_used": 0,
                "max_triggers": args.post_review_max_triggers,
                "last_rounds": 0,
                "last_run_at": "",
                "last_outcome": "",
                "last_returncode": 0,
            },
        )
        used = int(entry.get("triggers_used", 0))
        max_triggers = max(
            int(entry.get("max_triggers", args.post_review_max_triggers)),
            args.post_review_max_triggers,
        )
        entry["max_triggers"] = max_triggers
        if used >= max_triggers:
            review_lines.append(
                f"- PR #{pr_number}: skipped (limit {used}/{max_triggers})"
            )
            prs_map[key] = entry
            continue

        message = build_post_review_message(pr_number, args.post_review_rounds)
        command = ["opencode", "run", "--agent", "fluid-pr-agent", message]
        proc = subprocess.run(
            command, text=True, capture_output=True, check=False, cwd=ROOT
        )
        combined = (proc.stdout or "") + ("\n" + proc.stderr if proc.stderr else "")
        log_path = write_chunk_log(run_date, f"post-review-pr-{pr_number}", combined)

        outcome = parse_outcome_token(combined) or "UNKNOWN"
        entry["triggers_used"] = used + 1
        entry["last_rounds"] = args.post_review_rounds
        entry["last_run_at"] = dt.datetime.now().isoformat(timespec="seconds")
        entry["last_outcome"] = outcome
        entry["last_returncode"] = int(proc.returncode)
        prs_map[key] = entry

        review_lines.append(
            f"- PR #{pr_number}: outcome={outcome} rc={proc.returncode} used={entry['triggers_used']}/{entry['max_triggers']} log={log_path.relative_to(ROOT)}"
        )

    save_review_state(review_state)
    print("post_review_summary:")
    for line in review_lines:
        print(line)


if __name__ == "__main__":
    main()
