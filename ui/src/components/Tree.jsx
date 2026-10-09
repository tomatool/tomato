import { ResourceIcon } from '../lib/icons'

function Caret({ open }) {
  return <span className="tree-caret"><i className="ico-chev" data-open={open ? 'true' : undefined} /></span>
}

function Row({ kind, depth, selected, changed, pip, icon, name, meta, onClick }) {
  return (
    <button
      className="tree-row"
      type="button"
      data-kind={kind}
      style={{ '--depth': depth }}
      aria-selected={selected ? 'true' : undefined}
      data-changed={changed ? 'true' : undefined}
      onClick={onClick}
    >
      {icon ?? <span className="tree-caret" />}
      {pip != null && <i className="pip" data-status={pip} />}
      <span className="tree-name">{name}</span>
      {meta && <span className="tree-meta">{meta}</span>}
    </button>
  )
}

export function Tree({
  features, config, view, selectedFile, selectedScenario, scStatus, changed,
  openDirs, onToggleDir, onSelectConfig, onSelectFile, onSelectScenario,
  featureStatus, visibleScenarios,
}) {
  const byDir = new Map()
  for (const f of features) {
    const parts = f.filePath.split('/')
    parts.pop()
    const dir = parts.join('/')
    if (!byDir.has(dir)) byDir.set(dir, [])
    byDir.get(dir).push(f)
  }

  return (
    <div className="tree" role="tree">
      <Row
        kind="config"
        depth={0}
        selected={view === 'config'}
        icon={<ResourceIcon type="app" />}
        name={config?.path ?? 'tomato.yml'}
        meta={config?.resources ? `${config.resources.length} res` : ''}
        onClick={onSelectConfig}
      />
      <div className="tree-sep" />

      {[...byDir.keys()].sort().map((dir) => {
        const open = openDirs[dir] !== false
        return (
          <div key={dir || '.'}>
            {dir && (
              <Row
                kind="dir"
                depth={0}
                icon={<Caret open={open} />}
                name={`${dir.replace(/^\.\//, '')}/`}
                onClick={() => onToggleDir(dir)}
              />
            )}
            {open && byDir.get(dir).map((f) => {
              const expanded = selectedFile === f.filePath
              return (
                <div key={f.filePath}>
                  <Row
                    kind="feature"
                    depth={dir ? 1 : 0}
                    selected={expanded && view === 'feature'}
                    changed={changed.has(f.filePath)}
                    icon={<Caret open={expanded} />}
                    pip={featureStatus(f)}
                    name={f.name || f.filePath}
                    meta={String((f.scenarios ?? []).length)}
                    onClick={() => onSelectFile(f.filePath)}
                  />
                  {expanded && visibleScenarios(f).map((s, i) => (
                    <Row
                      key={s.name}
                      kind="scenario"
                      depth={(dir ? 1 : 0) + 1}
                      selected={view === 'feature' && selectedFile === f.filePath && selectedScenario === i}
                      pip={scStatus[s.name] ?? ''}
                      name={s.name}
                      onClick={() => onSelectScenario(f.filePath, i)}
                    />
                  ))}
                </div>
              )
            })}
          </div>
        )
      })}
    </div>
  )
}
