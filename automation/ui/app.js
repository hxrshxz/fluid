const STATUS_KEYS = ["pending", "in_progress", "blocked", "done"];

let appState = null;
let selectedChunkId = "";
let pauseState = { paused: false, reason: "" };
let reviewState = { pr_number: "", data: null };

function byId(id) {
  return document.getElementById(id);
}

function appendLog(message, isError = false) {
  const log = byId("outputLog");
  const now = new Date().toISOString();
  const prefix = isError ? "[ERROR]" : "[INFO]";
  log.textContent = `${log.textContent}${now} ${prefix} ${message}\n`;
  log.scrollTop = log.scrollHeight;
}

async function apiGet(path) {
  const response = await fetch(path);
  const payload = await response.json();
  if (!response.ok || !payload.ok) {
    throw new Error(payload.error || `GET ${path} failed`);
  }
  return payload;
}

async function apiPost(path, body) {
  const response = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body || {}),
  });
  const payload = await response.json();
  if (!response.ok || !payload.ok) {
    throw new Error(payload.error || `POST ${path} failed`);
  }
  return payload;
}

function getChunks() {
  if (!appState || !Array.isArray(appState.chunks)) {
    return [];
  }
  return appState.chunks;
}

function computeCounts(chunks) {
  const counts = {
    total: chunks.length,
    pending: 0,
    in_progress: 0,
    blocked: 0,
    done: 0,
  };
  for (const chunk of chunks) {
    if (STATUS_KEYS.includes(chunk.status)) {
      counts[chunk.status] += 1;
    }
  }
  return counts;
}

function renderStatusCards(chunks) {
  const counts = computeCounts(chunks);
  const cards = [
    ["total chunks", counts.total],
    ["pending", counts.pending],
    ["in_progress", counts.in_progress],
    ["blocked", counts.blocked],
    ["done", counts.done],
  ];
  byId("statusCards").innerHTML = cards
    .map(([label, value]) => `<article class="status-card"><span class="label">${label}</span><span class="value">${value}</span></article>`)
    .join("");
}

function renderWeekSummary(chunks) {
  const map = new Map();
  for (const chunk of chunks) {
    const week = chunk.week || "(unknown)";
    if (!map.has(week)) {
      map.set(week, { total: 0, pending: 0, in_progress: 0, blocked: 0, done: 0 });
    }
    const row = map.get(week);
    row.total += 1;
    if (STATUS_KEYS.includes(chunk.status)) {
      row[chunk.status] += 1;
    }
  }

  const tbody = byId("weekSummaryTable").querySelector("tbody");
  const weeks = Array.from(map.keys()).sort((a, b) => a.localeCompare(b, undefined, { numeric: true }));
  tbody.innerHTML = weeks
    .map((week) => {
      const data = map.get(week);
      return `<tr><td>${week}</td><td>${data.total}</td><td>${data.pending}</td><td>${data.in_progress}</td><td>${data.blocked}</td><td>${data.done}</td></tr>`;
    })
    .join("");
}

function refreshFilters(chunks) {
  const weekFilter = byId("weekFilter");
  const statusFilter = byId("statusFilter");

  const weekValue = weekFilter.value;
  const statusValue = statusFilter.value;

  const weeks = [...new Set(chunks.map((c) => c.week || "(unknown)"))].sort((a, b) => a.localeCompare(b, undefined, { numeric: true }));
  weekFilter.innerHTML = `<option value="">all</option>${weeks.map((w) => `<option value="${w}">${w}</option>`).join("")}`;

  statusFilter.innerHTML = `<option value="">all</option>${STATUS_KEYS.map((s) => `<option value="${s}">${s}</option>`).join("")}`;

  if (["", ...weeks].includes(weekValue)) {
    weekFilter.value = weekValue;
  }
  if (["", ...STATUS_KEYS].includes(statusValue)) {
    statusFilter.value = statusValue;
  }
}

