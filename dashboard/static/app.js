const TOKEN_KEY = "edgeos_token";
const tokenInput = document.getElementById("token");
const statusEl = document.getElementById("status");
const tbody = document.getElementById("nodes-body");
const addNodeForm = document.getElementById("add-node-form");

tokenInput.value = localStorage.getItem(TOKEN_KEY) || "";
tokenInput.addEventListener("input", () => {
  localStorage.setItem(TOKEN_KEY, tokenInput.value);
});

function showStatus(message, isError) {
  statusEl.textContent = message;
  statusEl.hidden = false;
  statusEl.className = "status " + (isError ? "error" : "ok");
  setTimeout(() => { statusEl.hidden = true; }, 4000);
}

function escapeHTML(s) {
  return String(s).replace(/[&<>"']/g, c => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  }[c]));
}

async function fetchNodes() {
  try {
    const res = await fetch("/api/v0/nodes");
    if (!res.ok) throw new Error("router returned " + res.status);
    const data = await res.json();
    renderKPIs(data.summary || {});
    renderNodes(data.nodes || []);
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="11" class="empty">Could not reach router: ${escapeHTML(err.message)}</td></tr>`;
  }
}

function renderKPIs(s) {
  document.getElementById("kpi-nodes").textContent = s.total_nodes ?? "–";
  document.getElementById("kpi-healthy").textContent = s.healthy_nodes ?? "–";
  document.getElementById("kpi-active").textContent = s.total_active_requests ?? "–";
  document.getElementById("kpi-tokpersec").textContent = (s.total_tok_per_sec ?? 0).toFixed(1);
  document.getElementById("kpi-models").textContent = s.distinct_models ?? "–";
  document.getElementById("kpi-served").textContent =
    `${s.requests_served_local ?? 0} / ${s.requests_served_cloud ?? 0}`;
  document.getElementById("kpi-failed").textContent = s.requests_failed ?? 0;
}

function renderNodes(nodes) {
  if (nodes.length === 0) {
    tbody.innerHTML = `<tr><td colspan="11" class="empty">No nodes discovered yet.</td></tr>`;
    return;
  }

  tbody.innerHTML = nodes.map(n => {
    const cap = n.cap || {};
    const node = cap.node || {};
    const engine = cap.engine || {};
    const load = cap.load || {};
    const model = (cap.models && cap.models[0]) || null;

    const state = model ? model.state : "disabled";
    const tokPerSec = model ? model.tok_per_sec.toFixed(1) : "-";
    const lastSeen = n.last_seen ? new Date(n.last_seen).toLocaleTimeString() : "-";

    return `<tr>
      <td>${escapeHTML(n.id)}</td>
      <td>${escapeHTML(node.hostname || "-")}</td>
      <td>${escapeHTML(node.platform || "-")}</td>
      <td>${escapeHTML(node.accel || "-")}</td>
      <td>${escapeHTML(model ? model.id : "-")}</td>
      <td><span class="pill ${escapeHTML(state)}">${escapeHTML(state)}</span></td>
      <td>${escapeHTML(tokPerSec)}</td>
      <td>${escapeHTML(load.active_requests ?? "-")}</td>
      <td>${engine.healthy ? "yes" : "no"}</td>
      <td>${escapeHTML(lastSeen)}</td>
      <td>
        <button onclick="doStop('${n.id}')">Stop</button>
        <button onclick="doReload('${n.id}')">Reload</button>
        <button class="danger" onclick="doRemove('${n.id}')">Remove</button>
      </td>
    </tr>`;
  }).join("");
}

async function callManagementAPI(method, path, body) {
  const token = tokenInput.value.trim();
  if (!token) {
    showStatus("Set a management token first.", true);
    return;
  }
  try {
    const res = await fetch(path, {
      method,
      headers: {
        "Content-Type": "application/json",
        "Authorization": "Bearer " + token,
      },
      body: body ? JSON.stringify(body) : undefined,
    });
    const text = await res.text();
    if (!res.ok) throw new Error(text || res.status);
    showStatus(text, false);
  } catch (err) {
    showStatus("Failed: " + err.message, true);
  } finally {
    fetchNodes();
  }
}

function doStop(id) {
  if (!confirm(`Stop the engine on node ${id}?`)) return;
  callManagementAPI("POST", `/api/v0/nodes/${id}/actions/stop`);
}

function doReload(id) {
  const modelPath = prompt("Path to the .gguf model to load:");
  if (!modelPath) return;
  callManagementAPI("POST", `/api/v0/nodes/${id}/actions/reload`, { model_path: modelPath });
}

function doRemove(id) {
  if (!confirm(`Remove node ${id}? It won't reappear via mDNS until re-added.`)) return;
  callManagementAPI("DELETE", `/api/v0/nodes/${id}`);
}

addNodeForm.addEventListener("submit", (e) => {
  e.preventDefault();
  const host = document.getElementById("add-host").value.trim();
  const portValue = document.getElementById("add-port").value.trim();
  if (!host) return;
  const body = { host };
  if (portValue) body.port = parseInt(portValue, 10);
  callManagementAPI("POST", "/api/v0/nodes", body);
  addNodeForm.reset();
});

fetchNodes();
setInterval(fetchNodes, 2000);
