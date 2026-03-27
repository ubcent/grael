const pollIntervalMs = 650

const state = {
  runId: "",
  afterSeq: 0,
  knownNodeIds: new Set(),
  previousNodes: new Map(),
  previousEdges: new Set(),
  lastSnapshot: null,
  replayFrames: [],
  replayIndex: -1,
  replayTimer: null,
  timer: null,
}

const elements = {
  form: document.getElementById("run-form"),
  runInput: document.getElementById("run-id"),
  workflowName: document.getElementById("workflow-name"),
  runState: document.getElementById("run-state"),
  lastSeq: document.getElementById("last-seq"),
  runIdValue: document.getElementById("run-id-value"),
  definitionHash: document.getElementById("definition-hash"),
  createdAt: document.getElementById("created-at"),
  finishedAt: document.getElementById("finished-at"),
  heroTitle: document.getElementById("hero-title"),
  heroSubtitle: document.getElementById("hero-subtitle"),
  phaseStrip: document.getElementById("phase-strip"),
  cursorPill: document.getElementById("cursor-pill"),
  nodeCountPill: document.getElementById("node-count-pill"),
  edgeCountPill: document.getElementById("edge-count-pill"),
  graphSurface: document.getElementById("graph-surface"),
  graphViewport: document.getElementById("graph-viewport"),
  graphCanvas: document.getElementById("graph-canvas"),
  graphEmpty: document.getElementById("graph-empty"),
  laneStrip: document.getElementById("lane-strip"),
  graphEdges: document.getElementById("graph-edges"),
  graphNodes: document.getElementById("graph-nodes"),
  topologySummary: document.getElementById("topology-summary"),
  replayToggle: document.getElementById("replay-toggle"),
  replayReset: document.getElementById("replay-reset"),
  replaySlider: document.getElementById("replay-slider"),
  replayCaption: document.getElementById("replay-caption"),
  timeline: document.getElementById("timeline"),
}

elements.form.addEventListener("submit", async (event) => {
  event.preventDefault()
  const runId = elements.runInput.value.trim()
  if (!runId) return
  connect(runId)
})

elements.replayToggle.addEventListener("click", () => {
  if (!state.replayFrames.length) return
  if (state.replayTimer) {
    stopReplay()
    return
  }
  if (state.replayIndex < 0 || state.replayIndex >= state.replayFrames.length - 1) {
    state.replayIndex = 0
  }
  startReplay()
})

elements.replayReset.addEventListener("click", () => {
  stopReplay()
  state.replayIndex = -1
  updateReplayControls()
  if (state.lastSnapshot) {
    renderLiveSnapshot(state.lastSnapshot)
  }
})

elements.replaySlider.addEventListener("input", (event) => {
  if (!state.replayFrames.length) return
  stopReplay()
  state.replayIndex = Number(event.target.value)
  renderReplayFrame()
})

boot()

function boot() {
  const params = new URLSearchParams(window.location.search)
  const runId = params.get("run_id")
  if (!runId) return
  elements.runInput.value = runId
  connect(runId)
}

function connect(runId) {
  state.runId = runId
  state.afterSeq = 0
  state.knownNodeIds = new Set()
  state.previousNodes = new Map()
  state.previousEdges = new Set()
  state.replayFrames = []
  state.replayIndex = -1
  state.lastSnapshot = null
  stopReplay()
  if (state.timer) window.clearInterval(state.timer)
  updateUrl(runId)
  fetchSnapshot()
  state.timer = window.setInterval(fetchSnapshot, pollIntervalMs)
}

async function fetchSnapshot() {
  if (!state.runId) return
  try {
    const response = await fetch(`/api/runs/${encodeURIComponent(state.runId)}/snapshot?after_seq=${state.afterSeq}`)
    const payload = await response.json()
    if (!response.ok) {
      renderError(payload.error || "Unable to load run snapshot.")
      return
    }
    renderSnapshot(payload)
    state.afterSeq = payload.cursor.current_seq
  } catch (error) {
    renderError(error.message || "Unable to load run snapshot.")
  }
}

