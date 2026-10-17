import { useEffect, useRef, useState } from 'react'
import { iconId } from '../lib/icons'

// Roles when the server sent no topology (an invalid config): derived from
// the step types the flow model saw, the way the server would classify them.
const CLIENT_TYPES = new Set(['http', 'http-client', 'grpc', 'grpc-client', 'websocket', 'websocket-client', 'browser'])
const MOCK_TYPES = new Set(['http-server', 'grpc-server', 'websocket-server', 'aws'])
const roleOf = (type) => (CLIENT_TYPES.has(type) ? 'client' : MOCK_TYPES.has(type) ? 'mock' : type === 'shell' ? 'host' : 'service')

// Geometry in CSS pixels. The drawing is laid out at this size rather than
// scaled to the pane; scaling the whole picture made a sixteen-resource
// suite unreadable. A pane too narrow for the columns gets the stack instead.
const PAD = 12; const TOP = 30
const RUNNER_W = 96; const CLIENT_W = 128; const APP_W = 120; const APP_H = 48
const NODE_W = 144; const NODE_H = 38; const ROW_GAP = 8
const GROUP_PAD = 8; const GROUP_TITLE = 34; const GROUP_GAP = 14
const GROUP_W = NODE_W + GROUP_PAD * 2
// The gap between columns, and the narrower one tried before giving up on
// columns and stacking instead.
const COL_GAP = 56; const COL_GAP_TIGHT = 28
const CH_NAME = 7.25; const CH_SUB = 6.1 // px per character of the mono labels, for truncation

function fit(text, px, ch) {
  const s = String(text ?? '')
  const max = Math.max(1, Math.floor(px / ch))
  return s.length <= max ? s : s.slice(0, max - 1) + '…'
}

function Marker({ id, cls }) {
  return (
    <marker id={id} viewBox="0 0 10 10" refX="8" refY="5" markerWidth="8" markerHeight="8" markerUnits="userSpaceOnUse" orient="auto">
      <path className={`th ${cls}`} d="M0,0 L10,5 L0,10 z" />
    </marker>
  )
}

function Node({ box, kind, name, sub, icon, active, status, used, mock }) {
  const { x, y, w, h } = box
  const nameT = fit(name, w - 38, CH_NAME)
  const subT = fit(sub, w - 38, CH_SUB)
  return (
    <g
      className="tnode"
      data-kind={kind}
      data-active={active ? 'true' : undefined}
      data-status={status || undefined}
      data-used={used === false ? 'false' : undefined}
      data-mock={mock ? 'true' : undefined}
    >
      <title>{sub ? `${name} · ${sub}` : name}</title>
      <rect x={x} y={y} width={w} height={h} rx="6" />
      <use href={`#ri-${icon}`} x={x + 10} y={y + h / 2 - 7} width="14" height="14" />
      <text className="tnode-name" x={x + 31} y={y + h / 2 - 2}>{nameT}</text>
      {subT && <text className="tnode-type" x={x + 31} y={y + h / 2 + 10}>{subT}</text>}
    </g>
  )
}

// A group is a container tomato starts, with the resources it backs inside,
// or one of the standing groups: what tomato serves itself, what lives
// outside tomato's control, and the host.
function Group({ g, box }) {
  const { x, y, w, h } = box
  return (
    <g className="tgroup" data-kind={g.kind} data-used={g.used ? undefined : 'false'}>
      <title>{g.sub ? `${g.title} · ${g.sub}` : g.title}</title>
      <rect x={x} y={y} width={w} height={h} rx="8" />
      <use href={`#ri-${g.icon}`} x={x + GROUP_PAD + 1} y={y + 8} width="12" height="12" />
      <text className="tgroup-title" x={x + GROUP_PAD + 18} y={y + 18}>{fit(g.title, w - GROUP_PAD * 2 - 18, CH_NAME)}</text>
      {g.sub && <text className="tgroup-sub" x={x + GROUP_PAD + 18} y={y + 29}>{fit(g.sub, w - GROUP_PAD * 2 - 18, CH_SUB)}</text>}
    </g>
  )
}

const stackH = (n, h) => (n ? n * h + (n - 1) * ROW_GAP : 0)
const groupH = (g) => GROUP_PAD * 2 + GROUP_TITLE + stackH(g.rows.length, NODE_H)

