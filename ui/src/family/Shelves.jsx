import FamilyNav from './FamilyNav'
import React, { useEffect, useState } from 'react'
import { Title } from 'react-admin'
import { useParams } from 'react-router-dom'
import {
  Link,
  Button,
  Card,
  CardContent,
  Typography,
  TextField,
  Checkbox,
  FormControlLabel,
  Dialog,
  DialogContent,
  DialogActions,
  DialogTitle,
  MenuItem,
} from '@material-ui/core'
import {
  call,
  useFamilyStyles,
  ReferencePicker,
  ReferenceRow,
  RefArtwork,
} from './common'
const fresh = () => ({
  kind: 'shelf',
  title: '',
  sharedWith: [],
  body: { description: '', references: [], hidden: false, cover: null },
  revision: 0,
})
export default function Shelves() {
  const c = useFamilyStyles(),
    { id } = useParams(),
    [items, setItems] = useState([]),
    [people, setPeople] = useState([]),
    [me, setMe] = useState(null),
    [edit, setEdit] = useState(null),
    [error, setError] = useState(''),
    [archived, setArchived] = useState(false),
    [offset, setOffset] = useState(0),
    [busy, setBusy] = useState(false)
  const load = () =>
    id
      ? call(`/shelves/${encodeURIComponent(id)}`).then((x) => setItems([x]))
      : call(`/shelves?archived=${archived}&offset=${offset}`).then(setItems)
  useEffect(() => {
    let active = true
    Promise.all([call('/people'), call('/me')])
      .then(([p, m]) => {
        if (active) {
          setPeople(p)
          setMe(m)
        }
      })
      .catch(() => setError('Family members unavailable.'))
    return () => {
      active = false
    }
  }, [])
  useEffect(() => {
    setError('')
    ;(id
      ? call('/shelves/' + encodeURIComponent(id)).then((x) => setItems([x]))
      : load()
    ).catch(() =>
      setError('Shelves unavailable. Your saved content is preserved.'),
    )
  }, [id, archived, offset])
  const save = async (x) => {
    setBusy(true)
    setError('')
    try {
      const saved = await call(
        x.id ? `/shelves/${x.id}` : '/shelves',
        x,
        x.id ? 'PUT' : 'POST',
      )
      setEdit(null)
      if (id && saved.id !== id)
        window.location.hash = `#/family/shelves/${saved.id}`
      else await load()
    } catch (e) {
      setError(
        e.status === 409
          ? 'This shelf changed. Close and reload before editing.'
          : 'Could not save. Check access and try again.',
      )
    } finally {
      setBusy(false)
    }
  }
  const move = (from, to) => {
    if (to < 0 || to >= edit.body.references.length) return
    const refs = [...edit.body.references]
    const [r] = refs.splice(from, 1)
    refs.splice(to, 0, r)
    setEdit({ ...edit, body: { ...edit.body, references: refs } })
  }
  return (
    <div className={c.page}>
      <FamilyNav />
      <Title title="Family Shelves" />
      <Typography variant="h4" gutterBottom>
        Family Shelves
      </Typography>
      <Typography paragraph>
        Keep albums, songs and playlists together. Every shelf starts private;
        share only with the people you choose.
      </Typography>
      <div className={c.toolbar}>
        <Button
          variant="contained"
          color="primary"
          onClick={() => setEdit(fresh())}
        >
          Create shelf
        </Button>
        <Button onClick={load}>Refresh</Button>
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
          label="Archived shelves"
        />
      </div>
      <Typography role="status">{error}</Typography>
      {!items.length && <Typography>No shelves in this view.</Typography>}
      {items.map((x) => (
        <Card key={x.id} style={{ marginBottom: 16 }}>
          <CardContent>
            <RefArtwork reference={x.body.cover || x.body.references[0]} />
            <Typography variant="h5">
              <Link color="inherit" href={`#/family/shelves/${x.id}`}>
                {x.title}
              </Link>
            </Typography>
            <Typography variant="caption">
              {x.canEdit
                ? x.sharedWith.length
                  ? 'Shared by you'
                  : 'Personal'
                : 'Shared with you'}
              {x.body.hidden && ' · Hidden on Home'}
            </Typography>
            <Typography paragraph className={c.text}>
              {x.body.description}
            </Typography>
            {x.body.references.map((r, i) => (
              <ReferenceRow key={r.id || i} reference={r} />
            ))}
            {x.canEdit && (
              <>
                <Button onClick={() => setEdit(x)}>Edit shelf</Button>
                <Button
                  disabled={busy}
                  onClick={() => save({ ...x, archived: !x.archived })}
                >
                  {x.archived ? 'Restore shelf' : 'Archive shelf'}
                </Button>
              </>
            )}
          </CardContent>
        </Card>
      ))}
      {!id && (
        <div className={c.toolbar}>
          <Button
            disabled={offset === 0}
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
        <DialogTitle>{edit?.id ? 'Edit shelf' : 'Create shelf'}</DialogTitle>
        {edit && (
          <DialogContent>
            <TextField
              fullWidth
              id="family-shelf-title"
              label="Shelf title"
              value={edit.title}
              onChange={(e) => setEdit({ ...edit, title: e.target.value })}
              inputProps={{ maxLength: 160 }}
            />
            <TextField
              fullWidth
              multiline
              id="family-shelf-description"
              label="Description"
              value={edit.body.description}
              onChange={(e) =>
                setEdit({
                  ...edit,
                  body: { ...edit.body, description: e.target.value },
                })
              }
              inputProps={{ maxLength: 16000 }}
            />
            <TextField
              fullWidth
              select
              SelectProps={{ multiple: true }}
              id="family-shelf-sharing"
              label="Share with (empty means private)"
              value={edit.sharedWith}
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
            <FormControlLabel
              control={
                <Checkbox
                  checked={edit.body.hidden}
                  onChange={(e) =>
                    setEdit({
                      ...edit,
                      body: { ...edit.body, hidden: e.target.checked },
                    })
                  }
                />
              }
              label="Hide on Home"
            />
            <TextField
              fullWidth
              select
              id="family-shelf-cover"
              label="Cover from your selection"
              value={
                edit.body.cover
                  ? edit.body.cover.kind + ':' + edit.body.cover.id
                  : ''
              }
              onChange={(e) =>
                setEdit({
                  ...edit,
                  body: {
                    ...edit.body,
                    cover:
                      edit.body.references.find(
                        (r) => r.kind + ':' + r.id === e.target.value,
                      ) || null,
                  },
                })
              }
            >
              <MenuItem value="">Automatic</MenuItem>
              {edit.body.references
                .filter((r) => r.id)
                .map((r) => (
                  <MenuItem key={r.kind + r.id} value={r.kind + ':' + r.id}>
                    {r.title}
                  </MenuItem>
                ))}
            </TextField>
            <Typography variant="h6">Ordered selection</Typography>
            {edit.body.references.map((r, i) => (
              <div
                key={r.id || i}
                draggable
                onDragStart={(e) =>
                  e.dataTransfer.setData('text/plain', String(i))
                }
                onDragOver={(e) => e.preventDefault()}
                onDrop={(e) => {
                  e.preventDefault()
                  const from = Number(e.dataTransfer.getData('text/plain'))
                  if (Number.isInteger(from)) move(from, i)
                }}
              >
                <ReferenceRow reference={r}>
                  <Button
                    aria-label={`Move ${r.title} up`}
                    disabled={i === 0}
                    onClick={() => move(i, i - 1)}
                  >
                    Up
                  </Button>
                  <Button
                    aria-label={`Move ${r.title} down`}
                    disabled={i === edit.body.references.length - 1}
                    onClick={() => move(i, i + 1)}
                  >
                    Down
                  </Button>
                  <Button
                    onClick={() =>
                      setEdit({
                        ...edit,
                        body: {
                          ...edit.body,
                          references: edit.body.references.filter(
                            (_, n) => n !== i,
                          ),
                          cover:
                            edit.body.cover?.id === r.id
                              ? null
                              : edit.body.cover,
                        },
                      })
                    }
                  >
                    Remove from shelf
                  </Button>
                </ReferenceRow>
              </div>
            ))}
            <ReferencePicker
              onAdd={(r) => {
                if (
                  !edit.body.references.some(
                    (x) => x.id === r.id && x.kind === r.kind,
                  ) &&
                  edit.body.references.length < 100
                )
                  setEdit({
                    ...edit,
                    body: {
                      ...edit.body,
                      references: [...edit.body.references, r],
                    },
                  })
              }}
            />
          </DialogContent>
        )}
        <DialogActions>
          <Typography role="alert">{error}</Typography>
          <Button disabled={busy} onClick={() => setEdit(null)}>
            Cancel
          </Button>
          <Button
            color="primary"
            disabled={busy || !edit?.title.trim()}
            onClick={() => save(edit)}
          >
            Save shelf
          </Button>
        </DialogActions>
      </Dialog>
    </div>
  )
}
