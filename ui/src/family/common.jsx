import React, { useState } from 'react'
import {
  Link,
  Button,
  Card,
  CardContent,
  Typography,
  TextField,
  MenuItem,
  Grid,
  makeStyles,
} from '@material-ui/core'
import { httpClient } from '../dataProvider'
import { getContrastRatio } from '@material-ui/core/styles'
import { Artwork } from '../common/Artwork'
export const call = (path, body, method) =>
  httpClient('/api/family' + path, {
    ...(method && { method }),
    ...(body !== undefined && {
      method: method || 'POST',
      body: JSON.stringify(body),
      headers: new Headers({ 'Content-Type': 'application/json' }),
    }),
  }).then(({ json }) => json)
const readableOn = (background) =>
  getContrastRatio(background, '#000') >= getContrastRatio(background, '#fff')
    ? '#000'
    : '#fff'

const familyControls = (theme) => ({
  '& .MuiButton-root': {
    minHeight: 44,
    borderRadius: 10,
    textTransform: 'none',
    fontWeight: 600,
    padding: theme.spacing(1, 2),
    maxWidth: '100%',
    overflowWrap: 'anywhere',
  },
  '& .MuiButton-containedPrimary': {
    backgroundColor: `${theme.palette.primary.main} !important`,
    color: `${readableOn(theme.palette.primary.main)} !important`,
  },
  '& .MuiButton-containedPrimary:hover': {
    backgroundColor: `${theme.palette.primary.dark} !important`,
    color: `${readableOn(theme.palette.primary.dark)} !important`,
  },
  '& .MuiButton-containedPrimary .MuiButton-label': {
    color: 'inherit !important',
  },
  '& .MuiButton-containedSecondary': {
    backgroundColor: `${theme.palette.secondary.main} !important`,
    color: `${readableOn(theme.palette.secondary.main)} !important`,
  },
  '& .MuiButton-containedSecondary:hover': {
    backgroundColor: `${theme.palette.secondary.dark} !important`,
    color: `${readableOn(theme.palette.secondary.dark)} !important`,
  },
  '& .MuiButton-containedSecondary .MuiButton-label': {
    color: 'inherit !important',
  },
  '& .MuiButton-text': { backgroundColor: 'transparent', boxShadow: 'none' },
  '& .MuiButton-text:hover': { backgroundColor: theme.palette.action.hover },
  '& .MuiButton-label': { whiteSpace: 'normal', textAlign: 'center' },
  '& .MuiButton-root:focus-visible, & a:focus-visible': {
    outline: `3px solid ${theme.palette.primary.main}`,
    outlineOffset: 3,
  },
  '& .MuiTextField-root': { marginBottom: theme.spacing(2), minWidth: 0 },
  '& .MuiInputBase-root': { minHeight: 44 },
})

