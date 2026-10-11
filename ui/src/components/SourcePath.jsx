import { useCallback, useState } from 'react'

// Where a thing on screen actually lives on disk.
//
// A page served over http cannot link to file:// — Chrome blocks the
// navigation and says nothing — so the path is shown and copied instead of
// linked. Copying is the part that matters: it is what gets you to the file
// in an editor or a terminal.
export function SourcePath({ path, label = 'source' }) {
  const [copied, setCopied] = useState(false)

  const copy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(path)
    } catch {
      // Clipboard access can be refused; the path is on screen either way.
      return
    }
    setCopied(true)
    setTimeout(() => setCopied(false), 1200)
  }, [path])

  if (!path) return null

  return (
    <span className="srcpath">
      {/* The text is laid out right-to-left so a long path keeps its tail —
          the filename — visible. That makes the leading "/" a neutral
          character at the paragraph edge, which bidi moves to the far end:
          the path renders as "tmp/x/f.yml/". A left-to-right mark in front
          of it keeps it where it belongs. It is display only; the title and
          the copied value are the plain path. */}
      <code className="srcpath-text" title={path}>{'\u200e' + path}</code>
      <button
        className="btn btn--ghost btn--sm"
        type="button"
        data-copied={copied ? 'true' : undefined}
        aria-label={`Copy the ${label} path`}
        title={`Copy the ${label} path`}
        onClick={copy}
      >
        {copied ? 'copied' : 'copy'}
      </button>
    </span>
  )
}
