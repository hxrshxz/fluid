#!/usr/bin/env python3
"""Generate weekly queue and state from Fluid issue checklist."""

import argparse
import json
import re
import subprocess
import sys
from pathlib import Path
from typing import Dict, List, Optional


REPO = "fluid-cloudnative/fluid"
ISSUE_NUMBER = 5676
ROOT = Path(__file__).resolve().parent.parent
QUEUE_PATH = ROOT / "automation" / "weekly_queue.yaml"
STATE_PATH = ROOT / "automation" / "state.json"

WEEK_RE = re.compile(
    r"^\s{0,3}#{1,6}\s*((?:Week\s+\d+[^\n]*)|(?:Buffer[^\n]*))\s*$",
    re.IGNORECASE,
)
CHECKBOX_RE = re.compile(r"^\s*[-*]\s*\[( |x|X)\]\s*(.+?)\s*$")
PATH_RE = re.compile(r"`([^`]+)`|([A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+)")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Fetch issue #5676 checklist, group files into package waves, "
            "and write automation/weekly_queue.yaml + automation/state.json."
        )
    )
    parser.add_argument(
        "--week",
        help="Limit output to one week section name (example: 'Week 1').",
    )
    parser.add_argument(
        "--chunk-size",
        type=int,
        default=5,
        help="Maximum files per package wave chunk (default: 5).",
    )
    return parser.parse_args()


def fail(message: str) -> None:
    print(f"error: {message}", file=sys.stderr)
    raise SystemExit(1)


