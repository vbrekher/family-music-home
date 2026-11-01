import React, { useEffect, useState } from 'react'
import { Button, Typography } from '@material-ui/core'
import { baseUrl } from '../utils'
import { call } from './common'

// Use the same protected native authentication header as the JSON client.
export async function mediaFetch(path, options = {}) {
  const response = await fetch(baseUrl('/api/family' + path), {
    ...options,
    headers: {
      'X-ND-Authorization': `Bearer ${localStorage.getItem('token') || ''}`,
    },
    credentials: 'same-origin',
  })
  if (!response.ok) throw new Error('Private media unavailable')
  return response
}
export function PrivateMedia({ item }) {
  const [url, setUrl] = useState(''),
    [error, setError] = useState(''),
    [full, setFull] = useState(false)
  const photo = item.mime.startsWith('image/')
  useEffect(() => {
    if (!photo && !full) return
    let active = true,
      objectURL
    mediaFetch(`/attachments/${item.id}${photo && !full ? '?thumb=true' : ''}`)
      .then((r) => r.blob())
      .then((b) => {
        objectURL = URL.createObjectURL(b)
        if (active) setUrl(objectURL)
      })
      .catch(
        () =>
          active &&
          setError('Attachment unavailable. Retry by reopening this entry.'),
      )
    return () => {
      active = false
      if (objectURL) URL.revokeObjectURL(objectURL)
    }
  }, [item.id, photo, full])
  return (
    <div style={{ margin: '12px 0' }}>
      {photo && url && (
        <img
          src={url}
          alt="Private family photograph"
          style={{ maxWidth: full ? '100%' : 128, maxHeight: full ? 600 : 128 }}
          loading="lazy"
        />
      )}
      {photo ? (
        <Button onClick={() => setFull(!full)}>
          {full ? 'Small photograph' : 'Open photograph'}
        </Button>
      ) : full && url ? (
        <audio
          controls
          preload="metadata"
          src={url}
          aria-label="Private voice note"
        />
      ) : (
        <Button onClick={() => setFull(true)}>Load voice note</Button>
      )}
      <Typography variant="caption">
        {item.hidden ? 'Withdrawn · ' : ''}
        {Math.ceil(item.size / 1024)} KiB
      </Typography>
      <Typography role="status">{error}</Typography>
    </div>
  )
}
export default function Media({ entry, plural, onChange }) {
  const [items, setItems] = useState([]),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(false)
  const load = () =>
    call(`/${plural}/${entry.id}/attachments?hidden=true`)
      .then(setItems)
      .catch(() => setError('Attachments could not be loaded.'))
  useEffect(() => {
    load()
  }, [entry.id])
  const upload = async (file) => {
    if (!file) return
    if (file.size > 5 * 1024 * 1024) {
      setError('Choose a file smaller than 5 MiB.')
      return
    }
    setBusy(true)
    setError('')
    try {
      await mediaFetch(`/${plural}/${entry.id}/attachments`, {
        method: 'POST',
        body: file,
      })
      await load()
    } catch (_) {
      setError(
        'Upload rejected. Use a JPEG/PNG photograph (up to 20 megapixels) or a valid MP3/WAV/M4A voice note up to 120 seconds. Account quota: 32 MiB; family: 128 MiB.',
      )
    } finally {
      setBusy(false)
    }
  }
  return (
    <section aria-label="Private attachments">
      <Typography variant="h6">Photographs & voice notes</Typography>
      {items.map((item) => (
        <div key={item.id}>
          <PrivateMedia item={item} />
          {entry.canEdit && (
            <Button
              disabled={busy}
              onClick={async () => {
                try {
                  await call(`/attachments/${item.id}/visibility`, {
                    hidden: !item.hidden,
                  })
                  await load()
                  onChange?.()
                } catch (_) {
                  setError('Could not change attachment visibility.')
                }
              }}
            >
              {item.hidden ? 'Restore attachment' : 'Withdraw attachment'}
            </Button>
          )}
          {entry.canEdit &&
            plural === 'capsules' &&
            !item.hidden &&
            item.mime.startsWith('image/') && (
              <Button disabled={busy} onClick={() => onChange?.(item.id)}>
                Use photograph as cover
              </Button>
            )}
        </div>
      ))}
      {entry.canEdit &&
        (plural === 'capsules' || entry.body.state === 'draft') && (
          <>
            <Button component="label" disabled={busy}>
              Add photograph or voice note
              <input
                type="file"
                hidden
                accept="image/jpeg,image/png,audio/mpeg,audio/wav,audio/mp4,.m4a"
                onChange={(e) => {
                  upload(e.target.files?.[0])
                  e.target.value = ''
                }}
              />
            </Button>
            <Typography variant="body2">
              Private to this entry’s readers. Photos are resized and metadata
              removed. Maximum 24 attachments; 5 MiB each. Voice notes: up to
              120 seconds. Withdrawn media is retained for recovery and counts
              toward quota.
            </Typography>
          </>
        )}
      <Typography role="status">
        {busy ? 'Validating private attachment…' : error}
      </Typography>
    </section>
  )
}
