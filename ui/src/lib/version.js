// What the top bar says about the tomato serving the page.
//
// A release build has a version like "2.4.0" and a full commit; a `go build`
// without the linker flags says "dev" and "unknown". The label is short, the
// title carries the rest, and nothing is shown until /api/config has answered.

export function describeVersion(tomato) {
  if (!tomato || !tomato.version) return null
  const dev = tomato.version === 'dev'
  const label = dev ? 'dev build' : `v${tomato.version.replace(/^v/, '')}`
  const parts = [`tomato ${tomato.version}`]
  if (tomato.commit && tomato.commit !== 'unknown') parts.push(`commit ${tomato.commit.slice(0, 12)}`)
  if (tomato.built && tomato.built !== 'unknown') parts.push(`built ${tomato.built}`)
  return { label, title: parts.join(' \u00b7 '), dev }
}
