const TOKEN_KEY = "edgeos_token";
const tokenInput = document.getElementById("token");
const statusEl = document.getElementById("status");
const tbody = document.getElementById("nodes-body");

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
    renderNodes(data.nodes || []);
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="11" class="empty">Could not reach router: ${escapeHTML(err.message)}</td></tr>`;
  }
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
        <button class="danger" onclick="doEvict('${n.id}')">Evict</button>
      </td>
    </tr>`;
  }).join("");
}

async function callManagementAPI(path, body) {
  const token = tokenInput.value.trim();
  if (!token) {
    showStatus("Set a management token first.", true);
    return;
  }
  try {
    const res = await fetch(path, {
      method: "POST",
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
  callManagementAPI(`/api/v0/nodes/${id}/actions/stop`);
}

function doReload(id) {
  const modelPath = prompt("Path to the .gguf model to load:");
  if (!modelPath) return;
  callManagementAPI(`/api/v0/nodes/${id}/actions/reload`, { model_path: modelPath });
}

function doEvict(id) {
  if (!confirm(`Evict node ${id} from the router's table now?`)) return;
  callManagementAPI(`/api/v0/nodes/${id}/evict`);
}

fetchNodes();
setInterval(fetchNodes, 2000);