// Columns, left to right: tomato | clients | app | services. Lines run left
// to right; one that skips a column climbs to a channel above or below the
// app's row, runs flat past it and drops to its target.
function layoutColumns({ clients, app, externals, services }, gap = COL_GAP) {
  const boxes = new Map(); const labels = []
  let x = PAD
  const col = { runner: x }
  x += RUNNER_W + gap
  const wide = clients.length > 0
  if (wide) { col.client = x; x += CLIENT_W + gap }
  const hasMid = Boolean(app) || externals.length > 0
  if (hasMid) { col.app = x; x += APP_W + gap }
  col.service = x
  const width = col.service + (services.length ? GROUP_W : 0) + PAD

  let y = TOP
  const groupBoxes = services.map((g) => {
    const b = { x: col.service, y, w: GROUP_W, h: groupH(g) }
    y += b.h + GROUP_GAP
    return b
  })
  const servicesH = services.length ? y - GROUP_GAP - TOP : 0
  const clientsH = stackH(clients.length, NODE_H)
  const midH = hasMid ? stackH(Math.max(1, externals.length), APP_H) : 0
  const contentH = Math.max(servicesH, clientsH, midH, APP_H)
  // The runner, the clients and the app sit on the middle of the services
  // when those are short, and near the top when the column is long: a tall
  // picture should not push the start of every line out of view.
  const midY = TOP + Math.min(contentH, Math.max(clientsH, midH, 320)) / 2
  const appTop = midY - APP_H / 2

  const put = (key, b, c) => { boxes.set(key, { ...b, c, cy: b.y + b.h / 2 }) }
  put('tomato', { x: col.runner, y: appTop, w: RUNNER_W, h: APP_H }, 0)
  clients.forEach((r, i) => put(r.name, { x: col.client, y: midY - clientsH / 2 + i * (NODE_H + ROW_GAP), w: CLIENT_W, h: NODE_H }, 1))
  if (app) put('app', { x: col.app, y: appTop, w: APP_W, h: APP_H }, 2)
  externals.forEach((a, i) => put(`x:${a}`, { x: col.app, y: midY - midH / 2 + i * (APP_H + ROW_GAP), w: APP_W, h: APP_H }, 2))
  services.forEach((g, gi) => {
    const b = groupBoxes[gi]
    put(g.key, { x: b.x, y: b.y, w: b.w, h: GROUP_TITLE + GROUP_PAD }, 3)
    g.rows.forEach((r, i) => put(r.name, { x: b.x + GROUP_PAD, y: b.y + GROUP_PAD + GROUP_TITLE + i * (NODE_H + ROW_GAP), w: NODE_W, h: NODE_H }, 3))
  })

  labels.push({ text: 'runner', x: col.runner + RUNNER_W / 2 })
  if (wide) labels.push({ text: 'clients', x: col.client + CLIENT_W / 2 })
  if (hasMid) labels.push({ text: app ? 'app' : 'calls', x: col.app + APP_W / 2 })
  if (services.length) labels.push({ text: 'services', x: col.service + GROUP_W / 2 })

  // Lines are drawn after the edges are known, so the channels can be
  // handed out by target height and the picture grow to fit them.
  const route = (edges) => {
    for (const e of edges) e.spans = e.b.c - e.a.c > 1 && hasMid
    const above = edges.filter((e) => e.spans && e.b.cy < midY).sort((p, q) => q.b.cy - p.b.cy)
    const below = edges.filter((e) => e.spans && e.b.cy >= midY).sort((p, q) => p.b.cy - q.b.cy)
    above.forEach((e, i) => { e.ch = appTop - 12 - i * 11; e.lane = i })
    below.forEach((e, i) => { e.ch = appTop + APP_H + 12 + i * 11; e.lane = i })
    const R = 8
    for (const e of edges) {
      const x1 = e.a.x + e.a.w; const y1 = e.a.cy; const x2 = e.b.x; const y2 = e.b.cy
      if (e.spans) {
        // Out of the source, up or down to the channel, across, then down
        // or up to the target, with rounded corners: the inner channel
        // takes the inner rise and drop, so lines nest instead of crossing,
        // and the last leg is straight so the arrowhead sits on the line.
        const riseX = x1 + 14 + e.lane * 8
        const dropX = x2 - 14 - e.lane * 8
        const up = e.ch < y1 ? -1 : 1
        const down = y2 < e.ch ? -1 : 1
        const r1 = Math.min(R, Math.abs(e.ch - y1) / 2)
        const r2 = Math.min(R, Math.abs(y2 - e.ch) / 2)
        e.d = `M${x1},${y1} L${riseX - r1},${y1} Q${riseX},${y1} ${riseX},${y1 + up * r1}`
          + ` L${riseX},${e.ch - up * r1} Q${riseX},${e.ch} ${riseX + r1},${e.ch}`
          + ` L${dropX - r2},${e.ch} Q${dropX},${e.ch} ${dropX},${e.ch + down * r2}`
          + ` L${dropX},${y2 - down * r2} Q${dropX},${y2} ${dropX + r2},${y2} L${x2 - 1},${y2}`
      } else {
        e.d = `M${x1},${y1} C${x1 + 28},${y1} ${x2 - 28},${y2} ${x2 - 1},${y2}`
      }
    }
    const top = Math.min(TOP - 24, ...above.map((e) => e.ch - 10))
    const bottom = Math.max(TOP + contentH, ...below.map((e) => e.ch + 10))
    return { top, height: bottom + 16, width }
  }
  return { boxes, groupBoxes, labels, route, labelY: () => TOP - 14 }
}

