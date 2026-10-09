import { StepRow } from './StepRow.jsx'
import { fmtDur, plural } from '../lib/format'
import { BACKGROUND_KEY } from '../lib/steps.jsx'

function Head({ collapsed, onToggle, children }) {
  return (
    <div
      className="scenario-head"
      role="button"
      tabIndex={0}
      aria-expanded={collapsed ? 'false' : 'true'}
      onClick={onToggle}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onToggle() }
      }}
    >
      <span className="tree-caret"><i className="ico-chev" data-open={collapsed ? undefined : 'true'} /></span>
      {children}
    </div>
  )
}

// Background is a feature-level field, not a scenario: no name, no status of
// its own, and it never appears in counts or the run bar.
export function BackgroundCard({ steps, collapsed, onToggle, runState }) {
  return (
    <div className="scenario" data-kind="background" data-collapsed={collapsed ? 'true' : undefined}>
      <Head collapsed={collapsed} onToggle={onToggle}>
        <span className="scenario-kw">Background</span>
        <span className="scenario-name" style={{ color: 'var(--fg-2)', fontWeight: 400 }}>
          {plural(steps.length, 'step')} · runs before each scenario
        </span>
      </Head>
      {!collapsed && (
        <div className="steps">
          {steps.map((s, i) => (
            <StepRow key={i} step={s} scenarioName={BACKGROUND_KEY} index={i} runState={runState} />
          ))}
        </div>
      )}
    </div>
  )
}

export function ScenarioCard({
  scenario, index, current, collapsed, onToggle, onRun, onTag, filter, runState,
}) {
  const status = runState.scStatus[scenario.name] ?? ''
  const dur = runState.scDur[scenario.name]
  const examples = (scenario.examples ?? [])
    .reduce((n, ex) => n + Math.max(0, (ex.rows ?? []).length - 1), 0)

  return (
    <div
      className="scenario"
      data-status={status || undefined}
      data-collapsed={collapsed ? 'true' : undefined}
      aria-current={current ? 'true' : undefined}
    >
      <Head collapsed={collapsed} onToggle={onToggle}>
        <i className="pip" data-status={status} />
        <span className="scenario-kw">{scenario.isOutline ? 'Scenario Outline' : 'Scenario'}</span>
        <span className="scenario-name">{scenario.name}</span>
        {examples > 0 && <span className="pill">{examples} examples</span>}
        {dur != null && <span className="scenario-time">{fmtDur(dur)}</span>}
        <div className="scenario-actions">
          <button
            className="btn btn--sm btn--ghost"
            type="button"
            onClick={(e) => { e.stopPropagation(); onRun(scenario.name) }}
          >
            <i className="ico-play" />Run
          </button>
        </div>
      </Head>

      {!collapsed && (
        <>
          {scenario.tags?.length > 0 && (
            <div className="scenario-tags">
              {scenario.tags.map((t) => {
                const name = t.replace(/^@/, '')
                return (
                  <button
                    className="tag"
                    type="button"
                    key={t}
                    aria-pressed={filter.toLowerCase() === `@${name.toLowerCase()}` ? 'true' : undefined}
                    onClick={() => onTag(name)}
                  >
                    {name}
                  </button>
                )
              })}
            </div>
          )}
          <div className="steps">
            {(scenario.steps ?? []).map((s, i) => (
              <StepRow key={i} step={s} scenarioName={scenario.name} index={i} runState={runState} />
            ))}
          </div>
          {(scenario.examples ?? []).map((ex, i) => (
            ex.rows?.length > 0 && (
              <div className="examples" key={i} style={{ padding: '0 var(--s-5) var(--s-5)' }}>
                <span className="label" style={{ display: 'block', margin: 'var(--s-4) 0 var(--s-3)' }}>
                  Examples{ex.name ? ` · ${ex.name}` : ''}
                </span>
                <table className="dtable">
                  <tbody>
                    <tr>{ex.rows[0].map((c, j) => <th key={j}>{c}</th>)}</tr>
                    {ex.rows.slice(1).map((r, j) => (
                      <tr key={j}>{r.map((c, k) => <td key={k}>{c}</td>)}</tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )
          ))}
        </>
      )}
    </div>
  )
}
