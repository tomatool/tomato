// Which part of a scenario a step is: the preparation, the probe that
// exercises the app, the check that judges what happened, or a capture that
// keeps a value for later steps. It is a pure function of what the step's
// definition says it does — its group and description, which the server
// sends with every matched step — and the keyword the author chose, so the
// same feature reads the same way everywhere.

export const KINDS = ['setup', 'probe', 'check', 'capture']
export const KIND_LABEL = { setup: 'setup', probe: 'probe', check: 'check', capture: 'capture' }

// Groups whose every definition is one kind, as the resources name them in
// their Steps(). A group that mixes kinds — "Publishing" sets a header and
// publishes, "Consuming" subscribes and waits — is left out and decided by
// the description's verb instead.
const GROUPS = {
  'Request Setup': 'setup', 'Stub Setup': 'setup', 'Data Setup': 'setup', 'Setup': 'setup',
  'Bucket Setup': 'setup', 'Object Setup': 'setup', 'Bindings': 'setup',
  'Request Execution': 'probe', 'Execution': 'probe', 'Sending': 'probe', 'Broadcast': 'probe',
  'Response Status': 'check', 'Response Headers': 'check', 'Response Body': 'check', 'Response JSON': 'check',
  'Response Timing': 'check', 'Verification': 'check', 'Assertions': 'check', 'String Assertions': 'check',
  'Database': 'check', 'Exit Code': 'check', 'Output': 'check', 'Files': 'check', 'Receiving': 'check',
  'Bucket Assertions': 'check', 'Object Assertions': 'check', 'Service Discovery': 'check', 'Identity': 'check',
  'Variable Capture': 'capture', 'Capture': 'capture', 'Server Info': 'capture',
  'Navigation': 'probe', 'Using the Page': 'probe', 'What the Page Shows': 'check',
}

// The first word of a definition's description, for the mixed groups.
const VERBS = {
  setup: ['set', 'sets', 'create', 'creates', 'declare', 'declares', 'bind', 'binds', 'purge', 'purges',
    'insert', 'inserts', 'truncate', 'truncates', 'delete', 'deletes', 'push', 'pushes', 'add', 'adds',
    'register', 'registers', 'start', 'starts', 'join', 'joins', 'increment', 'increments', 'decrement',
    'decrements', 'write', 'writes', 'upload', 'uploads', 'seed', 'seeds', 'execute', 'executes', 'replies'],
  probe: ['send', 'sends', 'run', 'runs', 'call', 'calls', 'publish', 'publishes', 'broadcast', 'broadcasts',
    'connect', 'connects', 'disconnect', 'disconnects'],
  check: ['assert', 'asserts', 'verify', 'verifies', 'wait', 'waits', 'expect', 'expects'],
  capture: ['save', 'saves', 'store', 'stores'],
}
const BY_VERB = Object.fromEntries(KINDS.flatMap((k) => VERBS[k].map((v) => [v, k])))

// definitionKind is what the definition does on its own, before the
// scenario's keyword is taken into account; '' when nothing is known.
export function definitionKind(step) {
  if (!step || step.unmatched) return ''
  if (GROUPS[step.group]) return GROUPS[step.group]
  const verb = String(step.description || '').trim().split(/\s+/)[0]?.toLowerCase().replace(/[^a-z]/g, '')
  return BY_VERB[verb] || ''
}

// stepKind is the kind in context. A Background is preparation whatever it
// contains. A probe under a Given is preparation too — `Given "api" sends
// "POST" to "/users"` creates the fixture the scenario is about, and the
// author said so with the keyword — while under a When it is the probe.
// A check stays a check under any keyword.
export function stepKind(step, { background = false } = {}) {
  const kind = definitionKind(step)
  if (!kind) return ''
  if (background) return 'setup'
  if (kind === 'probe' && step.phase === 'given') return 'setup'
  return kind
}
