#!/usr/bin/env python3
import argparse
import datetime as dt
import json
import mimetypes
import re
import subprocess
import sys
import tempfile
import time
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import parse_qs, urlparse


ROOT_DIR = Path(__file__).resolve().parents[1]
STATE_PATH = ROOT_DIR / "automation" / "state.json"
PAUSE_PATH = ROOT_DIR / "automation" / "PAUSE"
REVIEW_STATE_PATH = ROOT_DIR / "automation" / "review_state.json"
UI_DIR = ROOT_DIR / "automation" / "ui"

ALLOWED_STATUSES = {"pending", "in_progress", "blocked", "done"}
OUTCOME_TOKENS = (
    "IMPLEMENTED_NO_PR",
    "PR_READY_PENDING_CONFIRMATION",
    "BLOCKED",
)
OUTCOME_RE = re.compile(
    r"\b(IMPLEMENTED_NO_PR|PR_READY_PENDING_CONFIRMATION|BLOCKED)\b"
)
BOT_HINTS = ("bot", "coderabbit", "copilot")


def read_json_file(path: Path) -> dict:
    try:
        with path.open("r", encoding="utf-8") as handle:
            value = json.load(handle)
    except FileNotFoundError as exc:
        raise ValueError(f"missing file: {path}") from exc
    except json.JSONDecodeError as exc:
        raise ValueError(f"invalid json in {path}: {exc}") from exc
    if not isinstance(value, dict):
        raise ValueError(f"json root must be object: {path}")
    return value


