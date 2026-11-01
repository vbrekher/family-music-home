import React, { useEffect, useState } from 'react'
import { Title } from 'react-admin'
import { useParams } from 'react-router-dom'
import {
  Button,
  Card,
  CardContent,
  Typography,
  TextField,
  MenuItem,
  Link,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Checkbox,
  FormControlLabel,
} from '@material-ui/core'
import { call, useFamilyStyles, ReferenceRow, ReferencePicker } from './common'
import FamilyNav from './FamilyNav'
const transitions = {
  requested: ['reviewing', 'declined', 'unavailable', 'cancelled'],
  reviewing: [
    'looking',
    'obtained',
    'added',
    'declined',
    'unavailable',
    'cancelled',
  ],
  looking: ['obtained', 'added', 'declined', 'unavailable', 'cancelled'],
  obtained: ['added', 'unavailable', 'cancelled'],
  unavailable: ['reviewing', 'cancelled'],
  declined: ['reviewing'],
}
export default function Requests() {
  const c = useFamilyStyles(),
    { id } = useParams(),
    [items, setItems] = useState([]),
    [me, setMe] = useState(null),
    [edit, setEdit] = useState(null),
    [kind, setKind] = useState('track'),
    [lines, setLines] = useState(''),
    [preview, setPreview] = useState(null),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(false),
    [offset, setOffset] = useState(0),
    [archived, setArchived] = useState(false),
    [view, setView] = useState('all'),
    [status, setStatus] = useState(null),
    [history, setHistory] = useState([]),
    [historyOffset, setHistoryOffset] = useState(0)
  const load = async () => {
    try {
      const r = await call(
        id
          ? `/requests/${id}`
          : `/requests?offset=${offset}&archived=${archived}&view=${view}`,
      )
      setItems(id ? [r] : r)
      if (id)
        setHistory(
          await call(`/requests/${id}/history?offset=${historyOffset}`),
        )
    } catch (_) {
      setError('Requests could not be loaded. Check access and retry.')
    }
  }
  useEffect(() => {
    call('/me')
      .then(setMe)
      .catch(() => setError('Account unavailable.'))
  }, [])
  useEffect(() => {
    setError('')
    load()
  }, [id, offset, archived, view, historyOffset])
  const action = async (fn) => {
    setBusy(true)
    setError('')
    try {
      const reload = await fn()
      if (reload !== false) await load()
    } catch (e) {
      setError(
        e.status === 409
          ? 'Request changed. Refresh before editing.'
          : 'Could not save. Check fields, access and status transition.',
      )
    } finally {
      setBusy(false)
    }
  }
  const save = () =>
    action(async () => {
      const x = await call(
        edit.id ? `/requests/${edit.id}` : '/requests',
        edit,
        edit.id ? 'PUT' : 'POST',
      )
      setEdit(null)
      if (id !== x.id) {
        window.location.hash = `#/family/requests/${x.id}`
        return false
      }
    })
  return (
    <div className={c.page}>
      <Title title="Family Music Requests" />
      <FamilyNav />
      <Typography variant="h4" gutterBottom>
        Family Music Requests
      </Typography>
      <Typography paragraph>
        Ask for a song, album, artist or an ordered list. The Owner reviews each
        item. Acquisition and uploads remain manual through your existing music
        workflow.
      </Typography>
      <div className={c.toolbar}>
        <Button
          color="primary"
          variant="contained"
          onClick={() => {
            setEdit({
              title: '',
              sharedWith: [],
              revision: 0,
              body: { state: 'draft', description: '', items: [] },
            })
            setPreview(null)
            setLines('')
          }}
        >
          Create music request
        </Button>
        <Button onClick={load}>Refresh</Button>
        <TextField
          select
          id="family-request-view"
          label="View"
          value={view}
          onChange={(e) => {
            setView(e.target.value)
            setOffset(0)
          }}
        >
          {['all', 'mine', 'inbox', 'draft'].map((v) => (
            <MenuItem key={v} value={v}>
              {v}
            </MenuItem>
          ))}
        </TextField>
        <FormControlLabel
          control={
            <Checkbox
              checked={archived}
              onChange={(e) => {
                setArchived(e.target.checked)
                setOffset(0)
              }}
            />
          }
          label="Archived"
        />
      </div>
      <Typography role="status">{error}</Typography>
      {!items.length && <Typography>No requests in this view.</Typography>}
      {items.map((x) => (
        <Card key={x.id} style={{ marginBottom: 16 }}>
          <CardContent>
            <Typography variant="h5">
              <Link color="inherit" href={`#/family/requests/${x.id}`}>
                {x.title}
              </Link>
            </Typography>
            <Typography variant="caption">
              {x.body.state} ·{' '}
              {x.canEdit ? 'Your request' : 'Owner review inbox'}
            </Typography>
            <Typography paragraph className={c.text}>
              {x.body.description}
            </Typography>
            {x.body.items?.map((item) => (
              <div key={item.id} style={{ marginBottom: 16 }}>
                <Typography variant="h6">{item.query}</Typography>
                <Typography>
                  {item.kind} · {item.state}
                </Typography>
                {item.url && (
                  <Link
                    color="inherit"
                    href={item.url}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    Music listing reference
                  </Link>
                )}
                {item.resolution && (
                  <ReferenceRow reference={item.resolution} />
                )}
                <div>
                  {(x.body.state !== 'draft'
                    ? me?.admin
                      ? transitions[item.state] || []
                      : x.canEdit &&
                          (transitions[item.state] || []).includes('cancelled')
                        ? ['cancelled']
                        : []
                    : []
                  ).map((state) => (
                    <Button
                      key={state}
                      disabled={busy}
                      onClick={() =>
                        setStatus({
                          entry: x,
                          item,
                          state,
                          comment: '',
                          resolution: null,
                        })
                      }
                    >
                      {state === 'obtained' ? 'Purchased / obtained' : state}
                    </Button>
                  ))}
                </div>
              </div>
            ))}
            {x.canEdit && x.body.state === 'draft' && (
              <>
                <Button
                  onClick={() => {
                    setEdit(structuredClone(x))
                    setPreview(null)
                    setLines('')
                  }}
                >
                  Edit draft
                </Button>
                <Button
                  disabled={busy}
                  onClick={() =>
                    action(() =>
                      call(`/requests/${x.id}/submit`, {
                        revision: x.revision,
                      }),
                    )
                  }
                >
                  Submit to Owner
                </Button>
              </>
            )}
            <Button
              disabled={busy}
              onClick={() =>
                action(() =>
                  call(`/requests/${x.id}/archive`, { archived: !x.archived }),
                )
              }
            >
              {x.archived ? 'Restore request' : 'Archive request'}
            </Button>
          </CardContent>
        </Card>
      ))}
      {id && (
        <section aria-label="Request status history">
          <Typography variant="h6">Status history</Typography>
          {history.map((h, i) => (
            <Typography key={i} className={c.text}>
              {new Date(h.createdAt).toLocaleString()} ·{' '}
              {items[0]?.body.items.find((v) => v.id === h.itemId)?.query ||
                'Item'}{' '}
              · {h.state}
              {h.comment && ` — ${h.comment}`}
            </Typography>
          ))}
          <Button
            disabled={!historyOffset}
            onClick={() => setHistoryOffset(Math.max(0, historyOffset - 100))}
          >
            Newer history
          </Button>
          <Button
            disabled={history.length < 100}
            onClick={() => setHistoryOffset(historyOffset + 100)}
          >
            Older history
          </Button>
        </section>
      )}
      {!id && (
        <div className={c.toolbar}>
          <Button
            disabled={!offset}
            onClick={() => setOffset(Math.max(0, offset - 25))}
          >
            Previous
          </Button>
          <Button
            disabled={items.length < 25}
            onClick={() => setOffset(offset + 25)}
          >
            Next
          </Button>
        </div>
      )}
      <Dialog
        className={c.dialog}
        open={!!edit}
        onClose={() => !busy && setEdit(null)}
        fullWidth
        maxWidth="md"
      >
        <DialogTitle>Music request draft</DialogTitle>
        {edit && (
          <DialogContent>
            <TextField
              fullWidth
              id="family-request-title"
              label="Request title"
              value={edit.title}
              inputProps={{ maxLength: 160 }}
              onChange={(e) => setEdit({ ...edit, title: e.target.value })}
            />
            <TextField
              fullWidth
              multiline
              id="family-request-notes"
              label="Details and notes"
              value={edit.body.description || ''}
              inputProps={{ maxLength: 16000 }}
              onChange={(e) =>
                setEdit({
                  ...edit,
                  body: { ...edit.body, description: e.target.value },
                })
              }
            />
            <Typography variant="h6">Add one item or a batch</Typography>
            <TextField
              select
              id="family-request-kind"
              label="Request type"
              value={kind}
              onChange={(e) => {
                setKind(e.target.value)
                setPreview(null)
              }}
            >
              {['track', 'album', 'artist'].map((v) => (
                <MenuItem key={v} value={v}>
                  {v}
                </MenuItem>
              ))}
            </TextField>
            <TextField
              fullWidth
              multiline
              id="family-request-lines"
              label="One title or artist per line (up to 100)"
              value={lines}
              inputProps={{ maxLength: 32000 }}
              onChange={(e) => {
                setLines(e.target.value)
                setPreview(null)
              }}
            />
            <Button
              disabled={!lines.trim() || busy}
              onClick={() =>
                action(async () =>
                  setPreview(await call('/request-preview', { kind, lines })),
                )
              }
            >
              Preview list
            </Button>
            {preview && (
              <>
                <Typography>
                  {preview.items.length} ordered items · {preview.duplicates}{' '}
                  duplicates removed. Matches are suggestions, not proof of the
                  same edition.
                </Typography>
                {preview.items.map((v, i) => (
                  <div key={i}>
                    <Typography>
                      {i + 1}. {v.query}
                    </Typography>
                    {v.candidates?.map((r) => (
                      <ReferenceRow key={r.id} reference={r} />
                    ))}
                  </div>
                ))}
                <Button
                  disabled={edit.body.items.length + preview.items.length > 100}
                  onClick={() => {
                    const seen = new Set(
                      edit.body.items.map(
                        (v) =>
                          `${v.kind}:${v.query.trim().replace(/\s+/g, ' ').toLowerCase()}`,
                      ),
                    )
                    const additions = preview.items.filter((v) => {
                      const k = `${v.kind}:${v.query.trim().replace(/\s+/g, ' ').toLowerCase()}`
                      if (seen.has(k)) return false
                      seen.add(k)
                      return true
                    })
                    setEdit({
                      ...edit,
                      body: {
                        ...edit.body,
                        items: [...edit.body.items, ...additions],
                      },
                    })
                    setPreview(null)
                    setLines('')
                  }}
                >
                  Add preview to draft
                </Button>
              </>
            )}
            {edit.body.items.map((item, i) => (
              <div key={item.id || i}>
                <Typography>
                  {i + 1}. {item.kind}: {item.query}
                </Typography>
                <TextField
                  fullWidth
                  id={`family-request-url-${i}`}
                  label="Optional HTTPS music listing URL (never fetched)"
                  value={item.url || ''}
                  inputProps={{ maxLength: 2048 }}
                  onChange={(e) => {
                    const xs = [...edit.body.items]
                    xs[i] = { ...item, url: e.target.value }
                    setEdit({ ...edit, body: { ...edit.body, items: xs } })
                  }}
                />
                <Button
                  disabled={!i}
                  onClick={() => {
                    const xs = [...edit.body.items]
                    ;[xs[i - 1], xs[i]] = [xs[i], xs[i - 1]]
                    setEdit({ ...edit, body: { ...edit.body, items: xs } })
                  }}
                >
                  Up
                </Button>
                <Button
                  onClick={() =>
                    setEdit({
                      ...edit,
                      body: {
                        ...edit.body,
                        items: edit.body.items.filter((_, n) => n !== i),
                      },
                    })
                  }
                >
                  Remove item
                </Button>
              </div>
            ))}
          </DialogContent>
        )}
        <DialogActions>
          <Typography role="alert">{error}</Typography>
          <Button disabled={busy} onClick={() => setEdit(null)}>
            Cancel
          </Button>
          <Button
            disabled={busy || !edit?.title.trim() || !edit?.body.items.length}
            onClick={save}
          >
            Save draft
          </Button>
        </DialogActions>
      </Dialog>
      <Dialog
        className={c.dialog}
        open={!!status}
        onClose={() => !busy && setStatus(null)}
        fullWidth
        maxWidth="md"
      >
        <DialogTitle>Update request item</DialogTitle>
        {status && (
          <DialogContent>
            <Typography>
              {status.item.query} → {status.state}
            </Typography>
            <TextField
              fullWidth
              multiline
              id="family-request-comment"
              label="Comment for requester"
              value={status.comment}
              inputProps={{ maxLength: 4000 }}
              onChange={(e) =>
                setStatus({ ...status, comment: e.target.value })
              }
            />
            {['added', 'obtained'].includes(status.state) && (
              <>
                <Typography>
                  Select an existing track or album to resolve this item. Added
                  requires a library reference.
                </Typography>
                {status.resolution && (
                  <ReferenceRow reference={status.resolution} />
                )}
                <ReferencePicker
                  noPlaylists
                  onAdd={(r) => {
                    if (r.kind !== 'playlist')
                      setStatus({ ...status, resolution: r })
                  }}
                />
              </>
            )}
          </DialogContent>
        )}
        <DialogActions>
          <Typography role="alert">{error}</Typography>
          <Button disabled={busy} onClick={() => setStatus(null)}>
            Cancel
          </Button>
          <Button
            disabled={
              busy ||
              (status?.state === 'added' &&
                !status.resolution &&
                !status.item.resolution)
            }
            onClick={() =>
              action(async () => {
                await call(`/requests/${status.entry.id}/status`, {
                  revision: status.entry.revision,
                  itemId: status.item.id,
                  state: status.state,
                  comment: status.comment,
                  resolution: status.resolution,
                })
                setStatus(null)
              })
            }
          >
            Save status
          </Button>
        </DialogActions>
      </Dialog>
    </div>
  )
}