function renderSnapshot(snapshot) {
  state.lastSnapshot = snapshot
  state.replayFrames = snapshot.replay?.frames || []
  if (state.replayIndex >= state.replayFrames.length) {
    state.replayIndex = state.replayFrames.length - 1
  }
  if (inReplayMode()) {
    updateReplayControls()
    if (state.replayIndex >= 0) {
      renderReplayFrame()
    }
    return
  }
  renderLiveSnapshot(snapshot)
}

function renderLiveSnapshot(snapshot) {
  elements.heroTitle.textContent = `Watching ${displayWorkflowName(snapshot.run.workflow) || "run"} evolve from persisted truth.`
  elements.heroSubtitle.textContent = "The graph, timeline, and status panels are derived from GetRun and ListEvents only."
  elements.workflowName.textContent = displayWorkflowName(snapshot.run.workflow)
  elements.runState.textContent = snapshot.run.state || "-"
  elements.lastSeq.textContent = String(snapshot.run.last_seq || 0)
  elements.runIdValue.textContent = snapshot.run.run_id || "-"
  elements.definitionHash.textContent = snapshot.run.definition_hash || "Not captured"
  elements.createdAt.textContent = formatDate(snapshot.run.created_at)
  elements.finishedAt.textContent = snapshot.run.finished_at ? formatDate(snapshot.run.finished_at) : "Still running"
  elements.cursorPill.textContent = snapshot.cursor.has_changes
    ? `Live update +${snapshot.delta.length}`
    : `Watching seq ${snapshot.cursor.current_seq}`
  elements.nodeCountPill.textContent = `${snapshot.graph.nodes.length} nodes`
  elements.edgeCountPill.textContent = `${snapshot.graph.edges.length} edges`
  updateReplayControls()

  renderPhases(snapshot.phases || [])
  renderGraph(snapshot.graph)
  renderTimeline(snapshot.timeline.events)
}

function renderReplayFrame() {
  const frame = state.replayFrames[state.replayIndex]
  if (!frame || !state.lastSnapshot) return
  elements.heroTitle.textContent = `Replay: ${displayEventLabel(frame)}`
  elements.heroSubtitle.textContent = "Event-sourced playback from committed history."
  elements.workflowName.textContent = displayWorkflowName(state.lastSnapshot.run.workflow)
  elements.runState.textContent = frame.run_state || "-"
  elements.lastSeq.textContent = String(frame.seq || 0)
  elements.runIdValue.textContent = state.lastSnapshot.run.run_id || "-"
  elements.definitionHash.textContent = state.lastSnapshot.run.definition_hash || "Not captured"
  elements.createdAt.textContent = formatDate(state.lastSnapshot.run.created_at)
  elements.finishedAt.textContent = state.lastSnapshot.run.finished_at ? formatDate(state.lastSnapshot.run.finished_at) : "Still running"
  elements.cursorPill.textContent = `Replay seq ${frame.seq}`
  elements.nodeCountPill.textContent = `${frame.graph.nodes.length} nodes`
  elements.edgeCountPill.textContent = `${frame.graph.edges.length} edges`
  updateReplayControls()
  renderPhases(frame.phases || [])
  renderGraph(frame.graph)
  renderTimeline(frame.timeline || [])
}

function startReplay() {
  stopReplay()
  updateReplayControls()
  renderReplayFrame()
  state.replayTimer = window.setInterval(() => {
    if (state.replayIndex >= state.replayFrames.length - 1) {
      stopReplay()
      return
    }
    state.replayIndex += 1
    renderReplayFrame()
  }, 380)
}

function stopReplay() {
  if (state.replayTimer) {
    window.clearInterval(state.replayTimer)
    state.replayTimer = null
  }
  updateReplayControls()
}

function inReplayMode() {
  return state.replayTimer !== null || state.replayIndex >= 0
}

