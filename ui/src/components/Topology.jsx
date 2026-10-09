import { iconId } from '../lib/icons'
import { Peek } from './Peek.jsx'

function Marker({ id, cls }) {
  return (
    <marker id={id} viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto">
      <path className={`th ${cls}`} d="M0,0 L10,5 L0,10 z" />
    </marker>
  )
}

// The node list is the server's TopologyJSON — the app plus every resource
// tomato.yml defines, not just what this scenario touches — while the numbered
// edges come from the scenario's steps.
export function Topology({ model, topo, cursor, hot, onHot }) {
  const { msgs } = model
  const configured = topo?.resources ?? []
  const fallback = model.lanes.slice(1).map((l) => ({ name: l.name, type: l.type }))
  const res = (configured.length ? [...configured] : fallback)
    .sort((a, b) => (a.target ? 0 : 1) - (b.target ? 0 : 1) || a.name.localeCompare(b.name))

  const NH = 38; const GAP = 14; const TOP = 34
  const edgeCount = msgs.filter((m) => m.kind !== 'note').length
  const H = Math.max(230, TOP + res.length * (NH + GAP) + 56, 120 + edgeCount * 12)
  const appY = H / 2 - 24
  const sy = appY + 24

  const cur = cursor >= 0 ? msgs[cursor] : null
  const activeRes = cur?.resource ?? null
  const stateOf = (i) => (cursor < 0 ? undefined : i < cursor ? 'past' : i === cursor ? 'current' : 'future')

  // Each step gets its own channel above or below the app box. Keying the
  // channel on the resource instead would draw identical overlapping edges for
  // repeated resources, hiding all but the last — and the badge is the step
  // number, so they have to stay distinct.
  const drawable = msgs
    .filter((m) => m.kind !== 'note')
    .map((m) => {
      const i = res.findIndex((r) => r.name === m.resource)
      return i < 0 ? null : { m, ry: TOP + i * (NH + GAP) + NH / 2 }
    })
    .filter(Boolean)
  const up = drawable.filter((d) => d.ry < sy)
  const down = drawable.filter((d) => d.ry >= sy)
  up.forEach((d, i) => { d.ch = appY - 10 - (up.length - 1 - i) * 12 })
  down.forEach((d, i) => { d.ch = appY + 58 + i * 12 })

  const dock = hot != null ? msgs[hot] : cur

  return (
    <div className="tp">
      <svg className="tsvg" viewBox={`0 0 330 ${H}`} role="img" aria-label="Scenario topology, numbered by step">
        <defs>
          <Marker id="th-muted" cls="th--muted" />
          <Marker id="th-cur" cls="th--cur" />
          <Marker id="th-fail" cls="th--fail" />
        </defs>

        <text className="tcol" x="45" y="14">runner</text>
        <text className="tcol" x="165" y="14">app</text>
        <text className="tcol" x="285" y="14">resources</text>

        <g className="tnode" data-kind="runner" data-active={cur ? 'true' : undefined}>
          <rect x="6" y={appY} width="78" height="48" rx="6" />
          <use href="#ri-tomato" x="13" y={appY + 17} width="14" height="14" />
          <text className="tnode-name" x="32" y={appY + 22.5}>tomato</text>
          <text className="tnode-type" x="32" y={appY + 33.5}>runner</text>
        </g>

        <g className="tnode" data-kind="app">
          <rect x="126" y={appY} width="78" height="48" rx="6" />
          <use href="#ri-app" x="133" y={appY + 17} width="14" height="14" />
          <text className="tnode-name" x="152" y={appY + 22.5}>app</text>
          <text className="tnode-type" x="152" y={appY + 33.5}>
            {topo?.appPort ? `:${topo.appPort}` : 'under test'}
          </text>
        </g>

        {res.map((r, i) => {
          const y = TOP + i * (NH + GAP)
          const failed = msgs.some((m) => m.status === 'failed' && m.resource === r.name)
          // A client pointed at another resource is talking to a mock.
          const mock = Boolean(r.target && r.target !== 'app')
          return (
            <g
              className="tnode"
              key={r.name}
              data-active={activeRes === r.name ? 'true' : undefined}
              data-status={failed ? 'failed' : undefined}
              data-mock={mock ? 'true' : undefined}
            >
              <rect x="246" y={y} width="78" height={NH} rx="6" />
              <use href={`#ri-${iconId(r.type)}`} x="253" y={y + 12} width="14" height="14" />
              <text className="tnode-name" x="272" y={y + 17.5}>{r.name}</text>
              <text className="tnode-type" x="272" y={y + 28.5}>{r.type}</text>
            </g>
          )
        })}

        {drawable.map(({ m, ry, ch }) => {
          const d = `M84,${sy} C118,${sy} 128,${ch} 165,${ch} S228,${ry} 244,${ry}`
          return (
            <g
              className="tedge"
              key={m.n}
              data-state={stateOf(m.idx)}
              data-status={m.status || undefined}
              data-hot={hot === m.idx ? 'true' : undefined}
              onMouseEnter={() => onHot(m.idx)}
              onMouseLeave={() => onHot(null)}
            >
              <path className="tedge-hit" d={d} />
              <path className="tedge-line" pathLength="1" d={d} />
              <g className="tbadge">
                <circle cx="165" cy={ch} r="7" />
                <text x="165" y={ch}>{m.n}</text>
              </g>
            </g>
          )
        })}
      </svg>

      {dock && <div className="tp-dock"><Peek msg={dock} /></div>}

      <div className="tp-legend">
        <span className="tp-key">tomato acts or checks</span>
        <span className="tp-key" data-inferred="true">mock tomato serves</span>
      </div>
    </div>
  )
}
