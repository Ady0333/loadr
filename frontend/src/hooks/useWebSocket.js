import { useEffect, useRef, useState } from 'react'

// useWebSocket connects to `url` on mount, parses incoming JSON messages, and
// forwards them to `onMessage`. It auto-reconnects on unexpected close. The
// handler is kept in a ref so updating it doesn't tear down the socket.
export function useWebSocket(url, onMessage) {
  const [connected, setConnected] = useState(false)
  const handlerRef = useRef(onMessage)
  handlerRef.current = onMessage

  useEffect(() => {
    let socket
    let reconnectTimer
    let closedByUs = false

    const connect = () => {
      socket = new WebSocket(url)

      socket.onopen = () => setConnected(true)

      socket.onmessage = (event) => {
        try {
          handlerRef.current?.(JSON.parse(event.data))
        } catch {
          // Ignore non-JSON frames (e.g. control messages).
        }
      }

      socket.onclose = () => {
        setConnected(false)
        if (!closedByUs) {
          reconnectTimer = setTimeout(connect, 1500)
        }
      }

      socket.onerror = () => socket.close()
    }

    connect()

    return () => {
      closedByUs = true
      clearTimeout(reconnectTimer)
      socket?.close()
    }
  }, [url])

  return { connected }
}