// A stack, top to bottom: tomato, the clients, the app, the services. Every
// line leaves the stack sideways and runs down (or up) a channel beside it,
// tomato's traffic on the left, the app's traffic and the wiring on the right,
// so nothing crosses a box. This is the layout a narrow inspector gets.
function layoutStack({ clients, app, externals, services }, paneW) {
  const boxes = new Map(); const labels = []
  const halfW = Math.max(RUNNER_W, clients.length ? CLIENT_W : 0, (app || externals.length) ? APP_W : 0, services.length ? GROUP_W : 0) / 2
  const cx = paneW / 2
  const BAND_GAP = 26
  let y = TOP
  const put = (key, w, h, c, extra) => {
    const b = { x: cx - w / 2, y, w, h, c, cy: y + h / 2, ...extra }
    boxes.set(key, b); return b
  }
  // Captions sit on the stack's left edge, clear of the channels beside it.
  const band = (text) => { labels.push({ text, x: cx - halfW, y: y - 8 }) }

  band('runner'); put('tomato', RUNNER_W, APP_H, 0); y += APP_H
  if (clients.length) {
    y += BAND_GAP; band('clients')
    clients.forEach((r, i) => { put(r.name, CLIENT_W, NODE_H, 1); y += NODE_H + (i < clients.length - 1 ? ROW_GAP : 0) })
  }
  if (app) { y += BAND_GAP; band('app'); put('app', APP_W, APP_H, 2); y += APP_H }
  externals.forEach((a, i) => {
    if (i === 0) { y += BAND_GAP; band('calls') }
    put(`x:${a}`, APP_W, APP_H, 2); y += APP_H + ROW_GAP
  })
  const groupBoxes = []
  services.forEach((g, gi) => {
    if (gi === 0) { y += BAND_GAP; band('services') }
    const gb = { x: cx - GROUP_W / 2, y, w: GROUP_W, h: groupH(g) }
    groupBoxes.push(gb)
    boxes.set(g.key, { x: gb.x, y: gb.y, w: gb.w, h: GROUP_TITLE + GROUP_PAD, c: 3, cy: gb.y + (GROUP_TITLE + GROUP_PAD) / 2 })
    g.rows.forEach((r, i) => {
      const ry = gb.y + GROUP_PAD + GROUP_TITLE + i * (NODE_H + ROW_GAP)
      boxes.set(r.name, { x: gb.x + GROUP_PAD, y: ry, w: NODE_W, h: NODE_H, c: 3, cy: ry + NODE_H / 2 })
    })
    y += gb.h + (gi < services.length - 1 ? GROUP_GAP : 0)
  })
  const contentBottom = y

  // Lines end on the box they enter, so the arrowhead touches it. On the
  // left side the target is the box's left edge and the line arrives from
  // the left, so the end is one pixel before it; on the right, one after.
  const route = (edges) => {
    // Longer hops take the outer channels so the arcs nest instead of cross.
    const side = (e) => (e.inferred || !e.steps ? 'right' : 'left')
    const lanes = { left: [], right: [] }
    for (const e of edges.slice().sort((p, q) => Math.abs(q.b.cy - q.a.cy) - Math.abs(p.b.cy - p.a.cy))) {
      const s = side(e); e.side = s; e.lane = lanes[s].length; lanes[s].push(e)
    }
    const reach = { left: lanes.left.length, right: lanes.right.length }
    for (const e of edges) {
      const out = e.side === 'left' ? -1 : 1
      const chx = cx + out * (halfW + 14 + (reach[e.side] - 1 - e.lane) * 10)
      const x1 = e.side === 'left' ? e.a.x : e.a.x + e.a.w
      const x2 = e.side === 'left' ? e.b.x : e.b.x + e.b.w
      const y1 = e.a.cy; const y2 = e.b.cy
      const dir = y2 > y1 ? 1 : -1
      const r = Math.min(8, Math.abs(y2 - y1) / 2)
      e.d = `M${x1},${y1} L${chx - out * r},${y1} Q${chx},${y1} ${chx},${y1 + dir * r}`
        + ` L${chx},${y2 - dir * r} Q${chx},${y2} ${chx - out * r},${y2} L${x2 + out},${y2}`
    }
    const span = Math.max(reach.left, reach.right)
    const width = Math.max(paneW, 2 * (halfW + 14 + span * 10) + 24)
    return { top: TOP - 22, height: contentBottom + 16, width }
  }
  return { boxes, groupBoxes, labels, route, labelY: null }
}