function updateReplayControls() {
  const max = Math.max(0, state.replayFrames.length - 1)
  elements.replaySlider.max = String(max)
  elements.replaySlider.value = String(Math.max(0, state.replayIndex))
  elements.replaySlider.disabled = state.replayFrames.length === 0
  elements.replayToggle.textContent = state.replayTimer ? "Pause replay" : "Play replay"
  elements.replayReset.disabled = !state.lastSnapshot
  if (state.replayFrames.length === 0) {
    elements.replayCaption.textContent = "Live mode"
    return
  }
  if (state.replayTimer || state.replayIndex >= 0) {
    const frame = state.replayFrames[Math.max(0, state.replayIndex)]
    elements.replayCaption.textContent = `Frame ${Math.max(0, state.replayIndex) + 1}/${state.replayFrames.length}: ${frame?.label || "Replay"}`
    return
  }
  elements.replayCaption.textContent = "Live mode"
}

function renderError(message) {
  elements.heroTitle.textContent = "Unable to load this run yet."
  elements.heroSubtitle.textContent = message
  elements.cursorPill.textContent = "Disconnected"
}

function renderGraph(graph) {
  if (!graph.nodes.length) {
    elements.graphEmpty.style.display = "grid"
    elements.laneStrip.innerHTML = ""
    elements.graphEdges.innerHTML = ""
    elements.graphNodes.innerHTML = ""
    elements.topologySummary.innerHTML = ""
    return
  }
  elements.graphEmpty.style.display = "none"

  const layoutResult = layoutGraph(graph)
  const layout = layoutResult.positions
  sizeGraphCanvas(layoutResult)
  renderLanes(layoutResult.columns)
  renderEdges(graph.edges, layout)
  renderNodes(graph.nodes, layout)
  renderTopologySummary(graph)
}

function renderPhases(phases) {
  if (!phases.length) {
    elements.phaseStrip.innerHTML = ""
    return
  }
  elements.phaseStrip.innerHTML = phases.map((phase) => `
    <div class="phase-chip ${phase.complete ? "phase-complete" : "phase-active"}">
      <span class="phase-dot"></span>
      <div>
        <strong>${escapeHtml(phase.label)}</strong>
        <span>${phase.node_id ? escapeHtml(displayNodeName(phase.node_id)) : "run-wide"}</span>
      </div>
    </div>
  `).join("")
}

function layoutGraph(graph) {
  const nodes = graph.nodes || []
  const byId = new Map(nodes.map((node) => [node.id, node]))
  const spawnedBy = new Map()
  for (const edge of graph.edges || []) {
    if (edge.kind === "spawn" && !spawnedBy.has(edge.to)) {
      spawnedBy.set(edge.to, edge.from)
    }
  }
  const memo = new Map()

  const depthOf = (node) => {
    if (memo.has(node.id)) return memo.get(node.id)
    const dependencyDepth = !node.depends_on || node.depends_on.length === 0
      ? 0
      : Math.max(...node.depends_on.map((dep) => {
        const parent = byId.get(dep)
        return parent ? depthOf(parent) + 1 : 0
      }))
    const spawnParentID = spawnedBy.get(node.id)
    const spawnDepth = spawnParentID && byId.has(spawnParentID)
      ? depthOf(byId.get(spawnParentID)) + 1
      : 0
    const depth = Math.max(dependencyDepth, spawnDepth)
    memo.set(node.id, depth)
    return depth
  }

  const columns = new Map()
  nodes.forEach((node) => {
    const depth = depthOf(node)
    if (!columns.has(depth)) columns.set(depth, [])
    columns.get(depth).push(node)
  })

  const orderedColumns = [...columns.entries()].sort((a, b) => a[0] - b[0])
  const width = Math.max(elements.graphViewport.clientWidth, 820)
  const topPad = 126
  const bottomPad = 86
  const rowGap = 214
  const maxRows = Math.max(...orderedColumns.map(([, columnNodes]) => columnNodes.length), 1)
  const contentHeight = topPad + bottomPad + Math.max(0, maxRows-1) * rowGap
  const height = Math.max(elements.graphSurface.clientHeight, contentHeight, 520)
  const leftPad = 135
  const rightPad = 140
  const desiredColumnGap = 280
  const contentWidth = orderedColumns.length > 1
    ? leftPad + rightPad + desiredColumnGap * (orderedColumns.length - 1)
    : 460
  const canvasWidth = Math.max(width, contentWidth)
  const columnGap = orderedColumns.length > 1 ? (canvasWidth - leftPad - rightPad) / (orderedColumns.length - 1) : 0
  const positions = {}

  orderedColumns.forEach(([depth, columnNodes]) => {
    columnNodes.sort((a, b) => {
      const rankDiff = stateRank(b.state) - stateRank(a.state)
      if (rankDiff !== 0) return rankDiff
      return a.id.localeCompare(b.id)
    })
    const occupiedHeight = Math.max(0, columnNodes.length - 1) * rowGap
    const startY = (height - occupiedHeight) / 2
    columnNodes.forEach((node, index) => {
      positions[node.id] = {
        x: leftPad + columnGap * depth,
        y: startY + rowGap * index,
      }
    })
  })

  return {
    positions,
    canvasWidth,
    canvasHeight: height,
    columns: orderedColumns.map(([depth, columnNodes]) => ({
      depth,
      label: laneLabel(depth, orderedColumns.length),
      count: columnNodes.length,
      x: leftPad + columnGap * depth,
    })),
  }
}

