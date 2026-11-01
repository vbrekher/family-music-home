import React, { useEffect, useState } from 'react'
import { Button, Typography, Link } from '@material-ui/core'
import { call } from './common'
export default function Notifications() {
  const [items, setItems] = useState([]),
    [error, setError] = useState('')
  const load = () =>
    call('/notifications')
      .then(setItems)
      .catch(() => setError('Updates unavailable. Refresh Home to retry.'))
  useEffect(() => {
    load()
  }, [])
  const unread = items.filter((x) => !x.read).slice(0, 10)
  return (
    <section
      aria-label="Family inbox updates"
      style={{ marginBottom: unread.length || error ? 24 : 0 }}
    >
      {!!unread.length && (
        <details>
          <summary
            style={{
              minHeight: 44,
              cursor: 'pointer',
              padding: '12px 0',
              boxSizing: 'border-box',
              fontWeight: 600,
            }}
          >
            Family inbox updates ({unread.length})
          </summary>
          {unread.map((x) => (
            <div
              key={x.id}
              style={{
                display: 'flex',
                alignItems: 'center',
                flexWrap: 'wrap',
                gap: 8,
                marginBottom: 8,
              }}
            >
              <Link
                style={{
                  flex: '1 1 180px',
                  minWidth: 0,
                  overflowWrap: 'anywhere',
                }}
                color="inherit"
                href={`#/family/${x.kind === 'request' ? 'requests' : 'letters'}/${x.entryId}`}
              >
                {x.message}
              </Link>
              <Button
                onClick={async () => {
                  try {
                    await call(`/notifications/${x.id}/read`, {})
                    await load()
                  } catch (_) {
                    setError('Could not mark update read.')
                  }
                }}
              >
                Mark update read
              </Button>
            </div>
          ))}
        </details>
      )}
      <Typography role="status">{error}</Typography>
    </section>
  )
}
