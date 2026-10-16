import { Fragment } from 'react'
import { StepRow } from './StepRow.jsx'
import { fmtDur, plural } from '../lib/format'
import { backgroundStatus, failedBackgroundSteps, pickleIndex } from '../lib/run.js'
import { stepKind } from '../lib/kind'

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

// Background is a feature-level field, not a scenario: no name of its own,
// and it never appears in counts or the run bar. It runs once per scenario,
// though, and its status is reported under the scenario that ran it, so the
// rows show one scenario's run — `scenarioName`, the one running or the last
// that did (lib/run.js) — and the head says whose.
export function BackgroundCard({ steps, collapsed, onToggle, runState, scenarioName }) {
  const status = backgroundStatus(steps, scenarioName, runState)
  return (
    <div
      className="scenario"
      data-kind="background"
      data-status={status || undefined}
      data-collapsed={collapsed ? 'true' : undefined}
    >
      <Head collapsed={collapsed} onToggle={onToggle}>
        <i className="pip" data-status={status} />
        <span className="scenario-kw">Background</span>
        <span className="scenario-name" style={{ color: 'var(--fg-2)', fontWeight: 400 }}>
          {plural(steps.length, 'step')} · runs before each scenario
        </span>
        {scenarioName && (
          <span className="scenario-for" title={`What the Background did when “${scenarioName}” ran`}>
            as run for “{scenarioName}”
          </span>
        )}
      </Head>
      {!collapsed && (
        <div className="steps">
          {steps.map((s, i) => (
            <StepRow key={i} step={s} scenarioName={scenarioName} index={i} runState={runState} background />
          ))}
        </div>
      )}
    </div>
  )
}

export function ScenarioCard({
  scenario, index, current, collapsed, onToggle, onRun, onTag, filter, runState, background,
}) {
  const status = runState.scStatus[scenario.name] ?? ''
  const dur = runState.scDur[scenario.name]
  // A Background step that stopped this scenario is shown in its card too:
  // the Background card shows one scenario's run at a time, and the one that
  // failed in it is not always the one it shows.
  const failedBg = failedBackgroundSteps(background, scenario.name, runState)
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
            {failedBg.length > 0 && (
              <>
                <div className="steps-note">in the Background, before this scenario’s first step</div>
                {failedBg.map((r) => (
                  <StepRow key={`bg${r.index}`} step={r.step} scenarioName={scenario.name} index={r.index} runState={runState} background />
                ))}
                <div className="steps-rule" role="separator" aria-label="the Background failed above, so the scenario's own steps below did not run" />
              </>
            )}
            {(() => {
              // A rule between the preparation and the probe — or the first
              // check when the scenario has no probe of its own. A capture
              // belongs to whichever side it is on, so it never starts the live part.
              const steps = scenario.steps ?? []
              const kinds = steps.map((s) => stepKind(s))
              const firstLive = kinds.includes('probe') ? kinds.indexOf('probe') : kinds.indexOf('check')
              return steps.map((s, i) => (
                <Fragment key={i}>
                  {i === firstLive && i > 0 && <div className="steps-rule" role="separator" aria-label="preparation above, probe and checks below" />}
                  <StepRow step={s} scenarioName={scenario.name} index={pickleIndex(background, i)} runState={runState} />
                </Fragment>
              ))
            })()}
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
