import React, { useEffect, useRef, useState } from 'react'
import { Title } from 'react-admin'
import { useParams } from 'react-router-dom'
import RichMessage, { safeMessage } from './RichMessage'
export { safeMessage } from './RichMessage'
import {
  Button,
  Card,
  CardContent,
  Typography,
  TextField,
  MenuItem,
  Checkbox,
  FormControlLabel,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Link,
} from '@material-ui/core'
import {
  call,
  useFamilyStyles,
  ReferencePicker,
  ReferenceRow,
  RefArtwork,
} from './common'
import FamilyNav from './FamilyNav'
import Media, { PrivateMedia } from './Media'
const zone = () => Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
const fresh = (letter) => ({
  title: '',
  sharedWith: [],
  revision: 0,
  body: {
    state: 'draft',
    description: '',
    message: '',
    references: [],
    timezone: zone(),
    dateFrom: new Date().toISOString().slice(0, 10),
    dateTo: '',
    deliveryAt: '',
    anniversary: '',
    anniversaryTime: '09:00',
  },
})
export default function Journal({ letter = false }) {
  const c = useFamilyStyles(),
    { id } = useParams(),
    plural = letter ? 'letters' : 'capsules',
    label = letter ? 'Music Letters' : 'Music Time Capsules'
  const [items, setItems] = useState([]),
    [people, setPeople] = useState([]),
    [me, setMe] = useState(null),
    [edit, setEdit] = useState(null),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(false),
    [offset, setOffset] = useState(0),
    [archived, setArchived] = useState(false),
    [view, setView] = useState('all'),
    [q, setQ] = useState(''),
    [from, setFrom] = useState(''),
    [to, setTo] = useState(''),
    [next, setNext] = useState('')
  const sendKeys = useRef({})
  const load = async () => {
    try {
      const v = await call(
        id
          ? `/${plural}/${id}`
          : `/${plural}?offset=${offset}&archived=${archived}&view=${view}&q=${encodeURIComponent(q)}&from=${from}&to=${to}`,
      )
      setItems(id ? [v] : v)
      if (id && letter && !v.canEdit && v.unread) {
        await call(`/letters/${id}/read`, {})
        setItems([await call(`/letters/${id}`)])
      }
    } catch (_) {
      setError(
        'This view is unavailable. Check access or retry; saved content is preserved.',
      )
    }
  }
  useEffect(() => {
    Promise.all([call('/people'), call('/me')])
      .then(([p, m]) => {
        setPeople(p)
        setMe(m)
      })
      .catch(() => setError('Family members unavailable.'))
  }, [])
  useEffect(() => {
    setItems([])
    setError('')
    load()
  }, [id, plural, offset, archived, view])
  const action = async (fn) => {
    setBusy(true)
    setError('')
    try {
      const reload = await fn()
      if (reload !== false) await load()
    } catch (e) {
      setError(
        e.status === 409
          ? 'This entry changed. Refresh before editing.'
          : 'Could not save. Check fields, recipients and access, then retry.',
      )
    } finally {
      setBusy(false)
    }
  }
  const save = (x) =>
    action(async () => {
      const saved = await call(
        x.id ? `/${plural}/${x.id}` : `/${plural}`,
        x,
        x.id ? 'PUT' : 'POST',
      )
      setEdit(null)
      if (id !== saved.id) {
        window.location.hash = `#/family/${plural}/${saved.id}`
        return false
      }
    })
  const field = (key, value) =>
    setEdit({ ...edit, body: { ...edit.body, [key]: value } })
  const send = (x, resend) =>
    action(async () => {
      const attempt = x.id + ':' + x.revision + ':' + resend
      sendKeys.current[attempt] ||= crypto.randomUUID()
      await call(`/letters/${x.id}/send`, {
        revision: x.revision,
        key: sendKeys.current[attempt],
        resend,
      })
    })
  return (
    <div className={c.page}>
      <Title title={label} />
      <FamilyNav />
      <Typography variant="h4" gutterBottom>
        {label}
      </Typography>
      <Typography paragraph>
        {letter
          ? 'Private music messages for chosen recipients. Delivery and read receipts stay inside your family server.'
          : 'Dates, photographs, voice notes and music you want to remember. Drafts remain private until published.'}
      </Typography>
      <div className={c.toolbar}>
        <Button
          variant="contained"
          color="primary"
          onClick={() => {
            setEdit(fresh(letter))
            setNext('')
          }}
        >
          Create {letter ? 'letter' : 'capsule'}
        </Button>
        <Button onClick={load}>Refresh</Button>
        <TextField
          select
          id="family-journal-view"
          label="View"
          value={view}
          onChange={(e) => {
            setView(e.target.value)
            setOffset(0)
          }}
        >
          {(letter
            ? ['all', 'mine', 'sent', 'inbox', 'draft']
            : ['all', 'mine', 'draft']
          ).map((v) => (
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
      {!letter && !id && (
        <div className={c.toolbar}>
          <TextField
            id="family-memory-search"
            label="Search memories"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            inputProps={{ maxLength: 100 }}
          />
          <TextField
            type="date"
            id="family-memory-from"
            label="From"
            InputLabelProps={{ shrink: true }}
            value={from}
            onChange={(e) => setFrom(e.target.value)}
          />
          <TextField
            type="date"
            id="family-memory-to"
            label="To"
            InputLabelProps={{ shrink: true }}
            value={to}
            onChange={(e) => setTo(e.target.value)}
          />
          <Button
            onClick={() => {
              setOffset(0)
              load()
            }}
          >
            Search timeline
          </Button>
        </div>
      )}
      <Typography role="status">{error}</Typography>
      {!items.length && <Typography>No entries in this view.</Typography>}
      {items.map((x) => (
        <Card key={x.id} style={{ marginBottom: 16 }}>
          <CardContent>
            <RefArtwork reference={x.body.cover || x.body.references?.[0]} />
            <Typography variant="h5">
              <Link color="inherit" href={`#/family/${plural}/${x.id}`}>
                {x.title}
              </Link>
            </Typography>
            <Typography variant="caption">
              {x.body.state} ·{' '}
              {x.canEdit
                ? x.sharedWith.length
                  ? 'Shared by you'
                  : 'Personal'
                : 'Shared with you'}
              {x.unread && ' · Unread'}
            </Typography>
            {!letter && (
              <Typography>
                {x.body.dateFrom}
                {x.body.dateTo && ` — ${x.body.dateTo}`} · {x.body.timezone}
              </Typography>
            )}
            {letter ? (
              <div
                className={c.text}
                dangerouslySetInnerHTML={{
                  __html: safeMessage(x.body.message),
                }}
              />
            ) : (
              <Typography paragraph className={c.text}>
                {x.body.description}
              </Typography>
            )}
            {x.body.references?.map((r, i) => (
              <ReferenceRow key={r.id || i} reference={r} />
            ))}
            {id && (
              <Media
                entry={x}
                plural={plural}
                onChange={(cover) =>
                  cover
                    ? save({
                        ...x,
                        body: { ...x.body, coverAttachmentId: cover },
                      })
                    : load()
                }
              />
            )}
            {!id && x.body.coverAttachmentId && (
              <PrivateMedia
                item={{
                  id: x.body.coverAttachmentId,
                  mime: 'image/jpeg',
                  size: 0,
                }}
              />
            )}
            {x.canEdit && (!letter || x.body.state === 'draft') && (
              <Button
                onClick={() => {
                  setEdit(structuredClone(x))
                  setNext('')
                }}
              >
                Edit {letter ? 'letter' : 'capsule'}
              </Button>
            )}
            {letter && x.canEdit && x.body.state === 'draft' && (
              <Button disabled={busy} onClick={() => send(x, false)}>
                Send letter
              </Button>
            )}
            {letter && x.canEdit && x.body.state === 'sent' && (
              <Button disabled={busy} onClick={() => send(x, true)}>
                Send again
              </Button>
            )}
            {letter &&
              x.canEdit &&
              (x.body.state === 'scheduled' || x.body.anniversary) && (
                <Button
                  disabled={busy}
                  onClick={() =>
                    action(() =>
                      call(`/letters/${x.id}/cancel`, { revision: x.revision }),
                    )
                  }
                >
                  Cancel future delivery
                </Button>
              )}
            {(letter || x.canEdit) && (
              <Button
                disabled={busy}
                onClick={() =>
                  letter
                    ? action(() =>
                        call(`/letters/${x.id}/archive`, {
                          archived: !x.archived,
                        }),
                      )
                    : save({ ...x, archived: !x.archived })
                }
              >
                {x.archived ? 'Restore entry' : 'Archive entry'}
              </Button>
            )}
            {letter &&
              id &&
              x.deliveries?.map((d, i) => (
                <Typography key={i} variant="body2">
                  {people.find((p) => p.id === d.recipientId)?.name ||
                    'Recipient'}{' '}
                  ·{' '}
                  {d.deliveredAt
                    ? `Delivered ${new Date(d.deliveredAt).toLocaleString()}`
                    : `Scheduled ${new Date(d.dueAt).toLocaleString()}`}{' '}
                  ·{' '}
                  {d.readAt
                    ? `Read ${new Date(d.readAt).toLocaleString()}`
                    : 'Not read'}
                </Typography>
              ))}
          </CardContent>
        </Card>
      ))}
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
        <DialogTitle>
          {edit?.id ? 'Edit' : 'Create'} {letter ? 'letter' : 'capsule'}
        </DialogTitle>
        {edit && (
          <DialogContent>
            <TextField
              fullWidth
              id="family-journal-title"
              label="Title"
              value={edit.title}
              inputProps={{ maxLength: 160 }}
              onChange={(e) => setEdit({ ...edit, title: e.target.value })}
            />
            {letter ? (
              <RichMessage
                value={edit.body.message}
                onChange={(value) => field('message', value)}
              />
            ) : (
              <TextField
                fullWidth
                multiline
                id="family-journal-message"
                label="Notes"
                value={edit.body.description}
                inputProps={{ maxLength: 16000 }}
                onChange={(e) => field('description', e.target.value)}
              />
            )}
            <TextField
              fullWidth
              select
              SelectProps={{ multiple: true }}
              id="family-journal-recipients"
              label={letter ? 'Recipients' : 'Share with (empty means private)'}
              value={edit.sharedWith || []}
              onChange={(e) => setEdit({ ...edit, sharedWith: e.target.value })}
            >
              {people
                .filter((p) => p.id !== me?.id)
                .map((p) => (
                  <MenuItem key={p.id} value={p.id}>
                    {p.name}
                  </MenuItem>
                ))}
            </TextField>
            <TextField
              fullWidth
              id="family-journal-zone"
              label="IANA timezone"
              value={edit.body.timezone}
              onChange={(e) => field('timezone', e.target.value)}
            />
            {!letter ? (
              <>
                <TextField
                  type="date"
                  id="family-capsule-from"
                  label="Memory date"
                  InputLabelProps={{ shrink: true }}
                  value={edit.body.dateFrom}
                  onChange={(e) => field('dateFrom', e.target.value)}
                />
                <TextField
                  type="date"
                  id="family-capsule-to"
                  label="End date (optional)"
                  InputLabelProps={{ shrink: true }}
                  value={edit.body.dateTo || ''}
                  onChange={(e) => field('dateTo', e.target.value)}
                />
                <TextField
                  fullWidth
                  select
                  id="family-capsule-state"
                  label="Visibility stage"
                  value={edit.body.state}
                  onChange={(e) => field('state', e.target.value)}
                >
                  <MenuItem value="draft">Private draft</MenuItem>
                  <MenuItem value="published">
                    Published to selected readers
                  </MenuItem>
                </TextField>
              </>
            ) : (
              <>
                <Typography variant="body2">
                  Leave delivery date empty for instant delivery. Your browser
                  timezone is used for this date picker; its exact UTC instant
                  is saved. For yearly dates, use the IANA timezone above.
                </Typography>
                <TextField
                  type="datetime-local"
                  id="family-letter-date"
                  label="Deliver at (browser local time)"
                  InputLabelProps={{ shrink: true }}
                  defaultValue={
                    edit.body.deliveryAt
                      ? new Date(
                          new Date(edit.body.deliveryAt).getTime() -
                            new Date(edit.body.deliveryAt).getTimezoneOffset() *
                              60000,
                        )
                          .toISOString()
                          .slice(0, 16)
                      : ''
                  }
                  onChange={(e) =>
                    field(
                      'deliveryAt',
                      e.target.value
                        ? new Date(e.target.value).toISOString()
                        : '',
                    )
                  }
                />
                <TextField
                  id="family-letter-anniversary"
                  label="Yearly date MM-DD (optional)"
                  value={edit.body.anniversary || ''}
                  onChange={(e) => {
                    field('anniversary', e.target.value)
                    setNext('')
                  }}
                />
                <TextField
                  type="time"
                  id="family-letter-time"
                  label="Yearly local time"
                  InputLabelProps={{ shrink: true }}
                  value={edit.body.anniversaryTime || '09:00'}
                  onChange={(e) => {
                    field('anniversaryTime', e.target.value)
                    setNext('')
                  }}
                />
                <Button
                  onClick={async () => {
                    try {
                      const r = await call('/letters/preview-date', {
                        day: edit.body.anniversary,
                        time: edit.body.anniversaryTime,
                        timezone: edit.body.timezone,
                      })
                      setNext(`Next yearly delivery: ${r.next}`)
                    } catch (_) {
                      setNext('Check date, time and timezone.')
                    }
                  }}
                >
                  Preview yearly delivery
                </Button>
                <Typography role="status">{next}</Typography>
              </>
            )}
            <Typography variant="h6">Ordered music selection</Typography>
            {(edit.body.references || []).map((r, i) => (
              <ReferenceRow key={r.id || i} reference={r}>
                <Button
                  disabled={!i}
                  onClick={() => {
                    const refs = [...edit.body.references]
                    ;[refs[i - 1], refs[i]] = [refs[i], refs[i - 1]]
                    field('references', refs)
                  }}
                >
                  Up
                </Button>
                <Button
                  onClick={() =>
                    field(
                      'references',
                      edit.body.references.filter((_, n) => n !== i),
                    )
                  }
                >
                  Remove reference
                </Button>
              </ReferenceRow>
            ))}
            <ReferencePicker
              onAdd={(r) => {
                if (
                  edit.body.references.length < 100 &&
                  !edit.body.references.some(
                    (v) => v.id === r.id && v.kind === r.kind,
                  )
                )
                  field('references', [...edit.body.references, r])
              }}
            />
            <Typography variant="body2">
              Save first to attach private photographs and voice notes on the
              entry page.
            </Typography>
          </DialogContent>
        )}
        <DialogActions>
          <Typography role="alert">{error}</Typography>
          <Button disabled={busy} onClick={() => setEdit(null)}>
            Cancel
          </Button>
          <Button
            disabled={
              busy ||
              !edit?.title.trim() ||
              (letter && edit?.body.message?.length > 16000)
            }
            onClick={() => save(edit)}
          >
            Save {letter ? 'draft' : 'capsule'}
          </Button>
        </DialogActions>
      </Dialog>
    </div>
  )
}
export function Letters() {
  return <Journal letter />
}
