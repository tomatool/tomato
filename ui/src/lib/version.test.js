import { describe, expect, it } from 'vitest'
import { describeVersion } from './version.js'

describe('describeVersion', () => {
  it('shows nothing until the server has answered', () => {
    expect(describeVersion(null)).toBeNull()
    expect(describeVersion({})).toBeNull()
  })

  it('names a release with its commit and build date on the title', () => {
    const v = describeVersion({ version: '2.4.0', commit: 'c54a86df66f5d2c411b1e6b68e217a3759760e03', built: '2026-10-13T13:04:56Z' })
    expect(v.label).toBe('v2.4.0')
    expect(v.dev).toBe(false)
    expect(v.title).toBe('tomato 2.4.0 \u00b7 commit c54a86df66f5 \u00b7 built 2026-10-13T13:04:56Z')
  })

  it('does not double a leading v', () => {
    expect(describeVersion({ version: 'v2.4.0' }).label).toBe('v2.4.0')
  })

  it('calls a build without linker flags a dev build and hides the unknowns', () => {
    const v = describeVersion({ version: 'dev', commit: 'unknown', built: 'unknown' })
    expect(v.label).toBe('dev build')
    expect(v.dev).toBe(true)
    expect(v.title).toBe('tomato dev')
  })
})