function sizeGraphCanvas(layoutResult) {
  elements.graphCanvas.style.width = `${layoutResult.canvasWidth}px`
  elements.graphCanvas.style.height = `${layoutResult.canvasHeight}px`
}

function renderLanes(columns) {
  elements.laneStrip.innerHTML = columns.map((column) => `
    <div class="lane-pill" style="left:${column.x}px">
      <strong>${escapeHtml(column.label)}</strong>
      <span>${column.count} ${column.count === 1 ? "node" : "nodes"}</span>
    </div>
  `).join("")
}

function renderEdges(edges, layout) {
  const width = elements.graphCanvas.clientWidth
  const height = elements.graphCanvas.clientHeight
  elements.graphEdges.setAttribute("viewBox", `0 0 ${width} ${height}`)
  elements.graphEdges.setAttribute("width", String(width))
  elements.graphEdges.setAttribute("height", String(height))

  elements.graphEdges.innerHTML = edges.map((edge) => {
    const from = layout[edge.from]
    const to = layout[edge.to]
    if (!from || !to) return ""
    const edgeKey = `${edge.from}->${edge.to}:${edge.kind || "dependency"}`
    const isFresh = !state.previousEdges.has(edgeKey)
    const startX = from.x + 125
    const startY = from.y
    const endX = to.x - 125
    const endY = to.y
    const midX = (startX + endX) / 2
    const edgeTone = edge.kind === "spawn" ? " graph-edge-spawn" : " graph-edge-dependency"
    return `<path class="graph-edge${edgeTone}${isFresh ? " graph-edge-fresh" : ""}" d="M ${startX} ${startY} C ${midX} ${startY}, ${midX} ${endY}, ${endX} ${endY}" />`
  }).join("")
  state.previousEdges = new Set(edges.map((edge) => `${edge.from}->${edge.to}:${edge.kind || "dependency"}`))
}

function renderNodes(nodes, layout) {
  elements.graphNodes.innerHTML = nodes.map((node) => {
    const position = layout[node.id]
    const fresh = state.knownNodeIds.has(node.id) ? "" : " fresh"
    const previous = state.previousNodes.get(node.id)
    const changedState = previous && previous.state !== node.state
    const changedAttempt = previous && previous.attempt !== node.attempt
    const changedError = previous && previous.last_error !== node.last_error
    const changedSignals = previous && JSON.stringify(previous.signals || []) !== JSON.stringify(node.signals || [])
    const changed = changedState || changedAttempt || changedError || changedSignals
    state.knownNodeIds.add(node.id)
    const signalMarkup = (node.signals || []).map((signal) => `
      <span class="signal-pill signal-${signalClass(signal)}">${escapeHtml(signalLabel(signal))}</span>
    `).join("")
    return `
      <article class="graph-node graph-node-${toneClass(node)}${fresh}${changed ? " changed" : ""}" style="left:${position.x}px;top:${position.y}px">
        <div class="node-header">
          <div>
            <strong>${escapeHtml(displayNodeName(node.id))}</strong>
            <div class="node-activity">${escapeHtml(displayActivityName(node.activity_type))}</div>
          </div>
          <span class="badge ${badgeClass(node.state)}">${escapeHtml(node.state)}</span>
        </div>
        ${signalMarkup ? `<div class="signal-row">${signalMarkup}</div>` : ""}
        <div class="node-meta">
          <span>Attempt ${node.attempt || 0}</span>
          <span>Seq ${node.last_transition_seq || 0}</span>
        </div>
        ${node.worker_id ? `<div class="node-meta"><span>Worker</span><span>${escapeHtml(node.worker_id)}</span></div>` : ""}
        ${node.last_error ? `<div class="node-error">${escapeHtml(node.last_error)}</div>` : ""}
      </article>
    `
  }).join("")
  state.previousNodes = new Map(nodes.map((node) => [node.id, {
    state: node.state,
    attempt: node.attempt,
    last_error: node.last_error,
    signals: [...(node.signals || [])],
  }]))
}

