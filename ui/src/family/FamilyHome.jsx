import React, { useEffect, useState } from 'react'
import { Title } from 'react-admin'
import { useSelector } from 'react-redux'
import { Redirect } from 'react-router-dom'
import { useChildPolicy } from './childPolicy'
import {
  Link,
  Button,
  Card,
  CardContent,
  Typography,
  TextField,
  Checkbox,
  FormControlLabel,
  MenuItem,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
} from '@material-ui/core'
import FamilyNav from './FamilyNav'
import Notifications from './Notifications'
import { call, useFamilyStyles, AlbumCards, RefArtwork } from './common'
const labels = {
  continue: 'Continue Listening',
  shelves: 'My Shelves',
  requests: 'Family Music Requests',
  letters: 'Unread Music Letters',
  capsules: 'Memories',
  new: 'Newly Added',
  forgotten: 'Forgotten Albums',
}
export default function FamilyHome() {
  const child = useChildPolicy()
  const c = useFamilyStyles(),
    queue = useSelector((s) => s.player?.queue || []),
    [prefs, setPrefs] = useState(null),
    [data, setData] = useState({}),
    [error, setError] = useState(''),
    [draft, setDraft] = useState(null),
    [busy, setBusy] = useState(false)
  const load = () =>
    call('/preferences')
      .then(setPrefs)
      .catch(() =>
        setError(
          'Home could not be loaded. Your libraries are still available from the menu.',
        ),
      )
  useEffect(() => {
    load()
  }, [])
  useEffect(() => {
    if (!prefs) return
    let active = true
    setData({})
    prefs.widgets.forEach((k) => {
      if (k === 'forgotten' && !prefs.forgottenEnabled) return
      call(
        ['shelves', 'requests', 'letters', 'capsules'].includes(k)
          ? `/${k}${k === 'letters' ? '?view=unread' : ''}`
          : `/albums/${k}`,
      )
        .then((v) => {
          if (active) setData((d) => ({ ...d, [k]: v }))
        })
        .catch(() => {
          if (active) setData((d) => ({ ...d, [k]: null }))
        })
    })
    return () => {
      active = false
    }
  }, [prefs])
  const move = (from, to) => {
    if (to < 0 || to >= draft.widgets.length) return
    const xs = [...draft.widgets]
    const [x] = xs.splice(from, 1)
    xs.splice(to, 0, x)
    setDraft({ ...draft, widgets: xs })
  }
  const save = async () => {
    setBusy(true)
    try {
      setPrefs(await call('/preferences', draft, 'PUT'))
      setDraft(null)
      setError('')
    } catch (_) {
      setError('Home settings could not be saved. Try again.')
    } finally {
      setBusy(false)
    }
  }
  const dismiss = async (id) => {
    try {
      await call(`/forgotten/${id}/dismiss`, {})
      setData((d) => ({
        ...d,
        forgotten: d.forgotten.filter((x) => x.id !== id),
      }))
    } catch (_) {
      setError('Could not hide this suggestion. Try again.')
    }
  }
  if (child && (child.interface === 'kids' || !child.enabled))
    return <Redirect to="/family/kids" />
  return (
    <div className={c.page}>
      <Title title="Family Music Home" />
      <FamilyNav />
      <header className={c.hero}>
        <Typography variant="overline">
          YOUR MUSIC. YOUR FAMILY. YOUR MEMORIES.
        </Typography>
        <Typography variant="h3" gutterBottom>
          Your music, at home.
        </Typography>
        <Typography paragraph>
          A quiet place for the music you keep, share and return to.
        </Typography>
        <div className={c.toolbar}>
          <Button variant="contained" color="primary" href="#/family/shelves">
            My shelves
          </Button>
          <Button
            disabled={!prefs}
            onClick={() => setDraft({ ...prefs, widgets: [...prefs.widgets] })}
          >
            Customize Home
          </Button>
        </div>
      </header>
      <Typography role="status">{error}</Typography>
      <Notifications />
      {!prefs && !error && <Typography>Loading your Home…</Typography>}
      <div className={c.homeSections}>
        {prefs?.widgets.map((k) => (
          <section
            key={k}
            className={c.widget}
            data-wide={['new', 'forgotten', 'continue'].includes(k)}
            aria-label={labels[k]}
          >
            <Typography variant="h5" gutterBottom>
              {labels[k]}
            </Typography>
            {k === 'continue' && queue.length > 0 && (
              <Typography paragraph>
                Your existing player queue is ready below. Resume with its Play
                button.
              </Typography>
            )}
            {k === 'forgotten' && !prefs.forgottenEnabled ? (
              <Typography>
                Rediscovery is off. Enable it in Customize Home when you would
                like suggestions.
              </Typography>
            ) : data[k] === undefined ? (
              <Typography>Loading…</Typography>
            ) : data[k] === null ? (
              <Button onClick={load}>Could not load. Retry</Button>
            ) : ['requests', 'letters', 'capsules'].includes(k) ? (
              <>
                {data[k]
                  .filter((x) => k !== 'letters' || x.unread)
                  .slice(0, 6)
                  .map((x) => (
                    <Card
                      key={x.id}
                      variant="outlined"
                      style={{ marginBottom: 12 }}
                    >
                      <CardContent>
                        <Typography variant="h6">
                          <Link color="inherit" href={`#/family/${k}/${x.id}`}>
                            {x.title}
                          </Link>
                        </Typography>
                        <Typography>{x.body.state}</Typography>
                      </CardContent>
                    </Card>
                  ))}
                {!data[k].filter((x) => k !== 'letters' || x.unread).length && (
                  <Typography>Nothing waiting here.</Typography>
                )}
                <Button href={`#/family/${k}`}>Open {labels[k]}</Button>
              </>
            ) : k === 'shelves' ? (
              <>
                {data[k]
                  .filter((x) => !x.body.hidden)
                  .slice(0, 6)
                  .map((x) => (
                    <Card
                      key={x.id}
                      variant="outlined"
                      style={{ marginBottom: 12 }}
                    >
                      <CardContent>
                        <RefArtwork
                          reference={x.body.cover || x.body.references[0]}
                        />
                        <Typography variant="h6">
                          <Link
                            color="inherit"
                            href={`#/family/shelves/${x.id}`}
                          >
                            {x.title}
                          </Link>
                        </Typography>
                        <Typography>
                          {x.body.references.length} selections ·{' '}
                          {x.canEdit ? 'Yours' : 'Shared with you'}
                        </Typography>
                      </CardContent>
                    </Card>
                  ))}
                {!data[k].filter((x) => !x.body.hidden).length && (
                  <Typography>
                    Create a private shelf or share one with your family.
                  </Typography>
                )}
                <Button href="#/family/shelves">All shelves</Button>
              </>
            ) : (
              <AlbumCards
                items={data[k].slice(0, 6)}
                onDismiss={k === 'forgotten' ? dismiss : undefined}
              />
            )}
          </section>
        ))}
      </div>
      <Dialog
        className={c.dialog}
        open={!!draft}
        onClose={() => !busy && setDraft(null)}
        fullWidth
      >
        <DialogTitle>Make Home yours</DialogTitle>
        {draft && (
          <DialogContent>
            <Typography paragraph>
              Choose your sections and their order. This changes only your
              account.
            </Typography>
            {[
              ...draft.widgets.filter((key) => labels[key]),
              ...Object.keys(labels).filter(
                (key) => !draft.widgets.includes(key),
              ),
            ].map((k) => (
              <div key={k} className={c.widgetOrder}>
                <FormControlLabel
                  control={
                    <Checkbox
                      checked={draft.widgets.includes(k)}
                      onChange={(e) =>
                        setDraft({
                          ...draft,
                          widgets: e.target.checked
                            ? [...draft.widgets, k]
                            : draft.widgets.filter((x) => x !== k),
                        })
                      }
                    />
                  }
                  label={labels[k]}
                />
                <Button
                  disabled={draft.widgets.indexOf(k) <= 0}
                  onClick={() =>
                    move(draft.widgets.indexOf(k), draft.widgets.indexOf(k) - 1)
                  }
                >
                  Up
                </Button>
                <Button
                  disabled={
                    !draft.widgets.includes(k) ||
                    draft.widgets.indexOf(k) === draft.widgets.length - 1
                  }
                  onClick={() =>
                    move(draft.widgets.indexOf(k), draft.widgets.indexOf(k) + 1)
                  }
                >
                  Down
                </Button>
              </div>
            ))}
            <FormControlLabel
              control={
                <Checkbox
                  checked={draft.forgottenEnabled}
                  onChange={(e) =>
                    setDraft({ ...draft, forgottenEnabled: e.target.checked })
                  }
                />
              }
              label="Enable forgotten album suggestions"
            />
            <TextField
              fullWidth
              type="number"
              id="family-forgotten-days"
              label="Days since last listen"
              value={draft.forgottenDays}
              inputProps={{ min: 7, max: 3650 }}
              onChange={(e) =>
                setDraft({ ...draft, forgottenDays: Number(e.target.value) })
              }
            />
            <TextField
              fullWidth
              select
              id="family-forgotten-mode"
              label="Rediscovery mode"
              value={draft.forgottenMode}
              onChange={(e) =>
                setDraft({ ...draft, forgottenMode: e.target.value })
              }
            >
              <MenuItem value="long">Long time no listen</MenuItem>
              <MenuItem value="favorites">Old favorites</MenuItem>
              <MenuItem value="surprise">Surprise album (weekly)</MenuItem>
            </TextField>
            <Typography role="alert">{error}</Typography>
          </DialogContent>
        )}
        <DialogActions>
          <Button disabled={busy} onClick={() => setDraft(null)}>
            Cancel
          </Button>
          <Button
            disabled={
              busy || draft?.forgottenDays < 7 || draft?.forgottenDays > 3650
            }
            onClick={save}
            variant="contained"
            color="primary"
          >
            Save Home
          </Button>
        </DialogActions>
      </Dialog>
    </div>
  )
}
