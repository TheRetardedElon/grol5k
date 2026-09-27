package main

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>GROL5000</title>
<style>
:root { color-scheme: dark; --bg:#0b0d10; --panel:#15191f; --line:#2a313b; --acc:#e11d2e; --text:#e8edf2; --mute:#8b98a5; }
* { box-sizing: border-box; }
html, body { margin:0; height:100%; background:var(--bg); color:var(--text); font: 15px/1.4 ui-sans-serif, system-ui, sans-serif; }
.app { display:grid; grid-template-columns: 220px 1fr; height:100%; }
nav { background:var(--panel); border-right:1px solid var(--line); padding:20px 12px; }
.brand { font-weight:700; letter-spacing:.08em; margin:0 12px 4px; }
.sub { color:var(--mute); font-size:12px; margin:0 12px 24px; }
a.item { display:block; color:var(--text); text-decoration:none; padding:10px 12px; border-radius:8px; }
a.item.active, a.item:hover { background:#1e252e; }
main { display:flex; flex-direction:column; min-width:0; }
header { display:flex; justify-content:space-between; align-items:center; padding:16px 24px; border-bottom:1px solid var(--line); }
#dot { width:8px; height:8px; border-radius:50%; background:#666; display:inline-block; margin-right:8px; }
#log { flex:1; overflow:auto; padding:24px; white-space:pre-wrap; }
.composer { display:flex; gap:8px; padding:16px 24px; border-top:1px solid var(--line); }
input { flex:1; background:#0f1318; color:var(--text); border:1px solid var(--line); border-radius:8px; padding:12px; }
button { background:var(--acc); color:#fff; border:0; border-radius:8px; padding:12px 16px; font-weight:600; }
.note { color:var(--mute); padding:0 24px 12px; font-size:13px; }
</style>
</head>
<body>
<div class="app">
<nav>
  <p class="brand">GROL5000</p>
  <p class="sub">Global Robotic Overlord Logic</p>
  <a class="item active" href="#bot">Grok Bot</a>
  <a class="item" href="#overview">Overview</a>
  <a class="item" href="#devices">Devices</a>
  <a class="item" href="#system">System</a>
  <a class="item" href="#activity">Activity</a>
  <a class="item" href="#build">Build</a>
  <a class="item" href="#settings">Settings</a>
</nav>
<main>
  <header>
    <div><span id="dot"></span><span id="state">checking gateway</span></div>
    <div>operator console :8790</div>
  </header>
  <p class="note">M3A skeleton. Chat talks to grol-ai-gateway only. No house mutations. Inherited Home Assistant remains on :8123 until M5.</p>
  <div id="log"></div>
  <form class="composer" id="f">
    <input id="q" autocomplete="off" placeholder="Talk to Grok Bot"/>
    <button type="submit">Send</button>
  </form>
</main>
</div>
<script>
const log = document.getElementById('log');
const dot = document.getElementById('dot');
const state = document.getElementById('state');
function line(t){ log.textContent += t + '\n'; log.scrollTop = log.scrollHeight; }
async function refresh(){
  try {
    const s = await (await fetch('/api/status')).json();
    const g = s.gateway || {};
    const ok = g.ok === true;
    dot.style.background = ok ? (g.provisioned ? '#3ddc84' : '#f5c542') : '#e11d2e';
    state.textContent = ok ? (g.provisioned ? 'gateway online, Grok provisioned' : 'gateway online, Grok not provisioned') : 'gateway unreachable';
  } catch(e) {
    dot.style.background = '#e11d2e';
    state.textContent = 'console error';
  }
}
document.getElementById('f').addEventListener('submit', async (e) => {
  e.preventDefault();
  const q = document.getElementById('q');
  const text = q.value.trim();
  if (!text) return;
  q.value = '';
  line('you: ' + text);
  const r = await fetch('/api/chat', {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify({messages:[{role:'user', content:text}]})});
  const j = await r.json();
  line('bot: ' + (j.text || JSON.stringify(j)));
});
refresh();
setInterval(refresh, 5000);
</script>
</body>
</html>
`
