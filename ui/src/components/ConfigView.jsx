import { ResourceIcon } from '../lib/icons'
import { tokenizeStep } from '../lib/steps.jsx'

function Section({ title, yamlKey, icon, pill, children }) {
  return (
    <section className="cfg-section">
      <div className="cfg-head">
        {icon}
        <span className="cfg-title">{title}</span>
        <span className="cfg-key">{yamlKey}</span>
        {pill}
      </div>
      {children}
    </section>
  )
}

function KV({ pairs, showEmpty }) {
  const rows = showEmpty ? pairs : pairs.filter(([, v]) => v != null && v !== '')
  return (
    <div className="cfg-kv">
      {rows.map(([k, v]) => {
        const empty = !v || (Array.isArray(v) && v.length === 0)
        return (
          <span key={k} style={{ display: 'contents' }}>
            <span className="cfg-k">{k}</span>
            <span className="cfg-v" data-empty={showEmpty && empty ? 'true' : undefined}>
              {showEmpty && empty ? 'none' : Array.isArray(v) ? v.join(', ') : v}
            </span>
          </span>
        )
      })}
    </div>
  )
}

function Table({ head, rows }) {
  return (
    <div className="cfg-wrap">
      <table className="dtable">
        <tbody>
          <tr>{head.map((h) => <th key={h}>{h}</th>)}</tr>
          {rows.map((r, i) => <tr key={i}>{r.map((c, j) => <td key={j}>{c}</td>)}</tr>)}
        </tbody>
      </table>
    </div>
  )
}

