import { useEffect, useRef, useState } from 'react'

// One WebSocket to the server, reconnecting on close. Messages are handed to
// `onMessage`; the hook only owns the connection and its state.
export function useSocket(onMessage) {
  const [state, setState] = useState('offline')
  const handler = useRef(onMessage)
  handler.current = onMessage

  useEffect(() => {
    let socket
    let timer
    let closed = false

    const connect = () => {
      const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
      socket = new WebSocket(`${proto}//${location.host}/ws`)
      socket.onopen = () => setState('online')
      socket.onerror = () => setState('error')
      socket.onclose = () => {
        if (closed) return
        setState('offline')
        timer = setTimeout(connect, 2000)
      }
      socket.onmessage = (ev) => {
        let msg
        try { msg = JSON.parse(ev.data) } catch { return }
        handler.current?.(msg)
      }
    }

    connect()
    return () => {
      closed = true
      clearTimeout(timer)
      socket?.close()
    }
  }, [])

  return state
}
