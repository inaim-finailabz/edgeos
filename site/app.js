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

  const tryIt = document.getElementById("try-it");

  function select(lang) {
    codeEl.textContent = snippets[lang];
    [...tabs.children].forEach(btn => btn.setAttribute("aria-selected", String(btn.dataset.lang === lang)));
    // Only the browser snippet can actually run client-side -- Python/
    // Node/Go/curl would need a hosted execution sandbox, a much bigger
    // (and more security-sensitive) feature than this site takes on.
    tryIt.hidden = lang !== "browser";
  }

  tabs.addEventListener("click", (e) => {
    const btn = e.target.closest("button[data-lang]");
    if (btn) select(btn.dataset.lang);
  });

  select("python");

  // "Run" makes a real request from the visitor's own browser straight to
  // the router URL they supply -- nothing proxies through this site or
  // its backend, so this works against localhost or any reachable router.
  const tryRouter = document.getElementById("try-router");
  const tryModel = document.getElementById("try-model");
  const tryPrompt = document.getElementById("try-prompt");
  const tryOutput = document.getElementById("try-output");
  const tryRunBtn = document.getElementById("try-run");

  tryRunBtn.addEventListener("click", async () => {
    const router = tryRouter.value.trim().replace(/\/$/, "");
    const model = tryModel.value.trim();
    const prompt = tryPrompt.value.trim();
    if (!router || !model || !prompt) {
      tryOutput.textContent = "Fill in router URL, model, and a prompt first.";
      return;
    }

    tryRunBtn.disabled = true;
    tryOutput.textContent = "";
    try {
      const res = await fetch(`${router}/v1/chat/completions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ model, messages: [{ role: "user", content: prompt }], stream: true, max_tokens: 300 }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        tryOutput.textContent = "Error: " + (err.error ? err.error.message : res.status);
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
          if (piece) tryOutput.textContent += piece;
        }
      }
    } catch (err) {
      tryOutput.textContent = "Error: " + err.message + " (CORS or network issue reaching that router URL?)";
    } finally {
      tryRunBtn.disabled = false;
    }
  });
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
