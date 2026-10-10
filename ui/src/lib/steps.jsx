// Step text is tokenised into elements rather than an HTML string: the first
// quoted token is the resource, later ones are arguments, and {{vars}} and
// <placeholders> are highlighted inside both. Returning nodes means no
// dangerouslySetInnerHTML anywhere, so a step containing markup is just text.

function withVars(text, keyPrefix) {
  const out = []
  const re = /\{\{[^}]*\}\}|<[A-Za-z0-9_][^>]*>/g
  let last = 0
  let m
  while ((m = re.exec(text)) !== null) {
    if (m.index > last) out.push(text.slice(last, m.index))
    out.push(<span className="var" key={`${keyPrefix}v${m.index}`}>{m[0]}</span>)
    last = m.index + m[0].length
  }
  if (last < text.length) out.push(text.slice(last))
  return out
}

export function tokenizeStep(text) {
  const out = []
  const re = /"([^"]*)"/g
  let last = 0
  let quoted = 0
  let m
  while ((m = re.exec(text)) !== null) {
    if (m.index > last) out.push(...withVars(text.slice(last, m.index), `p${m.index}`))
    quoted += 1
    out.push(
      <span className={quoted === 1 ? 'ref' : 'arg'} key={`q${m.index}`}>
        &quot;{withVars(m[1], `q${m.index}`)}&quot;
      </span>,
    )
    last = m.index + m[0].length
  }
  if (last < text.length) out.push(...withVars(text.slice(last), 'tail'))
  return out
}

// Keys for per-step run state. Step text repeats across scenarios, so the
// position is what identifies a step.
export const stepKey = (scenario, i) => `${scenario}\u0000${i}`

export const BACKGROUND_KEY = 'background::feature'