// The nodes are the server's TopologyJSON: tomato, the app under test, the
// containers tomato starts and every resource tomato.yml defines. Resources
// sit inside the container that backs them, so two redis resources on one
// container read as one service with two handles. Dotted lines are the wiring
// configuration declares (a client's base_url, the app's env templates); the
// solid and dashed lines are the scenario's traffic, one per route. By default only what the scenario touches is drawn;
// the rest is listed underneath, or drawn dimmed with "show all".
export function Topology({ model, topo, cursor, hot, pinned, onHot, onPin }) {
  const { msgs } = model
  const [showAll, setShowAll] = useState(false)
  const paneRef = useRef(null)
  const [paneW, setPaneW] = useState(0)
  useEffect(() => {
    const el = paneRef.current
    if (!el || typeof ResizeObserver === 'undefined') return undefined
    const ro = new ResizeObserver((entries) => setPaneW(entries[0].contentRect.width))
    ro.observe(el)
    setPaneW(el.clientWidth)
    return () => ro.disconnect()
  }, [])

  const resources = topo?.resources?.length
    ? topo.resources
    : model.lanes.slice(1).map((l) => ({ name: l.name, type: l.type, role: roleOf(l.type) }))
  const containers = topo?.containers ?? []
  const app = topo?.app ?? (topo?.appPort ? { name: 'app', mode: 'command', port: topo.appPort, uses: [] } : null)
  const byName = new Map(resources.map((r) => [r.name, r]))
  const cur = cursor >= 0 ? msgs[cursor] : null

  // What the scenario touches: the resources its steps name, the containers
  // behind them, the targets its clients call and the app behind a mock.
  const touched = new Set()
  for (const m of msgs) {
    if (m.kind === 'note' || !m.resource) continue
    const r = byName.get(m.resource)
    if (!r) continue
    touched.add(r.name)
    if (r.container && r.role !== 'client') touched.add(`c:${r.container}`)
    if (r.role === 'client') touched.add(r.target || `x:${r.address}`)
    if (r.role === 'mock') touched.add('app')
  }
  if (touched.has('app') && app) for (const u of app.uses ?? []) touched.add(byName.has(u) ? u : `c:${u}`)
  const nothingTouched = touched.size === 0
  const isUsed = (key) => nothingTouched || touched.has(key)
  const drawn = (key) => showAll || isUsed(key)

  const clients = resources.filter((r) => r.role === 'client' && drawn(r.name))
  const externals = app ? [] : [...new Set(clients.filter((r) => !r.target && r.address).map((r) => r.address))]

  // A container that only runs clients (the chromium a browser drives) is
  // part of tomato's side of the picture, not a service of the app's.
  const clientOnly = new Set(containers.map((c) => c.name).filter((name) => {
    const users = resources.filter((r) => r.container === name)
    return users.length > 0 && users.every((r) => r.role === 'client')
  }))
  const groups = []
  const byContainer = new Map()
  for (const c of containers.filter((c) => !clientOnly.has(c.name))) {
    const g = { kind: 'container', key: `c:${c.name}`, title: c.name, sub: c.image, icon: 'container', rows: [] }
    byContainer.set(c.name, g); groups.push(g)
  }
  const served = { kind: 'served', key: 'g:served', title: 'served by tomato', sub: 'the app calls these', icon: 'tomato', rows: [] }
  const outside = { kind: 'external', key: 'g:external', title: 'outside tomato', sub: 'not started by tomato', icon: 'app', rows: [] }
  const host = { kind: 'host', key: 'g:host', title: 'host', sub: 'this machine', icon: 'shell', rows: [] }
  for (const r of resources) {
    if (r.role === 'client' || !drawn(r.name)) continue
    if (r.container && byContainer.has(r.container)) byContainer.get(r.container).rows.push(r)
    else if (r.role === 'mock') served.rows.push(r)
    else if (r.role === 'host') host.rows.push(r)
    else outside.rows.push(r)
  }
  groups.push(served, outside, host)
  const services = groups.filter((g) => g.rows.length || (g.kind === 'container' && drawn(g.key)))
  for (const g of services) g.used = isUsed(g.key) || g.rows.some((r) => isUsed(r.name))
  // What the scenario touches comes first, so it stays in view when the
  // rest is shown; within that, the order is the config's.
  services.sort((p, q) => Number(q.used) - Number(p.used))

  const notInvolved = nothingTouched ? [] : [
    ...resources.filter((r) => !isUsed(r.name)).map((r) => r.name),
    ...containers.filter((c) => !clientOnly.has(c.name) && !isUsed(`c:${c.name}`)).map((c) => `${c.name} (container)`),
  ]

  // Columns when the pane has room for them — with tighter gaps when it
  // nearly does — and the stack otherwise.
  const shape = { clients, app, externals, services }
  let L = layoutColumns(shape)
  if (paneW > 0 && paneW < L.route([]).width) L = layoutColumns(shape, COL_GAP_TIGHT)
  const stacked = paneW > 0 && paneW < L.route([]).width
  if (stacked) L = layoutStack(shape, Math.max(paneW, 240))
  const { boxes } = L

  // The scenario's traffic, collapsed into routes. A request through a client
  // is two routes, tomato → the client and the client → what it addresses,
  // because that is what goes over the wire; a mock's traffic came from the app.
  const routes = new Map()
  const addHop = (from, to, msg, inferred) => {
    if (!boxes.has(from) || !boxes.has(to)) return
    const key = `${from}>${to}`
    if (!routes.has(key)) routes.set(key, { from, to, inferred, steps: [] })
    routes.get(key).steps.push(msg)
  }
  for (const m of msgs) {
    if (m.kind === 'note' || !m.resource) continue
    const r = byName.get(m.resource)
    if (!r) continue
    if (r.role === 'client') {
      addHop('tomato', r.name, m, false)
      addHop(r.name, r.target || `x:${r.address}`, m, false)
    } else if (r.role === 'mock') {
      addHop('app', r.name, m, true)
    } else {
      addHop('tomato', r.name, m, false)
    }
  }

  // The wiring configuration declares, drawn dotted unless the scenario's
  // traffic already runs on that pair.
  const wires = []
  const addWire = (from, to) => {
    if (boxes.has(from) && boxes.has(to) && !routes.has(`${from}>${to}`)) wires.push({ from, to })
  }
  for (const r of clients) addWire(r.name, r.target || `x:${r.address}`)
  if (app) for (const u of app.uses ?? []) addWire('app', byName.has(u) ? u : `c:${u}`)

  const all = [...routes.values(), ...wires].map((e) => ({ ...e, a: boxes.get(e.from), b: boxes.get(e.to) }))
  const { top, height, width } = L.route(all)

  // What the cursor is touching right now, so the nodes on that path light up.
  const live = new Set()
  if (cur?.resource) {
    live.add('tomato'); live.add(cur.resource)
    const r = byName.get(cur.resource)
    if (r?.role === 'client') live.add(r.target || `x:${r.address}`)
    if (r?.role === 'mock') live.add('app')
    if (r?.container) live.add(`c:${r.container}`)
  }
  const failedFor = (name) => msgs.some((m) => m.status === 'failed' && m.resource === name)

  const edges = all.filter((e) => e.steps).map((e) => {
    const onRoute = cur ? e.steps.some((m) => m.idx === cur.idx) : false
    const reached = e.steps.some((m) => cursor < 0 || m.idx <= cursor)
    const failed = e.steps.some((m) => m.status === 'failed')
    // Hovering a route explains it with the step the cursor is on when that
    // step uses it, and with the first step that does otherwise.
    const subject = cur && onRoute ? cur : e.steps[0]
    const isHot = (hot != null && e.steps.some((m) => m.idx === hot)) || (pinned != null && e.steps.some((m) => m.idx === pinned))
    const isPinned = pinned != null && e.steps.some((m) => m.idx === pinned)
    return (
      <g
        className="tedge"
        key={`${e.from}>${e.to}`}
        data-state={onRoute ? 'current' : cursor < 0 ? undefined : reached ? 'past' : 'future'}
        data-status={failed ? 'failed' : undefined}
        data-inferred={e.inferred ? 'true' : undefined}
        data-hot={isHot ? 'true' : undefined}
        data-pinned={isPinned ? 'true' : undefined}
        onMouseEnter={() => onHot(subject.idx)}
        onMouseLeave={() => onHot(null)}
        onClick={() => onPin(subject.idx)}
      >
        <title>{`${e.steps.length === 1 ? 'step' : 'steps'} ${e.steps.map((m) => m.n).join(', ')} on this hop`}</title>
        <path className="tedge-hit" d={e.d} />
        <path className="tedge-line" pathLength="1" d={e.d} />
      </g>
    )
  })
  const wireEls = all.filter((e) => !e.steps).map((e) => <path key={`w:${e.from}>${e.to}`} className="twire" d={e.d} />)

  const appSub = app ? [app.mode, app.port ? `:${app.port}` : ''].filter(Boolean).join(' ') : ''
  const nodeEl = (key, props) => <Node key={key} box={boxes.get(key)} {...props} />

  return (
    <div className="tp" ref={paneRef} data-layout={stacked ? 'stack' : 'columns'} data-all={showAll ? 'true' : undefined}>
      <div className="tp-scroll">
        <svg
          className="tsvg" width={width} height={height - top} viewBox={`0 ${top} ${width} ${height - top}`}
          role="img" aria-label="What this scenario talks to"
        >
          <defs>
            <Marker id="th-muted" cls="th--muted" />
            <Marker id="th-cur" cls="th--cur" />
            <Marker id="th-fail" cls="th--fail" />
          </defs>

          {L.labels.map((l) => (
            stacked
              ? <text key={l.text} className="tcol" data-align="left" x={l.x} y={l.y}>{l.text}</text>
              : <text key={l.text} className="tcol" x={l.x} y={L.labelY()}>{l.text}</text>
          ))}

          {services.map((g, i) => <Group key={g.key} g={g} box={L.groupBoxes[i]} />)}
          {wireEls}
          {edges}

          {nodeEl('tomato', { kind: 'runner', name: 'tomato', sub: 'runner', icon: 'tomato', active: live.has('tomato') })}
          {app && nodeEl('app', { kind: 'app', name: app.name, sub: appSub, icon: 'app', active: live.has('app'), used: isUsed('app') })}
          {externals.map((a) => nodeEl(`x:${a}`, { kind: 'external', name: a, sub: 'not under tomato', icon: 'app', active: live.has(`x:${a}`), used: isUsed(`x:${a}`) }))}
          {clients.map((r) => nodeEl(r.name, {
            kind: 'client', name: r.name, sub: r.type, icon: iconId(r.type),
            active: live.has(r.name), status: failedFor(r.name) ? 'failed' : '', used: isUsed(r.name),
          }))}
          {services.flatMap((g) => g.rows.map((r) => nodeEl(r.name, {
            kind: r.role, name: r.name, sub: r.role === 'mock' && r.port ? `${r.type} :${r.port}` : r.type, icon: iconId(r.type),
            active: live.has(r.name), status: failedFor(r.name) ? 'failed' : '', used: isUsed(r.name), mock: r.role === 'mock',
          })))}
        </svg>
      </div>

      <div className="tp-legend">
        <span className="tp-key">traffic tomato sends</span>
        <span className="tp-key" data-inferred="true">traffic the app sends</span>
        <span className="tp-key" data-wire="true">wired by tomato.yml</span>
        <span className="muted">hover a hop for the steps on it; the Flow tab has their order</span>
      </div>

      {notInvolved.length > 0 && (
        <div className="tp-unused" data-shown={showAll ? 'true' : undefined}>
          <span className="tp-unused-label">not involved in this scenario</span>
          <span className="tp-unused-list">{notInvolved.join(', ')}</span>
          <button className="btn btn--ghost btn--sm" type="button" onClick={() => setShowAll((v) => !v)}>
            {showAll ? 'hide them' : `show all ${resources.length + containers.length}`}
          </button>
        </div>
      )}
    </div>
  )
}
