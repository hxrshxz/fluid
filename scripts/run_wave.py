#!/usr/bin/env python3
"""Pick and update execution chunks from automation/state.json."""

import argparse
import json
import sys
from pathlib import Path
from typing import Dict, List, Optional


ROOT = Path(__file__).resolve().parent.parent
STATE_PATH = ROOT / "automation" / "state.json"
VALID_STATUS = ["pending", "in_progress", "blocked", "done", "skipped"]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Select the next pending package-wave chunk (or a specific chunk id), "
            "print details and a copy-ready fluid-pr-agent prompt, and optionally update state."
        )
    )
    parser.add_argument(
        "--chunk-id",
        help="Specific chunk id to show/update. If omitted, picks next pending by order.",
    )
    parser.add_argument(
        "--set-status",
        choices=VALID_STATUS,
        help="Update selected chunk status.",
    )
    parser.add_argument(
        "--pr-url",
        help="Update selected chunk PR URL (record only; does not create PR).",
    )
    parser.add_argument(
        "--notes",
        help="Update selected chunk notes.",
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


def chunk_sort_key(chunk: Dict[str, object]) -> tuple:
    return (
        int(chunk.get("global_order", 0)),
        int(chunk.get("week_index", 0)),
        int(chunk.get("order", 0)),
        str(chunk.get("id", "")),
    )


def select_chunk(
    chunks: List[Dict[str, object]], chunk_id: Optional[str]
) -> Dict[str, object]:
    if chunk_id:
        for chunk in chunks:
            if str(chunk.get("id")) == chunk_id:
                return chunk
        fail(f"chunk id not found: {chunk_id}")

    for chunk in sorted(chunks, key=chunk_sort_key):
        if str(chunk.get("status", "pending")) == "pending":
            return chunk
    fail("no pending chunk found")


def format_prompt(chunk: Dict[str, object]) -> str:
    files = chunk.get("files", [])
    file_lines = "\n".join(f"- {path}" for path in files)
    return (
        "Use fluid-pr-agent for this chunk:\n"
        f"- Chunk ID: {chunk.get('id')}\n"
        f"- Week: {chunk.get('week')}\n"
        f"- Package: {chunk.get('package')}\n"
        f"- Wave: {chunk.get('wave')}\n"
        "- Files:\n"
        f"{file_lines}\n\n"
        "Execution policy:\n"
        "- Implement only these files for this chunk.\n"
        "- Keep changes minimal and deterministic.\n"
        "- Run relevant validation commands before proposing commit.\n"
        "- DO NOT open or create any pull request without explicit user confirmation.\n"
    )


def maybe_update_chunk(chunk: Dict[str, object], args: argparse.Namespace) -> bool:
    changed = False
    if args.set_status is not None and chunk.get("status") != args.set_status:
        chunk["status"] = args.set_status
        changed = True
    if args.pr_url is not None and chunk.get("pr_url") != args.pr_url:
        chunk["pr_url"] = args.pr_url
        changed = True
    if args.notes is not None and chunk.get("notes") != args.notes:
        chunk["notes"] = args.notes
        changed = True
    return changed


def main() -> None:
    args = parse_args()
    state = load_state()
    chunks = state.get("chunks", [])
    if not chunks:
        fail("state has no chunks. Run scripts/plan_week.py first.")

    target = select_chunk(chunks, args.chunk_id)
    changed = maybe_update_chunk(target, args)
    if changed:
        atomic_write_text(
            STATE_PATH, json.dumps(state, indent=2, sort_keys=True) + "\n"
        )

    print(f"Chunk ID : {target.get('id')}")
    print(f"Week     : {target.get('week')}")
    print(f"Order    : {target.get('order')}")
    print(f"Package  : {target.get('package')}")
    print(f"Wave     : {target.get('wave')}")
    print(f"Status   : {target.get('status', 'pending')}")
    print(f"PR URL   : {target.get('pr_url', '')}")
    print(f"Notes    : {target.get('notes', '')}")
    print("Files:")
    for file_path in target.get("files", []):
        print(f"  - {file_path}")

    print("\nReady-to-copy prompt:\n")
    print(format_prompt(target))

    if changed:
        print(f"updated {STATE_PATH.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
