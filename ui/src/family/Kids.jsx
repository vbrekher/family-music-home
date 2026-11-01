import React, { useEffect, useState } from 'react'
import { Title, useDataProvider } from 'react-admin'
import { useDispatch, useSelector } from 'react-redux'
import {
  Button,
  Card,
  CardContent,
  Grid,
  Typography,
  makeStyles,
  Link,
  TextField,
  MenuItem,
} from '@material-ui/core'
import { call, useFamilyStyles, RefArtwork } from './common'
import { playTracks, changeTheme } from '../actions'
import themes from '../themes'
import { AUTO_THEME_ID } from '../consts'
import { httpClient } from '../dataProvider'
import subsonic from '../subsonic'
import FamilyNav from './FamilyNav'
const useStyles = makeStyles((theme) => ({
  card: {
    height: '100%',
    '& img': { width: '100%', height: 'auto', maxWidth: 180 },
    '& button': { minHeight: 48, margin: theme.spacing(0.5) },
  },
  title: { fontSize: (p) => (p.large ? 24 : 19), overflowWrap: 'anywhere' },
  tabs: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(1),
    marginBottom: theme.spacing(3),
    '& button': { minHeight: 48 },
  },
}))
export default function Kids() {
  const c = useFamilyStyles(),
    [data, setData] = useState(null),
    [me, setMe] = useState(null),
    [section, setSection] = useState('songs'),
    [offset, setOffset] = useState(0),
    [error, setError] = useState(''),
    [busy, setBusy] = useState('')
  const themeId = useSelector((s) => s.theme || AUTO_THEME_ID)
  const k = useStyles({ large: data?.profile?.density !== 'comfortable' }),
    provider = useDataProvider(),
    dispatch = useDispatch()
  const load = () =>
    call(`/kids?section=${section}&offset=${offset}`)
      .then(setData)
      .catch(() =>
        setError(
          'Music is temporarily unavailable. Try again when your connection is back.',
        ),
      )
  useEffect(() => {
    call('/me')
      .then(setMe)
      .catch(() => setError('Sign in to your family server.'))
  }, [])
  useEffect(() => {
    let active = true
    call(`/kids?section=${section}&offset=${offset}`)
      .then((v) => {
        if (active) setData(v)
      })
      .catch(() => {
        if (active)
          setError(
            'Music is temporarily unavailable. Try again when your connection is back.',
          )
      })
    return () => {
      active = false
    }
  }, [section, offset])
  const play = async (track) => {
    setBusy(track.id)
    setError('')
    try {
      const { data } = await provider.getOne('song', { id: track.id })
      dispatch(playTracks({ [data.id]: data }, [data.id]))
    } catch (_) {
      setError('This recording is unavailable. Your collection has been kept.')
    } finally {
      setBusy('')
    }
  }
  const favorite = async (track) => {
    setBusy(track.id)
    try {
      await (track.starred
        ? subsonic.unstar(track.id)
        : subsonic.star(track.id))
      await load()
    } catch (_) {
      setError('Could not save your favorite. Please try again.')
    } finally {
      setBusy('')
    }
  }
  const theme = async (id) => {
    try {
      await httpClient('/api/personal/theme', {
        method: 'PUT',
        body: JSON.stringify({ theme: id }),
        headers: new Headers({ 'Content-Type': 'application/json' }),
      })
      dispatch(changeTheme(id))
    } catch (_) {
      setError('Could not save your theme. Please try again.')
    }
  }
  return (
    <div className={c.page}>
      <Title title="Kids Music World" />
      <FamilyNav />
      <Typography variant="h3" gutterBottom>
        Music for you
      </Typography>
      <Typography paragraph>
        Songs, stories and collections chosen for your account. Tap Play when
        you are ready.
      </Typography>
      <Typography role="status">{error}</Typography>
      {me && !me.child && (
        <Card>
          <CardContent>
            <Typography>
              No child profile has been created for this account.
            </Typography>
            {me.admin && (
              <Button
                href="#/family/children"
                variant="contained"
                color="primary"
              >
                Manage future child accounts
              </Button>
            )}
          </CardContent>
        </Card>
      )}
      {data?.profile && !data.profile.enabled && (
        <Typography role="status">
          Your parent has paused this account. Your music history and favorites
          are safe.
        </Typography>
      )}
      {data?.profile?.enabled && (
        <>
          <nav className={k.tabs} aria-label="Choose your music">
            {[
              ['songs', 'Songs'],
              ['stories', 'Stories'],
              ['favorites', 'Favorites'],
              ['collections', 'Collections'],
            ].map(([id, name]) => (
              <Button
                key={id}
                variant={section === id ? 'contained' : 'outlined'}
                color="primary"
                aria-pressed={section === id}
                onClick={() => {
                  setSection(id)
                  setOffset(0)
                }}
              >
                {name}
              </Button>
            ))}
            <Button onClick={load}>Refresh</Button>
            <TextField
              select
              id="family-kids-theme"
              label="Your theme"
              value={themeId}
              onChange={(e) => theme(e.target.value)}
              style={{ minWidth: 180, maxWidth: '100%' }}
            >
              <MenuItem value={AUTO_THEME_ID}>Follow device</MenuItem>
              {Object.keys(themes).map((id) => (
                <MenuItem key={id} value={id}>
                  {themes[id].themeName}
                </MenuItem>
              ))}
            </TextField>
          </nav>
          <Grid container spacing={2}>
            {(data.tracks || []).map((track) => (
              <Grid item xs={12} sm={6} md={4} key={track.id}>
                <Card className={k.card}>
                  <CardContent>
                    <RefArtwork reference={{ kind: 'track', ...track }} />
                    <Typography className={k.title} component="h2">
                      {track.title}
                    </Typography>
                    <Typography>{track.artist}</Typography>
                    {!track.available && (
                      <Typography>Temporarily unavailable</Typography>
                    )}
                    <Button
                      variant="contained"
                      color="primary"
                      disabled={!!busy || !track.available}
                      onClick={() => play(track)}
                      aria-label={`Play ${track.title}`}
                    >
                      Play
                    </Button>
                    <Button
                      disabled={!!busy}
                      onClick={() => favorite(track)}
                      aria-label={`${track.starred ? 'Remove favorite' : 'Favorite'} ${track.title}`}
                    >
                      {track.starred ? '★ Favorite' : '☆ Favorite'}
                    </Button>
                  </CardContent>
                </Card>
              </Grid>
            ))}
          </Grid>
          {(data.shelves || []).map((shelf) => (
            <Card key={shelf.id} className={k.card}>
              <CardContent>
                <Typography component="h2" className={k.title}>
                  {shelf.title}
                </Typography>
                <Typography>{shelf.body.description}</Typography>
                <Button
                  href={`#/family/shelves/${encodeURIComponent(shelf.id)}`}
                >
                  Open collection
                </Button>
              </CardContent>
            </Card>
          ))}
          {!data.tracks?.length && !data.shelves?.length && (
            <Typography>
              No music in this section yet. Your parent can choose the libraries
              and collections for you.
            </Typography>
          )}
          <div className={c.toolbar}>
            <Button
              disabled={!offset}
              onClick={() =>
                setOffset(
                  Math.max(0, offset - (section === 'collections' ? 25 : 24)),
                )
              }
            >
              Previous
            </Button>
            <Button
              disabled={
                (section === 'collections'
                  ? data.shelves?.length
                  : data.tracks?.length) < (section === 'collections' ? 25 : 24)
              }
              onClick={() =>
                setOffset(offset + (section === 'collections' ? 25 : 24))
              }
            >
              Next
            </Button>
            <Link href="#/family/requests">Ask for music</Link>
          </div>
        </>
      )}
    </div>
  )
}
