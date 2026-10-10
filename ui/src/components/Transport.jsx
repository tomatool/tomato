// Walks a scenario step by step; Flow and Topology share the cursor.
export function Transport({ msgs, cursor, playing, onSeek, onPlay, onReplay }) {
  if (!msgs.length) return null
  const cur = cursor >= 0 ? msgs[cursor] : null

  return (
    <div className="transport">
      <div className="transport-row">
        <button
          className="btn btn--ghost btn--sm btn--icon" type="button" title="Previous step  ←"
          disabled={cursor < 0} onClick={() => onSeek(cursor - 1)}
        >
          <i className="ico-step" data-dir="back" />
        </button>
        <button
          className="btn btn--sm btn--icon" type="button" title="Play / pause  space"
          aria-pressed={playing ? 'true' : undefined} onClick={onPlay}
        >
          <i className={playing ? 'ico-pause' : 'ico-play'} />
        </button>
        <button
          className="btn btn--ghost btn--sm btn--icon" type="button" title="Next step  →"
          disabled={cursor >= msgs.length - 1} onClick={() => onSeek(cursor + 1)}
        >
          <i className="ico-step" />
        </button>
        <button
          className="btn btn--ghost btn--sm btn--icon" type="button" title="Replay from step 1"
          onClick={onReplay}
        >
          <i className="ico-replay" />
        </button>
        <div className="transport-cap">
          <span className="transport-kw" data-phase={cur?.phase || undefined}>{cur?.kw ?? ''}</span>
          <span className="transport-text">
            {cur ? cur.text : 'Press play to walk through the scenario'}
          </span>
        </div>
        <span className="transport-pos">{cursor + 1}/{msgs.length}</span>
      </div>
      <div className="scrub">
        {msgs.map((m, i) => (
          <button
            className="scrub-tick"
            type="button"
            key={m.n}
            data-state={cursor < 0 ? undefined : i < cursor ? 'past' : i === cursor ? 'current' : 'future'}
            data-status={m.status || undefined}
            title={`${m.kw} ${m.text}`}
            onClick={() => onSeek(i)}
          >
            {m.n}
          </button>
        ))}
      </div>
    </div>
  )
}
