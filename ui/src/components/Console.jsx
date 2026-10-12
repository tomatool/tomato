import { useEffect, useRef } from 'react'
import { ansiToNodes } from '../lib/ansi.jsx'
import { fmtClock } from '../lib/format'
import { SourcePath } from './SourcePath.jsx'

export function Console({ lines, logTabs, activeLog, runId, running, height, open = true, onToggle, onTab, onResizeStart }) {
  const body = useRef(null)
  const stuck = useRef(true)

  useEffect(() => {
    const node = body.current
    if (node && stuck.current) node.scrollTop = node.scrollHeight
  }, [lines, activeLog])

  const onScroll = () => {
    const node = body.current
    if (node) stuck.current = node.scrollTop + node.clientHeight >= node.scrollHeight - 8
  }

  return (
    <section className="pane pane--console" data-open={open ? 'true' : 'false'}>
      {open && <div className="resize-y" onMouseDown={onResizeStart} />}
      <div className="tabs">
        <button
          className="tab" type="button"
          aria-selected={activeLog ? undefined : 'true'}
          onClick={() => onTab(null)}
        >
          Output
        </button>
        {logTabs.map((t) => (
          <button
            className="tab" type="button" key={`${t.run}/${t.name}`}
            aria-selected={activeLog?.run === t.run && activeLog?.name === t.name ? 'true' : undefined}
            onClick={() => onTab(t)}
          >
            {t.name}
          </button>
        ))}
        {activeLog?.absPath && <SourcePath path={activeLog.absPath} label="log file" />}
        <span style={{ flex: 1 }} />
        {runId && (
          <span className="live" data-state={running ? undefined : 'offline'} style={{ alignSelf: 'center' }}>
            run {runId}
          </span>
        )}
        <button
          className="btn btn--ghost btn--sm btn--icon" type="button" style={{ alignSelf: 'center' }}
          title={open ? 'Minimise output  -' : 'Show output  -'}
          aria-expanded={open ? 'true' : 'false'}
          onClick={onToggle}
        >
          <i className="ico-chev" data-open={open ? 'true' : undefined} />
        </button>
      </div>
      {open && (
      <div className="console" ref={body} style={{ height }} onScroll={onScroll}>
        {lines.length === 0 ? (
          <div className="line">
            <span className="line-time" />
            <span className="c-dim">
              Run output appears here. Press R to run everything, or r to run the focused scenario.
            </span>
          </div>
        ) : lines.map((l, i) => (
          <div className="line" key={i} data-status={l.status || undefined}>
            <span className="line-time">{l.at == null ? '' : fmtClock(l.at)}</span>
            <span>{ansiToNodes(l.text)}</span>
          </div>
        ))}
      </div>
      )}
    </section>
  )
}