export const useFamilyStyles = makeStyles((theme) => ({
  page: {
    minWidth: 0,
    width: '100%',
    maxWidth: 1440,
    margin: '0 auto',
    padding: theme.spacing(3, 4, 12),
    [theme.breakpoints.down('sm')]: { padding: theme.spacing(2, 2, 12) },
    boxSizing: 'border-box',
    '& > h4, & > h5': {
      fontWeight: 650,
      marginBottom: theme.spacing(2),
      letterSpacing: '-0.025em',
    },
    ...familyControls(theme),
    '& .MuiCard-root': {
      borderRadius: 16,
      border: `1px solid ${theme.palette.divider}`,
      boxShadow: 'none',
      overflowWrap: 'anywhere',
      minWidth: 0,
    },
    '& .MuiCardContent-root': {
      padding: theme.spacing(3),
      '&:last-child': { paddingBottom: theme.spacing(3) },
    },
    '& .MuiCardActions-root': {
      flexWrap: 'wrap',
      gap: theme.spacing(1),
      padding: theme.spacing(1, 3, 3),
      '& > :not(:first-child)': { marginLeft: 0 },
    },
    '& .MuiTypography-h3': {
      fontSize: 'clamp(1.8rem, 3.5vw, 2.5rem)',
      fontWeight: 650,
      letterSpacing: '-0.04em',
      lineHeight: 1.15,
    },
    '& .MuiTypography-h5': {
      fontSize: '1.35rem',
      fontWeight: 600,
      letterSpacing: '-0.02em',
    },
    '& .MuiTypography-h6': { lineHeight: 1.4, fontWeight: 600 },
    '& section': { scrollMarginTop: 80 },
    '& table': { overflowWrap: 'anywhere' },
    '& [role="status"]:empty': { display: 'none' },
  },
  facts: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(min(100%, 240px), 1fr))',
    gap: theme.spacing(2),
    margin: theme.spacing(2, 0, 0),
    '& > div': {
      minWidth: 0,
      padding: theme.spacing(2),
      borderRadius: 10,
      background: theme.palette.action.hover,
    },
    '& dt': { color: theme.palette.text.secondary, fontSize: 12 },
    '& dd': { margin: theme.spacing(1, 0, 0), overflowWrap: 'anywhere' },
  },
  widgetOrder: {
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) auto auto',
    alignItems: 'center',
    gap: theme.spacing(1),
    marginBottom: theme.spacing(1),
    '& .MuiFormControlLabel-root': { marginRight: 0, minWidth: 0 },
    [theme.breakpoints.down('xs')]: {
      gridTemplateColumns: '1fr 1fr',
      '& .MuiFormControlLabel-root': { gridColumn: '1 / -1' },
    },
  },
  dialog: {
    ...familyControls(theme),
    '& .MuiDialog-paper': {
      borderRadius: 16,
      margin: 16,
      overflowWrap: 'anywhere',
    },
    '& .MuiDialog-paperFullWidth': { width: 'calc(100% - 32px)' },
    '& .MuiDialogTitle-root': { padding: theme.spacing(3) },
    '& .MuiDialogContent-root': { padding: theme.spacing(1, 3, 2) },
    '& .MuiDialogActions-root': {
      padding: theme.spacing(2, 3),
      gap: theme.spacing(1),
      flexWrap: 'wrap',
      '& > :not(:first-child)': { marginLeft: 0 },
    },
  },
  hero: {
    marginBottom: theme.spacing(3),
    padding: theme.spacing(3),
    [theme.breakpoints.down('xs')]: { padding: theme.spacing(3, 2) },
    background: `linear-gradient(120deg, ${theme.palette.background.paper}, ${theme.palette.action.hover})`,
    border: `1px solid ${theme.palette.divider}`,
    borderRadius: 20,
  },
  cover: { width: 64, height: 64, borderRadius: 8, flexShrink: 0 },
  albumCover: {
    width: '100%',
    aspectRatio: '1',
    borderRadius: 12,
    display: 'block',
    marginBottom: theme.spacing(2),
  },
  homeSections: {
    display: 'grid',
    gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
    gap: theme.spacing(3),
    [theme.breakpoints.down('sm')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  },
  widget: {
    minWidth: 0,
    padding: theme.spacing(3),
    background: theme.palette.background.paper,
    border: `1px solid ${theme.palette.divider}`,
    borderRadius: 20,
    '&[data-wide="true"]': { gridColumn: '1 / -1' },
    [theme.breakpoints.down('xs')]: { padding: theme.spacing(2) },
  },
  row: {
    display: 'flex',
    gap: theme.spacing(2),
    alignItems: 'center',
    marginBottom: theme.spacing(1),
    minWidth: 0,
  },
  text: { overflowWrap: 'anywhere', whiteSpace: 'pre-wrap' },
  toolbar: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(1),
    marginBottom: theme.spacing(2),
    alignItems: 'center',
    '& > *': { maxWidth: '100%' },
  },
}))
export const refUrl = (r) =>
  r.id
    ? `#/${r.kind === 'track' ? 'song' : r.kind}/${encodeURIComponent(r.id)}/show`
    : undefined
