"""Checks the profile tomato reported for tests/testdata/profile: reads a
`tomato run --format tomato` from stdin and prints "profile ok", or what is
wrong and exits 1. Used by tests/features/profile.feature."""
import json
import sys

profiles, samples = {}, 0
for line in sys.stdin:
    if "TOMATO_EVENT:" not in line:
        continue
    event = json.loads(line.split("TOMATO_EVENT:", 1)[1])
    samples += len(event.get("samples") or [])
    if event.get("type") == "scenario_end" and event.get("profile"):
        profiles[event["scenario"]] = event["profile"]

problems = []


def check(ok, what):
    if not ok:
        problems.append(what)


MB = 1 << 20
for name in ("burns CPU", "holds memory", "logs an error", "does nothing much"):
    check(name in profiles and profiles[name].get("app"), f"no profile for {name!r}")
if not problems:
    cpu, mem, log, idle = (profiles[n] for n in ("burns CPU", "holds memory", "logs an error", "does nothing much"))
    check(300 <= cpu["app"]["cpuMs"] <= 1500, f"burns CPU: {cpu['app']['cpuMs']}ms of CPU, want about 400")
    check(idle["app"]["cpuMs"] < 200, f"does nothing much: {idle['app']['cpuMs']}ms of CPU")
    grew = mem["app"]["rssEnd"] - mem["app"]["rssStart"]
    check(grew >= 80 * MB, f"holds memory: grew {grew // MB} MB, want about 96")
    check(mem["app"]["peakRss"] >= mem["app"]["rssEnd"], "holds memory: peak below the end")
    logs = log.get("logs") or {}
    check(logs.get("errors") == 1 and logs.get("warnings") == 1, f"logs an error: {logs}")
    check(not cpu.get("logs"), f"burns CPU counted log lines: {cpu.get('logs')}")
    check(all(p["durationMs"] >= 0 and p["end"] >= p["start"] for p in profiles.values()), "a window ends before it starts")
check(samples > 0, "no samples")

if problems:
    print("\n".join(problems))
    sys.exit(1)
print("profile ok")