function filterChunks(chunks) {
  const weekFilter = byId("weekFilter").value.trim();
  const statusFilter = byId("statusFilter").value.trim();
  const packageFilter = byId("packageFilter").value.trim().toLowerCase();

  return chunks.filter((chunk) => {
    if (weekFilter && (chunk.week || "") !== weekFilter) {
      return false;
    }
    if (statusFilter && (chunk.status || "") !== statusFilter) {
      return false;
    }
    if (packageFilter && !(chunk.package || "").toLowerCase().includes(packageFilter)) {
      return false;
    }
    return true;
  });
}

function selectChunk(chunkId) {
  selectedChunkId = chunkId || "";
  const selectedChunk = getChunks().find((c) => c.id === selectedChunkId);
  byId("selectedChunkLabel").textContent = `Selected chunk: ${selectedChunkId || "none"}`;
  byId("chunkStatus").value = "";
  byId("chunkPrUrl").value = selectedChunk?.pr_url || "";
  byId("chunkNotes").value = selectedChunk?.notes || "";
  renderChunkTable();
}

function renderPauseState() {
  const quickPauseToggle = byId("quickPauseToggle");
  const pauseStateLabel = byId("pauseStateLabel");
  if (!quickPauseToggle || !pauseStateLabel) {
    return;
  }
  quickPauseToggle.textContent = pauseState.paused ? "Resume" : "Pause";
  pauseStateLabel.textContent = `Pause state: ${pauseState.paused ? "paused" : "active"}${
    pauseState.reason ? ` (${pauseState.reason})` : ""
  }`;
}

function readReviewInputs() {
  const rawPrNumber = (byId("reviewPrNumber").value || "").trim();
  const prNumber = rawPrNumber ? Number.parseInt(rawPrNumber, 10) : null;
  const rounds = Number.parseInt(byId("reviewRounds").value || "3", 10);
  const maxTriggers = Number.parseInt(byId("reviewMaxTriggers").value || "5", 10);
  const pollRounds = Number.parseInt(byId("reviewPollRounds").value || "6", 10);
  const pollIntervalSeconds = Number.parseInt(byId("reviewPollIntervalSeconds").value || "30", 10);
  const pollTimeoutMinutes = Number.parseInt(byId("reviewPollTimeoutMinutes").value || "10", 10);
  const resolveLatestPr = byId("reviewResolveLatestPr").checked;
  return {
    prNumber,
    rounds,
    maxTriggers,
    pollRounds,
    pollIntervalSeconds,
    pollTimeoutMinutes,
    resolveLatestPr,
  };
}

function renderReviewStateLabel() {
  const label = byId("reviewStateLabel");
  if (!reviewState.data || !reviewState.pr_number) {
    label.textContent = "Review state: unknown";
    return;
  }
  const d = reviewState.data;
  label.textContent = `Review state: PR #${reviewState.pr_number} | used ${d.triggers_used || 0}/${d.max_triggers || 0} | last_outcome=${d.last_outcome || ""}`;
}

function renderChunkTable() {
  const chunks = filterChunks(getChunks());
  const tbody = byId("chunkTable").querySelector("tbody");
  tbody.innerHTML = chunks
    .map((chunk) => {
      const selected = chunk.id === selectedChunkId ? "selected" : "";
      const prUrl = chunk.pr_url ? `<a href="${chunk.pr_url}" target="_blank" rel="noreferrer">link</a>` : "";
      const status = chunk.status || "pending";
      return `<tr class="${selected}" data-id="${chunk.id}"><td>${chunk.id}</td><td>${chunk.week || ""}</td><td>${chunk.package || ""}</td><td><span class="status-badge status-${status}">${status}</span></td><td>${prUrl}</td><td><button type="button" class="select-button" data-select-chunk="${chunk.id}">Select</button></td></tr>`;
    })
    .join("");

  for (const row of tbody.querySelectorAll("tr")) {
    row.addEventListener("click", () => {
      selectChunk(row.dataset.id || "");
    });
  }

  for (const button of tbody.querySelectorAll("button[data-select-chunk]")) {
    button.addEventListener("click", (event) => {
      event.stopPropagation();
      selectChunk(button.dataset.selectChunk || "");
    });
  }
}

