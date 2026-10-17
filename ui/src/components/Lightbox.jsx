import { useEffect, useRef } from 'react'
import { fmtBytes, when } from '../lib/format'
import { shotPlace, takenBy } from '../lib/screenshots.js'
import { SourcePath } from './SourcePath.jsx'

// One screenshot at full size, over the page. Esc closes it and the arrow
// keys step through the list: the App's key handler sends those here while
// it is open, so the transport underneath does not move with them.
export function Lightbox({ shots, index, runState, onClose, onMove }) {
  const shot = shots[index]
  const closeRef = useRef(null)

  // Focus comes to the dialog and goes back where it was when it closes.
  useEffect(() => {
    const before = document.activeElement
    closeRef.current?.focus()
    return () => before?.focus?.()
  }, [])

  if (!shot) return null
  const by = takenBy(runState, shot.path)
  const last = index >= shots.length - 1

  return (
    <div className="lightbox" role="dialog" aria-modal="true" aria-label={`Screenshot ${shot.name}`} onClick={onClose}>
      <div className="lightbox-frame" onClick={(e) => e.stopPropagation()}>
        <div className="lightbox-head">
          <span className="lightbox-name">{shot.name}</span>
          {shots.length > 1 && <span className="lightbox-pos">{index + 1} / {shots.length}</span>}
          <span style={{ flex: 1 }} />
          <button className="btn btn--ghost btn--sm btn--icon" type="button" title="Previous  ←" disabled={index <= 0} onClick={() => onMove(-1)}>
            <i className="ico-chev" data-dir="left" />
          </button>
          <button className="btn btn--ghost btn--sm btn--icon" type="button" title="Next  →" disabled={last} onClick={() => onMove(1)}>
            <i className="ico-chev" />
          </button>
          <button className="btn btn--ghost btn--sm" type="button" ref={closeRef} onClick={onClose}>
            Close <span className="kbd">esc</span>
          </button>
        </div>
        <div className="lightbox-body">
          <img className="lightbox-img" src={shot.url} alt={`Screenshot ${shot.name}`} />
        </div>
        <div className="lightbox-foot">
          {by ? (
            <span className="lightbox-for" data-status={by.status || undefined}>
              <i className="pip" data-status={by.status} />
              <span>{by.scenario}</span>
              <span className="muted">· {by.resource}</span>
            </span>
          ) : (
            shot.dir && <span className="muted">{shotPlace(shot.dir)}</span>
          )}
          {shot.modTime && <span className="muted">{when(shot.modTime)} · {fmtBytes(shot.size)}</span>}
          <SourcePath path={shot.absPath ?? shot.path} label="screenshot" />
        </div>
      </div>
    </div>
  )
}
