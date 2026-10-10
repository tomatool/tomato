import { stepKey } from './steps.jsx'

// What a scenario does, lane by lane: one message per step, numbered in step
// order. This is derived from the feature file and tomato.yml — tomato does
// not record the bytes it sent, so a message carries the step and its payload
// rather than a captured request.
export function flowModel(scenario, runState) {
  if (!scenario) return null
  const { stepStatus = {}, stepDur = {}, stepError = {} } = runState

  const lanes = [{ name: 'tomato', type: 'runner', icon: 'tomato', runner: true }]
  const laneOf = {}
  for (const step of scenario.steps ?? []) {
    if (!step.resource || laneOf[step.resource] != null) continue
    laneOf[step.resource] = lanes.length
    lanes.push({ name: step.resource, type: step.resourceType ?? '', icon: step.resourceType })
  }

  const msgs = (scenario.steps ?? []).map((step, i) => {
    const k = stepKey(scenario.name, i)
    const lane = step.resource != null ? laneOf[step.resource] : null
    const words = String(step.text).replace(/"[^"]*"/g, ' ').trim().split(/\s+/)
    const verb = words[0] || 'step'
    return {
      n: i + 1,
      idx: i,
      kind: lane == null ? 'note' : 'call',
      dir: lane == null ? 'none' : 'right',
      a: 1,
      b: lane == null ? 2 : lane + 2,
      status: stepStatus[k] ?? (step.unmatched ? 'undefined' : ''),
      phase: step.phase ?? '',
      kw: String(step.keyword).trim(),
      label: step.resource ? `${verb} · ${step.resource}` : step.text,
      verb: verb.toUpperCase(),
      resource: step.resource ?? null,
      target: step.resource
        ? `${step.resource}${step.resourceType ? ` · ${step.resourceType}` : ''}`
        : 'scenario',
      text: step.text,
      body: step.docString
        || (step.table?.length ? step.table.map((r) => `| ${r.join(' | ')} |`).join('\n') : ''),
      dur: stepDur[k],
      err: stepError[k],
    }
  })

  return { scenario, lanes, msgs }
}
