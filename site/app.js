// Language picker: same snippets as examples/, condensed for display.
(function initLangTabs() {
  const tabs = document.getElementById("lang-tabs");
  if (!tabs) return; // section only exists on the main page
  const codeEl = document.getElementById("lang-code");

  const snippets = {
    python: `from openai import OpenAI

client = OpenAI(base_url="http://localhost:8081/v1", api_key="unused-in-v0")

stream = client.chat.completions.create(
    model="<id from GET /v0/nodes>",
    messages=[{"role": "user", "content": "hi"}],
    stream=True,
)
for chunk in stream:
    print(chunk.choices[0].delta.content or "", end="")`,

    node: `import OpenAI from "openai";

const client = new OpenAI({ baseURL: "http://localhost:8081/v1", apiKey: "unused-in-v0" });

const stream = await client.chat.completions.create({
  model: "<id from GET /v0/nodes>",
  messages: [{ role: "user", content: "hi" }],
  stream: true,
});
for await (const chunk of stream) {
  process.stdout.write(chunk.choices[0].delta.content || "");
}`,

    go: `resp, _ := http.Post(
    "http://localhost:8081/v1/chat/completions",
    "application/json",
    strings.NewReader(\`{"model":"<id>","messages":[{"role":"user","content":"hi"}],"stream":true}\`),
)
defer resp.Body.Close()
// read resp.Body as SSE: lines prefixed "data: ", terminated by "data: [DONE]"`,

    browser: `const res = await fetch("http://localhost:8081/v1/chat/completions", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    model: "<id from GET /v0/nodes>",
    messages: [{ role: "user", content: "hi" }],
    stream: true,
  }),
});
const reader = res.body.getReader();
// decode chunks, split on "\\n\\n", parse each "data: {...}" line`,

    curl: `curl -N http://localhost:8081/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -d '{"model":"<id from GET /v0/nodes>","messages":[{"role":"user","content":"hi"}],"stream":true}'`,
  };

  function select(lang) {
    codeEl.textContent = snippets[lang];
    [...tabs.children].forEach(btn => btn.setAttribute("aria-selected", String(btn.dataset.lang === lang)));
  }

  tabs.addEventListener("click", (e) => {
    const btn = e.target.closest("button[data-lang]");
    if (btn) select(btn.dataset.lang);
  });

  select("python");
})();

// Carousel: plain vanilla, auto-advance + dot navigation.
(function initCarousel() {
  const track = document.getElementById("carousel-track");
  const dotsEl = document.getElementById("carousel-dots");
  const slides = track.children;
  let index = 0;
  let timer;

  for (let i = 0; i < slides.length; i++) {
    const dot = document.createElement("button");
    dot.setAttribute("aria-label", `Slide ${i + 1}`);
    dot.addEventListener("click", () => goTo(i));
    dotsEl.appendChild(dot);
  }

  function render() {
    track.style.transform = `translateX(-${index * 100}%)`;
    [...dotsEl.children].forEach((d, i) => d.classList.toggle("active", i === index));
  }

  function goTo(i) {
    index = (i + slides.length) % slides.length;
    render();
    resetTimer();
  }

  function resetTimer() {
    clearInterval(timer);
    timer = setInterval(() => goTo(index + 1), 5000);
  }

  render();
  resetTimer();
})();

// Live demo: read-only, gated behind GitHub sign-in. The tunnel URL to the
// real backend is never sent to the browser -- /api/demo/data (a Netlify
// Function) checks the session server-side and proxies it only if valid.
(async function initDemo() {
  const signedOut = document.getElementById("demo-signed-out");
  const signedIn = document.getElementById("demo-signed-in");
  const loginEl = document.getElementById("demo-login");
  const tbody = document.getElementById("demo-nodes-body");

  function escapeHTML(s) {
    return String(s).replace(/[&<>"']/g, c => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[c]));
  }

  let me;
  try {
    me = await (await fetch("/api/auth/me")).json();
  } catch {
    return; // leave the signed-out CTA showing
  }
  if (!me.authenticated) return;

  signedOut.hidden = true;
  signedIn.hidden = false;
  loginEl.textContent = me.login;

  async function refresh() {
    try {
      const res = await fetch("/api/demo/data");
      if (!res.ok) throw new Error("status " + res.status);
      const data = await res.json();
      renderDemo(data.summary || {}, data.nodes || []);
    } catch (err) {
      tbody.innerHTML = `<tr><td colspan="6" class="empty">Demo backend unreachable: ${escapeHTML(err.message)}</td></tr>`;
    }
  }

  function renderDemo(s, nodes) {
    document.getElementById("d-kpi-nodes").textContent = s.total_nodes ?? "–";
    document.getElementById("d-kpi-healthy").textContent = s.healthy_nodes ?? "–";
    document.getElementById("d-kpi-active").textContent = s.total_active_requests ?? "–";
    document.getElementById("d-kpi-tokpersec").textContent = (s.total_tok_per_sec ?? 0).toFixed(1);

    if (nodes.length === 0) {
      tbody.innerHTML = `<tr><td colspan="6" class="empty">No nodes online right now.</td></tr>`;
      return;
    }
    tbody.innerHTML = nodes.map(n => {
      const cap = n.cap || {};
      const node = cap.node || {};
      const engine = cap.engine || {};
      const model = (cap.models && cap.models[0]) || null;
      return `<tr>
        <td>${escapeHTML(n.id)}</td>
        <td>${escapeHTML(node.hostname || "-")}</td>
        <td>${escapeHTML(model ? model.id : "-")}</td>
        <td>${model ? `<span class="pill">${escapeHTML(model.state)}</span>` : "-"}</td>
        <td>${model ? model.tok_per_sec.toFixed(1) : "-"}</td>
        <td>${engine.healthy ? "yes" : "no"}</td>
      </tr>`;
    }).join("");
  }

  refresh();
  setInterval(refresh, 3000);
})();
