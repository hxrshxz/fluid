#!/usr/bin/env bash
set -euo pipefail

MARKER="# FLUID_DAILY_AUTORUN"
SCHEDULE="0 9 * * *"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

usage() {
  cat <<'EOF'
Usage: bash scripts/install_daily_cron.sh [--dry-run] [--help]

Install or update a daily cron entry for scripts/daily_autorun.py.

Environment:
  AUTO_OPEN_PR=1   Include --auto-open-pr in cron command.
EOF
}

shell_quote() {
  printf "'%s'" "${1//\'/\'\"\'\"\'}"
}

DRY_RUN=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

mkdir -p "${REPO_ROOT}/automation/logs"

AUTO_OPEN_FLAG=""
if [[ "${AUTO_OPEN_PR:-0}" == "1" ]]; then
  AUTO_OPEN_FLAG=" --auto-open-pr"
fi

REPO_Q="$(shell_quote "${REPO_ROOT}")"
CRON_CMD="cd ${REPO_Q} && python3 scripts/daily_autorun.py${AUTO_OPEN_FLAG} >> automation/logs/cron.log 2>&1"
ENTRY="${SCHEDULE} ${CRON_CMD} ${MARKER}"

CURRENT_CRON=""
if crontab -l >/dev/null 2>&1; then
  CURRENT_CRON="$(crontab -l)"
fi

FILTERED="$(printf '%s\n' "${CURRENT_CRON}" | sed '/FLUID_DAILY_AUTORUN/d')"
if [[ -n "${FILTERED}" ]]; then
  NEW_CRON="${FILTERED}"$'\n'"${ENTRY}"$'\n'
else
  NEW_CRON="${ENTRY}"$'\n'
fi

if [[ ${DRY_RUN} -eq 1 ]]; then
  echo "dry-run: would install cron entry:"
  echo "${ENTRY}"
  exit 0
fi

printf '%s' "${NEW_CRON}" | crontab -
echo "installed/updated cron entry:"
echo "${ENTRY}"
