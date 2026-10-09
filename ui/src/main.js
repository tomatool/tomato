/* tomato ui — renderer for the "Modernist" design system.
 *
 * Plain DOM, no framework. Vite bundles it into command/ui_assets/dist, which
 * the Go binary embeds. The contract from the design system is that state lives
 * in data-* and aria-* attributes and classes never change at runtime, so every
 * render here writes attributes, not class names.
 */
// Modules are strict by default.
import './styles.css';

// ——— state ———————————————————————————————————————————————————————————

var features = [];          // FeatureJSON[]
var cfg = null;             // parsed /api/config payload
var runs = [];              // runlog.RunInfo[]
var topo = null;            // TopologyJSON from the server (app + every resource)

var view = 'empty';         // 'empty' | 'feature' | 'config' | 'log'
var selectedFile = null;
var selectedScenario = null; // index into the visible scenario list
var cfgMode = 'structured';
var sideTab = 'flow';
var filter = '';

var scStatus = {};          // scenario name -> status
var scDur = {};             // scenario name -> ms
var stStatus = {};          // "scenario\u0000index" -> status
var stDur = {};             // "scenario\u0000index" -> ms
var stError = {};           // "scenario\u0000index" -> message
var running = false;
var runId = null;
var runStartedAt = 0;
var changed = Object.create(null);
var expandedDirs = Object.create(null);
var collapsedScenarios = Object.create(null);
var openRun = null;
var logTabs = [];           // [{run, name}]
var activeLog = null;
var hot = null;             // flow/step hover index
var cursor = -1;            // transport position
var playTimer = null;

var ws = null;

// ——— small helpers ———————————————————————————————————————————————————

function esc(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}
function attr(s) { return esc(s); }
function el(id) { return document.getElementById(id); }
function key(scenario, i) { return scenario + '\u0000' + i; }

function fmtDur(ms) {
  if (ms == null) return '';
  if (ms < 1000) return Math.round(ms) + 'ms';
  if (ms < 60000) return (ms / 1000).toFixed(2) + 's';
  var m = Math.floor(ms / 60000);
  return m + 'm' + Math.round((ms % 60000) / 1000) + 's';
}
function fmtClock(ms) {
  var t = Math.max(0, ms) / 1000;
  var m = Math.floor(t / 60), s = t - m * 60;
  return String(m).padStart(2, '0') + ':' + s.toFixed(1).padStart(4, '0');
}
function clip(s, n) {
  s = String(s);
  return s.length > n ? s.slice(0, n - 1) + '…' : s;
}

function plural(n, word) { return n + ' ' + word + (n === 1 ? '' : 's'); }

function fmtBytes(n) {
  if (n == null) return '';
  if (n < 1024) return n + ' B';
  if (n < 1024 * 1024) return Math.round(n / 1024) + ' KB';
  return (n / 1048576).toFixed(1) + ' MB';
}

// Resource type -> sprite id. Aliases per the design system's icon map.
var ICON_ALIAS = {
  'http-client': 'http', 'http': 'http', 'http-server': 'http-server',
  'grpc-client': 'grpc', 'grpc': 'grpc',
  'websocket-client': 'websocket', 'websocket': 'websocket',
  'websocket-server': 'websocket-server',
  'postgresql': 'postgres', 'postgres': 'postgres',
  'cassandra': 'scylladb', 'scylladb': 'scylladb',
  'minio': 's3', 's3': 's3',
  'redis': 'redis', 'kafka': 'kafka', 'rabbitmq': 'rabbitmq',
  'shell': 'shell', 'aws': 'aws'
};
function icon(type) {
  var id = ICON_ALIAS[String(type || '').toLowerCase()] || 'app';
  return '<svg class="ri" aria-hidden="true"><use href="#ri-' + id + '"></use></svg>';
}

// Step text -> ref / arg / var spans. First quoted token is the resource.
function tokenize(text) {
  var out = '', i = 0, quoted = 0;
  var re = /"([^"]*)"/g, m;
  while ((m = re.exec(text)) !== null) {
    out += markVars(text.slice(i, m.index));
    quoted++;
    out += '<span class="' + (quoted === 1 ? 'ref' : 'arg') + '">&quot;' +
      markVars(m[1]) + '&quot;</span>';
    i = m.index + m[0].length;
  }
  return out + markVars(text.slice(i));
}
function markVars(s) {
  var out = '', i = 0;
  var re = /\{\{[^}]*\}\}|<[A-Za-z0-9_][^>]*>/g, m;
  while ((m = re.exec(s)) !== null) {
    out += esc(s.slice(i, m.index));
    out += '<span class="var">' + esc(m[0]) + '</span>';
    i = m.index + m[0].length;
  }
  return out + esc(s.slice(i));
}

function visibleScenarios(f) {
  var list = (f.scenarios || []);
  if (!filter) return list;
  var q = filter.toLowerCase();
  if (q.charAt(0) === '@') {
    return list.filter(function (s) {
      return (s.tags || []).some(function (t) { return t.toLowerCase().indexOf(q) === 0; });
    });
  }
  return list.filter(function (s) {
    if (s.name.toLowerCase().indexOf(q) >= 0) return true;
    return (s.steps || []).some(function (st) { return st.text.toLowerCase().indexOf(q) >= 0; });
  });
}
function matchesFilter(f) {
  if (!filter) return true;
  var q = filter.toLowerCase();
  if (f.name.toLowerCase().indexOf(q) >= 0) return true;
  if ((f.tags || []).some(function (t) { return t.toLowerCase().indexOf(q) >= 0; })) return true;
  return visibleScenarios(f).length > 0;
}

function featureStatus(f) {
  var any = false, failed = false, undef = false, runningAny = false, all = true;
  (f.scenarios || []).forEach(function (s) {
    var st = scStatus[s.name];
    if (!st) { all = false; return; }
    any = true;
    if (st === 'failed') failed = true;
    else if (st === 'undefined') undef = true;
    else if (st === 'running' || st === 'queued') runningAny = true;
  });
  if (runningAny) return 'running';
  if (failed) return 'failed';
  if (undef) return 'undefined';
  if (any && all) return 'passed';
  return '';
}

function allScenarios() {
  var out = [];
  features.forEach(function (f) {
    (f.scenarios || []).forEach(function (s) { out.push({ f: f, s: s }); });
  });
  return out;
}

// ——— websocket ———————————————————————————————————————————————————————

function connect() {
  var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  ws = new WebSocket(proto + '//' + location.host + '/ws');
  ws.onopen = function () { setLive('', 'watching ' + ((cfg && cfg.watching) || './features')); };
  ws.onclose = function () { setLive('offline', 'disconnected'); setTimeout(connect, 2000); };
  ws.onerror = function () { setLive('error', 'connection error'); };
  ws.onmessage = function (ev) {
    var msg;
    try { msg = JSON.parse(ev.data); } catch (e) { return; }
    handle(msg);
  };
}

function setLive(state, text) {
  var n = el('live');
  n.setAttribute('data-state', state || '');
  n.textContent = text;
}