def atomic_write_json(path: Path, payload: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(
        mode="w",
        encoding="utf-8",
        dir=str(path.parent),
        prefix=".tmp-state-",
        delete=False,
    ) as handle:
        json.dump(payload, handle, ensure_ascii=False, indent=2, sort_keys=True)
        handle.write("\n")
        temp_path = Path(handle.name)
    temp_path.replace(path)


def run_command(command: list[str]) -> dict:
    proc = subprocess.run(
        command,
        cwd=ROOT_DIR,
        capture_output=True,
        text=True,
        check=False,
    )
    return {
        "ok": proc.returncode == 0,
        "returncode": proc.returncode,
        "stdout": proc.stdout,
        "stderr": proc.stderr,
        "command": " ".join(command),
    }


def read_pause_state() -> dict:
    if not PAUSE_PATH.exists():
        return {"paused": False, "reason": ""}
    reason = PAUSE_PATH.read_text(encoding="utf-8").strip()
    return {"paused": True, "reason": reason}


def read_review_state() -> dict:
    if not REVIEW_STATE_PATH.exists():
        return {"version": 1, "prs": {}}
    value = read_json_file(REVIEW_STATE_PATH)
    prs = value.get("prs")
    if not isinstance(prs, dict):
        raise ValueError("review_state.prs must be an object")
    return value


def write_review_state(payload: dict) -> None:
    atomic_write_json(REVIEW_STATE_PATH, payload)


def parse_int(value: object, field: str) -> int:
    if isinstance(value, bool):
        raise ValueError(f"{field} must be integer")
    if isinstance(value, int):
        return value
    if isinstance(value, str) and value.strip().isdigit():
        return int(value.strip())
    raise ValueError(f"{field} must be integer")


def parse_optional_int(value: object, field: str) -> int | None:
    if value is None:
        return None
    if isinstance(value, str) and not value.strip():
        return None
    return parse_int(value, field)


def parse_bool(value: object, field: str, default: bool) -> bool:
    if value is None:
        return default
    if isinstance(value, bool):
        return value
    if isinstance(value, str):
        lowered = value.strip().lower()
        if lowered in {"true", "1", "yes", "on"}:
            return True
        if lowered in {"false", "0", "no", "off"}:
            return False
    raise ValueError(f"{field} must be boolean")


def parse_outcome_token(text: str) -> str:
    match = OUTCOME_RE.search(text)
    return match.group(1) if match else "UNKNOWN"


def run_json_command(command: list[str]) -> dict:
    result = run_command(command)
    if not result.get("ok"):
        details = (result.get("stderr") or result.get("stdout") or "").strip()
        return {
            "ok": False,
            "error": details or f"command failed: {result.get('command', '')}",
            "command": result.get("command", ""),
        }
    raw = (result.get("stdout") or "").strip()
    if not raw:
        return {"ok": True, "data": None, "command": result.get("command", "")}
    try:
        data = json.loads(raw)
    except json.JSONDecodeError as exc:
        return {
            "ok": False,
            "error": f"invalid json output: {exc}",
            "command": result.get("command", ""),
        }
    return {"ok": True, "data": data, "command": result.get("command", "")}


def looks_like_bot(login: str) -> bool:
    lowered = login.strip().lower()
    if not lowered:
        return False
    if lowered.endswith("[bot]"):
        return True
    return any(token in lowered for token in BOT_HINTS)


def resolve_latest_open_pr_number() -> dict:
    command = [
        "gh",
        "pr",
        "list",
        "--state",
        "open",
        "--limit",
        "1",
        "--json",
        "number",
    ]
    result = run_json_command(command)
    if not result.get("ok"):
        return result
    data = result.get("data")
    if not isinstance(data, list) or not data:
        return {"ok": False, "error": "no open pull requests found"}
    first = data[0]
    if not isinstance(first, dict):
        return {"ok": False, "error": "invalid response from gh pr list"}
    number = first.get("number")
    if not isinstance(number, int) or number <= 0:
        return {"ok": False, "error": "failed to resolve latest open PR number"}
    return {"ok": True, "pr_number": number}


def resolve_repo_slug() -> dict:
    result = run_json_command(["gh", "repo", "view", "--json", "nameWithOwner"])
    if not result.get("ok"):
        return result
    data = result.get("data")
    if not isinstance(data, dict):
        return {"ok": False, "error": "invalid response from gh repo view"}
    slug = data.get("nameWithOwner")
    if not isinstance(slug, str) or "/" not in slug:
        return {"ok": False, "error": "failed to resolve repository slug"}
    return {"ok": True, "slug": slug}


def fetch_bot_review_comments(repo_slug: str, pr_number: int) -> dict:
    endpoints = [
        f"repos/{repo_slug}/issues/{pr_number}/comments",
        f"repos/{repo_slug}/pulls/{pr_number}/reviews",
        f"repos/{repo_slug}/pulls/{pr_number}/comments",
    ]
    matches: list[dict] = []
    for endpoint in endpoints:
        result = run_json_command(["gh", "api", endpoint])
        if not result.get("ok"):
            return {
                "ok": False,
                "error": f"failed to fetch {endpoint}: {result.get('error', '')}",
                "endpoint": endpoint,
            }
        data = result.get("data")
        if data is None:
            continue
        if not isinstance(data, list):
            return {
                "ok": False,
                "error": f"unexpected response shape for {endpoint}",
                "endpoint": endpoint,
            }
        for item in data:
            if not isinstance(item, dict):
                continue
            user = item.get("user")
            login = ""
            if isinstance(user, dict):
                maybe_login = user.get("login")
                if isinstance(maybe_login, str):
                    login = maybe_login
            if not looks_like_bot(login):
                continue
            matches.append(
                {
                    "login": login,
                    "url": item.get("html_url") or item.get("url") or "",
                    "source": endpoint,
                    "created_at": item.get("created_at")
                    or item.get("submitted_at")
                    or "",
                }
            )
    return {"ok": True, "comments": matches}


def parse_github_timestamp(value: str) -> dt.datetime | None:
    if not isinstance(value, str):
        return None
    text = value.strip()
    if not text:
        return None
    if text.endswith("Z"):
        text = text[:-1] + "+00:00"
    try:
        parsed = dt.datetime.fromisoformat(text)
    except ValueError:
        return None
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=dt.timezone.utc)
    return parsed


def latest_bot_comment(comments: list[dict]) -> dict | None:
    latest: dict | None = None
    latest_at: dt.datetime | None = None
    latest_url = ""
    for comment in comments:
        if not isinstance(comment, dict):
            continue
        created_at = parse_github_timestamp(str(comment.get("created_at") or ""))
        if created_at is None:
            continue
        url = str(comment.get("url") or "")
        if latest is None or latest_at is None:
            latest = comment
            latest_at = created_at
            latest_url = url
            continue
        if created_at > latest_at or (created_at == latest_at and url > latest_url):
            latest = comment
            latest_at = created_at
            latest_url = url
    return latest


