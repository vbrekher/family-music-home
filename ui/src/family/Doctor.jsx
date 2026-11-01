import React, { useEffect, useRef, useState } from 'react'
import { Title } from 'react-admin'
import { useParams } from 'react-router-dom'
import {
  Button,
  Typography,
  Card,
  CardContent,
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
import { call, useFamilyStyles, ReferencePicker, ReferenceRow } from './common'
import { mediaFetch } from './Media'
import FamilyNav from './FamilyNav'
const MiB = 1024 * 1024
const pretty = (n) => `${(n / MiB).toFixed(2)} MiB`
export function Spectrum({ result }) {
  if (!result.spectralBands?.length) return null
  const maximum = Math.max(...result.spectralBands, 0.0000001)
  return (
    <figure style={{ margin: '12px 0' }}>
      <svg
        role="img"
        aria-label="Sampled spectral band energy, not proof of audio provenance"
        viewBox="0 0 320 80"
        style={{ width: '100%', maxWidth: 400 }}
      >
        <title>Sampled relative band energy</title>
        {result.spectralBands.map((n, i) => (
          <rect
            key={i}
            x={(i * 320) / result.spectralBands.length + 2}
            y={60 - (n / maximum) * 55}
            width={320 / result.spectralBands.length - 4}
            height={(n / maximum) * 55}
            fill="currentColor"
          />
        ))}
        <text x="0" y="77" fill="currentColor" fontSize="10">
          0 Hz
        </text>
        <text x="250" y="77" fill="currentColor" fontSize="10">
          {Math.round((result.measuredSampleRate || 0) / 2)} Hz
        </text>
      </svg>
      <figcaption>
        {result.bandwidthNote} Bands are normalized within this sampled signal;
        they are not a quality score.
      </figcaption>
    </figure>
  )
}
export default function Doctor() {
  const c = useFamilyStyles(),
    { id } = useParams(),
    [me, setMe] = useState(null),
    [jobs, setJobs] = useState([]),
    [job, setJob] = useState(null),
    [items, setItems] = useState([]),
    [findings, setFindings] = useState([]),
    [duplicates, setDuplicates] = useState([]),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(false),
    [offset, setOffset] = useState(0),
    [itemOffset, setItemOffset] = useState(0),
    [findingOffset, setFindingOffset] = useState(0),
    [severity, setSeverity] = useState(''),
    [create, setCreate] = useState(null),
    [selected, setSelected] = useState([]),
    [budgets, setBudgets] = useState({}),
    [quality, setQuality] = useState(true),
    [fingerprint, setFingerprint] = useState(false),
    [confirmed, setConfirmed] = useState(false),
    [seconds, setSeconds] = useState(120),
    [maxRows, setMaxRows] = useState(1000),
    [remove, setRemove] = useState(false),
    [runningMetadata, setRunningMetadata] = useState(false),
    [budgetEdit, setBudgetEdit] = useState(null),
    [notice, setNotice] = useState('')
  const run = useRef(false),
    view = useRef(id),
    runGeneration = useRef(0)
  view.current = id
  useEffect(() => {
    runGeneration.current++
    run.current = false
    setRunningMetadata(false)
    setJob(null)
    setItems([])
    setFindings([])
    setDuplicates([])
  }, [id])
  const load = async () => {
    try {
      if (id) {
        const [j, i, f, d] = await Promise.all([
          call(`/doctor/jobs/${id}`),
          call(`/doctor/jobs/${id}/items?offset=${itemOffset}`),
          call(
            `/doctor/jobs/${id}/findings?offset=${findingOffset}&severity=${severity}`,
          ),
          call(`/doctor/jobs/${id}/duplicates`),
        ])
        if (view.current !== id) return
        setJob(j)
        setItems(i)
        setFindings(f)
        setDuplicates(d)
      } else setJobs(await call(`/doctor/jobs?offset=${offset}`))
    } catch (_) {
      setError(
        'Diagnostics unavailable. Existing findings and original music are preserved.',
      )
    }
  }
  useEffect(() => {
    let active = true
    call('/me')
      .then((m) => {
        if (active) setMe(m)
      })
      .catch(() => setError('Account unavailable.'))
    return () => {
      active = false
      run.current = false
    }
  }, [])
  useEffect(() => {
    if (!me?.admin) return
    setError('')
    load()
  }, [id, me?.admin, offset, itemOffset, findingOffset, severity])
  useEffect(() => {
    if (!id || !me?.admin || ['completed', 'cancelled'].includes(job?.state))
      return
    const timer = setInterval(
      () => {
        if (!run.current && !document.hidden) load()
      },
      job?.state === 'active' ? 2500 : 5000,
    )
    return () => clearInterval(timer)
  }, [id, me?.admin, itemOffset, findingOffset, severity, job?.state])
  const action = async (fn) => {
    setBusy(true)
    setError('')
    try {
      const reload = await fn()
      if (reload !== false) await load()
    } catch (e) {
      setError(
        e.status === 409
          ? 'Run state changed or another run is active. Refresh and inspect its state.'
          : e.status === 413
            ? 'Diagnostic storage or I/O quota reached. Export and remove finished reports, or review explicit source budgets.'
            : 'Operation rejected. Check access, selected files, budgets and fields. No original music was changed.',
      )
    } finally {
      setBusy(false)
    }
  }
  const control = (name, retry = false) => {
    run.current = false
    setRunningMetadata(false)
    return action(() =>
      call(`/doctor/jobs/${id}/control`, { action: name, retry }),
    )
  }
  const start = () =>
    action(async () => {
      const settings =
        create === 'metadata'
          ? { maxRows }
          : {
              sourceBudgets: Object.fromEntries(
                [...new Set(selected.map((x) => x.libraryId))].map((k) => [
                  k,
                  Math.floor(Number(budgets[k]) * MiB),
                ]),
              ),
              quality,
              fingerprint,
              decodeSeconds: seconds,
              confirmed,
            }
      const x = await call('/doctor/jobs', {
        mode: create,
        settings,
        trackIds: selected.map((x) => x.id),
      })
      setCreate(null)
      window.location.hash = `#/family/doctor/${x.id}`
      return false
    })
  const metadata = async () => {
    if (run.current) return
    const generation = ++runGeneration.current
    run.current = true
    setRunningMetadata(true)
    setError('')
    try {
      let current = job
      if (current.state === 'paused')
        current = await call(`/doctor/jobs/${id}/control`, { action: 'resume' })
      while (
        run.current &&
        generation === runGeneration.current &&
        ['queued', 'active'].includes(current.state)
      ) {
        current = await call(`/doctor/jobs/${id}/step`, {})
        if (view.current !== id || generation !== runGeneration.current) break
        setJob(current)
        if (current.state === 'completed') break
        await new Promise((resolve) => setTimeout(resolve, 50))
      }
      if (view.current === id && generation === runGeneration.current)
        await load()
    } catch (_) {
      if (view.current === id && generation === runGeneration.current)
        setError(
          'Audit paused or interrupted. Saved cursor/findings remain; refresh to resume.',
        )
    } finally {
      if (generation === runGeneration.current) {
        run.current = false
        setRunningMetadata(false)
      }
    }
  }
  const download = () =>
    action(async () => {
      const r = await mediaFetch(`/doctor/jobs/${id}/export`)
      const url = URL.createObjectURL(await r.blob())
      const a = document.createElement('a')
      a.href = url
      a.download = 'library-doctor.json'
      a.click()
      setTimeout(() => URL.revokeObjectURL(url), 1000)
    })
  const originArg = "'" + window.location.origin.replace(/'/g, "'\\''") + "'"
  const command = `python3 macos/navidrome_family_doctor.py --server ${originArg} --job ${job?.id || ''}${job?.state === 'paused' ? ' --resume' : ''}`
  if (me && !me.admin)
    return (
      <div className={c.page}>
        <FamilyNav />
        <Typography>
          This read-only investigation section is available to the Owner. Your
          normal libraries remain available.
        </Typography>
      </div>
    )
  return (
    <div className={c.page}>
      <Title title="Library Doctor" />
      <FamilyNav />
      <Typography variant="h4" gutterBottom>
        Library Doctor
      </Typography>
      <Typography paragraph>
        Investigate your collection without retagging, rewriting, moving or
        deleting original music. Start with catalog metadata; audio reads
        require selected files, explicit source budgets and the local Mac
        companion.
      </Typography>
      <div className={c.toolbar}>
        {!id && (
          <>
            <Button
              color="primary"
              variant="contained"
              disabled={!me?.admin || busy}
              onClick={() => {
                setCreate('metadata')
                setMaxRows(1000)
              }}
            >
              New catalog audit
            </Button>
            <Button
              disabled={!me?.admin || busy}
              onClick={() => {
                setCreate('deep')
                setSelected([])
                setBudgets({})
                setConfirmed(false)
              }}
            >
              Investigate selected audio
            </Button>
          </>
        )}
        <Button onClick={load}>Refresh</Button>
      </div>
      <Typography role="status">{error || notice}</Typography>
      {!id && (
        <>
          {!jobs.length && (
            <Typography>
              No diagnostic runs yet. Browsing Home never starts one.
            </Typography>
          )}
          {jobs.map((x) => (
            <Card key={x.id} style={{ marginBottom: 12 }}>
              <CardContent>
                <Typography variant="h6">
                  <Link color="inherit" href={`#/family/doctor/${x.id}`}>
                    {x.mode === 'metadata'
                      ? 'Catalog audit'
                      : 'Selected audio investigation'}{' '}
                    · {new Date(x.createdAt).toLocaleString()}
                  </Link>
                </Typography>
                <Typography>
                  {x.state} · {x.processed} records processed · {x.findingCount}{' '}
                  recorded findings{x.findingsCapped ? ' (capped)' : ''}
                </Typography>
              </CardContent>
            </Card>
          ))}
          <Button
            disabled={!offset}
            onClick={() => setOffset(Math.max(0, offset - 25))}
          >
            Previous
          </Button>
          <Button
            disabled={jobs.length < 25}
            onClick={() => setOffset(offset + 25)}
          >
            Next
          </Button>
        </>
      )}
      {job && (
        <>
          <Card style={{ marginBottom: 20 }}>
            <CardContent>
              <Typography variant="h5">
                {job.mode === 'metadata'
                  ? 'Catalog metadata audit'
                  : 'Mac audio investigation'}
              </Typography>
              <Typography>
                {job.state} · {job.processed} records processed ·{' '}
                {job.findingCount} findings
                {job.findingsCapped
                  ? ' (priority limits reached; use smaller windows for detail)'
                  : ''}
              </Typography>
              <Typography variant="body2">
                Findings reserve separate info/warning/error slots, so
                optional-tag noise cannot consume error capacity. Run finished
                does not mean every file is healthy. Offline/incomplete attempts
                retain previous successful evidence. No music reconciliation or
                deletion is performed.
              </Typography>
              {job.mode === 'metadata' ? (
                <>
                  <Typography>
                    Row budget: {job.settings.maxRows}. Cursor:{' '}
                    {job.cursor || 'start'}. A completed budget window may cover
                    only part of your collection.
                  </Typography>
                  {['queued', 'active', 'paused'].includes(job.state) && (
                    <Button
                      disabled={busy || runningMetadata}
                      onClick={metadata}
                    >
                      Run / resume SQL audit
                    </Button>
                  )}
                  {job.state === 'completed' && job.cursor && (
                    <Button
                      disabled={busy}
                      onClick={() =>
                        action(async () => {
                          const x = await call('/doctor/jobs', {
                            mode: 'metadata',
                            settings: { ...job.settings, afterId: job.cursor },
                          })
                          window.location.hash = `#/family/doctor/${x.id}`
                          return false
                        })
                      }
                    >
                      Continue in another bounded window
                    </Button>
                  )}
                </>
              ) : (
                <>
                  <Typography paragraph>
                    Only the chosen files can be read. Reserved bytes are a
                    conservative upper bound: each retry reserves another
                    complete file payload. Protocol headers are not included.
                  </Typography>
                  {job.budgets.map((b) => (
                    <Typography key={b.libraryId}>
                      Library {b.libraryId}: {pretty(b.reservedBytes)} reserved
                      / {pretty(b.limitBytes)} approved
                    </Typography>
                  ))}
                  <Typography>
                    Checks: SHA-256
                    {job.settings.quality
                      ? ' · strict decode, full-scale samples and sampled spectrum'
                      : ''}
                    {job.settings.fingerprint
                      ? ' · optional local Chromaprint'
                      : ''}
                    . Local analysis deadline: {job.settings.decodeSeconds}{' '}
                    seconds per file.
                  </Typography>
                  <Typography paragraph>
                    Run the command from your checked-out project on the Mac
                    after building its local Doctor image. It reuses the
                    existing Family Sync Owner Keychain item. There is no
                    always-on analysis worker.
                  </Typography>
                  <Typography component="code" className={c.text}>
                    {command}
                  </Typography>
                  <Button
                    onClick={async () => {
                      try {
                        await navigator.clipboard.writeText(command)
                        setNotice('Mac command copied.')
                      } catch (_) {
                        setNotice('Select and copy the displayed command.')
                      }
                    }}
                  >
                    Copy Mac command
                  </Button>
                  <Button
                    disabled={busy || job.state === 'cancelled'}
                    onClick={() =>
                      setBudgetEdit(
                        Object.fromEntries(
                          job.budgets.map((b) => [
                            b.libraryId,
                            b.limitBytes / MiB,
                          ]),
                        ),
                      )
                    }
                  >
                    Review / raise payload budgets
                  </Button>
                </>
              )}
              <div className={c.toolbar}>
                {['queued', 'active'].includes(job.state) && (
                  <Button disabled={busy} onClick={() => control('pause')}>
                    Pause run
                  </Button>
                )}
                {job.mode === 'deep' &&
                  ['paused', 'completed'].includes(job.state) && (
                    <Button
                      disabled={busy}
                      onClick={() => control('resume', true)}
                    >
                      Queue unfinished items for retry
                    </Button>
                  )}
                {!['completed', 'cancelled'].includes(job.state) && (
                  <Button disabled={busy} onClick={() => control('cancel')}>
                    Cancel run
                  </Button>
                )}
                <Button disabled={busy} onClick={download}>
                  Export diagnostic JSON
                </Button>
                {['completed', 'cancelled'].includes(job.state) && (
                  <Button disabled={busy} onClick={() => setRemove(true)}>
                    Remove finished report
                  </Button>
                )}
              </div>
            </CardContent>
          </Card>
          <section aria-label="Duplicate evidence">
            <Typography variant="h5">Duplicate evidence</Typography>
            {!duplicates.length && (
              <Typography>
                No proven byte matches or sufficiently strong acoustical
                candidates in this run. Matching titles alone are never
                evidence.
              </Typography>
            )}
            {duplicates.map((d, i) => (
              <Card key={i} style={{ marginBottom: 12 }}>
                <CardContent>
                  <Typography variant="h6">
                    {d.kind === 'exact-bytes'
                      ? 'Byte-identical streams'
                      : 'Probable recording match'}
                  </Typography>
                  <Typography>{d.evidence}</Typography>
                  {d.similarity != null && (
                    <Typography>
                      Fingerprint bit agreement:{' '}
                      {(d.similarity * 100).toFixed(1)}% · not a probability of
                      identical mastering
                    </Typography>
                  )}
                  {d.trackIds.map((key) => (
                    <Button
                      key={key}
                      href={`#/family/passport?track=${encodeURIComponent(key)}`}
                    >
                      Passport {key}
                    </Button>
                  ))}
                </CardContent>
              </Card>
            ))}
          </section>
          <section aria-label="Diagnostic findings">
            <Typography variant="h5">Findings & evidence</Typography>
            <TextField
              select
              id="family-doctor-severity"
              label="Severity"
              value={severity}
              onChange={(e) => {
                setSeverity(e.target.value)
                setFindingOffset(0)
              }}
            >
              {[
                ['', 'All'],
                ['info', 'Info'],
                ['warning', 'Warning'],
                ['error', 'Error'],
              ].map(([v, label]) => (
                <MenuItem key={v} value={v}>
                  {label}
                </MenuItem>
              ))}
            </TextField>
            {findings.map((f) => (
              <Card key={f.id} style={{ marginBottom: 12 }}>
                <CardContent>
                  <Typography>
                    {f.severity} · {f.code.replaceAll('_', ' ')}
                  </Typography>
                  <Typography>{f.evidence}</Typography>
                  <Button
                    href={`#/family/passport?track=${encodeURIComponent(f.trackId)}`}
                  >
                    Inspect recording {f.trackId}
                  </Button>
                </CardContent>
              </Card>
            ))}
            {!findings.length && (
              <Typography>
                No recorded findings in this view. Unanalyzed audio remains
                unknown.
              </Typography>
            )}
            <Button
              disabled={!findingOffset}
              onClick={() => setFindingOffset(Math.max(0, findingOffset - 100))}
            >
              Previous findings
            </Button>
            <Button
              disabled={findings.length < 100}
              onClick={() => setFindingOffset(findingOffset + 100)}
            >
              More findings
            </Button>
          </section>
          <section aria-label="Investigated recordings">
            <Typography variant="h5">Recordings in this window</Typography>
            {items.map((x) => (
              <Card key={x.track.id} style={{ marginBottom: 12 }}>
                <CardContent>
                  <Typography variant="h6">{x.track.title}</Typography>
                  <Typography>
                    {x.track.artist} · {x.track.library} · {x.state} · attempts:{' '}
                    {x.attempts}
                  </Typography>
                  <Typography variant="body2">
                    Recorded payload bytes: {pretty(x.bytesRead)}. Interrupted
                    transfers may have only a lower-bound record; approved
                    reservations bound retries.
                  </Typography>
                  {x.sha256 && (
                    <Typography variant="caption" className={c.text}>
                      SHA-256: {x.sha256}
                    </Typography>
                  )}
                  {x.result.note && <Typography>{x.result.note}</Typography>}
                  {x.result.decode && (
                    <>
                      <Typography>Local decode: {x.result.decode}.</Typography>
                      <Typography>
                        {x.result.samples > 0
                          ? `Near-full-scale samples: ${x.result.nearFullScale} / ${x.result.samples} · observed peak: ${x.result.peak}. This is a clipping heuristic.`
                          : 'PCM measurements have not been collected.'}
                      </Typography>
                      <Spectrum result={x.result} />
                      <Typography>
                        {x.result.fingerprint?.length
                          ? `${x.result.fingerprint.length} Chromaprint values (up to 120 seconds).`
                          : 'Acoustical fingerprint not collected.'}
                      </Typography>
                    </>
                  )}
                  <Button
                    href={`#/family/passport?track=${encodeURIComponent(x.track.id)}`}
                  >
                    Quality passport
                  </Button>
                </CardContent>
              </Card>
            ))}
            <Button
              disabled={!itemOffset}
              onClick={() => setItemOffset(Math.max(0, itemOffset - 50))}
            >
              Previous recordings
            </Button>
            <Button
              disabled={items.length < 50}
              onClick={() => setItemOffset(itemOffset + 50)}
            >
              More recordings
            </Button>
          </section>
        </>
      )}
      <Dialog
        className={c.dialog}
        open={!!create}
        onClose={() => !busy && setCreate(null)}
        fullWidth
        maxWidth="md"
      >
        <DialogTitle>
          {create === 'metadata'
            ? 'New bounded catalog audit'
            : 'Investigate selected audio on the Mac'}
        </DialogTitle>
        <DialogContent>
          {create === 'metadata' ? (
            <>
              <Typography paragraph>
                Only SQLite metadata is read. Missing optional tags and
                numbering gaps are evidence to review, not an audio-quality
                score.
              </Typography>
              <TextField
                fullWidth
                type="number"
                id="family-doctor-max-rows"
                label="Maximum catalog records in this window"
                value={maxRows}
                inputProps={{ min: 1, max: 10000 }}
                onChange={(e) => setMaxRows(Number(e.target.value))}
              />
            </>
          ) : (
            <>
              <Typography paragraph>
                Select up to 100 recordings. Nothing is downloaded until you
                explicitly run the Mac command. Sources remain read-only.
              </Typography>
              {selected.map((x) => (
                <ReferenceRow
                  key={x.id}
                  reference={{ ...x, kind: 'track', available: x.available }}
                  showArtwork={false}
                >
                  <Typography>
                    {x.library} · {pretty(x.size)}
                  </Typography>
                  <Button
                    onClick={() =>
                      setSelected(selected.filter((v) => v.id !== x.id))
                    }
                  >
                    Remove selection
                  </Button>
                </ReferenceRow>
              ))}
              <ReferencePicker
                hideArtwork
                trackOnly
                onAdd={async (r) => {
                  try {
                    const x = await call(`/doctor/tracks/${r.id}`)
                    setSelected((old) =>
                      old.length >= 100 || old.some((v) => v.id === x.id)
                        ? old
                        : [...old, x],
                    )
                    setBudgets((old) => ({
                      ...old,
                      [x.libraryId]: old[x.libraryId] || 128,
                    }))
                  } catch (_) {
                    setError('Recording unavailable for investigation.')
                  }
                }}
              />
              {[...new Set(selected.map((x) => x.libraryId))].map((lib) => (
                <TextField
                  key={lib}
                  fullWidth
                  type="number"
                  id={`family-doctor-budget-${lib}`}
                  label={`${selected.find((x) => x.libraryId === lib)?.library} approved payload MiB (includes retries)`}
                  value={budgets[lib] || 128}
                  inputProps={{ min: 1, max: 1024 }}
                  onChange={(e) =>
                    setBudgets({ ...budgets, [lib]: Number(e.target.value) })
                  }
                />
              ))}
              <FormControlLabel
                control={
                  <Checkbox
                    checked={quality}
                    onChange={(e) => setQuality(e.target.checked)}
                  />
                }
                label="Strict audio decode, clipping heuristic and limited sampled spectrum"
              />
              <FormControlLabel
                control={
                  <Checkbox
                    checked={fingerprint}
                    onChange={(e) => setFingerprint(e.target.checked)}
                  />
                }
                label="Optional local acoustical fingerprint (not mastering identification)"
              />
              <TextField
                fullWidth
                type="number"
                id="family-doctor-seconds"
                label="Local analysis seconds per file"
                value={seconds}
                inputProps={{ min: 10, max: 600 }}
                onChange={(e) => setSeconds(Number(e.target.value))}
              />
              <FormControlLabel
                control={
                  <Checkbox
                    checked={confirmed}
                    onChange={(e) => setConfirmed(e.target.checked)}
                  />
                }
                label="I explicitly approve reading these files within these source budgets"
              />
              <Typography variant="body2">
                Per source: at most 1 GiB; total: at most 2 GiB per run.
                Analysis uses an isolated local Docker container with no
                network, credentials or writable original music.
              </Typography>
            </>
          )}
        </DialogContent>
        <DialogActions>
          <Typography role="alert">{error}</Typography>
          <Button disabled={busy} onClick={() => setCreate(null)}>
            Cancel
          </Button>
          <Button
            disabled={
              busy ||
              (create === 'deep' &&
                (!selected.length ||
                  !confirmed ||
                  seconds < 10 ||
                  seconds > 600)) ||
              (create === 'metadata' && (maxRows < 1 || maxRows > 10000))
            }
            onClick={start}
          >
            Create bounded run
          </Button>
        </DialogActions>
      </Dialog>
      <Dialog
        className={c.dialog}
        open={!!budgetEdit}
        onClose={() => !busy && setBudgetEdit(null)}
        fullWidth
      >
        <DialogTitle>Explicitly raise source payload limits</DialogTitle>
        <DialogContent>
          <Typography>
            Reservations already consumed by previous attempts are retained.
            Raising a limit does not start a worker or change original music.
          </Typography>
          {budgetEdit &&
            Object.keys(budgetEdit).map((lib) => (
              <TextField
                key={lib}
                fullWidth
                type="number"
                id={`family-doctor-raise-${lib}`}
                label={`Library ${lib} limit MiB`}
                value={budgetEdit[lib]}
                onChange={(e) =>
                  setBudgetEdit({
                    ...budgetEdit,
                    [lib]: Number(e.target.value),
                  })
                }
              />
            ))}
        </DialogContent>
        <DialogActions>
          <Button disabled={busy} onClick={() => setBudgetEdit(null)}>
            Cancel
          </Button>
          <Button
            disabled={busy}
            onClick={() =>
              action(async () => {
                await call(`/doctor/jobs/${id}/budgets`, {
                  limits: Object.fromEntries(
                    Object.entries(budgetEdit).map(([k, v]) => [
                      k,
                      Math.floor(v * MiB),
                    ]),
                  ),
                })
                setBudgetEdit(null)
              })
            }
          >
            Approve new limits
          </Button>
        </DialogActions>
      </Dialog>
      <Dialog
        className={c.dialog}
        open={remove}
        onClose={() => !busy && setRemove(false)}
      >
        <DialogTitle>Remove this finished diagnostic report?</DialogTitle>
        <DialogContent>
          <Typography>
            This removes only the run’s progress and findings. Original music,
            catalog entries and independently stored last successful track
            evidence are kept. Export the report first if you want to retain it.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button disabled={busy} onClick={() => setRemove(false)}>
            Keep report
          </Button>
          <Button
            disabled={busy}
            onClick={() =>
              action(async () => {
                await call(`/doctor/jobs/${id}/remove-report`, {
                  confirmed: true,
                })
                setRemove(false)
                window.location.hash = '#/family/doctor'
                return false
              })
            }
          >
            Remove diagnostic report
          </Button>
        </DialogActions>
      </Dialog>
    </div>
  )
}