function handle(m) {
  switch (m.type) {
    case 'init':
    case 'update':
      features = m.features || [];
      if (m.runs) { runs = m.runs; }
      if (m.topology) { topo = m.topology; }
      if (m.changedFiles) {
        m.changedFiles.forEach(function (p) {
          changed[p] = true;
          setTimeout(function () { delete changed[p]; renderTree(); }, 3000);
          var parts = p.split('/');
          for (var i = 0; i < parts.length - 1; i++) {
            expandedDirs[parts.slice(0, i + 1).join('/')] = true;
          }
        });
      }
      if (view === 'empty' && features.length) selectFile(features[0].filePath, true);
      renderAll();
      break;

    case 'error':
      setLive('error', m.error || 'error');
      break;

    case 'run_started':
      running = true;
      runStartedAt = Date.now();
      scStatus = {}; scDur = {}; stStatus = {}; stDur = {}; stError = {};
      allScenarios().forEach(function (p) { scStatus[p.s.name] = 'queued'; });
      el('console').innerHTML = '';
      renderAll();
      break;

    case 'scenario_running':
      scStatus[m.scenario] = 'running';
      renderAll();
      break;

    case 'scenario_passed':
      scStatus[m.scenario] = 'passed';
      if (m.durationMs != null) scDur[m.scenario] = m.durationMs;
      renderAll();
      break;

    case 'scenario_failed':
      scStatus[m.scenario] = 'failed';
      if (m.durationMs != null) scDur[m.scenario] = m.durationMs;
      renderAll();
      break;

    case 'step_started':
      if (m.scenario != null && m.stepIndex != null) {
        stStatus[key(m.scenario, m.stepIndex)] = 'running';
        renderDoc();
      }
      break;

    case 'step_finished':
      if (m.scenario != null && m.stepIndex != null) {
        var k = key(m.scenario, m.stepIndex);
        stStatus[k] = m.status || '';
        if (m.durationMs != null) stDur[k] = m.durationMs;
        if (m.output) stError[k] = m.output;
      }
      renderDoc();
      break;

    case 'run_output':
      appendLine(m.output, m.status);
      break;

    case 'run_finished':
    case 'run_error':
      running = false;
      Object.keys(scStatus).forEach(function (n) {
        if (scStatus[n] === 'queued' || scStatus[n] === 'running') delete scStatus[n];
      });
      if (m.output && m.type === 'run_error') appendLine('<span class="c-red">' + esc(m.output) + '</span>', 'failed');
      renderAll();
      break;

    case 'runs_update':
      runs = m.runs || [];
      renderSide(); renderConsoleTabs();
      break;

    case 'run_created':
      runId = m.runId;
      renderConsoleTabs();
      break;
  }
}

// ——— top bar ——————————————————————————————————————————————————————————

function renderTop() {
  var f = features.find(function (x) { return x.filePath === selectedFile; });
  var crumb = el('crumb');
  if (view === 'config') {
    crumb.innerHTML = '<b>' + esc((cfg && cfg.path) || 'tomato.yml') + '</b>';
  } else if (view === 'log' && activeLog) {
    crumb.innerHTML = '.tomato/runs/' + esc(activeLog.run) + ' <span>&rsaquo;</span> <b>' + esc(activeLog.name) + '</b>';
  } else if (f) {
    var parts = f.filePath.split('/');
    var base = parts.pop();
    crumb.innerHTML = esc(parts.join('/') + (parts.length ? '/' : '')) + '<b>' + esc(base) + '</b>';
  } else {
    crumb.textContent = '';
  }

  var all = allScenarios();
  var segs = all.map(function (p) {
    return '<i class="runbar-seg" data-status="' + attr(scStatus[p.s.name] || '') + '"></i>';
  }).join('');
  el('runbar').innerHTML = segs;

  var pass = 0, fail = 0, und = 0, total = 0;
  all.forEach(function (p) {
    var st = scStatus[p.s.name];
    if (st === 'passed') pass++;
    else if (st === 'failed') fail++;
    else if (st === 'undefined') und++;
    if (st) total++;
  });
  var bits = [];
  if (pass) bits.push('<span class="tally-item"><i class="pip" data-status="passed"></i><b>' + pass + '</b></span>');
  if (fail) bits.push('<span class="tally-item"><i class="pip" data-status="failed"></i><b>' + fail + '</b></span>');
  if (und) bits.push('<span class="tally-item"><i class="pip" data-status="undefined"></i><b>' + und + '</b></span>');
  if (!total) bits.push('<span class="tally-item muted">' + all.length + ' scenarios</span>');
  var elapsed = 0;
  Object.keys(scDur).forEach(function (n) { elapsed += scDur[n]; });
  if (elapsed) bits.push('<span>' + fmtDur(elapsed) + '</span>');
  el('tally').innerHTML = bits.join('');

  el('runAllBtn').hidden = running;
  el('stopBtn').hidden = !running;
  el('runFailedBtn').hidden = running || fail === 0;
  el('featureCount').textContent = String(features.length);
  el('railMeta').innerHTML = (fail ? '<i class="pip" data-status="failed"></i>' : '') + all.length;
  el('runsCount').textContent = String(runs.length);
}

// ——— feature tree ————————————————————————————————————————————————————