def is_newer_comment(comment: dict, marker_created_at: str, marker_url: str) -> bool:
    if not isinstance(comment, dict):
        return False
    current_at = parse_github_timestamp(str(comment.get("created_at") or ""))
    if current_at is None:
        return False
    current_url = str(comment.get("url") or "")
    marker_at = parse_github_timestamp(marker_created_at)
    if marker_at is None:
        return True
    if current_at > marker_at:
        return True
    if current_at < marker_at:
        return False
    return current_url > marker_url


def poll_for_bot_review_comments(
    pr_number: int,
    rounds: int,
    interval_seconds: int,
    timeout_minutes: int,
    marker_created_at: str,
    marker_url: str,
) -> dict:
    repo_result = resolve_repo_slug()
    if not repo_result.get("ok"):
        return {
            "ok": False,
            "status": "error",
            "error": repo_result.get("error", "failed to resolve repository"),
        }
    repo_slug = str(repo_result.get("slug"))

    started_at = dt.datetime.now()
    deadline = started_at + dt.timedelta(minutes=timeout_minutes)
    attempts = 0
    last_error = ""

    while attempts < rounds:
        attempts += 1
        check = fetch_bot_review_comments(repo_slug, pr_number)
        if not check.get("ok"):
            last_error = str(check.get("error", "failed to fetch PR comments"))
            break
        comments = check.get("comments")
        if isinstance(comments, list) and comments:
            latest = latest_bot_comment(comments)
            if latest is not None and is_newer_comment(
                latest,
                marker_created_at=marker_created_at,
                marker_url=marker_url,
            ):
                return {
                    "ok": True,
                    "status": "comments_found",
                    "attempts": attempts,
                    "pr_number": pr_number,
                    "comments_count": len(comments),
                    "latest_comment": latest,
                    "started_at": started_at.isoformat(timespec="seconds"),
                    "finished_at": dt.datetime.now().isoformat(timespec="seconds"),
                }

        now = dt.datetime.now()
        if attempts >= rounds:
            break
        if now >= deadline:
            return {
                "ok": True,
                "status": "timeout",
                "attempts": attempts,
                "pr_number": pr_number,
                "comments_count": 0,
                "started_at": started_at.isoformat(timespec="seconds"),
                "finished_at": now.isoformat(timespec="seconds"),
                "message": "polling timed out before new bot comments appeared",
            }

        remaining = int((deadline - now).total_seconds())
        sleep_for = max(1, min(interval_seconds, remaining))
        time.sleep(sleep_for)

    finished_at = dt.datetime.now()
    if last_error:
        return {
            "ok": False,
            "status": "error",
            "attempts": attempts,
            "pr_number": pr_number,
            "started_at": started_at.isoformat(timespec="seconds"),
            "finished_at": finished_at.isoformat(timespec="seconds"),
            "error": last_error,
        }
    return {
        "ok": True,
        "status": "max_rounds_reached",
        "attempts": attempts,
        "pr_number": pr_number,
        "comments_count": 0,
        "started_at": started_at.isoformat(timespec="seconds"),
        "finished_at": finished_at.isoformat(timespec="seconds"),
        "message": "no new bot comments found within polling rounds",
    }