// A small YAML highlighter for the raw tab: keys, strings, numbers, booleans
// and comments. It never has to be a parser — it only colours what is there.
function yamlLine(line, key) {
  const comment = line.match(/^(\s*)(#.*)$/)
  if (comment) {
    return <span key={key}>{comment[1]}<span className="y-c">{comment[2]}</span>{'\n'}</span>
  }
  const out = []
  let rest = line
  const k = rest.match(/^(\s*-?\s*)([A-Za-z_][\w.-]*)(\s*:)/)
  if (k) {
    out.push(k[1], <span className="y-k" key="k">{k[2] + k[3]}</span>)
    rest = rest.slice(k[0].length)
  }
  let trailing = null
  const c = rest.match(/\s#.*$/)
  if (c) { trailing = <span className="y-c" key="c">{c[0]}</span>; rest = rest.slice(0, c.index) }

  const t = rest.trim()
  if (t) {
    const lead = rest.slice(0, rest.indexOf(t))
    const cls = /^(true|false|null|~)$/i.test(t) ? 'y-b'
      : /^-?\d+(\.\d+)?([a-z]{1,2})?$/i.test(t) ? 'y-n'
        : 'y-s'
    out.push(lead, <span className={cls} key="v">{tokenizeStep(t)}</span>)
  } else {
    out.push(rest)
  }
  if (trailing) out.push(trailing)
  return <span key={key}>{out}{'\n'}</span>
}

export function ConfigView({ config, mode, onMode, featureCount, scenarioCount }) {
  if (!config) return <div className="empty"><h3>Loading configuration…</h3></div>

  if (config.error) {
    return (
      <div className="doc">
        <div className="feature-head">
          <span className="feature-kw">CONFIG</span>
          <h1>tomato.yml</h1>
        </div>
        <div className="callout" data-status="failed">
          <span className="callout-title">could not read the config</span>
          <div className="callout-body">{config.error}</div>
        </div>
      </div>
    )
  }

  const s = config.settings ?? {}
  const app = config.app ?? {}
  const hooks = config.hooks ?? {}
  const features = config.features ?? {}

  return (
    <div className="doc">
      <div className="feature-head">
        <span className="feature-kw">CONFIG</span>
        <div className="row" style={{ justifyContent: 'space-between', alignItems: 'center' }}>
          <h1>{config.path ?? 'tomato.yml'}</h1>
          <div className="seg">
            <button
              className="seg-opt" type="button"
              aria-pressed={mode === 'structured' ? 'true' : undefined}
              onClick={() => onMode('structured')}
            >
              Structured
            </button>
            <button
              className="seg-opt" type="button"
              aria-pressed={mode === 'yaml' ? 'true' : undefined}
              onClick={() => onMode('yaml')}
            >
              YAML
            </button>
          </div>
        </div>
        <span className="feature-path">
          {config.path}{config.version ? ` · version ${config.version}` : ''}
        </span>
        <div className="row">
          <span className="pill" data-status={config.valid ? 'passed' : 'failed'}>
            {config.valid ? 'valid' : 'invalid'}
          </span>
          <span className="pill">{(config.containers ?? []).length} containers</span>
          <span className="pill">{(config.resources ?? []).length} resources</span>
          <span className="pill">{config.hookCount ?? 0} hooks</span>
        </div>
      </div>

      {mode === 'yaml' ? (
        <pre className="docstring yaml" data-lang="yaml">
          {String(config.content ?? '').split('\n').map((l, i) => yamlLine(l, i))}
        </pre>
      ) : (
        <>
          <Section title="Settings" yamlKey="settings:">
            <KV pairs={[
              ['timeout', s.timeout], ['parallel', s.parallel],
              ['fail_fast', String(Boolean(s.failFast))], ['output', s.output], ['reset', s.reset],
            ]} />
          </Section>

          {app.configured && (
            <Section title="App" yamlKey="app:" icon={<ResourceIcon type="app" />}>
              <KV pairs={[
                ['command', app.command], ['image', app.image], ['port', app.port],
                ['ready', app.ready], ['wait', app.wait],
              ]} />
              {app.env?.length > 0 && (
                <>
                  <span className="label" style={{ marginTop: 4 }}>env</span>
                  <div className="cfg-kv">
                    {app.env.map((e) => (
                      <span key={e.key} style={{ display: 'contents' }}>
                        <span className="cfg-k">{e.key}</span>
                        <span className="cfg-v">
                          <span>{tokenizeStep(e.value)}</span>
                          {e.resolved && e.resolved !== e.value && (
                            <span className="cfg-resolved">{e.resolved}</span>
                          )}
                        </span>
                      </span>
                    ))}
                  </div>
                </>
              )}
            </Section>
          )}

          {config.containers?.length > 0 && (
            <Section title="Containers" yamlKey="containers:">
              <Table
                head={['name', 'image', 'ports', 'wait for']}
                rows={config.containers.map((c) => [
                  c.name,
                  c.image || (c.preset ? `preset: ${c.preset}` : ''),
                  (c.ports ?? []).join(', '),
                  c.waitFor ?? '',
                ])}
              />
            </Section>
          )}

          {config.resources?.length > 0 && (
            <Section title="Resources" yamlKey="resources:">
              <Table
                head={['resource', 'connects to', 'options']}
                rows={config.resources.map((r) => [
                  <span className="cfg-res" key="r">
                    <ResourceIcon type={r.type} />
                    <span className="res">
                      <span className="res-name">{r.name}</span>
                      <span className="res-type">{r.type}</span>
                    </span>
                  </span>,
                  r.connectsTo ?? '',
                  r.options ?? '',
                ])}
              />
            </Section>
          )}

          <Section title="Hooks" yamlKey="hooks:">
            <KV showEmpty pairs={[
              ['before_all', hooks.beforeAll], ['after_all', hooks.afterAll],
              ['before_scenario', hooks.beforeScenario], ['after_scenario', hooks.afterScenario],
            ]} />
          </Section>

          <Section title="Features" yamlKey="features:">
            <KV pairs={[
              ['paths', (features.paths ?? []).join(', ')],
              ['tags', features.tags],
              ['matched', `${featureCount} features · ${scenarioCount} scenarios`],
            ]} />
          </Section>
        </>
      )}
    </div>
  )
}
