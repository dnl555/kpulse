// kpulse UI: vanilla JS, polls /api/v1/* every 5s, no external deps.

const POLL_INTERVAL_MS = 5000;
const ROUTES = ["dashboard", "alerts", "monitors", "channels", "about"];
const state = { cluster: null, active: null, recent: null, monitors: null, channels: null };

function $(sel, root = document) { return root.querySelector(sel); }
function $$(sel, root = document) { return Array.from(root.querySelectorAll(sel)); }

function esc(s) {
  if (s == null) return "";
  return String(s).replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
}

function timeAgo(iso) {
  if (!iso || iso === "0001-01-01T00:00:00Z") return "—";
  const d = new Date(iso);
  const s = Math.floor((Date.now() - d.getTime()) / 1000);
  if (s < 0) return "just now";
  if (s < 60) return `${s}s ago`;
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  return `${Math.floor(s / 86400)}d ago`;
}

async function fetchJSON(path) {
  const res = await fetch(path, { headers: { Accept: "application/json" } });
  if (!res.ok) throw new Error(`${path}: HTTP ${res.status}`);
  return res.json();
}

async function refresh() {
  try {
    const [cluster, active, recent, monitors, channels] = await Promise.all([
      fetchJSON("/api/v1/cluster"),
      fetchJSON("/api/v1/alerts/active"),
      fetchJSON("/api/v1/alerts/recent"),
      fetchJSON("/api/v1/monitors"),
      fetchJSON("/api/v1/channels"),
    ]);
    Object.assign(state, { cluster, active, recent, monitors, channels });
    renderCurrent();
  } catch (e) {
    console.error("refresh failed", e);
    toast(`API unreachable: ${e.message}`, true);
  }
}

function renderCurrent() {
  const route = currentRoute();
  if (state.cluster) {
    $("#sidebar-cluster").textContent = state.cluster.name || "—";
    $("#sidebar-version").textContent = state.cluster.version || "—";
  }
  $$(".nav-link").forEach(a => a.classList.toggle("active", a.dataset.route === route));
  $$(".page").forEach(p => p.classList.add("hidden"));
  const page = $(`#page-${route}`);
  if (page) page.classList.remove("hidden");

  const renderers = {
    dashboard: renderDashboard,
    alerts: renderAlerts,
    monitors: renderMonitors,
    channels: renderChannels,
    about: renderAbout,
  };
  if (renderers[route]) renderers[route](page);
}

