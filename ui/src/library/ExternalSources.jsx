import React, { useEffect, useState } from 'react'
import { Title, usePermissions } from 'react-admin'
import {
  Card,
  CardContent,
  Typography,
  Button,
  Grid,
  Chip,
  CircularProgress,
} from '@material-ui/core'
import { httpClient } from '../dataProvider'
import { useFamilyStyles } from '../family/common'

export default function ExternalSources() {
  const { permissions } = usePermissions()
  const c = useFamilyStyles()
  const [sources, setSources] = useState([])
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')
  const load = () =>
    httpClient('/api/external-source').then(({ json }) => setSources(json))
  useEffect(() => {
    if (permissions !== 'admin') return
    load().catch(() => setMessage('Sources could not be loaded.'))
    const timer = setInterval(() => load().catch(() => {}), 15000)
    return () => clearInterval(timer)
  }, [permissions])
  const action = async (source, operation) => {
    if (
      operation === 'remove' &&
      !window.confirm(
        'Remove this indexed library association and its catalog entries? Remote music files will remain untouched.',
      )
    )
      return
    setBusy(true)
    setMessage('')
    try {
      await httpClient(
        `/api/external-source/${encodeURIComponent(source.id)}/${operation}`,
        {
          method: 'POST',
          headers: new Headers(
            operation === 'remove' ? { 'X-ND-Confirm-Remove': source.id } : {},
          ),
        },
      )
      await load()
      setMessage(
        operation === 'test' ? 'Connection verified.' : 'Source updated.',
      )
    } catch (_) {
      setMessage(
        'Source unavailable or action refused. Existing music is preserved.',
      )
      await load().catch(() => {})
    } finally {
      setBusy(false)
    }
  }
  if (permissions !== 'admin')
    return <Typography>Administrator access required.</Typography>
  return (
    <div className={c.page}>
      <Title title="External Music Sources" />
      <header className={c.hero}>
        <Typography variant="h4" gutterBottom>
          External Music Sources
        </Typography>
        <Typography paragraph>
          Play music directly from approved S3, SFTP or HTTPS WebDAV sources.
          Music stays at its source. Disabling a source preserves its indexed
          songs and playlists.
        </Typography>
        <Typography paragraph>
          Choose Add library for an approved source, then grant family access in
          Users. Connection details and credentials must first be approved and
          securely configured on the server.
        </Typography>
      </header>
      {busy && <CircularProgress size={24} aria-label="Working" />}
      <Typography role="status">{message}</Typography>
      {!sources.length && (
        <Typography>
          No remote sources configured. Your local libraries remain available.
          To connect a remote collection, follow the storage installation guide.
        </Typography>
      )}
      <Grid container spacing={3} aria-label="External music sources">
        {sources.map((source) => (
          <Grid item xs={12} lg={6} key={source.id}>
            <Card component="section" aria-label={source.id}>
              <CardContent>
                <div className={c.toolbar}>
                  <Typography variant="h6" style={{ flex: 1, minWidth: 0 }}>
                    {source.id}
                  </Typography>
                  <Chip label={source.type.toUpperCase()} size="small" />
                </div>
                <Typography paragraph>
                  {source.enabled ? source.status : 'Disabled'}
                </Typography>
                <Typography variant="body2" color="textSecondary" paragraph>
                  Last successful scan:{' '}
                  {source.lastSuccessfulScan &&
                  !source.lastSuccessfulScan.startsWith('0001')
                    ? new Date(source.lastSuccessfulScan).toLocaleString()
                    : 'Not scanned'}
                </Typography>
                <div className={c.toolbar}>
                  <Button
                    variant="outlined"
                    disabled={busy || !source.enabled}
                    onClick={() => action(source, 'test')}
                  >
                    Test connection
                  </Button>
                  <Button
                    color="primary"
                    variant="contained"
                    disabled={busy || !source.enabled}
                    onClick={() =>
                      action(source, source.libraryId ? 'scan' : 'add')
                    }
                  >
                    {source.libraryId ? 'Scan' : 'Add library'}
                  </Button>
                  <Button
                    disabled={busy}
                    onClick={() =>
                      action(source, source.enabled ? 'disable' : 'enable')
                    }
                  >
                    {source.enabled ? 'Disable' : 'Enable'}
                  </Button>
                </div>
                {!!source.libraryId && (
                  <div className={c.toolbar}>
                    <Button href={`#/library/${source.libraryId}`}>
                      Library settings
                    </Button>
                    <Button
                      disabled={busy}
                      onClick={() => action(source, 'remove')}
                    >
                      Remove association
                    </Button>
                  </div>
                )}
                <Typography variant="caption" color="textSecondary">
                  Read-only connection · Original music stays at its source
                </Typography>
              </CardContent>
            </Card>
          </Grid>
        ))}
      </Grid>
    </div>
  )
}
