// SGR parameter -> the design system's console classes. The stylesheet owns
// the palette, so nothing here emits a colour.
const CLASS = {
  1: 'c-bold', 2: 'c-dim', 30: 'c-dim', 90: 'c-dim',
  31: 'c-red', 91: 'c-red', 32: 'c-green', 92: 'c-green',
  33: 'c-yellow', 93: 'c-yellow', 34: 'c-blue', 94: 'c-blue',
  35: 'c-magenta', 95: 'c-magenta', 36: 'c-cyan', 96: 'c-cyan',
}

const SEQ = /\x1b\[([0-9;]*)m|\[([0-9;]*)m/g

// ansiToNodes turns one line into spans. Unterminated sequences simply end
// with the line instead of bleeding into whatever is appended next.
export function ansiToNodes(line) {
  const out = []
  let open = []
  let last = 0
  let m
  SEQ.lastIndex = 0
  const push = (text, key) => {
    if (!text) return
    out.push(open.length ? <span className={open.join(' ')} key={key}>{text}</span> : text)
  }
  while ((m = SEQ.exec(line)) !== null) {
    push(line.slice(last, m.index), `t${m.index}`)
    last = m.index + m[0].length
    const params = String(m[1] ?? m[2] ?? '')
    const codes = params.split(';').filter(Boolean)
    if (!codes.length || codes.includes('0') || codes.includes('00')) {
      open = []
      continue
    }
    const next = codes.map((c) => CLASS[c]).filter(Boolean)
    if (next.length) open = next
  }
  push(line.slice(last), 'tail')
  return out
}
