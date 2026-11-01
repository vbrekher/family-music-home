import React, { useState } from 'react'
import { Title } from 'react-admin'
import {
  Button,
  Typography,
  TextField,
  MenuItem,
  Checkbox,
  FormControlLabel,
  Card,
  CardContent,
} from '@material-ui/core'
import { call, useFamilyStyles } from './common'
import { mediaFetch } from './Media'
import FamilyNav from './FamilyNav'
export default function Legacy() {
  const c = useFamilyStyles(),
    [options, setOptions] = useState({
      scope: 'personal',
      from: '',
      to: '',
      attachments: true,
      archived: true,
      catalog: true,
      playlists: true,
    }),
    [job, setJob] = useState(null),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(false),
    [verification, setVerification] = useState(null)
  const prepare = async () => {
    setBusy(true)
    setError('')
    setJob(null)
    try {
      setJob(await call('/legacy/prepare', options))
    } catch (_) {
      setError(
        'Could not prepare an archive. Check the dates or select a smaller scope.',
      )
    } finally {
      setBusy(false)
    }
  }
  const download = async () => {
    setBusy(true)
    setError('')
    try {
      const response = await mediaFetch(`/legacy/${job.id}/download`)
      if (!response.ok) throw new Error()
      const blob = await response.blob()
      const url = URL.createObjectURL(blob),
        a = document.createElement('a')
      a.href = url
      a.download = 'family-history.zip'
      document.body.appendChild(a)
      a.click()
      a.remove()
      setTimeout(() => URL.revokeObjectURL(url), 1000)
    } catch (_) {
      setError(
        'Download could not finish. The link may have expired or the selected scope may be too large. Prepare a smaller archive and verify the downloaded file.',
      )
    } finally {
      setBusy(false)
    }
  }
  const verify = async (file) => {
    if (!file) return
    setBusy(true)
    setError('')
    setVerification(null)
    try {
      if (file.size > 192 * 1024 * 1024) throw new Error()
      const response = await mediaFetch('/legacy/verify', {
        method: 'POST',
        body: file,
        headers: { 'Content-Type': 'application/zip' },
      })
      if (!response.ok) throw new Error()
      setVerification(await response.json())
    } catch (_) {
      setError(
        'Archive verification failed. The file may be incomplete, altered or an unsupported format. No family data was imported.',
      )
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className={c.page}>
      <Title title="Family Legacy Archive" />
      <FamilyNav />
      <Typography variant="h4" gutterBottom>
        Save your family music history
      </Typography>
      <Typography paragraph>
        Keep memories, letters, shelves, requests, personal choices and
        permitted attachments in portable JSON, Markdown and M3U files. Your own
        original music stays in its library.
      </Typography>
      <Typography role="status">{error}</Typography>
      <Card>
        <CardContent>
          <Typography variant="h6">Choose what to save</Typography>
          <TextField
            fullWidth
            select
            label="Archive scope"
            id="family-legacy-scope"
            value={options.scope}
            onChange={(e) => setOptions({ ...options, scope: e.target.value })}
          >
            <MenuItem value="personal">My own content</MenuItem>
            <MenuItem value="allowed-shared">
              My content and content explicitly available to me
            </MenuItem>
          </TextField>
          <Typography variant="body2">
            Dates apply to creation of family entries. Optional listening
            summaries and playlists use their current permitted state.
          </Typography>
          <TextField
            type="date"
            label="Entries created from (UTC)"
            id="family-legacy-from"
            InputLabelProps={{ shrink: true }}
            value={options.from}
            onChange={(e) => setOptions({ ...options, from: e.target.value })}
          />
          <TextField
            type="date"
            label="Entries created through (UTC)"
            id="family-legacy-to"
            InputLabelProps={{ shrink: true }}
            value={options.to}
            onChange={(e) => setOptions({ ...options, to: e.target.value })}
          />
          {[
            ['attachments', 'Include permitted photos and voice notes'],
            ['archived', 'Include archived entries'],
            ['catalog', 'Include my favorites and listening summaries'],
            ['playlists', 'Include permitted current playlists'],
          ].map(([key, label]) => (
            <FormControlLabel
              key={key}
              control={
                <Checkbox
                  checked={options[key]}
                  onChange={(e) =>
                    setOptions({ ...options, [key]: e.target.checked })
                  }
                />
              }
              label={label}
            />
          ))}
          <Typography paragraph>
            This private ZIP is not encrypted. Keep it in encrypted storage. It
            contains no original audio or server credentials. Playlist paths are
            relocation references for your own music.
          </Typography>
          <Button
            disabled={busy}
            variant="contained"
            color="primary"
            onClick={prepare}
          >
            Prepare private archive
          </Button>
          {job && (
            <>
              <Typography>
                Ready for this account until{' '}
                {new Date(job.expiresAt).toLocaleString()}. Permissions are
                checked again when you download.
              </Typography>
              <Button disabled={busy} onClick={download}>
                Download archive
              </Button>
            </>
          )}
        </CardContent>
      </Card>
      <Card>
        <CardContent>
          <Typography variant="h6">Verify a saved archive</Typography>
          <Typography paragraph>
            Check the schema and every file hash without importing or changing
            family data. Integrity does not prove who created a file.
          </Typography>
          <Button component="label" disabled={busy}>
            Choose ZIP to verify
            <input
              type="file"
              accept=".zip,application/zip"
              aria-label="Archive to verify"
              style={{ position: 'absolute', width: 1, height: 1, opacity: 0 }}
              onChange={(e) => {
                verify(e.target.files?.[0])
                e.target.value = ''
              }}
            />
          </Button>
          {verification?.verified && (
            <Typography role="status">
              Verified: {verification.entries} family records,{' '}
              {verification.files} files. The ZIP is unencrypted. No data was
              imported.
            </Typography>
          )}
          <Typography variant="body2">
            Complete account and server restoration uses the separately
            documented encrypted backup of the database and private media. This
            archive is a portable personal record, with read-only verification.
          </Typography>
        </CardContent>
      </Card>
    </div>
  )
}
