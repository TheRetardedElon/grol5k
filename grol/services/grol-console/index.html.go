package main

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>GROL5000</title>
<style>
:root { color-scheme: dark; --bg:#0b0d10; --panel:#15191f; --line:#2a313b; --acc:#e11d2e; --text:#e8edf2; --mute:#8b98a5; --good:#3ddc84; --warn:#f5c542; }
* { box-sizing: border-box; }
html, body { margin:0; height:100%; background:var(--bg); color:var(--text); font:15px/1.45 ui-sans-serif,system-ui,sans-serif; }
button,input { font:inherit; }
.app { display:grid; grid-template-columns:220px 1fr; height:100%; }
nav { display:flex; flex-direction:column; background:var(--panel); border-right:1px solid var(--line); padding:20px 12px; }
.brand { font-weight:800; letter-spacing:.08em; margin:0 12px 4px; }
.sub { color:var(--mute); font-size:12px; margin:0 12px 24px; }
button.item { width:100%; text-align:left; color:var(--text); background:transparent; border:0; padding:10px 12px; border-radius:8px; cursor:pointer; }
button.item.active, button.item:hover { background:#1e252e; }
.navfoot { margin-top:auto; color:var(--mute); font-size:12px; padding:12px; }
main { display:flex; flex-direction:column; min-width:0; }
header { display:flex; justify-content:space-between; align-items:center; padding:16px 24px; border-bottom:1px solid var(--line); }
#dot { width:8px; height:8px; border-radius:50%; background:#666; display:inline-block; margin-right:8px; }
#viewTitle { font-weight:700; }
#botView { display:flex; flex:1; flex-direction:column; min-height:0; }
#messages { flex:1; overflow:auto; padding:24px; }
.msg { max-width:900px; margin:0 auto 16px; }
.msg .who { color:var(--mute); font-size:12px; margin-bottom:5px; text-transform:uppercase; letter-spacing:.06em; }
.msg .body { white-space:pre-wrap; overflow-wrap:anywhere; }
.msg.user .body { color:#fff; }
.msg.bot .body { color:#dce5ed; }
.msg.error .body { color:#ff8d99; }
.composer { display:flex; gap:8px; padding:16px 24px; border-top:1px solid var(--line); }
input { flex:1; background:#0f1318; color:var(--text); border:1px solid var(--line); border-radius:8px; padding:12px; outline:none; }
input:focus { border-color:#59697a; }
button.send { background:var(--acc); color:#fff; border:0; border-radius:8px; padding:12px 18px; font-weight:700; cursor:pointer; }
button.send:disabled { opacity:.45; cursor:default; }
.note { color:var(--mute); padding:12px 24px 0; font-size:13px; }
#placeholder { display:none; flex:1; padding:42px; }
.card { max-width:800px; padding:24px; border:1px solid var(--line); border-radius:12px; background:var(--panel); }
.card h2 { margin-top:0; }
.card p { color:var(--mute); }
@media (max-width:720px) {
  .app { grid-template-columns:72px 1fr; }
  .brand { font-size:11px; overflow:hidden; }
  .sub,.item span,.navfoot { display:none; }
  button.item { text-align:center; }
}
</style>
</head>
<body>
<div class="app">
<nav>
  <p class="brand">GROL5000</p>
  <p class="sub">Global Robotic Overlord Logic</p>
  <button class="item active" data-view="bot">● <span>Grok Bot</span></button>
  <button class="item" data-view="overview">⌂ <span>Overview</span></button>
  <button class="item" data-view="devices">◇ <span>Devices</span></button>
  <button class="item" data-view="system">▣ <span>System</span></button>
  <button class="item" data-view="activity">≡ <span>Activity</span></button>
  <button class="item" data-view="build">◆ <span>Build</span></button>
  <button class="item" data-view="settings">⚙ <span>Settings</span></button>
  <div class="navfoot">M3B · resident bot · no mutations</div>
</nav>
<main>
  <header>
    <div><span id="dot"></span><span id="state">checking resident services</span></div>
    <div id="viewTitle">Grok Bot</div>
  </header>

  <section id="botView">
    <p class="note">Grok Bot owns the local session and context, then talks to grol-ai-gateway. No Home Assistant, host, Docker, broker, or Build mutation path exists in M3B.</p>
    <div id="messages"></div>
    <form class="composer" id="chatForm">
      <input id="q" autocomplete="off" placeholder="Talk to Grok Bot"/>
      <button class="send" id="send" type="submit">Send</button>
    </form>
  </section>

  <section id="placeholder">
    <div class="card">
      <h2 id="placeholderTitle"></h2>
      <p id="placeholderText"></p>
    </div>
  </section>
</main>
</div>

<script>
const messages = document.getElementById('messages');
const dot = document.getElementById('dot');
const state = document.getElementById('state');
const input = document.getElementById('q');
const send = document.getElementById('send');
const botView = document.getElementById('botView');
const placeholder = document.getElementById('placeholder');
const viewTitle = document.getElementById('viewTitle');
let sessionId = localStorage.getItem('grol.bot.session_id') || '';

const labels = {
  bot:['Grok Bot',''],
  overview:['Overview','Live GROL health and summaries land here in M3C.'],
  devices:['Devices','Read-only Home Assistant entity/device inventory lands here in M3C.'],
  system:['System','Host, OS, Supervisor, Core, gateway, and broker health land here in M3C.'],
  activity:['Activity','Grok Bot proposals, approvals, and system events land here.'],
  build:['Build','Grok Build remains isolated and unavailable in M3B.'],
  settings:['Settings','Provisioning and operator preferences land here without exposing secrets to the model.']
};

function addMessage(kind, who, text='') {
  const wrap = document.createElement('div');
  wrap.className = 'msg ' + kind;
  const whoEl = document.createElement('div');
  whoEl.className = 'who';
  whoEl.textContent = who;
  const body = document.createElement('div');
  body.className = 'body';
  body.textContent = text;
  wrap.append(whoEl, body);
  messages.appendChild(wrap);
  messages.scrollTop = messages.scrollHeight;
  return body;
}

async function refresh() {
  try {
    const response = await fetch('/api/status', {cache:'no-store'});
    const s = await response.json();
    const b = s.bot || {};
    const g = s.gateway || {};
    const botOK = b.reachable === true && b.ok === true;
    const gatewayOK = g.reachable === true && g.ok === true;
    dot.style.background = botOK && gatewayOK ? (g.provisioned ? 'var(--good)' : 'var(--warn)') : 'var(--acc)';
    if (!botOK) state.textContent = 'grol-bot unreachable';
    else if (!gatewayOK) state.textContent = 'bot online · gateway unreachable';
    else if (!g.provisioned) state.textContent = 'bot online · gateway online · Grok not provisioned';
    else state.textContent = 'bot online · ' + (g.model || 'Grok') + ' · streaming';
  } catch (_) {
    dot.style.background = 'var(--acc)';
    state.textContent = 'console error';
  }
}

function parseEventBlock(block) {
  let event = 'message';
  const data = [];
  for (const line of block.split('\n')) {
    if (line.startsWith('event:')) event = line.slice(6).trim();
    if (line.startsWith('data:')) data.push(line.slice(5).trim());
  }
  if (!data.length) return null;
  try { return {event, data: JSON.parse(data.join('\n'))}; }
  catch (_) { return null; }
}

async function streamChat(text) {
  addMessage('user', 'you', text);
  const botBody = addMessage('bot', 'grok bot', '');
  send.disabled = true;
  input.disabled = true;

  try {
    const response = await fetch('/api/chat', {
      method:'POST',
      headers:{'Content-Type':'application/json','Accept':'text/event-stream'},
      body:JSON.stringify({
        session_id: sessionId,
        messages:[{role:'user', content:text}],
        stream:true
      })
    });

    const contentType = response.headers.get('content-type') || '';
    if (!response.ok) {
      let detail = 'request failed (' + response.status + ')';
      try {
        const j = await response.json();
        if (j.error) detail += ': ' + j.error;
      } catch (_) {}
      throw new Error(detail);
    }

    if (!contentType.startsWith('text/event-stream')) {
      const j = await response.json();
      if (j.session_id) {
        sessionId = j.session_id;
        localStorage.setItem('grol.bot.session_id', sessionId);
      }
      botBody.textContent = j.text || JSON.stringify(j);
      return;
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let gotText = false;

    for (;;) {
      const {value, done} = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, {stream:true}).replace(/\r\n/g, '\n');

      let split;
      while ((split = buffer.indexOf('\n\n')) !== -1) {
        const block = buffer.slice(0, split);
        buffer = buffer.slice(split + 2);
        const evt = parseEventBlock(block);
        if (!evt) continue;

        if (evt.event === 'session') {
          if (evt.data.session_id) {
            sessionId = evt.data.session_id;
            localStorage.setItem('grol.bot.session_id', sessionId);
          }
        } else if (evt.event === 'delta') {
          botBody.textContent += evt.data.text || '';
          gotText = true;
          messages.scrollTop = messages.scrollHeight;
        } else if (evt.event === 'error') {
          throw new Error(evt.data.error || 'provider stream error');
        }
      }
    }

    if (!gotText && !botBody.textContent) {
      botBody.textContent = '(Grok returned no text)';
    }
  } catch (err) {
    botBody.parentElement.classList.add('error');
    botBody.textContent = 'Error: ' + err.message;
  } finally {
    send.disabled = false;
    input.disabled = false;
    input.focus();
  }
}

document.getElementById('chatForm').addEventListener('submit', async (e) => {
  e.preventDefault();
  const text = input.value.trim();
  if (!text || send.disabled) return;
  input.value = '';
  await streamChat(text);
});

document.querySelectorAll('button.item').forEach((button) => {
  button.addEventListener('click', () => {
    document.querySelectorAll('button.item').forEach(b => b.classList.remove('active'));
    button.classList.add('active');
    const id = button.dataset.view;
    const [title, desc] = labels[id] || [id, ''];
    viewTitle.textContent = title;
    if (id === 'bot') {
      botView.style.display = 'flex';
      placeholder.style.display = 'none';
      return;
    }
    botView.style.display = 'none';
    placeholder.style.display = 'block';
    document.getElementById('placeholderTitle').textContent = title;
    document.getElementById('placeholderText').textContent = desc;
  });
});

refresh();
setInterval(refresh, 5000);
input.focus();
</script>
</body>
</html>
`;