function renderTimeline(events) {
  if (!events.length) {
    elements.timeline.innerHTML = `
      <div class="empty-state compact">
        <h4>No events yet</h4>
        <p>Once a run is connected, the causal event stream will appear here.</p>
      </div>
    `
    return
  }
  const items = [...events].reverse().slice(0, 80)
  elements.timeline.innerHTML = items.map((event) => `
    <article class="timeline-item${state.afterSeq > 0 && event.seq > state.afterSeq ? " timeline-item-fresh" : ""}">
        <div class="timeline-copy">
          <div class="node-header">
          <strong>${escapeHtml(displayEventLabel(event))}</strong>
          <span class="badge family-${event.family}">${escapeHtml(event.family)}</span>
        </div>
        <p>${escapeHtml(formatDate(event.timestamp))}</p>
      </div>
      <div class="timeline-seq">#${event.seq}</div>
    </article>
  `).join("")
}

function renderTopologySummary(graph) {
  const dependencyRows = graph.nodes
    .filter((node) => (node.depends_on || []).length > 0)
    .map((node) => `
      <div class="topology-row">
        <span class="topology-target">${escapeHtml(displayNodeName(node.id))}</span>
        <span class="topology-arrow">depends on</span>
        <span class="topology-deps">${escapeHtml(node.depends_on.map((dep) => displayNodeName(dep)).join(", "))}</span>
      </div>
    `)
  const spawnedRows = graph.edges
    .filter((edge) => edge.kind === "spawn")
    .map((edge) => `
      <div class="topology-row">
        <span class="topology-target">${escapeHtml(displayNodeName(edge.to))}</span>
        <span class="topology-arrow">spawned by</span>
        <span class="topology-deps">${escapeHtml(displayNodeName(edge.from))}</span>
      </div>
    `)
  if (!dependencyRows.length && !spawnedRows.length) {
    elements.topologySummary.innerHTML = `
      <div class="topology-card">
        <p class="eyebrow">Topology</p>
        <h4>Waiting for dependencies to appear</h4>
        <p>This run currently has no visible dependency edges.</p>
      </div>
    `
    return
  }
  elements.topologySummary.innerHTML = `
    <div class="topology-card">
      <p class="eyebrow">Topology</p>
      <h4>Dependency map</h4>
      <div class="topology-grid">
        ${spawnedRows.join("")}
        ${dependencyRows.join("")}
      </div>
    </div>
  `
}

function badgeClass(stateName) {
  switch (stateName) {
    case "READY": return "badge-ready"
    case "RUNNING": return "badge-running"
    case "AWAITING_APPROVAL": return "badge-awaiting"
    case "COMPLETED": return "badge-completed"
    case "FAILED": return "badge-failed"
    case "CANCELLED": return "badge-cancelled"
    default: return "badge-pending"
  }
}

function toneClass(node) {
  if ((node.signals || []).includes("checkpoint")) return "checkpoint"
  if ((node.signals || []).includes("retried") || (node.signals || []).includes("retrying")) return "retry"
  if (node.state === "FAILED") return "failed"
  if (node.state === "COMPLETED") return "completed"
  return "default"
}

