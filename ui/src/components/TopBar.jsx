import { fmtDur } from '../lib/format'
import { describeVersion } from '../lib/version'

const THEME_LABEL = { dark: 'Switch to the light theme', light: 'Switch to the dark theme' }

export function TopBar({
  theme, onToggleTheme, tomato,
  crumb, live, liveText, scenarios, scStatus, scDur, running,
  onRunAll, onRunFailed, onStop,
}) {
  const version = describeVersion(tomato)
  let passed = 0; let failed = 0; let undef = 0; let reported = 0
  for (const s of scenarios) {
    const st = scStatus[s.name]
    if (st === 'passed') passed += 1
    else if (st === 'failed') failed += 1
    else if (st === 'undefined') undef += 1
    if (st) reported += 1
  }
  const elapsed = Object.values(scDur).reduce((a, b) => a + b, 0)

  return (
    <header className="topbar">
      <span className="brand"><i className="brand-mark" />tomato</span>
      {version && (
        <span className="brand-version" title={version.title} data-dev={version.dev || undefined}>
          {version.label}
        </span>
      )}
      <button
        className="btn btn--ghost btn--icon"
        type="button"
        data-mode={theme}
        title={THEME_LABEL[theme]}
        aria-label={THEME_LABEL[theme]}
        onClick={onToggleTheme}
      >
        <i className="ico-theme" />
      </button>
      <span className="topbar-sep" />
      <span className="crumb">{crumb}</span>
      <span className="topbar-spacer" />
      <span className="live" data-state={live}>{liveText}</span>
      <span className="topbar-sep" />
      <div className="runbar" style={{ width: 140 }}>
        {scenarios.map((s) => (
          <i className="runbar-seg" key={s.name} data-status={scStatus[s.name] ?? ''} />
        ))}
      </div>
      <span className="tally">
        {passed > 0 && <span className="tally-item"><i className="pip" data-status="passed" /><b>{passed}</b></span>}
        {failed > 0 && <span className="tally-item"><i className="pip" data-status="failed" /><b>{failed}</b></span>}
        {undef > 0 && <span className="tally-item"><i className="pip" data-status="undefined" /><b>{undef}</b></span>}
        {reported === 0 && <span className="tally-item muted">{scenarios.length} scenarios</span>}
        {elapsed > 0 && <span>{fmtDur(elapsed)}</span>}
      </span>
      {!running && failed > 0 && (
        <button className="btn" type="button" onClick={onRunFailed}>Run failed</button>
      )}
      {running ? (
        <button className="btn btn--danger" type="button" onClick={onStop}>
          <i className="ico-stop" />Stop <span className="kbd">esc</span>
        </button>
      ) : (
        <button className="btn btn--primary" type="button" onClick={onRunAll}>
          <i className="ico-play" />Run all <span className="kbd">R</span>
        </button>
      )}
    </header>
  )
}