function currentRoute() {
  const h = (location.hash || "#/dashboard").replace(/^#\//, "").split("/")[0];
  if (h === "" || h === "dashboard") return "dashboard";
  return ROUTES.includes(h) ? h : "dashboard";
}

function renderDashboard(page) {
  if (!state.active || !state.recent || !state.monitors) {
    page.innerHTML = `<p class="empty">Loading…</p>`;
    return;
  }
  const counts = countBySeverity(state.active.alerts);
  const totalFires = (state.monitors.monitors || []).reduce((s, m) => s + (m.fires || 0), 0);
  const totalResolves = (state.monitors.monitors || []).reduce((s, m) => s + (m.resolves || 0), 0);

  page.innerHTML = `
    <div class="page-header">
      <div>
        <h1>Dashboard</h1>
        <p class="subtitle">Live snapshot of ${esc(state.cluster.name)}.</p>
      </div>
    </div>

    <div class="stat-grid">
      <div class="stat-card severity-critical"><div class="stat-label">Active critical</div><div class="stat-value">${counts.critical}</div></div>
      <div class="stat-card severity-warning"><div class="stat-label">Active warning</div><div class="stat-value">${counts.warning}</div></div>
      <div class="stat-card severity-info"><div class="stat-label">Active info</div><div class="stat-value">${counts.info}</div></div>
      <div class="stat-card severity-ok"><div class="stat-label">Total fires</div><div class="stat-value muted">${totalFires}</div></div>
      <div class="stat-card severity-ok"><div class="stat-label">Total resolves</div><div class="stat-value muted">${totalResolves}</div></div>
    </div>

    <div class="panel">
      <div class="panel-header"><h2>Active alerts</h2><a href="#/alerts" class="btn">Open</a></div>
      <div class="panel-body">${alertListHTML(state.active.alerts.slice(0, 5)) || emptyHTML("Nothing firing right now.")}</div>
    </div>

    <div class="panel">
      <div class="panel-header"><h2>Recent activity</h2><a href="#/alerts" class="btn">Open</a></div>
      <div class="panel-body">${recentListHTML((state.recent.alerts || []).slice(0, 8)) || emptyHTML("No activity yet.")}</div>
    </div>
  `;
}

function renderAlerts(page) {
  if (!state.active || !state.recent) {
    page.innerHTML = `<p class="empty">Loading…</p>`;
    return;
  }
  page.innerHTML = `
    <div class="page-header">
      <div>
        <h1>Alerts</h1>
        <p class="subtitle">${state.active.count} active, ${state.recent.count} recent (in-memory ring buffer).</p>
      </div>
      <button class="btn danger" id="reset-btn">Reset dedupe</button>
    </div>

    <div class="panel">
      <div class="panel-header"><h2>Active (${state.active.count})</h2></div>
      <div class="panel-body">${alertListHTML(state.active.alerts) || emptyHTML("Nothing firing right now.")}</div>
    </div>

    <div class="panel">
      <div class="panel-header"><h2>Recent (${state.recent.count})</h2></div>
      <div class="panel-body">${recentListHTML(state.recent.alerts) || emptyHTML("No activity in this session.")}</div>
    </div>
  `;
  const btn = $("#reset-btn", page);
  if (btn) btn.onclick = async () => {
    if (!confirm("Clear in-memory dedupe + active set + persisted state? Alerts will re-fire for any currently-firing condition.")) return;
    try {
      const r = await fetch("/reset-dedupe", { method: "POST" });
      const j = await r.json();
      toast(`Cleared ${j.active_cleared} active, ${j.dedupe_cleared} dedupe entries.`);
      refresh();
    } catch (e) {
      toast(`Reset failed: ${e.message}`, true);
    }
  };
}

function renderMonitors(page) {
  if (!state.monitors) { page.innerHTML = `<p class="empty">Loading…</p>`; return; }
  const rows = (state.monitors.monitors || []).map(m => {
    const dot = m.enabled ? `<span class="dot on"></span>` : `<span class="dot off"></span>`;
    const knobs = formatKnobs(m.knobs);
    return `
      <tr>
        <td>${dot}<span class="monitor-name">${esc(m.name)}</span></td>
        <td>${m.enabled ? "enabled" : "<span style='color:var(--text-muted)'>disabled</span>"}</td>
        <td>${esc(knobs)}</td>
        <td>${m.fires || 0}</td>
        <td>${m.resolves || 0}</td>
        <td>${timeAgo(m.last_fired)}</td>
      </tr>
    `;
  }).join("");
  page.innerHTML = `
    <div class="page-header">
      <div>
        <h1>Monitors</h1>
        <p class="subtitle">${state.monitors.count} configured. Fires/resolves are per-process counters since startup.</p>
      </div>
    </div>
    <div class="panel">
      <table class="monitor-table">
        <thead><tr><th>Name</th><th>Status</th><th>Knobs</th><th>Fires</th><th>Resolves</th><th>Last fired</th></tr></thead>
        <tbody>${rows || `<tr><td colspan="6" class="empty">No monitors found.</td></tr>`}</tbody>
      </table>
    </div>
  `;
}

function renderChannels(page) {
  if (!state.channels) { page.innerHTML = `<p class="empty">Loading…</p>`; return; }
  const rows = (state.channels.channels || []).map(c => `
    <tr>
      <td><span class="monitor-name">${esc(c.name)}</span></td>
      <td><button class="btn" data-channel="${esc(c.name)}">Send test</button></td>
    </tr>
  `).join("");
  page.innerHTML = `
    <div class="page-header">
      <div>
        <h1>Channels</h1>
        <p class="subtitle">${state.channels.count} registered. "Send test" calls <code>POST /test-channel?name=...</code>.</p>
      </div>
    </div>
    <div class="panel">
      <table class="monitor-table">
        <thead><tr><th>Name</th><th>Action</th></tr></thead>
        <tbody>${rows || `<tr><td colspan="2" class="empty">No channels configured. Edit Secret/kpulse-secrets and ConfigMap/kpulse-config to add one.</td></tr>`}</tbody>
      </table>
    </div>
  `;
  $$("[data-channel]", page).forEach(b => b.onclick = async () => {
    const name = b.dataset.channel;
    b.disabled = true;
    b.textContent = "sending…";
    try {
      const r = await fetch(`/test-channel?name=${encodeURIComponent(name)}`);
      const t = await r.text();
      if (r.ok) toast(`Sent test alert via ${name}.`);
      else toast(`${name}: ${t.split("\n")[0]}`, true);
    } catch (e) {
      toast(`${name}: ${e.message}`, true);
    } finally {
      b.disabled = false;
      b.textContent = "Send test";
    }
  });
}

function renderAbout(page) {
  if (!state.cluster) { page.innerHTML = `<p class="empty">Loading…</p>`; return; }
  const c = state.cluster;
  page.innerHTML = `
    <div class="page-header">
      <div>
        <h1>About</h1>
        <p class="subtitle">Runtime configuration of this kpulse instance.</p>
      </div>
    </div>
    <div class="panel">
      <dl class="kv">
        <dt>Cluster name</dt><dd>${esc(c.name)}</dd>
        <dt>kpulse version</dt><dd>${esc(c.version)}</dd>
        <dt>Started at</dt><dd>${esc(c.started_at)}</dd>
        <dt>Uptime</dt><dd>${esc(timeAgo(c.started_at))}</dd>
        <dt>Namespaces include</dt><dd>${esc((c.namespaces_include || []).join(", ") || "(none)")}</dd>
        <dt>Namespaces exclude</dt><dd>${esc((c.namespaces_exclude || []).join(", ") || "(none)")}</dd>
        <dt>Dedupe window</dt><dd>${esc(c.dedupe_window)}</dd>
        <dt>Digest</dt><dd>${c.digest_enabled ? `enabled, every ${esc(c.digest_interval)}` : "disabled"}</dd>
        <dt>Resolution alerts</dt><dd>${c.resolution_enabled ? "enabled" : "disabled"}</dd>
      </dl>
    </div>
    <div class="panel">
      <div class="panel-header"><h2>Links</h2></div>
      <div class="panel-body kv" style="padding:18px">
        <dt>Docs</dt><dd><a href="https://docs.kpulse.io" target="_blank">docs.kpulse.io</a></dd>
        <dt>Source</dt><dd><a href="https://github.com/dnl555/kpulse" target="_blank">github.com/dnl555/kpulse</a></dd>
      </div>
    </div>
  `;
}

function countBySeverity(alerts) {
  const c = { critical: 0, warning: 0, info: 0 };
  (alerts || []).forEach(a => { if (c[a.severity] != null) c[a.severity]++; });
  return c;
}

function alertListHTML(alerts) {
  if (!alerts || alerts.length === 0) return "";
  return `<ul class="alert-list">` + alerts.map(a => `
    <li class="alert-row">
      <span class="severity-pill ${esc(a.severity)}">${esc(a.severity)}</span>
      <div class="alert-body">
        <div class="alert-title">${esc(a.title || a.reason || a.monitor)}</div>
        <div class="alert-meta"><strong>monitor</strong> ${esc(a.monitor)} &middot; <strong>object</strong> ${esc(a.namespace || "")}${a.namespace ? "/" : ""}${esc(a.object)} ${a.reason ? `&middot; <strong>reason</strong> ${esc(a.reason)}` : ""}</div>
        ${a.body ? `<div class="alert-detail">${esc(a.body)}</div>` : ""}
      </div>
      <span class="alert-time">${timeAgo(a.fired_at)}</span>
    </li>
  `).join("") + `</ul>`;
}

function recentListHTML(recent) {
  if (!recent || recent.length === 0) return "";
  return `<ul class="alert-list">` + recent.map(a => `
    <li class="alert-row">
      <span class="severity-pill ${esc(a.state === "resolved" ? "resolved" : a.severity)}">${esc(a.state === "resolved" ? "resolved" : a.severity)}</span>
      <div class="alert-body">
        <div class="alert-title">${esc(a.title || a.reason || a.monitor)}</div>
        <div class="alert-meta"><strong>monitor</strong> ${esc(a.monitor)} &middot; <strong>state</strong> ${esc(a.state)} &middot; <strong>object</strong> ${esc(a.namespace || "")}${a.namespace ? "/" : ""}${esc(a.object)} ${a.channels && a.channels.length ? `&middot; <strong>sent to</strong> ${esc(a.channels.join(", "))}` : ""}</div>
      </div>
      <span class="alert-time">${timeAgo(a.at)}</span>
    </li>
  `).join("") + `</ul>`;
}

function emptyHTML(text) { return `<p class="empty">${esc(text)}</p>`; }

function formatKnobs(knobs) {
  if (!knobs) return "";
  return Object.entries(knobs).map(([k, v]) => {
    if (Array.isArray(v)) return `${k}=[${v.length}]`;
    if (typeof v === "object" && v !== null) return `${k}={…}`;
    return `${k}=${v}`;
  }).join(" ");
}

function toast(msg, isError = false) {
  const existing = $(".toast");
  if (existing) existing.remove();
  const el = document.createElement("div");
  el.className = `toast${isError ? " error" : ""}`;
  el.textContent = msg;
  document.body.appendChild(el);
  setTimeout(() => el.remove(), 4500);
}

// ============ boot ============

$("#theme-toggle").onclick = () => {
  const cur = document.documentElement.dataset.theme;
  const next = cur === "dark" ? "light" : "dark";
  document.documentElement.dataset.theme = next;
  try { localStorage.setItem("kpulse-theme", next); } catch (e) {}
};
try {
  const t = localStorage.getItem("kpulse-theme");
  if (t) document.documentElement.dataset.theme = t;
} catch (e) {}

window.addEventListener("hashchange", renderCurrent);
refresh();
setInterval(refresh, POLL_INTERVAL_MS);