function signalLabel(signal) {
  switch (signal) {
    case "spawned": return "Spawned"
    case "retrying": return "Retrying"
    case "retried": return "Recovered"
    case "checkpoint": return "Approval"
    case "approved": return "Approved"
    case "timed_out": return "Timed out"
    default: return signal
  }
}

function signalClass(signal) {
  switch (signal) {
    case "retrying":
    case "retried":
      return "retry"
    case "checkpoint":
    case "approved":
      return "checkpoint"
    case "spawned":
      return "spawned"
    case "timed_out":
      return "failed"
    default:
      return "neutral"
  }
}

function stateRank(stateName) {
  switch (stateName) {
    case "RUNNING": return 6
    case "AWAITING_APPROVAL": return 5
    case "READY": return 4
    case "FAILED": return 3
    case "COMPLETED": return 2
    case "PENDING": return 1
    default: return 0
  }
}

function laneLabel(depth, totalColumns) {
  if (depth === 0) return "Discovery"
  if (depth === totalColumns - 1) return "Finish"
  return "Parallel work"
}

function formatDate(value) {
  if (!value) return "-"
  return new Date(value).toLocaleString()
}

function displayWorkflowName(name) {
  if (!name) return "-"
  if (name === "core-demo") return "Morning Incident Briefing"
  return titleCase(name)
}

function displayNodeName(id) {
  if (!id) return "-"
  const labels = {
    "collect-customer-escalations": "Collect customer escalations",
    "pull-checkout-metrics": "Pull checkout metrics",
    "prepare-brief-outline": "Prepare brief outline",
    "decide-follow-up-checks": "Decide follow-up checks",
    "verify-checkout-latency": "Verify checkout latency",
    "confirm-payment-auth-drop": "Confirm payment auth drop",
    "review-support-spike": "Review support spike",
    "assemble-incident-brief": "Assemble incident brief",
    "editor-approval": "Editor approval",
    "publish-morning-brief": "Publish morning brief",
  }
  return labels[id] || titleCase(id)
}

function displayActivityName(activityType) {
  const labels = {
    collect_signals: "Signal collection",
    collect_metrics: "Metrics pull",
    prepare_brief: "Brief prep",
    plan_follow_up: "Planning step",
    investigate: "Investigation step",
    draft_brief: "Brief assembly",
    review: "Human gate",
    publish: "Publish step",
  }
  return labels[activityType] || titleCase(activityType || "")
}

function displayEventLabel(event) {
  if (!event) return "Replay"
  if (!event.node_id) return humanizeEventType(event.type || event.label || "")
  return `${displayNodeName(event.node_id)} ${humanizeEventType(event.type || "")}${event.attempt ? ` (attempt ${event.attempt})` : ""}`
}

function humanizeEventType(value) {
  const labels = {
    WorkflowStarted: "started",
    WorkflowCompleted: "completed",
    WorkflowFailed: "failed",
    LeaseGranted: "leased",
    HeartbeatRecorded: "heartbeat",
    LeaseExpired: "lease expired",
    TimerScheduled: "timer scheduled",
    TimerFired: "timer fired",
    NodeReady: "became ready",
    NodeStarted: "started",
    NodeCompleted: "completed",
    NodeFailed: "failed",
    CheckpointReached: "requested approval",
    CheckpointApproved: "approved",
    CancellationRequested: "cancel requested",
    NodeCancelled: "cancelled",
    CancellationCompleted: "cancellation completed",
    CompensationStarted: "compensation started",
    CompensationTaskStarted: "compensation started",
    CompensationTaskCompleted: "compensation completed",
    CompensationTaskExpired: "compensation expired",
    CompensationTaskFailed: "compensation failed",
    CompensationCompleted: "compensation finished",
  }
  return labels[value] || titleCase(String(value || "event"))
}

function titleCase(value) {
  return String(value || "")
    .replaceAll("-", " ")
    .replaceAll("_", " ")
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .split(/\s+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ")
}

function updateUrl(runId) {
  const url = new URL(window.location.href)
  url.searchParams.set("run_id", runId)
  window.history.replaceState({}, "", url)
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;")
}