def atomic_write_text(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temp_path = path.with_suffix(path.suffix + ".tmp")
    temp_path.write_text(content, encoding="utf-8")
    temp_path.replace(path)


def run_gh_issue_view() -> Dict[str, object]:
    command = [
        "gh",
        "issue",
        "view",
        str(ISSUE_NUMBER),
        "-R",
        REPO,
        "--json",
        "body,title,url,number",
    ]
    proc = subprocess.run(command, text=True, capture_output=True, check=False)
    if proc.returncode != 0:
        fail(f"failed to fetch issue: {proc.stderr.strip() or 'unknown gh error'}")
    try:
        data = json.loads(proc.stdout)
    except json.JSONDecodeError as exc:
        fail(f"invalid JSON from gh: {exc}")
    if not isinstance(data, dict) or "body" not in data:
        fail("gh output missing required fields")
    return data


def normalize_week_name(raw: str) -> str:
    text = " ".join(raw.strip().split())
    if text.lower().startswith("week"):
        return (
            "Week " + text.split(None, 1)[1] if len(text.split(None, 1)) > 1 else "Week"
        )
    return text


def extract_paths(text: str) -> List[str]:
    paths: List[str] = []
    for match in PATH_RE.finditer(text):
        candidate = match.group(1) or match.group(2) or ""
        candidate = candidate.strip().strip(",.;:")
        if "/" not in candidate:
            continue
        if candidate.startswith("http://") or candidate.startswith("https://"):
            continue
        paths.append(candidate)
    return paths


def parse_weeks(body: str) -> Dict[str, List[str]]:
    weeks: Dict[str, List[str]] = {}
    seen: Dict[str, set] = {}
    current_week: Optional[str] = None

    for line in body.splitlines():
        week_match = WEEK_RE.match(line)
        if week_match:
            current_week = normalize_week_name(week_match.group(1))
            weeks.setdefault(current_week, [])
            seen.setdefault(current_week, set())
            continue

        cb_match = CHECKBOX_RE.match(line)
        if not cb_match or current_week is None:
            continue

        mark = cb_match.group(1)
        if mark.lower() == "x":
            continue

        for path in extract_paths(cb_match.group(2)):
            if path in seen[current_week]:
                continue
            seen[current_week].add(path)
            weeks[current_week].append(path)

    return weeks


def package_key(file_path: str) -> str:
    parts = file_path.split("/")
    if not parts:
        return "misc"
    if (
        len(parts) >= 4
        and parts[0] == "pkg"
        and parts[1] == "controllers"
        and parts[2] == "v1alpha1"
    ):
        return "/".join(parts[:4])
    if (
        len(parts) >= 4
        and parts[0] == "pkg"
        and parts[1] == "webhook"
        and parts[2] == "plugins"
    ):
        return "/".join(parts[:4])
    if len(parts) >= 3 and parts[0] == "pkg" and parts[1] == "ddc":
        return "/".join(parts[:3])
    if len(parts) >= 2 and parts[0] == "api" and parts[1] == "v1alpha1":
        return "api/v1alpha1"
    directory = str(Path(file_path).parent).replace("\\", "/")
    if directory and directory != ".":
        return directory
    if len(parts) >= 2 and parts[0] in {
        "cmd",
        "api",
        "docs",
        "sdk",
        "test",
        "tools",
        "hack",
        "csi",
    }:
        return parts[0]
    if len(parts) >= 2:
        return "/".join(parts[:2])
    return parts[0]


def slug(text: str) -> str:
    lowered = text.strip().lower()
    lowered = re.sub(r"[^a-z0-9]+", "-", lowered)
    lowered = lowered.strip("-")
    return lowered or "na"


def build_chunks(
    weeks: Dict[str, List[str]], chunk_size: int
) -> List[Dict[str, object]]:
    chunks: List[Dict[str, object]] = []
    global_order = 1
    for week_index, (week_name, file_paths) in enumerate(weeks.items(), start=1):
        by_package: Dict[str, List[str]] = {}
        for file_path in file_paths:
            key = package_key(file_path)
            by_package.setdefault(key, []).append(file_path)

        order = 1
        for pkg in sorted(by_package.keys()):
            files = by_package[pkg]
            wave = 1
            for start in range(0, len(files), chunk_size):
                selected = files[start : start + chunk_size]
                chunk_id = f"{slug(week_name)}__{slug(pkg)}__w{wave}"
                chunks.append(
                    {
                        "id": chunk_id,
                        "week": week_name,
                        "week_index": week_index,
                        "order": order,
                        "global_order": global_order,
                        "package": pkg,
                        "wave": wave,
                        "package_wave": f"{pkg}-wave{wave}",
                        "files": selected,
                    }
                )
                order += 1
                global_order += 1
                wave += 1
    return chunks


def read_existing_state() -> Dict[str, object]:
    if not STATE_PATH.exists():
        return {
            "version": 1,
            "issue": {"repo": REPO, "number": ISSUE_NUMBER},
            "chunks": [],
        }
    try:
        data = json.loads(STATE_PATH.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        fail(f"failed to read {STATE_PATH}: {exc}")
    if not isinstance(data, dict):
        fail(f"invalid state format in {STATE_PATH}")
    if not isinstance(data.get("chunks", []), list):
        fail(f"invalid chunks array in {STATE_PATH}")
    return data


def build_state(
    issue_data: Dict[str, object], chunks: List[Dict[str, object]]
) -> Dict[str, object]:
    existing = read_existing_state()
    existing_by_id: Dict[str, Dict[str, object]] = {}
    for item in existing.get("chunks", []):
        if isinstance(item, dict) and isinstance(item.get("id"), str):
            existing_by_id[item["id"]] = item

    state_chunks: List[Dict[str, object]] = []
    for chunk in chunks:
        old = existing_by_id.get(chunk["id"], {})
        state_chunks.append(
            {
                "id": chunk["id"],
                "week": chunk["week"],
                "week_index": chunk.get("week_index", old.get("week_index", 0)),
                "order": chunk["order"],
                "global_order": chunk.get("global_order", old.get("global_order", 0)),
                "package": chunk["package"],
                "wave": chunk["wave"],
                "status": old.get("status", "pending"),
                "pr_url": old.get("pr_url", ""),
                "notes": old.get("notes", ""),
                "files": chunk["files"],
            }
        )

    return {
        "version": 1,
        "issue": {
            "repo": REPO,
            "number": issue_data.get("number", ISSUE_NUMBER),
            "title": issue_data.get("title", ""),
            "url": issue_data.get("url", ""),
        },
        "chunks": state_chunks,
    }


def yaml_quote(value: str) -> str:
    escaped = value.replace("'", "''")
    return f"'{escaped}'"


def build_weekly_yaml(
    issue_data: Dict[str, object], chunks: List[Dict[str, object]]
) -> str:
    lines: List[str] = []
    lines.append("# Generated by scripts/plan_week.py. Do not edit by hand.")
    lines.append("issue:")
    lines.append(f"  repo: {yaml_quote(REPO)}")
    lines.append(f"  number: {issue_data.get('number', ISSUE_NUMBER)}")
    lines.append(f"  title: {yaml_quote(str(issue_data.get('title', '')))}")
    lines.append(f"  url: {yaml_quote(str(issue_data.get('url', '')))}")

    if not chunks:
        lines.append("weeks: []")
        return "\n".join(lines) + "\n"

    lines.append("weeks:")

    weeks: Dict[str, List[Dict[str, object]]] = {}
    for chunk in chunks:
        weeks.setdefault(str(chunk["week"]), []).append(chunk)

    for week_name, week_chunks in weeks.items():
        lines.append(f"  - name: {yaml_quote(week_name)}")
        lines.append("    chunks:")
        for chunk in week_chunks:
            lines.append(f"      - id: {yaml_quote(str(chunk['id']))}")
            lines.append(f"        week_index: {chunk.get('week_index', 0)}")
            lines.append(f"        order: {chunk['order']}")
            lines.append(f"        global_order: {chunk.get('global_order', 0)}")
            lines.append(
                f"        package_wave: {yaml_quote(str(chunk['package_wave']))}"
            )
            lines.append(f"        package: {yaml_quote(str(chunk['package']))}")
            lines.append(f"        wave: {chunk['wave']}")
            lines.append("        files:")
            for file_path in chunk["files"]:
                lines.append(f"          - {yaml_quote(file_path)}")

    return "\n".join(lines) + "\n"


def print_summary(chunks: List[Dict[str, object]]) -> None:
    week_counts: Dict[str, Dict[str, int]] = {}
    for chunk in chunks:
        week = str(chunk["week"])
        bucket = week_counts.setdefault(week, {"chunks": 0, "files": 0})
        bucket["chunks"] += 1
        bucket["files"] += len(chunk["files"])

    print("Week       Chunks  Files")
    print("---------  ------  -----")
    total_chunks = 0
    total_files = 0
    for week, counts in week_counts.items():
        print(f"{week:<9}  {counts['chunks']:<6}  {counts['files']:<5}")
        total_chunks += counts["chunks"]
        total_files += counts["files"]
    print("---------  ------  -----")
    print(f"TOTAL      {total_chunks:<6}  {total_files:<5}")


def main() -> None:
    args = parse_args()
    if args.chunk_size <= 0:
        fail("--chunk-size must be > 0")

    issue_data = run_gh_issue_view()
    weeks = parse_weeks(str(issue_data.get("body", "")))

    if args.week:
        wanted = args.week.strip().lower()
        weeks = {
            k: v
            for k, v in weeks.items()
            if k.lower() == wanted or k.lower().startswith(wanted + " ")
        }
        if not weeks:
            fail(f"week not found: {args.week}")

    chunks = build_chunks(weeks, args.chunk_size)
    queue_text = build_weekly_yaml(issue_data, chunks)
    state = build_state(issue_data, chunks)

    atomic_write_text(QUEUE_PATH, queue_text)
    atomic_write_text(STATE_PATH, json.dumps(state, indent=2, sort_keys=True) + "\n")

    print(f"wrote {QUEUE_PATH.relative_to(ROOT)}")
    print(f"wrote {STATE_PATH.relative_to(ROOT)}")
    print_summary(chunks)


if __name__ == "__main__":
    main()
