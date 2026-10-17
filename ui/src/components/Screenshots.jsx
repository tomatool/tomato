import { fmtBytes, when } from '../lib/format'
import { shotPlace, takenBy } from '../lib/screenshots.js'

// The inspector's Screenshots tab: every picture the browser resources left
// behind, newest first. A row says which step of this run took it when one
// did, and a click opens it at full size (Lightbox).
export function Screenshots({ shots, runState, onOpen }) {
  if (!shots.length) {
    return (
      <div className="empty">
        <h3>No screenshots yet</h3>
        <p>
          <code>"alice" saves screenshot "name"</code> keeps one, and a step that fails while a
          page is open leaves a picture of that page. They show up here as they are written.
        </p>
      </div>
    )
  }
  return (
    <div className="shots">
      {shots.map((s) => {
        const by = takenBy(runState, s.path)
        return (
          <button className="shot" type="button" key={s.path} title={s.path} onClick={() => onOpen(s.path)}>
            <img className="shot-thumb" src={s.url} alt="" loading="lazy" decoding="async" />
            <span className="shot-main">
              <span className="shot-name">{s.name}</span>
              <span className="shot-meta">{when(s.modTime)} · {fmtBytes(s.size)} · {shotPlace(s.dir)}</span>
              {by && (
                <span className="shot-for" data-status={by.status || undefined}>
                  <i className="pip" data-status={by.status} />
                  <span className="shot-for-text">{by.scenario}</span>
                  <span className="shot-for-res">{by.resource}</span>
                </span>
              )}
            </span>
          </button>
        )
      })}
    </div>
  )
}