async function loadState() {
  const payload = await apiGet("/api/state");
  appState = payload.state;
  const chunks = getChunks();
  refreshFilters(chunks);
  renderStatusCards(chunks);
  renderWeekSummary(chunks);
  renderChunkTable();
}

async function loadPauseState() {
  const payload = await apiGet("/api/pause");
  pauseState = { paused: payload.paused, reason: payload.reason || "" };
  byId("pauseToggle").checked = pauseState.paused;
  byId("pauseReason").value = pauseState.reason;
  renderPauseState();
}

async function setPauseState(paused, reason) {
  const payload = await apiPost("/api/pause", { paused, reason });
  pauseState = { paused: payload.paused, reason: payload.reason || "" };
  byId("pauseToggle").checked = pauseState.paused;
  byId("pauseReason").value = pauseState.reason;
  renderPauseState();
  return payload;
}

async function loadReviewStateForCurrentPr() {
  const { prNumber, resolveLatestPr } = readReviewInputs();
  if (prNumber !== null && (!Number.isInteger(prNumber) || prNumber <= 0)) {
    appendLog("PR number must be empty or a positive integer.", true);
    return;
  }
  if (prNumber === null && !resolveLatestPr) {
    appendLog("Enter a PR number or enable 'use latest open PR'.", true);
    return;
  }
  const params = new URLSearchParams();
  if (prNumber !== null) {
    params.set("pr_number", String(prNumber));
  }
  if (resolveLatestPr) {
    params.set("resolve_latest_pr", "true");
  }
  const payload = await apiGet(`/api/review/state?${params.toString()}`);
  const resolvedPr = payload.pr_number || (prNumber === null ? "" : String(prNumber));
  if (resolvedPr) {
    byId("reviewPrNumber").value = String(resolvedPr);
  }
  reviewState = { pr_number: String(resolvedPr), data: payload.data || {} };
  renderReviewStateLabel();
  appendLog(`Loaded review state for PR #${resolvedPr}`);
}