export function RefArtwork({ reference, large = false }) {
  const c = useFamilyStyles()
  if (!reference?.id) return null
  const record = {
    id: reference.id,
    ...(reference.kind === 'album'
      ? { albumArtist: reference.artist || 'Unknown' }
      : reference.kind === 'track'
        ? { album: 'Unknown' }
        : { sync: false }),
  }
  return (
    <Artwork
      record={record}
      size={large ? 400 : 128}
      className={large ? c.albumCover : c.cover}
      title={reference.title}
    />
  )
}
export function ReferenceRow({ reference, children, showArtwork = true }) {
  const c = useFamilyStyles()
  return (
    <div className={c.row}>
      {showArtwork && <RefArtwork reference={reference} />}
      <div style={{ minWidth: 0, flex: 1 }}>
        <Typography className={c.text}>
          {reference.id ? (
            <Link color="inherit" href={refUrl(reference)}>
              {reference.title}
            </Link>
          ) : (
            reference.title
          )}
        </Typography>
        <Typography variant="body2" color="textSecondary">
          {reference.artist}
          {!reference.available && ' · Temporarily unavailable'}
        </Typography>
        {children}
      </div>
    </div>
  )
}
export function ReferencePicker({
  onAdd,
  trackOnly = false,
  noPlaylists = false,
  albumOnly = false,
  hideArtwork = false,
}) {
  const [kind, setKind] = useState(albumOnly ? 'album' : 'track'),
    [q, setQ] = useState(''),
    [items, setItems] = useState([]),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(false)
  const search = async () => {
    setBusy(true)
    setError('')
    try {
      setItems(await call(`/search?kind=${kind}&q=${encodeURIComponent(q)}`))
    } catch (_) {
      setError('Search unavailable. Try again.')
    } finally {
      setBusy(false)
    }
  }
  return (
    <div>
      <Grid container spacing={2}>
        <Grid item xs={12} sm={3}>
          <TextField
            fullWidth
            select
            id="family-reference-type"
            label="Type"
            value={kind}
            onChange={(e) => {
              setKind(e.target.value)
              setItems([])
            }}
          >
            {(albumOnly
              ? ['album']
              : trackOnly
                ? ['track']
                : noPlaylists
                  ? ['track', 'album']
                  : ['track', 'album', 'playlist']
            ).map((k) => (
              <MenuItem key={k} value={k}>
                {k}
              </MenuItem>
            ))}
          </TextField>
        </Grid>
        <Grid item xs={12} sm={6}>
          <TextField
            fullWidth
            id="family-reference-search"
            label="Search your library"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            inputProps={{ maxLength: 100 }}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && q.trim().length >= 2) {
                e.preventDefault()
                search()
              }
            }}
          />
        </Grid>
        <Grid item xs={12} sm={3}>
          <Button disabled={busy || q.trim().length < 2} onClick={search}>
            Search
          </Button>
        </Grid>
      </Grid>
      <Typography role="status">{error}</Typography>
      {items.map((r) => (
        <ReferenceRow
          key={r.kind + r.id}
          reference={r}
          showArtwork={!hideArtwork}
        >
          <Button onClick={() => onAdd(r)}>Select {r.title}</Button>
        </ReferenceRow>
      ))}
    </div>
  )
}
export function AlbumCards({ items, onDismiss }) {
  return (
    <Grid container spacing={2}>
      {items.map((r) => (
        <Grid item xs={6} md={4} lg={3} key={r.id}>
          <Card variant="outlined">
            <CardContent>
              <Link color="inherit" href={refUrl(r)} aria-label={r.title}>
                <RefArtwork reference={r} large />
              </Link>
              <ReferenceRow reference={r} showArtwork={false} />
              {onDismiss && (
                <Button onClick={() => onDismiss(r.id)}>Not interested</Button>
              )}
            </CardContent>
          </Card>
        </Grid>
      ))}
      {!items.length && (
        <Grid item xs={12}>
          <Typography color="textSecondary">
            Nothing here yet. Your music stays in its existing libraries.
          </Typography>
        </Grid>
      )}
    </Grid>
  )
}
