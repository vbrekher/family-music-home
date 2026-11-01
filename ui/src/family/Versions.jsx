import React, { useEffect, useState } from 'react'
import { Title } from 'react-admin'
import { useParams } from 'react-router-dom'
import {
  Button,
  Typography,
  Card,
  CardContent,
  Grid,
  Link,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Checkbox,
  FormControlLabel,
} from '@material-ui/core'
import { call, useFamilyStyles, ReferencePicker, ReferenceRow } from './common'
import FamilyNav from './FamilyNav'
export default function Versions() {
  const c = useFamilyStyles(),
    { id } = useParams(),
    [items, setItems] = useState([]),
    [me, setMe] = useState(null),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(false),
    [offset, setOffset] = useState(0),
    [archived, setArchived] = useState(false),
    [edit, setEdit] = useState(null),
    [confirmed, setConfirmed] = useState(false)
  const load = async () => {
    try {
      const x = await call(
        id
          ? `/versions/${id}`
          : `/versions?offset=${offset}&archived=${archived}`,
      )
      setItems(id ? [x] : x)
    } catch (_) {
      setError('Versions unavailable. Check catalog access and retry.')
    }
  }
  useEffect(() => {
    call('/me')
      .then(setMe)
      .catch(() => setError('Account unavailable.'))
  }, [])
  useEffect(() => {
    setItems([])
    setError('')
    load()
  }, [id, archived, offset])
  const action = async (fn) => {
    setBusy(true)
    setError('')
    try {
      const reload = await fn()
      if (reload !== false) await load()
    } catch (e) {
      setError(
        e.status === 409
          ? 'This group changed. Refresh before editing.'
          : 'Could not save. Choose 2–20 accessible editions with one release-group ID, or confirm their relationship manually.',
      )
    } finally {
      setBusy(false)
    }
  }
  const create = (basis) =>
    action(async () => {
      const x = await call('/versions', {
        albumIds: edit.map((x) => x.id),
        basis,
        confirmed,
      })
      setEdit(null)
      if (id !== x.id) {
        window.location.hash = `#/family/versions/${x.id}`
        return false
      }
    })
  return (
    <div className={c.page}>
      <Title title="Version Collector" />
      <FamilyNav />
      <Typography variant="h4" gutterBottom>
        Version Collector
      </Typography>
      <Typography paragraph>
        Keep distinct editions together while every album keeps its own
        identity, favorites and library. MusicBrainz release-group tags or your
        explicit confirmation establish a relationship. Matching titles alone do
        not.
      </Typography>
      <Typography paragraph>
        CD lossless and high sample rates describe formats. They do not prove
        better mastering or sound.
      </Typography>
      <div className={c.toolbar}>
        {me?.admin && (
          <Button
            variant="contained"
            color="primary"
            onClick={() => {
              setEdit([])
              setConfirmed(false)
            }}
          >
            Link editions
          </Button>
        )}
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
          label="Archived groups"
        />
      </div>
      <Typography role="status">{error}</Typography>
      {!items.length && (
        <Typography>
          No visible edition groups yet. Your original albums remain available
          in Albums.
        </Typography>
      )}
      {items.map((x) => (
        <Card key={x.id} style={{ marginBottom: 20 }}>
          <CardContent>
            <Typography variant="h5">
              <Link color="inherit" href={`#/family/versions/${x.id}`}>
                {x.members[0]?.title || 'Edition group'}
              </Link>
            </Typography>
            <Typography variant="body2">
              {x.basis === 'musicbrainz'
                ? `Shared MusicBrainz release group: ${x.releaseGroup}`
                : 'Relationship explicitly confirmed by the curator'}{' '}
              · Your preferred edition is a personal choice.
            </Typography>
            <Grid container spacing={2}>
              {x.members.map((a) => (
                <Grid item key={a.id} xs={12} md={6} lg={4}>
                  <Card variant="outlined">
                    <CardContent>
                      <ReferenceRow reference={a} />
                      <Typography>
                        {a.sourceType || 'Source'} · {a.library} ·{' '}
                        {a.releaseDate || 'Release date unknown'}
                      </Typography>
                      <Typography>
                        {a.edition || 'Edition not tagged'} ·{' '}
                        {a.label || 'Label unknown'} ·{' '}
                        {a.releaseType || 'Release type unknown'}
                      </Typography>
                      {a.releaseId && (
                        <Typography variant="caption" className={c.text}>
                          Release ID: {a.releaseId}
                        </Typography>
                      )}
                      {a.technical.map((t, i) => (
                        <Typography key={i} variant="body2">
                          {t.container || 'Container unknown'} ·{' '}
                          {t.codec || 'Codec unknown'} ·{' '}
                          {t.sampleRate
                            ? `${t.sampleRate} Hz`
                            : 'Sample rate unknown'}{' '}
                          ·{' '}
                          {t.bitDepth
                            ? `${t.bitDepth} bit`
                            : 'Bit depth unknown'}{' '}
                          ·{' '}
                          {t.channels
                            ? `${t.channels} channels`
                            : 'Channels unknown'}
                        </Typography>
                      ))}
                      <Typography variant="caption">
                        Metadata sampled from {a.sampledTracks} of{' '}
                        {a.totalTracks} catalog tracks. No audio analysis
                        implied.
                      </Typography>
                      {a.firstTrackId && (
                        <Button
                          href={`#/family/passport?track=${encodeURIComponent(a.firstTrackId)}`}
                        >
                          Track quality passport
                        </Button>
                      )}
                      <Button
                        disabled={busy}
                        color={x.preferred === a.id ? 'primary' : 'default'}
                        onClick={() =>
                          action(() =>
                            call(
                              `/versions/${x.id}/preference`,
                              { albumId: x.preferred === a.id ? '' : a.id },
                              'PUT',
                            ),
                          )
                        }
                      >
                        {x.preferred === a.id
                          ? 'Your preferred edition · clear'
                          : 'Prefer this edition'}
                      </Button>
                    </CardContent>
                  </Card>
                </Grid>
              ))}
            </Grid>
            {x.canEdit && (
              <Button
                disabled={busy}
                onClick={() =>
                  action(() =>
                    call(`/versions/${x.id}/archive`, {
                      revision: x.revision,
                      archived: !x.archived,
                    }),
                  )
                }
              >
                {x.archived ? 'Restore group' : 'Archive group'}
              </Button>
            )}
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
        <DialogTitle>Link existing album editions</DialogTitle>
        {edit && (
          <DialogContent>
            <Typography paragraph>
              Select editions already in your libraries. This creates a catalog
              relationship; it does not merge, move or rewrite music.
            </Typography>
            {edit.map((a) => (
              <ReferenceRow key={a.id} reference={a}>
                <Button
                  onClick={() => setEdit(edit.filter((x) => x.id !== a.id))}
                >
                  Remove from selection
                </Button>
              </ReferenceRow>
            ))}
            <ReferencePicker
              albumOnly
              onAdd={(a) => {
                if (edit.length < 20 && !edit.some((x) => x.id === a.id))
                  setEdit([...edit, a])
              }}
            />
            <FormControlLabel
              control={
                <Checkbox
                  checked={confirmed}
                  onChange={(e) => setConfirmed(e.target.checked)}
                />
              }
              label="I have checked that these are editions of the same release group"
            />
            <Typography variant="body2">
              Use MusicBrainz tags when every selection has the same nonempty
              release-group ID. Otherwise manual confirmation is required; a
              matching name is insufficient.
            </Typography>
          </DialogContent>
        )}
        <DialogActions>
          <Typography role="alert">{error}</Typography>
          <Button disabled={busy} onClick={() => setEdit(null)}>
            Cancel
          </Button>
          <Button
            disabled={busy || edit?.length < 2}
            onClick={() => create('musicbrainz')}
          >
            Link by release-group tags
          </Button>
          <Button
            disabled={busy || edit?.length < 2 || !confirmed}
            onClick={() => create('confirmed')}
          >
            Confirm editions
          </Button>
        </DialogActions>
      </Dialog>
    </div>
  )
}