function renderTree() {
  var rows = [];
  rows.push(row({
    kind: 'config', depth: 0, selected: view === 'config',
    caret: '', icon: '<svg class="ri" aria-hidden="true"><use href="#ri-app"></use></svg>',
    name: (cfg && cfg.path) || 'tomato.yml',
    meta: cfg && cfg.resources ? cfg.resources.length + ' res' : '',
    act: 'config'
  }));
  rows.push('<div class="tree-sep"></div>');

  // group features by directory
  var tree = {};
  features.filter(matchesFilter).forEach(function (f) {
    var parts = f.filePath.split('/');
    parts.pop();
    var dir = parts.join('/');
    (tree[dir] = tree[dir] || []).push(f);
  });

  Object.keys(tree).sort().forEach(function (dir) {
    var open = expandedDirs[dir] !== false;
    if (dir) {
      rows.push(row({
        kind: 'dir', depth: 0, caret: chev(open),
        name: dir.replace(/^\.\//, '') + '/', act: 'dir', arg: dir
      }));
      if (!open) return;
    }
    tree[dir].forEach(function (f) {
      var fOpen = selectedFile === f.filePath;
      rows.push(row({
        kind: 'feature', depth: dir ? 1 : 0, caret: chev(fOpen),
        selected: fOpen && view === 'feature',
        pip: featureStatus(f), name: f.name || f.filePath,
        meta: String((f.scenarios || []).length),
        changed: !!changed[f.filePath], act: 'file', arg: f.filePath
      }));
      if (!fOpen) return;
      visibleScenarios(f).forEach(function (s, i) {
        rows.push(row({
          kind: 'scenario', depth: (dir ? 1 : 0) + 1, caret: '',
          selected: view === 'feature' && selectedFile === f.filePath && selectedScenario === i,
          pip: scStatus[s.name] || '', name: s.name,
          act: 'scenario', arg: f.filePath, arg2: String(i)
        }));
      });
    });
  });

  el('tree').innerHTML = rows.join('');
}

function chev(open) {
  return '<span class="tree-caret"><i class="ico-chev"' + (open ? ' data-open="true"' : '') + '></i></span>';
}

function row(o) {
  var a = ['class="tree-row"', 'data-kind="' + attr(o.kind) + '"', 'style="--depth:' + (o.depth || 0) + '"'];
  if (o.selected) a.push('aria-selected="true"');
  if (o.changed) a.push('data-changed="true"');
  if (o.act) a.push('data-act="' + attr(o.act) + '"');
  if (o.arg != null) a.push('data-arg="' + attr(o.arg) + '"');
  if (o.arg2 != null) a.push('data-arg2="' + attr(o.arg2) + '"');
  return '<button ' + a.join(' ') + '>' +
    (o.caret || '<span class="tree-caret"></span>') +
    (o.pip ? '<i class="pip" data-status="' + attr(o.pip) + '"></i>' : (o.icon || '')) +
    '<span class="tree-name">' + esc(o.name) + '</span>' +
    (o.meta ? '<span class="tree-meta">' + esc(o.meta) + '</span>' : '') +
    '</button>';
}

// ——— feature document ————————————————————————————————————————————————

function renderDoc() {
  var doc = el('doc');
  if (view === 'config') { doc.innerHTML = renderConfig(); return; }
  if (view === 'log') { return; } // console owns log rendering
  var f = features.find(function (x) { return x.filePath === selectedFile; });
  if (!f) {
    doc.innerHTML = '<div class="empty"><h3>No feature selected</h3>' +
      '<p>Pick a <code>.feature</code> file on the left, or press <span class="kbd">/</span> to filter. ' +
      '<span class="kbd">R</span> runs everything.</p></div>';
    return;
  }

  var list = visibleScenarios(f);
  var html = '<div class="doc">' +
    '<div class="feature-head">' +
      '<span class="feature-kw">FEATURE</span>' +
      '<h1>' + esc(f.name) + '</h1>' +
      '<span class="feature-path">' + esc(f.filePath) + '</span>' +
      (f.description ? '<p class="feature-desc">' + esc(f.description) + '</p>' : '') +
      (f.tags && f.tags.length ? '<div class="row">' + f.tags.map(tagBtn).join('') + '</div>' : '') +
    '</div>';

  if (f.background && f.background.length) html += renderBackground(f);

  if (!list.length) {
    html += '<div class="empty"><h3>Nothing matches “' + esc(filter) + '”</h3>' +
      '<p>Press <span class="kbd">esc</span> to clear the filter.</p></div>';
  }

  list.forEach(function (s, i) { html += renderScenario(s, i); });
  doc.innerHTML = html + '</div>';

  var cur = doc.querySelector('.scenario[aria-current="true"]');
  if (cur && cur.scrollIntoView) cur.scrollIntoView({ block: 'nearest' });
}

// A feature can have a scenario called anything, so the background's toggle key
// is prefixed rather than a sentinel character: a NUL here became U+FFFD once
// the browser parsed it back out of the data-arg attribute, and the toggle
// wrote to a key the render never read.
var BACKGROUND_KEY = 'background::feature';

// Background is a feature-level field, not a scenario: it has no name, no
// status of its own and never appears in counts or the run bar.
function renderBackground(f) {
  var collapsed = collapsedScenarios[BACKGROUND_KEY] !== false;
  return '<div class="scenario" data-kind="background"' +
    (collapsed ? ' data-collapsed="true"' : '') + '>' +
    '<div class="scenario-head" data-act="toggleScenario" data-arg="' + BACKGROUND_KEY + '"' +
    ' role="button" tabindex="0" aria-expanded="' + (collapsed ? 'false' : 'true') + '">' +
    chev(!collapsed) +
    '<span class="scenario-kw">Background</span>' +
    '<span class="scenario-name" style="color:var(--fg-2);font-weight:400">' +
    plural(f.background.length, 'step') + ' · runs before each scenario</span></div>' +
    '<div class="steps">' +
    f.background.map(function (st, i) { return renderStep(st, { name: BACKGROUND_KEY }, i); }).join('') +
    '</div></div>';
}

function tagBtn(t) {
  var name = String(t).replace(/^@/, '');
  var pressed = filter.toLowerCase() === '@' + name.toLowerCase();
  return '<button class="tag" data-act="tag" data-arg="' + attr(name) + '"' +
    (pressed ? ' aria-pressed="true"' : '') + '>' + esc(name) + '</button>';
}

function renderScenario(s, i) {
  var st = scStatus[s.name] || '';
  var collapsed = collapsedScenarios[s.name];
  if (collapsed === undefined) collapsed = st === 'passed';
  var kw = s.isOutline ? 'Scenario Outline' : 'Scenario';

  var a = ['class="scenario"', 'data-index="' + i + '"'];
  if (st) a.push('data-status="' + attr(st) + '"');
  if (collapsed) a.push('data-collapsed="true"');
  if (selectedScenario === i) a.push('aria-current="true"');

  // The whole head toggles, so it needs the same caret the Background has —
  // without one there is nothing to say the card opens.
  var head = '<div class="scenario-head" data-act="toggleScenario" data-arg="' + attr(s.name) + '"' +
    ' role="button" tabindex="0" aria-expanded="' + (collapsed ? 'false' : 'true') + '">' +
    chev(!collapsed) +
    '<i class="pip" data-status="' + attr(st) + '"></i>' +
    '<span class="scenario-kw">' + kw + '</span>' +
    '<span class="scenario-name">' + esc(s.name) + '</span>' +
    (s.isOutline && s.examples && s.examples.length
      ? '<span class="pill">' + exampleRows(s) + ' examples</span>' : '') +
    (scDur[s.name] != null ? '<span class="scenario-time">' + fmtDur(scDur[s.name]) + '</span>' : '') +
    '<div class="scenario-actions">' +
      '<button class="btn btn--sm btn--ghost" data-act="runScenario" data-arg="' + attr(s.name) + '">' +
      '<i class="ico-play"></i>Run</button></div>' +
    '</div>';

  var body = '';
  if (s.tags && s.tags.length) body += '<div class="scenario-tags">' + s.tags.map(tagBtn).join('') + '</div>';
  body += '<div class="steps">' +
    (s.steps || []).map(function (step, si) { return renderStep(step, s, si); }).join('') +
    '</div>';
  if (s.examples && s.examples.length) body += renderExamples(s);

  return '<div ' + a.join(' ') + '>' + head + body + '</div>';
}

function exampleRows(s) {
  var n = 0;
  (s.examples || []).forEach(function (ex) { n += Math.max(0, (ex.rows || []).length - 1); });
  return n;
}

function renderStep(step, s, si) {
  var k = key(s.name, si);
  // `unmatched` is known from parsing alone: the step has no definition, so it
  // reads as undefined before the suite has ever run.
  var st = stStatus[k] || (step.unmatched ? 'undefined' : '');
  var phase = step.phase || '';
  var isAnd = /^(and|but)$/i.test(String(step.keyword).trim());

  var a = ['class="step"'];
  if (phase) a.push('data-phase="' + attr(phase) + '"');
  if (isAnd) a.push('data-and="true"');
  if (st) a.push('data-status="' + attr(st) + '"');
  if (st === 'running') a.push('aria-current="true"');

  var right = '';
  if (stDur[k] != null) right += '<span class="step-dur">' + fmtDur(stDur[k]) + '</span>';
  if (step.resource) {
    right += '<span class="res"><span class="res-name">' + esc(step.resource) + '</span>' +
      (step.resourceType ? '<span class="res-type">' + esc(step.resourceType) + '</span>' : '') + '</span>';
  }

  var html = '<div ' + a.join(' ') + '>' +
    '<i class="pip" data-status="' + attr(st) + '"></i>' +
    '<span class="step-kw">' + esc(String(step.keyword).trim()) + '</span>' +
    '<span class="step-text">' + tokenize(step.text) + '</span>' +
    '<span class="step-res">' + right + '</span>';

  if (step.description) {
    html += '<div class="step-desc">' + esc(step.description) + '</div>';
  }
  if (step.docString) {
    html += '<div class="step-attach"><pre class="docstring"' +
      (step.docStringLang ? ' data-lang="' + attr(step.docStringLang) + '"' : '') + '>' +
      esc(step.docString) + '</pre></div>';
  }
  if (step.table && step.table.length) {
    html += '<div class="step-attach">' + table(step.table) + '</div>';
  }
  if (st === 'failed' && stError[k]) {
    html += '<div class="step-attach callout" data-status="failed">' +
      '<span class="callout-title">step failed</span>' +
      '<div class="callout-body">' + esc(stError[k]) + '</div></div>';
  } else if (st === 'undefined') {
    html += '<div class="step-attach callout" data-status="undefined">' +
      '<span class="callout-title">no step definition matches this text</span>' +
      '<div class="callout-body">Run <code>tomato steps</code> to list the available steps.</div></div>';
  }
  return html + '</div>';
}

function table(rows) {
  var head = rows[0] || [];
  var body = rows.slice(1);
  return '<table class="dtable"><tr>' +
    head.map(function (c) { return '<th>' + esc(c) + '</th>'; }).join('') + '</tr>' +
    body.map(function (r) {
      return '<tr>' + r.map(function (c) { return '<td>' + markVars(c) + '</td>'; }).join('') + '</tr>';
    }).join('') + '</table>';
}

function renderExamples(s) {
  return (s.examples || []).map(function (ex) {
    if (!ex.rows || !ex.rows.length) return '';
    return '<div class="examples" style="padding:0 var(--s-5) var(--s-5)">' +
      '<span class="label" style="display:block;margin:var(--s-4) 0 var(--s-3)">Examples' +
      (ex.name ? ' · ' + esc(ex.name) : '') + '</span>' + table(ex.rows) + '</div>';
  }).join('');
}

// ——— config view —————————————————————————————————————————————————————

function renderConfig() {
  if (!cfg) return '<div class="empty"><h3>Loading configuration…</h3></div>';
  if (cfg.error) {
    return '<div class="doc"><div class="feature-head"><span class="feature-kw">CONFIG</span>' +
      '<h1>tomato.yml</h1></div><div class="callout" data-status="failed">' +
      '<span class="callout-title">could not read the config</span>' +
      '<div class="callout-body">' + esc(cfg.error) + '</div></div></div>';
  }

  var pills = '<span class="pill" data-status="' + (cfg.valid ? 'passed' : 'failed') + '">' +
    (cfg.valid ? 'valid' : 'invalid') + '</span>' +
    '<span class="pill">' + (cfg.containers || []).length + ' containers</span>' +
    '<span class="pill">' + (cfg.resources || []).length + ' resources</span>' +
    '<span class="pill">' + (cfg.hookCount || 0) + ' hooks</span>';

  var html = '<div class="doc"><div class="feature-head">' +
    '<span class="feature-kw">CONFIG</span>' +
    '<div class="row" style="justify-content:space-between;align-items:center">' +
      '<h1>' + esc(cfg.path || 'tomato.yml') + '</h1>' +
      '<div class="seg">' +
        '<button class="seg-opt" data-act="cfgMode" data-arg="structured"' +
          (cfgMode === 'structured' ? ' aria-pressed="true"' : '') + '>Structured</button>' +
        '<button class="seg-opt" data-act="cfgMode" data-arg="yaml"' +
          (cfgMode === 'yaml' ? ' aria-pressed="true"' : '') + '>YAML</button>' +
      '</div></div>' +
    '<span class="feature-path">' + esc(cfg.path || '') +
      (cfg.version ? ' · version ' + esc(cfg.version) : '') + '</span>' +
    '<div class="row">' + pills + '</div></div>';

  if (cfgMode === 'yaml') {
    return html + '<pre class="docstring yaml" data-lang="yaml">' + yaml(cfg.content || '') + '</pre></div>';
  }

  var s = cfg.settings || {};
  html += section('Settings', 'settings:', '', kv([
    ['timeout', s.timeout], ['parallel', s.parallel], ['fail_fast', String(!!s.failFast)],
    ['output', s.output], ['reset', s.reset]
  ]));

  if (cfg.app && cfg.app.configured) {
    var a = cfg.app;
    var body = kv([
      ['command', a.command], ['image', a.image], ['port', a.port],
      ['ready', a.ready], ['wait', a.wait]
    ]);
    if (a.env && a.env.length) {
      body += '<span class="label" style="margin-top:4px">env</span><div class="cfg-kv">' +
        a.env.map(function (e) {
          return '<span class="cfg-k">' + esc(e.key) + '</span><span class="cfg-v">' +
            '<span>' + markVars(e.value) + '</span>' +
            (e.resolved && e.resolved !== e.value
              ? '<span class="cfg-resolved">' + esc(e.resolved) + '</span>' : '') +
            '</span>';
        }).join('') + '</div>';
    }
    html += section('App', 'app:', icon('app'), body);
  }

  if ((cfg.containers || []).length) {
    html += section('Containers', 'containers:', '', '<div class="cfg-wrap">' + table(
      [['name', 'image', 'ports', 'wait for']].concat(cfg.containers.map(function (c) {
        return [c.name, c.image || (c.preset ? 'preset: ' + c.preset : ''),
          (c.ports || []).join(', '), c.waitFor || ''];
      }))) + '</div>');
  }

  if ((cfg.resources || []).length) {
    var rowsHtml = cfg.resources.map(function (r) {
      return '<tr><td><span class="cfg-res">' + icon(r.type) +
        '<span class="res"><span class="res-name">' + esc(r.name) + '</span>' +
        '<span class="res-type">' + esc(r.type) + '</span></span></span></td>' +
        '<td>' + esc(r.connectsTo || '') + '</td><td>' + esc(r.options || '') + '</td></tr>';
    }).join('');
    html += section('Resources', 'resources:', '', '<div class="cfg-wrap">' +
      '<table class="dtable"><tr><th>resource</th><th>connects to</th><th>options</th></tr>' +
      rowsHtml + '</table></div>');
  }

  var h = cfg.hooks || {};
  html += section('Hooks', 'hooks:', '', kvEmpty([
    ['before_all', h.beforeAll], ['after_all', h.afterAll],
    ['before_scenario', h.beforeScenario], ['after_scenario', h.afterScenario]
  ]));

  var ft = cfg.features || {};
  html += section('Features', 'features:', '', kv([
    ['paths', (ft.paths || []).join(', ')], ['tags', ft.tags],
    ['matched', features.length + ' features · ' + allScenarios().length + ' scenarios']
  ]));

  return html + '</div>';
}

function section(title, k, ic, body) {
  return '<section class="cfg-section"><div class="cfg-head">' + (ic || '') +
    '<span class="cfg-title">' + esc(title) + '</span>' +
    '<span class="cfg-key">' + esc(k) + '</span></div>' + body + '</section>';
}
function kv(pairs) {
  var out = pairs.filter(function (p) { return p[1] != null && p[1] !== ''; }).map(function (p) {
    return '<span class="cfg-k">' + esc(p[0]) + '</span><span class="cfg-v">' + esc(p[1]) + '</span>';
  }).join('');
  return '<div class="cfg-kv">' + out + '</div>';
}
function kvEmpty(pairs) {
  var out = pairs.map(function (p) {
    var v = p[1];
    var empty = !v || (Array.isArray(v) && !v.length);
    return '<span class="cfg-k">' + esc(p[0]) + '</span>' +
      '<span class="cfg-v"' + (empty ? ' data-empty="true"' : '') + '>' +
      esc(empty ? 'none' : (Array.isArray(v) ? v.join(', ') : v)) + '</span>';
  }).join('');
  return '<div class="cfg-kv">' + out + '</div>';
}

// Minimal YAML highlighter: keys, strings, numbers, booleans, comments.
function yaml(src) {
  return String(src).split('\n').map(function (line) {
    var m = line.match(/^(\s*)(#.*)$/);
    if (m) return esc(m[1]) + '<span class="y-c">' + esc(m[2]) + '</span>';
    var out = '', rest = line;
    var k = rest.match(/^(\s*-?\s*)([A-Za-z_][\w.-]*)(\s*:)/);
    if (k) {
      out += esc(k[1]) + '<span class="y-k">' + esc(k[2] + k[3]) + '</span>';
      rest = rest.slice(k[0].length);
    }
    var c = rest.match(/\s#.*$/);
    var tail = '';
    if (c) { tail = '<span class="y-c">' + esc(c[0]) + '</span>'; rest = rest.slice(0, c.index); }
    return out + yamlVal(rest) + tail;
  }).join('\n');
}
function yamlVal(v) {
  if (!v) return '';
  var t = v.trim();
  if (!t) return esc(v);
  var lead = v.slice(0, v.indexOf(t));
  var cls = /^(true|false|null|~)$/i.test(t) ? 'y-b'
    : /^-?\d+(\.\d+)?([a-z]{1,2})?$/i.test(t) ? 'y-n'
    : 'y-s';
  return esc(lead) + '<span class="' + cls + '">' + markVars(t) + '</span>';
}

// ——— inspector: flow / topology / runs ———————————————————————————————

// Lanes and messages derived from the selected scenario's steps. This is what
// the scenario *does* (resource per step, in order) rather than bytes captured
// on the wire — tomato does not record payloads yet.
function flowModel() {
  var f = features.find(function (x) { return x.filePath === selectedFile; });
  if (!f) return null;
  var list = visibleScenarios(f).filter(function (s) { return s.kind !== 'background'; });
  var s = list[selectedScenario] || list[0];
  if (!s) return null;

  var lanes = [{ name: 'tomato', type: 'runner', icon: 'tomato', runner: true }];
  var laneOf = {};
  (s.steps || []).forEach(function (step) {
    if (!step.resource || laneOf[step.resource] != null) return;
    laneOf[step.resource] = lanes.length;
    lanes.push({ name: step.resource, type: step.resourceType || '', icon: step.resourceType });
  });

  var msgs = (s.steps || []).map(function (step, i) {
    var k = key(s.name, i);
    var lane = step.resource != null ? laneOf[step.resource] : null;
    var words = String(step.text).replace(/"[^"]*"/g, ' ').trim().split(/\s+/);
    return {
      n: i + 1, idx: i, scenario: s.name,
      kind: lane == null ? 'note' : 'call',
      dir: lane == null ? 'none' : 'right',
      a: 1, b: lane == null ? 2 : lane + 2,
      status: stStatus[k] || '',
      phase: step.phase || '', kw: String(step.keyword).trim(),
      // Sequence labels are nowrap by design, and the inspector is narrow —
      // clip rather than let a long step text run out of the pane.
      label: step.resource
        ? clip((words[0] || 'step') + ' · ' + step.resource, 24)
        : clip(step.text, 24),
      verb: (words[0] || 'STEP').toUpperCase(),
      target: step.resource ? step.resource + (step.resourceType ? ' · ' + step.resourceType : '') : 'scenario',
      text: step.text,
      body: step.docString || (step.table && step.table.length ? step.table.map(function (r) { return '| ' + r.join(' | ') + ' |'; }).join('\n') : ''),
      dur: stDur[k], err: stError[k]
    };
  });
  return { scenario: s, lanes: lanes, msgs: msgs };
}

function renderSide() {
  var side = el('side');
  el('flowCount').textContent = String((flowModel() || { msgs: [] }).msgs.length);
  document.querySelectorAll('[data-tab]').forEach(function (b) {
    if (b.getAttribute('data-tab') === sideTab) b.setAttribute('aria-selected', 'true');
    else b.removeAttribute('aria-selected');
  });
  renderTransport();
  if (sideTab === 'runs') { side.innerHTML = renderRuns(); return; }
  var m = flowModel();
  if (!m) {
    side.innerHTML = '<div class="empty"><h3>No scenario selected</h3>' +
      '<p>Pick a scenario to see what it talks to.</p></div>';
    return;
  }
  side.innerHTML = sideTab === 'topology' ? renderTopology(m) : renderFlow(m);
}

function renderFlow(m) {
  var heads = m.lanes.map(function (l) {
    return '<div class="seq-head"' + (l.runner ? ' data-runner="true"' : '') + '>' +
      icon(l.icon) + '<span class="node-name">' + esc(l.name) + '</span>' +
      '<span class="node-type">' + esc(l.type) + '</span></div>';
  }).join('');
  var lines = m.lanes.map(function () { return '<i class="seq-line"></i>'; }).join('');

  var body = m.msgs.map(function (x) {
    var state = cursor < 0 ? '' : (x.idx < cursor ? 'past' : x.idx === cursor ? 'current' : 'future');
    var a = ['class="msg"', 'data-kind="' + attr(x.kind) + '"', 'data-dir="' + attr(x.dir) + '"',
      'data-act="hot"', 'data-arg="' + x.idx + '"'];
    if (state) a.push('data-state="' + state + '"');
    if (x.status) a.push('data-status="' + attr(x.status) + '"');
    if (hot === x.idx) a.push('data-hot="true"');
    if (x.n >= m.msgs.length - 1) a.push('data-up="true"');
    return '<div ' + a.join(' ') + '>' +
      '<div class="msg-arrow" data-from="' + x.a + '" data-to="' + x.b + '">' +
      '<span class="msg-label"><span class="msg-num">' + x.n + '</span>' +
      '<span title="' + attr(x.kw + ' ' + x.text) + '">' + esc(x.label) + '</span></span>' +
      '<span class="msg-line"></span></div>' +
      (hot === x.idx ? '<div class="peek-anchor">' + peek(x) + '</div>' : '') +
      '</div>';
  }).join('');

  return '<div class="seq" style="--lanes:' + m.lanes.length + '">' +
    '<div class="seq-heads">' + heads + '</div>' +
    '<div class="seq-body"><div class="seq-lines">' + lines + '</div>' + body + '</div>' +
    '<p class="muted" style="font-size:11px;margin-top:8px">Derived from the scenario\'s steps. ' +
    'Hover a line to see the step and its payload.</p></div>';
}

function peek(x) {
  var kvs = [['step', x.kw + ' ' + x.text]];
  if (x.dur != null) kvs.push(['took', fmtDur(x.dur)]);
  return '<div class="peek"' + (x.status ? ' data-status="' + attr(x.status) + '"' : '') + '>' +
    '<div class="peek-head"><span class="msg-num">' + x.n + '</span>' +
    '<span class="peek-verb">' + esc(x.verb) + '</span>' +
    '<span class="peek-target">' + esc(x.target) + '</span></div>' +
    '<div class="peek-kv">' + kvs.map(function (p) {
      return '<span class="peek-k">' + esc(p[0]) + '</span><span class="peek-v">' + esc(p[1]) + '</span>';
    }).join('') + '</div>' +
    (x.body ? '<pre class="docstring peek-body">' + esc(x.body) + '</pre>' : '') +
    '<div class="peek-foot"><i class="pip" data-status="' + attr(x.status) + '"></i><span>' +
    esc(x.err || (x.status ? x.status : 'not run yet')) + '</span></div></div>';
}

// Topology as SVG. The node list is the server's TopologyJSON — the app plus
// every resource tomato.yml defines, not just the ones this scenario touches —
// while the numbered edges come from the scenario's steps. A resource with a
// `target` is one tomato calls (an http/grpc/websocket client); the rest are
// dependencies the app uses, so they sit further out.
function renderTopology(m) {
  var all = (topo && topo.resources) || [];
  if (!all.length) {
    // No config to draw: fall back to whatever the scenario referenced.
    all = m.lanes.slice(1).map(function (l) { return { name: l.name, type: l.type }; });
  }
  var res = all.slice().sort(function (x, y) {
    return (x.target ? 0 : 1) - (y.target ? 0 : 1) || x.name.localeCompare(y.name);
  });

  var NH = 38, GAP = 14, TOP = 34;
  var nEdge = m.msgs.filter(function (x) { return x.kind !== 'note'; }).length;
  var H = Math.max(230, TOP + res.length * (NH + GAP) + 56, 120 + nEdge * 12);
  var appY = H / 2 - 24;

  var nodes = '', edges = '';
  nodes += '<text class="tcol" x="45" y="14">runner</text>' +
    '<text class="tcol" x="165" y="14">app</text>' +
    '<text class="tcol" x="285" y="14">resources</text>';

  var curMsg = cursor >= 0 ? m.msgs[cursor] : null;
  var activeRes = curMsg ? curMsg.target.split(' · ')[0] : null;

  nodes += '<g class="tnode" data-kind="runner"' + (curMsg ? ' data-active="true"' : '') + '>' +
    '<rect x="6" y="' + appY + '" width="78" height="48" rx="6"></rect>' +
    '<use href="#ri-tomato" x="13" y="' + (appY + 17) + '" width="14" height="14"></use>' +
    '<text class="tnode-name" x="32" y="' + (appY + 22.5) + '">tomato</text>' +
    '<text class="tnode-type" x="32" y="' + (appY + 33.5) + '">runner</text></g>';

  var appPort = topo && topo.appPort ? ':' + topo.appPort : 'under test';
  nodes += '<g class="tnode" data-kind="app">' +
    '<rect x="126" y="' + appY + '" width="78" height="48" rx="6"></rect>' +
    '<use href="#ri-app" x="133" y="' + (appY + 17) + '" width="14" height="14"></use>' +
    '<text class="tnode-name" x="152" y="' + (appY + 22.5) + '">app</text>' +
    '<text class="tnode-type" x="152" y="' + (appY + 33.5) + '">' + esc(appPort) + '</text></g>';

  res.forEach(function (r, i) {
    var y = TOP + i * (NH + GAP);
    var failed = m.msgs.some(function (x) {
      return x.status === 'failed' && x.target.split(' · ')[0] === r.name;
    });
    // A client pointed at another resource is talking to a mock tomato serves.
    var mock = !!(r.target && r.target !== 'app');
    nodes += '<g class="tnode"' +
      (activeRes === r.name ? ' data-active="true"' : '') +
      (failed ? ' data-status="failed"' : '') +
      (mock ? ' data-mock="true"' : '') + '>' +
      '<rect x="246" y="' + y + '" width="78" height="' + NH + '" rx="6"></rect>' +
      '<use href="#ri-' + (ICON_ALIAS[String(r.type).toLowerCase()] || 'app') + '" x="253" y="' + (y + 12) + '" width="14" height="14"></use>' +
      '<text class="tnode-name" x="272" y="' + (y + 17.5) + '">' + esc(r.name) + '</text>' +
      '<text class="tnode-type" x="272" y="' + (y + 28.5) + '">' + esc(r.type) + '</text></g>';
  });

  // Each step gets its own channel above or below the app box. Keying the
  // channel on the resource instead would draw identical overlapping edges
  // when a scenario hits the same resource twice, hiding all but the last —
  // and the badge is the step number, so they must stay distinct.
  var drawable = m.msgs.map(function (x) {
    if (x.kind === 'note') return null;
    var name = x.target.split(' · ')[0];
    var ri = res.findIndex(function (r) { return r.name === name; });
    if (ri < 0) return null;
    return { x: x, ry: TOP + ri * (NH + GAP) + NH / 2 };
  }).filter(Boolean);

  var up = drawable.filter(function (d) { return d.ry < appY + 24; });
  var dn = drawable.filter(function (d) { return d.ry >= appY + 24; });
  up.forEach(function (d, i) { d.ch = appY - 10 - (up.length - 1 - i) * 12; });
  dn.forEach(function (d, i) { d.ch = appY + 58 + i * 12; });

  drawable.forEach(function (d) {
    var x = d.x, ry = d.ry, ch = d.ch, sy = appY + 24;
    var path = 'M84,' + sy +
      ' C118,' + sy + ' 128,' + ch + ' 165,' + ch +
      ' S228,' + ry + ' 244,' + ry;
    var state = cursor < 0 ? '' : (x.idx < cursor ? 'past' : x.idx === cursor ? 'current' : 'future');
    var a2 = ['class="tedge"', 'data-act="hot"', 'data-arg="' + x.idx + '"'];
    if (state) a2.push('data-state="' + state + '"');
    if (x.status) a2.push('data-status="' + attr(x.status) + '"');
    if (hot === x.idx) a2.push('data-hot="true"');
    edges += '<g ' + a2.join(' ') + '>' +
      '<path class="tedge-hit" d="' + path + '"></path>' +
      '<path class="tedge-line" pathLength="1" d="' + path + '"></path>' +
      '<g class="tbadge"><circle cx="165" cy="' + ch + '" r="7"></circle>' +
      '<text x="165" y="' + ch + '">' + x.n + '</text></g></g>';
  });

  var dockMsg = hot != null ? m.msgs[hot] : curMsg;

  return '<div class="tp">' +
    '<svg class="tsvg" viewBox="0 0 330 ' + H + '" role="img" aria-label="Scenario topology, numbered by step">' +
    '<defs>' +
      marker('th-muted', 'th--muted') + marker('th-cur', 'th--cur') + marker('th-fail', 'th--fail') +
    '</defs>' + nodes + edges + '</svg>' +
    (dockMsg ? '<div class="tp-dock">' + peek(dockMsg) + '</div>' : '') +
    '<div class="tp-legend"><span class="tp-key">tomato acts or checks</span>' +
    '<span class="tp-key" data-inferred="true">mock tomato serves</span></div></div>';
}

function marker(id, cls) {
  return '<marker id="' + id + '" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto">' +
    '<path class="th ' + cls + '" d="M0,0 L10,5 L0,10 z"></path></marker>';
}

function renderRuns() {
  if (!runs.length) {
    return '<div class="empty"><h3>No runs yet</h3><p>Runs are written to <code>.tomato/runs</code>. ' +
      'Press <span class="kbd">R</span> to start one.</p></div>';
  }
  return '<div class="runs">' + runs.map(function (r) {
    var id = String(r.name).split('_').pop();
    var sel = openRun === r.name;
    var logs = (r.logs || []).map(function (l) {
      return '<button class="log-link" data-act="log" data-arg="' + attr(r.name) + '" data-arg2="' + attr(l.name) + '">' +
        esc(l.name) + ' <small>' + fmtBytes(l.size) + '</small></button>';
    }).join('');
    return '<div class="run" data-act="run" data-arg="' + attr(r.name) + '"' +
      (sel ? ' aria-selected="true"' : '') + '>' +
      '<i class="pip" data-status="' + attr(r.status || '') + '"></i>' +
      '<div><div class="run-id">' + esc(id) + '</div><div class="run-when">' +
      esc(when(r.timestamp)) + '</div></div>' +
      '<span class="tally"></span>' +
      (sel && logs ? '<div class="run-logs">' + logs + '</div>' : '') + '</div>';
  }).join('') + '</div>';
}

function when(ts) {
  var d = new Date(ts);
  if (isNaN(d.getTime())) return String(ts || '');
  var now = new Date();
  var same = d.toDateString() === now.toDateString();
  var y = new Date(now.getTime() - 86400000).toDateString() === d.toDateString();
  var hh = d.toTimeString().slice(0, 8);
  return (same ? 'today ' : y ? 'yesterday ' : d.toISOString().slice(0, 10) + ' ') + hh;
}

// ——— transport ———————————————————————————————————————————————————————

function renderTransport() {
  var host = el('transport');
  if (sideTab === 'runs') { host.innerHTML = ''; return; }
  var m = flowModel();
  if (!m || !m.msgs.length) { host.innerHTML = ''; return; }

  var cur = cursor >= 0 ? m.msgs[cursor] : null;
  var ticks = m.msgs.map(function (x) {
    var state = cursor < 0 ? '' : (x.idx < cursor ? 'past' : x.idx === cursor ? 'current' : 'future');
    return '<button class="scrub-tick" data-act="seek" data-arg="' + x.idx + '"' +
      (state ? ' data-state="' + state + '"' : '') +
      (x.status ? ' data-status="' + attr(x.status) + '"' : '') +
      ' title="' + attr(x.kw + ' ' + x.text) + '">' + x.n + '</button>';
  }).join('');

  host.innerHTML = '<div class="transport"><div class="transport-row">' +
    '<button class="btn btn--ghost btn--sm btn--icon" data-act="prev" title="Previous step  ←"' +
      (cursor < 0 ? ' disabled' : '') + '><i class="ico-step" data-dir="back"></i></button>' +
    '<button class="btn btn--sm btn--icon" data-act="play" title="Play / pause  space"' +
      (playTimer ? ' aria-pressed="true"' : '') + '><i class="' + (playTimer ? 'ico-pause' : 'ico-play') + '"></i></button>' +
    '<button class="btn btn--ghost btn--sm btn--icon" data-act="next" title="Next step  →"' +
      (cursor >= m.msgs.length - 1 ? ' disabled' : '') + '><i class="ico-step"></i></button>' +
    '<button class="btn btn--ghost btn--sm btn--icon" data-act="replay" title="Replay from step 1"><i class="ico-replay"></i></button>' +
    '<div class="transport-cap">' +
      '<span class="transport-kw" data-phase="' + attr(cur ? cur.phase : '') + '">' + esc(cur ? cur.kw : '') + '</span>' +
      '<span class="transport-text">' + esc(cur ? cur.text : 'Press play to walk through the scenario') + '</span>' +
    '</div>' +
    '<span class="transport-pos">' + (cursor + 1) + '/' + m.msgs.length + '</span>' +
    '</div><div class="scrub">' + ticks + '</div></div>';
}

function seek(i) {
  var m = flowModel();
  var last = m ? m.msgs.length - 1 : -1;
  cursor = Math.max(-1, Math.min(last, i));
  renderSide(); renderDoc();
}
function stopPlay() { if (playTimer) { clearInterval(playTimer); playTimer = null; } }
function play(from) {
  stopPlay();
  var m = flowModel();
  if (!m || !m.msgs.length) return;
  var last = m.msgs.length - 1;
  var i = from != null ? from : (cursor >= last ? -1 : cursor);
  var tick = function () {
    i++;
    cursor = Math.min(last, i);
    if (i >= last) stopPlay();
    renderSide(); renderDoc();
  };
  tick();
  if (i < last) playTimer = setInterval(tick, 1400);
  renderTransport();
}

// ——— console ——————————————————————————————————————————————————————————

// A black rectangle reads as broken, so say what the pane is for until the
// first run writes to it.
function consoleHint() {
  var c = el('console');
  if (activeLog || running || runStartedAt || c.childElementCount) return;
  c.innerHTML = '<div class="line"><span class="line-time"></span><span class="c-dim">' +
    'Run output appears here. Press R to run everything, or r to run the focused scenario.' +
    '</span></div>';
}

function renderConsoleTabs() {
  var tabs = ['<button class="tab" data-act="ctab" data-arg=""' +
    (activeLog ? '' : ' aria-selected="true"') + '>Output</button>'];
  logTabs.forEach(function (t) {
    var on = activeLog && activeLog.run === t.run && activeLog.name === t.name;
    tabs.push('<button class="tab" data-act="ctab" data-arg="' + attr(t.run + '/' + t.name) + '"' +
      (on ? ' aria-selected="true"' : '') + '>' + esc(t.name) + '</button>');
  });
  tabs.push('<span style="flex:1"></span>');
  if (runId) tabs.push('<span class="live" data-state="' + (running ? '' : 'offline') +
    '" style="align-self:center">run ' + esc(runId) + '</span>');
  el('consoleTabs').innerHTML = tabs.join('');
}

function appendLine(html, status) {
  var c = el('console');
  if (activeLog) return; // viewing a log, don't scroll the run output
  if (c.firstElementChild && c.firstElementChild.querySelector('.c-dim') &&
      c.childElementCount === 1 && !runStartedAt) {
    c.innerHTML = '';
  }
  var stuck = c.scrollTop + c.clientHeight >= c.scrollHeight - 8;
  var d = document.createElement('div');
  d.className = 'line';
  if (status) d.setAttribute('data-status', status);
  d.innerHTML = '<span class="line-time">' + fmtClock(Date.now() - runStartedAt) + '</span><span>' +
    (html || '') + '</span>';
  c.appendChild(d);
  while (c.childElementCount > 4000) c.removeChild(c.firstChild);
  if (stuck) c.scrollTop = c.scrollHeight;
}

function showLog(run, name) {
  if (!logTabs.some(function (t) { return t.run === run && t.name === name; })) {
    logTabs.push({ run: run, name: name });
    if (logTabs.length > 6) logTabs.shift();
  }
  activeLog = { run: run, name: name };
  renderConsoleTabs();
  var c = el('console');
  c.innerHTML = '<div class="line"><span class="line-time"></span><span class="c-dim">loading…</span></div>';
  fetch('/api/runs/' + encodeURIComponent(run) + '/logs/' + encodeURIComponent(name))
    .then(function (r) { return r.text(); })
    .then(function (txt) {
      c.innerHTML = txt.split('\n').map(function (l) {
        return '<div class="line"><span class="line-time"></span><span>' + ansi(l) + '</span></div>';
      }).join('');
    })
    .catch(function (e) {
      c.innerHTML = '<div class="line" data-status="failed"><span class="line-time"></span>' +
        '<span class="c-red">' + esc(String(e)) + '</span></div>';
    });
}

// ANSI -> the design system's .c-* classes (never inline colour).
var ANSI_CLASS = {
  '1': 'c-bold', '2': 'c-dim', '30': 'c-dim', '90': 'c-dim',
  '31': 'c-red', '91': 'c-red', '32': 'c-green', '92': 'c-green',
  '33': 'c-yellow', '93': 'c-yellow', '34': 'c-blue', '94': 'c-blue',
  '35': 'c-magenta', '95': 'c-magenta', '36': 'c-cyan', '96': 'c-cyan'
};
function ansi(line) {
  var out = '', open = 0, i = 0;
  var re = /\x1b\[([0-9;]*)m|\[([0-9;]*)m/g, m;
  while ((m = re.exec(line)) !== null) {
    out += esc(line.slice(i, m.index));
    i = m.index + m[0].length;
    var codes = String(m[1] || m[2] || '').split(';').filter(Boolean);
    if (!codes.length || codes.indexOf('0') >= 0) {
      while (open > 0) { out += '</span>'; open--; }
      continue;
    }
    var cls = codes.map(function (c) { return ANSI_CLASS[c]; }).filter(Boolean);
    if (cls.length) { out += '<span class="' + cls.join(' ') + '">'; open++; }
  }
  out += esc(line.slice(i));
  while (open > 0) { out += '</span>'; open--; }
  return out;
}

// ——— actions ——————————————————————————————————————————————————————————

function selectFile(path, quiet) {
  selectedFile = path;
  selectedScenario = 0;
  view = 'feature';
  activeLog = null;
  cursor = -1;
  stopPlay();
  if (!quiet) renderAll();
}

function post(url) {
  return fetch(url, { method: 'POST' }).catch(function () {});
}
function runAll() { if (!running) post('/api/run'); }
function runScenario(name) { if (!running) post('/api/run?scenario=' + encodeURIComponent(name)); }
function runFailed() {
  var names = Object.keys(scStatus).filter(function (n) { return scStatus[n] === 'failed'; });
  if (names.length === 1) runScenario(names[0]);
  else if (names.length) runAll();
}
function stopRun() { if (running) post('/api/stop'); }

function moveScenario(d) {
  var f = features.find(function (x) { return x.filePath === selectedFile; });
  if (!f) return;
  var list = visibleScenarios(f).filter(function (s) { return s.kind !== 'background'; });
  if (!list.length) return;
  selectedScenario = Math.max(0, Math.min(list.length - 1, (selectedScenario || 0) + d));
  cursor = -1; stopPlay();
  renderAll();
}
function focusedScenarioName() {
  var f = features.find(function (x) { return x.filePath === selectedFile; });
  if (!f) return null;
  var list = visibleScenarios(f).filter(function (s) { return s.kind !== 'background'; });
  var s = list[selectedScenario];
  return s ? s.name : null;
}

function loadConfig() {
  fetch('/api/config').then(function (r) { return r.json(); }).then(function (j) {
    cfg = j;
    if (view === 'config') renderDoc();
    renderTree();
  }).catch(function (e) { cfg = { error: String(e) }; });
}

// ——— render ———————————————————————————————————————————————————————————

function renderAll() { renderTop(); renderTree(); renderDoc(); renderSide(); renderConsoleTabs(); consoleHint(); }

// ——— wiring ———————————————————————————————————————————————————————————

document.addEventListener('click', function (e) {
  var t = e.target.closest('[data-act],[data-toggle],[data-tab]');
  if (!t) return;

  var toggle = t.getAttribute('data-toggle');
  if (toggle) {
    var app = el('app'), k = 'data-' + toggle;
    app.setAttribute(k, app.getAttribute(k) === 'closed' ? 'open' : 'closed');
    return;
  }
  var tab = t.getAttribute('data-tab');
  if (tab) { sideTab = tab; el('app').setAttribute('data-side', 'open'); renderSide(); return; }

  var act = t.getAttribute('data-act');
  var arg = t.getAttribute('data-arg');
  var arg2 = t.getAttribute('data-arg2');

  switch (act) {
    case 'config': view = 'config'; activeLog = null; renderAll(); break;
    case 'dir':
      expandedDirs[arg] = expandedDirs[arg] === false;
      renderTree();
      break;
    case 'file': selectFile(arg); break;
    case 'scenario':
      selectFile(arg, true); selectedScenario = parseInt(arg2, 10) || 0; renderAll();
      break;
    case 'toggleScenario':
      // Invert what is on screen, not the stored value: a card with nothing
      // stored yet is showing a computed default (Background starts closed, a
      // passing scenario folds itself away), and inverting `undefined` would
      // write back the state it is already in — a dead first click.
      toggleCard(t, arg);
      break;
    case 'runScenario': runScenario(arg); break;
    case 'tag':
      filter = (filter.toLowerCase() === '@' + arg.toLowerCase()) ? '' : '@' + arg;
      el('filter').value = filter;
      renderAll();
      break;
    case 'cfgMode': cfgMode = arg; renderDoc(); break;
    case 'hot': hot = parseInt(arg, 10); renderSide(); break;
    case 'seek': seek(parseInt(arg, 10)); break;
    case 'prev': seek(cursor - 1); break;
    case 'next': seek(cursor + 1); break;
    case 'play': playTimer ? (stopPlay(), renderTransport()) : play(); break;
    case 'replay': play(-1); break;
    case 'run':
      openRun = openRun === arg ? null : arg;
      renderSide();
      break;
    case 'log': showLog(arg, arg2); break;
    case 'ctab':
      if (!arg) { activeLog = null; renderConsoleTabs(); el('console').innerHTML = ''; }
      else { var p = arg.split('/'); showLog(p[0], p.slice(1).join('/')); }
      break;
  }
});

// toggleCard flips a scenario or Background card from whatever it currently
// shows, then re-renders.
function toggleCard(node, name) {
  var card = node.closest ? node.closest('.scenario') : null;
  var showing = card && card.getAttribute('data-collapsed') === 'true';
  collapsedScenarios[name] = !showing;
  renderDoc();
}

document.addEventListener('mouseover', function (e) {
  var t = e.target.closest('.msg[data-arg],.tedge[data-arg]');
  if (!t) return;
  var i = parseInt(t.getAttribute('data-arg'), 10);
  if (hot !== i) { hot = i; renderSide(); }
});
document.addEventListener('mouseleave', function (e) {
  if (e.target.id === 'side' && hot != null) { hot = null; renderSide(); }
}, true);

el('filter').addEventListener('input', function (e) {
  filter = e.target.value.trim();
  renderAll();
});

el('runAllBtn').addEventListener('click', runAll);
el('stopBtn').addEventListener('click', stopRun);
el('runFailedBtn').addEventListener('click', runFailed);

document.addEventListener('keydown', function (e) {
  var t = e.target;

  // A head is a button, so Enter and Space must work on it.
  if ((e.key === 'Enter' || e.key === ' ') && t && t.classList &&
      t.classList.contains('scenario-head')) {
    e.preventDefault();
    toggleCard(t, t.getAttribute('data-arg'));
    return;
  }

  var typing = t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable);

  if (e.key === 'Escape') {
    if (typing) { t.blur(); }
    if (filter) { filter = ''; el('filter').value = ''; renderAll(); }
    else if (running) stopRun();
    return;
  }
  if (typing || e.metaKey || e.ctrlKey || e.altKey) return;

  switch (e.key) {
    case '/': e.preventDefault(); el('filter').focus(); el('filter').select(); break;
    case 'j': moveScenario(1); break;
    case 'k': moveScenario(-1); break;
    case 'r': var n = focusedScenarioName(); if (n) runScenario(n); break;
    case 'R': runAll(); break;
    case '[': var a = el('app'); a.setAttribute('data-tree', a.getAttribute('data-tree') === 'closed' ? 'open' : 'closed'); break;
    case ']': var b = el('app'); b.setAttribute('data-side', b.getAttribute('data-side') === 'closed' ? 'open' : 'closed'); break;
    case 'ArrowRight': if (sideTab !== 'runs') { e.preventDefault(); seek(cursor + 1); } break;
    case 'ArrowLeft': if (sideTab !== 'runs') { e.preventDefault(); seek(cursor - 1); } break;
    case ' ': if (sideTab !== 'runs') { e.preventDefault(); playTimer ? (stopPlay(), renderTransport()) : play(); } break;
  }
});

// ——— pane resizing ———————————————————————————————————————————————————
//
// Widths and heights a person sets are per-viewer conveniences, so they live in
// localStorage and every access is guarded: a private window or blocked site
// data makes these throw rather than return empty.
function remember(k, v) {
  try { localStorage.setItem('tomato-ui.' + k, String(v)); } catch (e) { /* not fatal */ }
}
function recall(k) {
  try { return localStorage.getItem('tomato-ui.' + k); } catch (e) { return null; }
}

var SIDE_MIN = 240, SIDE_DEFAULT = 340, CONSOLE_MIN = 80;

function setSideWidth(px) {
  var max = Math.max(SIDE_MIN, window.innerWidth - 420);
  var w = Math.round(Math.min(Math.max(px, SIDE_MIN), max));
  el('app').style.setProperty('--side-open-w', w + 'px');
  remember('sideWidth', w);
  return w;
}

(function () {
  var handle = el('sideResize'), app = el('app');
  var dragging = false, startX = 0, startW = 0;

  var saved = parseInt(recall('sideWidth'), 10);
  if (saved > 0) setSideWidth(saved);

  handle.addEventListener('mousedown', function (e) {
    // Dragging the edge of a collapsed rail would be confusing; open it first.
    if (app.getAttribute('data-side') === 'closed') return;
    dragging = true;
    startX = e.clientX;
    startW = document.querySelector('.pane--side').offsetWidth;
    handle.setAttribute('data-dragging', 'true');
    app.setAttribute('data-resizing', 'true');
    e.preventDefault();
  });

  handle.addEventListener('dblclick', function () { setSideWidth(SIDE_DEFAULT); });

  document.addEventListener('mousemove', function (e) {
    if (!dragging) return;
    // The handle is on the pane's left edge, so moving left widens it.
    setSideWidth(startW - (e.clientX - startX));
  });

  document.addEventListener('mouseup', function () {
    if (!dragging) return;
    dragging = false;
    handle.removeAttribute('data-dragging');
    app.removeAttribute('data-resizing');
  });

  // Keep the pane inside the window when the window itself shrinks.
  window.addEventListener('resize', function () {
    var cur = parseInt(recall('sideWidth'), 10);
    if (cur > 0) setSideWidth(cur);
  });
})();

// console height
(function () {
  var handle = el('consoleResize'), con = el('console');
  var dragging = false, startY = 0, startH = 0;

  var saved = parseInt(recall('consoleHeight'), 10);
  if (saved > 0) con.style.height = saved + 'px';

  handle.addEventListener('mousedown', function (e) {
    dragging = true; startY = e.clientY; startH = con.offsetHeight;
    handle.setAttribute('data-dragging', 'true');
    el('app').setAttribute('data-resizing', 'true');
    e.preventDefault();
  });
  document.addEventListener('mousemove', function (e) {
    if (!dragging) return;
    var h = Math.round(Math.min(Math.max(startH + (startY - e.clientY), CONSOLE_MIN),
      window.innerHeight * 0.7));
    con.style.height = h + 'px';
    remember('consoleHeight', h);
  });
  document.addEventListener('mouseup', function () {
    if (!dragging) return;
    dragging = false;
    handle.removeAttribute('data-dragging');
    el('app').removeAttribute('data-resizing');
  });
})();

loadConfig();
connect();
renderAll();
