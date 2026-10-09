import { iconId } from '../lib/icons'
import { Peek } from './Peek.jsx'

// Resource types tomato serves itself: the app sends traffic to these, tomato
// only observes it arriving.
const MOCK_TYPES = new Set(['http-server', 'websocket-server'])

function Marker({ id, cls }) {
  return (
    <marker id={id} viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto">
      <path className={`th ${cls}`} d="M0,0 L10,5 L0,10 z" />
    </marker>
  )
}

function Node({ x, y, w, h, kind, name, type, active, status, mock }) {
  return (
    <g
      className="tnode"
      data-kind={kind}
      data-active={active ? 'true' : undefined}
      data-status={status || undefined}
      data-mock={mock ? 'true' : undefined}
    >
      <rect x={x} y={y} width={w} height={h} rx="6" />
      <use href={`#ri-${iconId(type)}`} x={x + 9} y={y + h / 2 - 7} width="14" height="14" />
      <text className="tnode-name" x={x + 29} y={y + h / 2 - 1.5}>{name}</text>
      <text className="tnode-type" x={x + 29} y={y + h / 2 + 9.5}>{type}</text>
    </g>
  )
}

// The nodes are the server's TopologyJSON — the app and every resource
// tomato.yml defines. The edges are network traffic: one per route the
// scenario puts packets on, not one per step. A request through an
// http-client is two routes, tomato → the client and the client → whatever
// its base_url addresses, because that is what actually goes over the wire.
// The per-step view is the Flow tab; this one answers "what talks to what".
export function Topology({ model, topo, cursor, hot, onHot }) {
  const { msgs } = model
  const all = topo?.resources?.length
    ? topo.resources
    : model.lanes.slice(1).map((l) => ({ name: l.name, type: l.type }))

  // A resource with a target is something tomato sends *through* rather than
  // *to*: it sits between the runner and whatever it addresses.
  const clients = all.filter((r) => r.target)
  const endpoints = all.filter((r) => !r.target)
  const byName = new Map(all.map((r) => [r.name, r]))
  const wide = clients.length > 0

  const COL = wide
    ? { runner: 8, client: 148, app: 288, right: 420, w: 100, rightW: 92, vb: 520 }
    : { runner: 8, app: 190, right: 360, w: 100, rightW: 92, vb: 460 }

  const NH = 40; const BH = 48; const GAP = 14; const TOP = 30
  const stackH = (n) => (n ? n * NH + (n - 1) * GAP : 0)
  const contentH = Math.max(stackH(clients.length), stackH(endpoints.length), BH)
  const clientY = (i) => TOP + (contentH - stackH(clients.length)) / 2 + i * (NH + GAP)
  const rightY = (i) => TOP + (contentH - stackH(endpoints.length)) / 2 + i * (NH + GAP)
  const boxY = TOP + (contentH - BH) / 2
  const midY = boxY + BH / 2

  const cur = cursor >= 0 ? msgs[cursor] : null

  const anchor = (name) => {
    if (name === 'tomato') return { x: COL.runner, w: COL.w, y: midY }
    if (name === 'app') return { x: COL.app, w: COL.w, y: midY }
    const ci = clients.findIndex((r) => r.name === name)
    if (ci >= 0) return { x: COL.client, w: COL.w, y: clientY(ci) + NH / 2 }
    const ei = endpoints.findIndex((r) => r.name === name)
    if (ei >= 0) return { x: COL.right, w: COL.rightW, y: rightY(ei) + NH / 2 }
    return null
  }

  // Collapse the scenario's steps into the routes they put traffic on.
  const routes = new Map()
  const addHop = (from, to, msg, inferred) => {
    const key = `${from}>${to}`
    if (!routes.has(key)) routes.set(key, { from, to, inferred, steps: [] })
    routes.get(key).steps.push(msg)
  }
  for (const m of msgs) {
    if (m.kind === 'note' || !m.resource) continue
    const res = byName.get(m.resource)
    if (!res) continue
    if (res.target && wide) {
      addHop('tomato', res.name, m, false)
      addHop(res.name, res.target, m, false)
    } else if (MOCK_TYPES.has(res.type)) {
      // tomato serves this one, so the traffic it sees came from the app.
      addHop('app', res.name, m, true)
    } else {
      addHop('tomato', res.name, m, false)
    }
  }

  // A route that skips a column has to clear the boxes in between, so it runs
  // through a channel just outside the runner/app row.
  const list = [...routes.values()].map((r) => {
    const a = anchor(r.from); const b = anchor(r.to)
    const spans = a && b && Math.abs(b.x - (a.x + a.w)) > 60
    return { ...r, a, b, spans }
  }).filter((r) => r.a && r.b)

  const above = list.filter((r) => r.spans && r.b.y < midY)
  const below = list.filter((r) => r.spans && r.b.y >= midY)
  above.forEach((r, i) => { r.ch = boxY - 10 - (above.length - 1 - i) * 12 })
  below.forEach((r, i) => { r.ch = boxY + BH + 10 + i * 12 })

  const lowest = Math.max(boxY + BH, ...list.map((r) => r.ch ?? 0))
  const H = Math.max(220, TOP + contentH + 16, lowest + 20)

  // What the cursor is touching right now, so the nodes on that path light up.
  const live = new Set()
  if (cur?.resource) {
    live.add(cur.resource)
    const r = byName.get(cur.resource)
    if (r?.target) live.add(r.target)
    if (r && MOCK_TYPES.has(r.type)) live.add('app')
    live.add('tomato')
  }

  const failedFor = (name) => msgs.some((m) => m.status === 'failed' && m.resource === name)

  const edges = list.map((r) => {
    const onRoute = cur ? r.steps.some((m) => m.idx === cur.idx) : false
    const reached = r.steps.some((m) => cursor < 0 || m.idx <= cursor)
    const failed = r.steps.some((m) => m.status === 'failed')
    // Hovering a route explains it with the step the cursor is on when that
    // step uses it, and with the first step that does otherwise.
    const subject = (cur && onRoute ? cur : r.steps[0])
    const isHot = hot != null && r.steps.some((m) => m.idx === hot)

    const fromX = r.a.x + r.a.w
    const toX = r.b.x
    const bx = (fromX + toX) / 2
    const by = r.spans ? r.ch : (r.a.y + r.b.y) / 2
    // A spanning route climbs to its channel inside the first gap, runs flat
    // past the columns it skips, then drops to its target. Easing across the
    // whole width instead would put the line through the boxes in between,
    // and since nodes paint last it would look like it started at one of them.
    const d = r.spans
      ? `M${fromX},${r.a.y} C${fromX + 16},${r.a.y} ${fromX + 20},${by} ${fromX + 40},${by}`
        + ` L${toX - 40},${by}`
        + ` C${toX - 20},${by} ${toX - 16},${r.b.y} ${toX - 5},${r.b.y}`
      : `M${fromX},${r.a.y} C${fromX + 22},${r.a.y} ${toX - 22},${r.b.y} ${toX - 5},${r.b.y}`

    return (
      <g
        className="tedge"
        key={`${r.from}>${r.to}`}
        data-state={onRoute ? 'current' : cursor < 0 ? undefined : reached ? 'past' : 'future'}
        data-status={failed ? 'failed' : undefined}
        data-inferred={r.inferred ? 'true' : undefined}
        data-hot={isHot ? 'true' : undefined}
        onMouseEnter={() => onHot(subject.idx)}
        onMouseLeave={() => onHot(null)}
      >
        <path className="tedge-hit" d={d} />
        <path className="tedge-line" pathLength="1" d={d} />
        <g className="tbadge">
          <circle cx={bx} cy={by} r="7.5" />
          <text x={bx} y={by}>{r.steps.length}</text>
        </g>
      </g>
    )
  })

  const dock = hot != null ? msgs[hot] : cur

  return (
    <div className="tp">
      <svg className="tsvg" viewBox={`0 0 ${COL.vb} ${H}`} role="img" aria-label="Network traffic for this scenario">
        <defs>
          <Marker id="th-muted" cls="th--muted" />
          <Marker id="th-cur" cls="th--cur" />
          <Marker id="th-fail" cls="th--fail" />
        </defs>

        <text className="tcol" x={COL.runner + COL.w / 2} y="12">runner</text>
        {wide && <text className="tcol" x={COL.client + COL.w / 2} y="12">clients</text>}
        <text className="tcol" x={COL.app + COL.w / 2} y="12">app</text>
        <text className="tcol" x={COL.right + COL.rightW / 2} y="12">resources</text>

        {edges}

        <Node
          x={COL.runner} y={boxY} w={COL.w} h={BH} kind="runner"
          name="tomato" type="runner" active={live.has('tomato')}
        />
        <Node
          x={COL.app} y={boxY} w={COL.w} h={BH} kind="app"
          name="app" type={topo?.appPort ? `:${topo.appPort}` : 'under test'}
          active={live.has('app')}
        />
        {wide && clients.map((r, i) => (
          <Node
            key={r.name} x={COL.client} y={clientY(i)} w={COL.w} h={NH}
            name={r.name} type={r.type}
            active={live.has(r.name)} status={failedFor(r.name) ? 'failed' : ''}
          />
        ))}
        {endpoints.map((r, i) => (
          <Node
            key={r.name} x={COL.right} y={rightY(i)} w={COL.rightW} h={NH}
            name={r.name} type={r.type}
            active={live.has(r.name)} status={failedFor(r.name) ? 'failed' : ''}
            mock={MOCK_TYPES.has(r.type)}
          />
        ))}
      </svg>

      {dock && <div className="tp-dock"><Peek msg={dock} /></div>}

      <div className="tp-legend">
        <span className="tp-key">traffic tomato sends</span>
        <span className="tp-key" data-inferred="true">traffic the app sends</span>
        <span className="muted">the number is how many steps use that hop</span>
      </div>
    </div>
  )
}