def parse_json_body(handler: BaseHTTPRequestHandler) -> dict:
    raw_len = handler.headers.get("Content-Length", "0").strip()
    try:
        body_len = int(raw_len)
    except ValueError as exc:
        raise ValueError("invalid Content-Length") from exc

    if body_len <= 0:
        return {}

    content_type = handler.headers.get("Content-Type", "")
    if "application/json" not in content_type:
        raise ValueError("Content-Type must be application/json")

    raw = handler.rfile.read(body_len)
    try:
        payload = json.loads(raw.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ValueError("request body must be valid UTF-8 JSON") from exc
    if not isinstance(payload, dict):
        raise ValueError("request body must be a JSON object")
    return payload


class UIServerHandler(BaseHTTPRequestHandler):
    server_version = "FluidUI/0.1"

    def log_message(self, fmt: str, *args: object) -> None:
        sys.stderr.write(
            "%s - - [%s] %s\n"
            % (self.address_string(), self.log_date_time_string(), fmt % args)
        )

    def _send_json(self, payload: dict, status: int = HTTPStatus.OK) -> None:
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _send_text(
        self, content: bytes, content_type: str, status: int = HTTPStatus.OK
    ) -> None:
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(content)))
        self.end_headers()
        self.wfile.write(content)

    def _serve_static(self, path: str) -> None:
        request_path = "index.html" if path in {"", "/"} else path.lstrip("/")
        candidate = (UI_DIR / request_path).resolve()
        if UI_DIR.resolve() not in [candidate, *candidate.parents]:
            self._send_json(
                {"ok": False, "error": "invalid static path"},
                status=HTTPStatus.BAD_REQUEST,
            )
            return
        if not candidate.exists() or not candidate.is_file():
            self._send_json(
                {"ok": False, "error": "not found"}, status=HTTPStatus.NOT_FOUND
            )
            return
        content_type, _ = mimetypes.guess_type(str(candidate))
        content_type = content_type or "application/octet-stream"
        try:
            data = candidate.read_bytes()
        except OSError as exc:
            self._send_json(
                {"ok": False, "error": f"cannot read static file: {exc}"},
                status=HTTPStatus.INTERNAL_SERVER_ERROR,
            )
            return
        self._send_text(
            data,
            f"{content_type}; charset=utf-8"
            if content_type.startswith("text/")
            else content_type,
        )

    def do_GET(self) -> None:
        parsed = urlparse(self.path)
        if parsed.path == "/api/state":
            try:
                state = read_json_file(STATE_PATH)
            except ValueError as exc:
                self._send_json(
                    {"ok": False, "error": str(exc)},
                    status=HTTPStatus.INTERNAL_SERVER_ERROR,
                )
                return
            self._send_json({"ok": True, "state": state})
            return

        if parsed.path == "/api/pause":
            self._send_json({"ok": True, **read_pause_state()})
            return

        if parsed.path == "/api/review/state":
            try:
                state = read_review_state()
            except ValueError as exc:
                self._send_json(
                    {"ok": False, "error": str(exc)},
                    status=HTTPStatus.INTERNAL_SERVER_ERROR,
                )
                return

            query = parse_qs(parsed.query)
            pr_vals = query.get("pr_number", [])
            resolve_latest = query.get("resolve_latest_pr", [""])[
                0
            ].strip().lower() in {
                "true",
                "1",
                "yes",
                "on",
            }

            key = str(pr_vals[0]).strip() if pr_vals else ""

            if not key and resolve_latest:
                resolved = resolve_latest_open_pr_number()
                if not resolved.get("ok"):
                    self._send_json(
                        {
                            "ok": False,
                            "error": resolved.get(
                                "error", "failed to resolve latest open PR"
                            ),
                        },
                        status=HTTPStatus.BAD_REQUEST,
                    )
                    return
                key = str(resolved.get("pr_number", ""))

            if key:
                data = state.get("prs", {}).get(key, {})
                self._send_json({"ok": True, "pr_number": key, "data": data})
                return

            self._send_json({"ok": True, "state": state})
            return

        self._serve_static(parsed.path)

    def do_POST(self) -> None:
        parsed = urlparse(self.path)
        try:
            body = parse_json_body(self)
        except ValueError as exc:
            self._send_json(
                {"ok": False, "error": str(exc)}, status=HTTPStatus.BAD_REQUEST
            )
            return

        if parsed.path == "/api/refresh":
            result = run_command(["python3", "scripts/plan_week.py"])
            self._send_json(
                result, status=HTTPStatus.OK if result["ok"] else HTTPStatus.BAD_GATEWAY
            )
            return

        if parsed.path == "/api/report":
            result = run_command(["python3", "scripts/pr_queue_report.py"])
            result["output"] = (result.get("stdout") or "") + (
                result.get("stderr") or ""
            )
            self._send_json(
                result, status=HTTPStatus.OK if result["ok"] else HTTPStatus.BAD_GATEWAY
            )
            return

        if parsed.path == "/api/dry-run":
            command = ["python3", "scripts/daily_autorun.py", "--dry-run"]
            date_value = body.get("date")
            if date_value is not None:
                if not isinstance(date_value, str) or not date_value.strip():
                    self._send_json(
                        {
                            "ok": False,
                            "error": "date must be non-empty string when provided",
                        },
                        status=HTTPStatus.BAD_REQUEST,
                    )
                    return
                command.extend(["--date", date_value.strip()])
            result = run_command(command)
            self._send_json(
                result, status=HTTPStatus.OK if result["ok"] else HTTPStatus.BAD_GATEWAY
            )
            return

        if parsed.path == "/api/chunk/update":
            chunk_id = body.get("chunk_id")
            if not isinstance(chunk_id, str) or not chunk_id.strip():
                self._send_json(
                    {"ok": False, "error": "chunk_id is required"},
                    status=HTTPStatus.BAD_REQUEST,
                )
                return
            updates = {}
            if "status" in body:
                status = body.get("status")
                if not isinstance(status, str) or status not in ALLOWED_STATUSES:
                    self._send_json(
                        {
                            "ok": False,
                            "error": f"status must be one of: {', '.join(sorted(ALLOWED_STATUSES))}",
                        },
                        status=HTTPStatus.BAD_REQUEST,
                    )
                    return
                updates["status"] = status
            if "pr_url" in body:
                pr_url = body.get("pr_url")
                if not isinstance(pr_url, str):
                    self._send_json(
                        {"ok": False, "error": "pr_url must be string"},
                        status=HTTPStatus.BAD_REQUEST,
                    )
                    return
                updates["pr_url"] = pr_url.strip()
            if "notes" in body:
                notes = body.get("notes")
                if not isinstance(notes, str):
                    self._send_json(
                        {"ok": False, "error": "notes must be string"},
                        status=HTTPStatus.BAD_REQUEST,
                    )
                    return
                updates["notes"] = notes.strip()
            if not updates:
                self._send_json(
                    {
                        "ok": False,
                        "error": "at least one field is required: status/pr_url/notes",
                    },
                    status=HTTPStatus.BAD_REQUEST,
                )
                return
            try:
                state = read_json_file(STATE_PATH)
            except ValueError as exc:
                self._send_json(
                    {"ok": False, "error": str(exc)},
                    status=HTTPStatus.INTERNAL_SERVER_ERROR,
                )
                return

            chunks = state.get("chunks")
            if not isinstance(chunks, list):
                self._send_json(
                    {"ok": False, "error": "state.chunks must be a list"},
                    status=HTTPStatus.INTERNAL_SERVER_ERROR,
                )
                return

            target = None
            for chunk in chunks:
                if isinstance(chunk, dict) and chunk.get("id") == chunk_id.strip():
                    target = chunk
                    break
            if target is None:
                self._send_json(
                    {"ok": False, "error": f"chunk not found: {chunk_id.strip()}"},
                    status=HTTPStatus.NOT_FOUND,
                )
                return

            target.update(updates)
            try:
                atomic_write_json(STATE_PATH, state)
            except OSError as exc:
                self._send_json(
                    {"ok": False, "error": f"failed to persist state: {exc}"},
                    status=HTTPStatus.INTERNAL_SERVER_ERROR,
                )
                return
            self._send_json({"ok": True, "chunk": target})
            return

        if parsed.path == "/api/pause":
            paused = body.get("paused")
            if not isinstance(paused, bool):
                self._send_json(
                    {"ok": False, "error": "paused must be boolean"},
                    status=HTTPStatus.BAD_REQUEST,
                )
                return
            reason = body.get("reason", "")
            if not isinstance(reason, str):
                self._send_json(
                    {"ok": False, "error": "reason must be string"},
                    status=HTTPStatus.BAD_REQUEST,
                )
                return
            try:
                if paused:
                    PAUSE_PATH.parent.mkdir(parents=True, exist_ok=True)
                    PAUSE_PATH.write_text(reason.strip(), encoding="utf-8")
                else:
                    if PAUSE_PATH.exists():
                        PAUSE_PATH.unlink()
            except OSError as exc:
                self._send_json(
                    {"ok": False, "error": f"failed to update pause state: {exc}"},
                    status=HTTPStatus.INTERNAL_SERVER_ERROR,
                )
                return
            self._send_json({"ok": True, **read_pause_state()})
            return

        if parsed.path == "/api/review/reset":
            try:
                pr_number = parse_int(body.get("pr_number"), "pr_number")
            except ValueError as exc:
                self._send_json(
                    {"ok": False, "error": str(exc)},
                    status=HTTPStatus.BAD_REQUEST,
                )
                return
            if pr_number <= 0:
                self._send_json(
                    {"ok": False, "error": "pr_number must be > 0"},
                    status=HTTPStatus.BAD_REQUEST,
                )
                return

            try:
                state = read_review_state()
            except ValueError as exc:
                self._send_json(
                    {"ok": False, "error": str(exc)},
                    status=HTTPStatus.INTERNAL_SERVER_ERROR,
                )
                return

            key = str(pr_number)
            prs = state.setdefault("prs", {})
            prs[key] = {
                "triggers_used": 0,
                "max_triggers": 5,
                "last_rounds": 0,
                "last_run_at": "",
                "last_outcome": "",
                "last_returncode": 0,
                "last_trigger_comment_created_at": "",
                "last_trigger_comment_url": "",
            }
            try:
                write_review_state(state)
            except OSError as exc:
                self._send_json(
                    {"ok": False, "error": f"failed to persist review state: {exc}"},
                    status=HTTPStatus.INTERNAL_SERVER_ERROR,
                )
                return

            self._send_json({"ok": True, "pr_number": key, "data": prs[key]})
            return

        if parsed.path == "/api/review/run":
            try:
                pr_number = parse_optional_int(body.get("pr_number"), "pr_number")
                rounds = parse_int(body.get("rounds", 3), "rounds")
                max_triggers = parse_int(body.get("max_triggers", 5), "max_triggers")
                poll_rounds = parse_int(body.get("poll_rounds", 6), "poll_rounds")
                poll_interval_seconds = parse_int(
                    body.get("poll_interval_seconds", 30), "poll_interval_seconds"
                )
                poll_timeout_minutes = parse_int(
                    body.get("poll_timeout_minutes", 10), "poll_timeout_minutes"
                )
                resolve_latest_pr = parse_bool(
                    body.get("resolve_latest_pr"), "resolve_latest_pr", True
                )
            except ValueError as exc:
                self._send_json(
                    {"ok": False, "error": str(exc)},
                    status=HTTPStatus.BAD_REQUEST,
                )
                return

            if pr_number is None and resolve_latest_pr:
                resolved = resolve_latest_open_pr_number()
                if not resolved.get("ok"):
                    self._send_json(
                        {
                            "ok": False,
                            "error": resolved.get(
                                "error", "failed to resolve latest open PR"
                            ),
                        },
                        status=HTTPStatus.BAD_REQUEST,
                    )
                    return
                pr_number = int(resolved.get("pr_number", 0))

            if pr_number is None or pr_number <= 0:
                self._send_json(
                    {"ok": False, "error": "pr_number must be > 0"},
                    status=HTTPStatus.BAD_REQUEST,
                )
                return
            if rounds < 1 or rounds > 5:
                self._send_json(
                    {"ok": False, "error": "rounds must be between 1 and 5"},
                    status=HTTPStatus.BAD_REQUEST,
                )
                return
            if max_triggers < 1 or max_triggers > 20:
                self._send_json(
                    {"ok": False, "error": "max_triggers must be between 1 and 20"},
                    status=HTTPStatus.BAD_REQUEST,
                )
                return
            if poll_rounds < 1 or poll_rounds > 120:
                self._send_json(
                    {"ok": False, "error": "poll_rounds must be between 1 and 120"},
                    status=HTTPStatus.BAD_REQUEST,
                )
                return
            if poll_interval_seconds < 5 or poll_interval_seconds > 600:
                self._send_json(
                    {
                        "ok": False,
                        "error": "poll_interval_seconds must be between 5 and 600",
                    },
                    status=HTTPStatus.BAD_REQUEST,
                )
                return
            if poll_timeout_minutes < 1 or poll_timeout_minutes > 120:
                self._send_json(
                    {
                        "ok": False,
                        "error": "poll_timeout_minutes must be between 1 and 120",
                    },
                    status=HTTPStatus.BAD_REQUEST,
                )
                return

            try:
                state = read_review_state()
            except ValueError as exc:
                self._send_json(
                    {"ok": False, "error": str(exc)},
                    status=HTTPStatus.INTERNAL_SERVER_ERROR,
                )
                return

            key = str(pr_number)
            prs = state.setdefault("prs", {})
            entry = prs.get(
                key,
                {
                    "triggers_used": 0,
                    "max_triggers": max_triggers,
                    "last_rounds": 0,
                    "last_run_at": "",
                    "last_outcome": "",
                    "last_returncode": 0,
                    "last_trigger_comment_created_at": "",
                    "last_trigger_comment_url": "",
                },
            )
            used = int(entry.get("triggers_used", 0))
            current_max = int(entry.get("max_triggers", max_triggers))
            entry["max_triggers"] = max(current_max, max_triggers)
            if used >= int(entry["max_triggers"]):
                self._send_json(
                    {
                        "ok": False,
                        "error": "trigger limit reached for this PR",
                        "pr_number": key,
                        "data": entry,
                    },
                    status=HTTPStatus.BAD_REQUEST,
                )
                return

            marker_created_at = str(entry.get("last_trigger_comment_created_at") or "")
            marker_url = str(entry.get("last_trigger_comment_url") or "")
            poll_result = poll_for_bot_review_comments(
                pr_number,
                rounds=poll_rounds,
                interval_seconds=poll_interval_seconds,
                timeout_minutes=poll_timeout_minutes,
                marker_created_at=marker_created_at,
                marker_url=marker_url,
            )
            if not poll_result.get("ok"):
                self._send_json(
                    {
                        "ok": False,
                        "error": poll_result.get("error", "polling failed"),
                        "poll": poll_result,
                        "pr_number": key,
                    },
                    status=HTTPStatus.BAD_GATEWAY,
                )
                return
            if poll_result.get("status") != "comments_found":
                self._send_json(
                    {
                        "ok": True,
                        "triage_ran": False,
                        "pr_number": key,
                        "data": entry,
                        "poll": poll_result,
                        "message": poll_result.get(
                            "message",
                            "No new bot review comments found yet; re-run when review is posted.",
                        ),
                    }
                )
                return

            prompt = (
                f"Run fluid-pr-agent in post-review mode for PR #{pr_number}. "
                f"Use post_review_rounds={rounds}. Fetch reviewer/bot comments, "
                "classify must-fix/optional/ignore, apply only valid in-scope must-fix items, "
                "re-run verification, and return Review Disposition + Final State. "
                "Do not create/open PR."
            )
            result = run_command(
                ["opencode", "run", "--agent", "fluid-pr-agent", prompt]
            )
            combined = (result.get("stdout") or "") + (result.get("stderr") or "")
            entry["triggers_used"] = used + 1
            entry["last_rounds"] = rounds
            entry["last_run_at"] = dt.datetime.now().isoformat(timespec="seconds")
            entry["last_outcome"] = parse_outcome_token(combined)
            entry["last_returncode"] = int(result.get("returncode", 1))
            latest_comment = poll_result.get("latest_comment")
            if isinstance(latest_comment, dict):
                entry["last_trigger_comment_created_at"] = str(
                    latest_comment.get("created_at") or ""
                )
                entry["last_trigger_comment_url"] = str(latest_comment.get("url") or "")
            prs[key] = entry
            try:
                write_review_state(state)
            except OSError as exc:
                self._send_json(
                    {"ok": False, "error": f"failed to persist review state: {exc}"},
                    status=HTTPStatus.INTERNAL_SERVER_ERROR,
                )
                return

            self._send_json(
                {
                    "ok": result.get("ok", False),
                    "triage_ran": True,
                    "pr_number": key,
                    "data": entry,
                    "poll": poll_result,
                    "stdout": result.get("stdout", ""),
                    "stderr": result.get("stderr", ""),
                    "returncode": result.get("returncode", 1),
                    "command": result.get("command", ""),
                },
                status=HTTPStatus.OK
                if result.get("ok", False)
                else HTTPStatus.BAD_GATEWAY,
            )
            return

        self._send_json(
            {"ok": False, "error": "unknown endpoint"}, status=HTTPStatus.NOT_FOUND
        )


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Serve local Fluid automation queue UI"
    )
    parser.add_argument(
        "--host", default="127.0.0.1", help="Bind host (default: 127.0.0.1)"
    )
    parser.add_argument(
        "--port", default=8787, type=int, help="Bind port (default: 8787)"
    )
    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()

    if not UI_DIR.exists():
        parser.error(f"UI directory not found: {UI_DIR}")
    if not STATE_PATH.exists():
        parser.error(f"State file not found: {STATE_PATH}")

    server = ThreadingHTTPServer((args.host, args.port), UIServerHandler)
    print(f"Fluid UI server listening on http://{args.host}:{args.port}")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down server")
    finally:
        server.server_close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
