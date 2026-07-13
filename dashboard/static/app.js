const IS_PUBLIC = window.EDGEOS_PUBLIC === true;

const TOKEN_KEY = "edgeos_token";
const tokenInput = document.getElementById("token");
const statusEl = document.getElementById("status");
const tbody = document.getElementById("nodes-body");
const addNodeForm = document.getElementById("add-node-form");

if (IS_PUBLIC) {
  // Read-only public instance: the server-side proxy already refuses any
  // non-GET request (including chat completions -- no free inference for
  // random visitors), but there's no point showing controls that can't
  // work. (.style.display, not the hidden attribute: these elements set
  // their own `display` in CSS at equal specificity to the UA [hidden]
  // rule, so the later author rule would otherwise win and the attribute
  // would be silently ignored.)
  document.querySelector(".token-box").style.display = "none";
  document.querySelector(".add-node").style.display = "none";
  document.querySelector(".nav-item[data-view='chat']").style.display = "none";
} else {
  tokenInput.value = localStorage.getItem(TOKEN_KEY) || "";
  tokenInput.addEventListener("input", () => {
    localStorage.setItem(TOKEN_KEY, tokenInput.value);
  });
}

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
      <td>${IS_PUBLIC ? "" : `
        <button onclick="doStop('${n.id}')">Stop</button>
        <button onclick="doReload('${n.id}')">Reload</button>
        <button class="danger" onclick="doRemove('${n.id}')">Remove</button>
      `}</td>
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

// Sidebar nav: "Overview"/"Nodes" both show the fleet view v0 has (no
// separate per-view content yet, just active-state feedback); "Chat" is a
// real second view. Locked "Enterprise" items are inert by design -- they
// open the upgrade modal instead of doing anything, since those features
// don't exist in the OSS core (see docs/BUSINESS_MODEL.md).
const modal = document.getElementById("upgrade-modal");
const modalTitle = document.getElementById("upgrade-modal-title");
const viewTitle = document.getElementById("view-title");
const viewTitles = { overview: "Fleet overview", chat: "Chat" };

document.querySelectorAll(".nav-item").forEach(item => {
  item.addEventListener("click", () => {
    if (item.classList.contains("locked")) {
      modalTitle.textContent = item.dataset.feature;
      modal.hidden = false;
      return;
    }
    document.querySelectorAll(".nav-item.active").forEach(a => a.classList.remove("active"));
    item.classList.add("active");

    const view = item.dataset.view;
    document.querySelectorAll(".view").forEach(v => v.hidden = true);
    document.getElementById(`view-${view}`).hidden = false;
    viewTitle.textContent = viewTitles[view] || "";
    if (view === "chat") loadChatModels();
  });
});

// Settings popover: the management token is a credential, not something
// that belongs sitting permanently visible in the main header.
const settingsBtn = document.getElementById("settings-btn");
const settingsPopover = document.getElementById("settings-popover");
if (settingsBtn) {
  settingsBtn.addEventListener("click", (e) => {
    e.stopPropagation();
    settingsPopover.hidden = !settingsPopover.hidden;
  });
  document.addEventListener("click", (e) => {
    if (!settingsPopover.hidden && !settingsPopover.contains(e.target) && e.target !== settingsBtn) {
      settingsPopover.hidden = true;
    }
  });
}

document.getElementById("upgrade-modal-close").addEventListener("click", () => {
  modal.hidden = true;
});
modal.addEventListener("click", (e) => {
  if (e.target === modal) modal.hidden = true;
});

// Chat: real inference through the router, proxied the same way
// /api/v0/nodes is (the dashboard's reverse proxy isn't path-specific).
const chatModelSelect = document.getElementById("chat-model");
const chatLog = document.getElementById("chat-log");
const chatForm = document.getElementById("chat-form");
const chatInput = document.getElementById("chat-input");
const chatHistory = [];
let chatModelsLoaded = false;

async function loadChatModels() {
  if (chatModelsLoaded) return;
  try {
    const res = await fetch("/api/v0/nodes");
    const data = await res.json();
    const models = new Set();
    for (const n of data.nodes || []) {
      for (const m of (n.cap && n.cap.models) || []) {
        if (m.state === "loaded") models.add(m.id);
      }
    }
    chatModelSelect.innerHTML = "";
    if (models.size === 0) {
      chatModelSelect.innerHTML = '<option value="">no loaded models</option>';
      return;
    }
    for (const m of models) {
      const opt = document.createElement("option");
      opt.value = m;
      opt.textContent = m;
      chatModelSelect.appendChild(opt);
    }
    chatModelsLoaded = true;
  } catch {
    chatModelSelect.innerHTML = '<option value="">could not reach router</option>';
  }
}

function addChatMessage(role, text) {
  if (chatLog.querySelector(".chat-empty")) chatLog.innerHTML = "";
  const div = document.createElement("div");
  div.className = "chat-msg " + role;
  div.innerHTML = `<div class="chat-role">${role}</div><div class="chat-content"></div>`;
  div.querySelector(".chat-content").textContent = text;
  chatLog.appendChild(div);
  chatLog.scrollTop = chatLog.scrollHeight;
  return div.querySelector(".chat-content");
}

chatForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const text = chatInput.value.trim();
  const model = chatModelSelect.value;
  if (!text || !model) return;

  addChatMessage("user", text);
  chatHistory.push({ role: "user", content: text });
  chatInput.value = "";

  const assistantEl = addChatMessage("assistant", "");
  let assistantText = "";

  try {
    const res = await fetch("/api/v1/chat/completions", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ model, messages: chatHistory, stream: true, max_tokens: 400 }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      assistantEl.textContent = "Error: " + (err.error ? err.error.message : res.status);
      return;
    }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = "";
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      const lines = buf.split("\n\n");
      buf = lines.pop();
      for (const line of lines) {
        const dataLine = line.split("\n").find(l => l.startsWith("data: "));
        if (!dataLine) continue;
        const payload = dataLine.slice(6);
        if (payload === "[DONE]") continue;
        const chunk = JSON.parse(payload);
        const delta = chunk.choices?.[0]?.delta || {};
        const piece = delta.content || delta.reasoning_content;
        if (piece) {
          assistantText += piece;
          assistantEl.textContent = assistantText;
          chatLog.scrollTop = chatLog.scrollHeight;
        }
      }
    }
    chatHistory.push({ role: "assistant", content: assistantText });
  } catch (err) {
    assistantEl.textContent = "Error: " + err.message;
  }
});

fetchNodes();
setInterval(fetchNodes, 2000);