function bindHandlers() {
  byId("weekFilter").addEventListener("change", renderChunkTable);
  byId("statusFilter").addEventListener("change", renderChunkTable);
  byId("packageFilter").addEventListener("input", renderChunkTable);

  byId("refreshButton").addEventListener("click", async () => {
    try {
      appendLog("Running queue refresh...");
      const result = await apiPost("/api/refresh", {});
      appendLog(result.stdout || "(no output)");
      await loadState();
    } catch (error) {
      appendLog(error.message, true);
    }
  });

  byId("reportButton").addEventListener("click", async () => {
    try {
      appendLog("Running queue report...");
      const result = await apiPost("/api/report", {});
      appendLog(result.output || "(no output)");
    } catch (error) {
      appendLog(error.message, true);
    }
  });

  byId("dryRunButton").addEventListener("click", async () => {
    try {
      const dateValue = byId("dryRunDate").value;
      appendLog(`Running dry-run${dateValue ? ` for ${dateValue}` : ""}...`);
      const result = await apiPost("/api/dry-run", dateValue ? { date: dateValue } : {});
      appendLog((result.stdout || "") + (result.stderr || ""));
    } catch (error) {
      appendLog(error.message, true);
    }
  });

  byId("quickPauseToggle").addEventListener("click", async () => {
    try {
      const nextPaused = !pauseState.paused;
      const reason = byId("pauseReason").value;
      const payload = await setPauseState(nextPaused, reason);
      appendLog(`Pause toggle applied: paused=${payload.paused} reason=${payload.reason || ""}`);
    } catch (error) {
      appendLog(error.message, true);
    }
  });

  byId("pauseForm").addEventListener("submit", async (event) => {
    event.preventDefault();
    try {
      const paused = byId("pauseToggle").checked;
      const reason = byId("pauseReason").value;
      const payload = await setPauseState(paused, reason);
      appendLog(`Pause state updated: paused=${payload.paused} reason=${payload.reason || ""}`);
    } catch (error) {
      appendLog(error.message, true);
    }
  });

  byId("chunkForm").addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!selectedChunkId) {
      appendLog("Select a chunk first.", true);
      return;
    }
    const body = { chunk_id: selectedChunkId };
    const status = byId("chunkStatus").value;
    if (status) {
      body.status = status;
    }
    body.pr_url = byId("chunkPrUrl").value;
    body.notes = byId("chunkNotes").value;
    try {
      const payload = await apiPost("/api/chunk/update", body);
      appendLog(`Updated chunk ${payload.chunk.id}`);
      await loadState();
    } catch (error) {
      appendLog(error.message, true);
    }
  });

  byId("reviewStateButton").addEventListener("click", async () => {
    try {
      await loadReviewStateForCurrentPr();
    } catch (error) {
      appendLog(error.message, true);
    }
  });

  byId("reviewResetButton").addEventListener("click", async () => {
    try {
      const { prNumber } = readReviewInputs();
      if (!Number.isInteger(prNumber) || prNumber <= 0) {
        appendLog("Enter a valid PR number first.", true);
        return;
      }
      const payload = await apiPost("/api/review/reset", { pr_number: prNumber });
      reviewState = { pr_number: String(prNumber), data: payload.data || {} };
      renderReviewStateLabel();
      appendLog(`Reset review counter for PR #${prNumber}`);
    } catch (error) {
      appendLog(error.message, true);
    }
  });

  byId("reviewForm").addEventListener("submit", async (event) => {
    event.preventDefault();
    try {
      const {
        prNumber,
        rounds,
        maxTriggers,
        pollRounds,
        pollIntervalSeconds,
        pollTimeoutMinutes,
        resolveLatestPr,
      } = readReviewInputs();
      if (prNumber !== null && (!Number.isInteger(prNumber) || prNumber <= 0)) {
        appendLog("PR number must be empty or a positive integer.", true);
        return;
      }
      if (prNumber === null && !resolveLatestPr) {
        appendLog("Enter a PR number or enable 'use latest open PR'.", true);
        return;
      }
      const targetPrText = prNumber === null ? "latest open PR" : `PR #${prNumber}`;
      appendLog(
        `Running post-review triage for ${targetPrText} (rounds=${rounds}, max=${maxTriggers}, poll=on/${pollRounds}r/${pollIntervalSeconds}s/${pollTimeoutMinutes}m)...`,
      );
      const payload = await apiPost("/api/review/run", {
        pr_number: prNumber,
        rounds,
        max_triggers: maxTriggers,
        poll_rounds: pollRounds,
        poll_interval_seconds: pollIntervalSeconds,
        poll_timeout_minutes: pollTimeoutMinutes,
        resolve_latest_pr: resolveLatestPr,
      });
      const resolvedPr = payload.pr_number || (prNumber === null ? "" : String(prNumber));
      if (resolvedPr) {
        byId("reviewPrNumber").value = String(resolvedPr);
      }
      reviewState = { pr_number: String(resolvedPr), data: payload.data || {} };
      renderReviewStateLabel();
      if (payload.poll && payload.poll.status) {
        const poll = payload.poll;
        appendLog(`Poll status=${poll.status} attempts=${poll.attempts || 0} comments=${poll.comments_count || 0}`);
      }
      if (!payload.triage_ran) {
        appendLog(payload.message || "No new bot review comments found yet; triage was skipped.", true);
        return;
      }
      const out = (payload.stdout || "") + (payload.stderr || "");
      appendLog(out || "(no output)");
    } catch (error) {
      appendLog(error.message, true);
    }
  });
}

async function init() {
  bindHandlers();
  try {
    await loadState();
    await loadPauseState();
    renderReviewStateLabel();
    appendLog("UI ready.");
  } catch (error) {
    appendLog(error.message, true);
  }
}

init();
