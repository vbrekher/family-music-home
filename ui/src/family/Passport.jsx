import FamilyNav from './FamilyNav'
import React, { useEffect, useState } from 'react'
import { Title } from 'react-admin'
import { Typography, Card, CardContent } from '@material-ui/core'
import { call, useFamilyStyles, ReferencePicker } from './common'
const fields = {
  sourceType: 'Source',
  container: 'Container',
  codec: 'Codec',
  compression: 'Compression',
  libraryId: 'Library',
  sampleRate: 'Sample rate (Hz)',
  bitDepth: 'Bit depth',
  bitRate: 'Nominal bitrate (kbps)',
  channels: 'Channels',
  duration: 'Duration (seconds)',
  size: 'File size (bytes)',
  metadataUpdatedAt: 'Catalog metadata updated',
  integrity: 'Integrity analysis',
  mastering: 'Mastering',
  provenance: 'Audio provenance',
}
export default function Passport() {
  const c = useFamilyStyles(),
    [value, setValue] = useState(null),
    [error, setError] = useState('')
  const inspect = async (r) => {
    setError('')
    setValue(null)
    try {
      setValue(await call('/passport/' + encodeURIComponent(r.id)))
    } catch (_) {
      setError('Recording unavailable or access has changed.')
    }
  }
  useEffect(() => {
    const key = new URLSearchParams(
      window.location.hash.split('?')[1] || '',
    ).get('track')
    if (key && /^[A-Za-z0-9_-]{8,80}$/.test(key)) inspect({ id: key })
  }, [])
  return (
    <div className={c.page}>
      <FamilyNav />
      <Title title="Music Quality Passport" />
      <Typography variant="h4" gutterBottom>
        Music Quality Passport
      </Typography>
      <Typography paragraph>
        Technical facts from the catalog. A high sample rate or bit depth does
        not prove superior sound or the origin of a recording. Unknown
        information stays unknown.
      </Typography>
      <ReferencePicker trackOnly onAdd={inspect} />
      <Typography role="status">{error}</Typography>
      {value?.analysis && (
        <Card style={{ marginBottom: 16 }}>
          <CardContent>
            <Typography variant="h6">Requested local investigation</Typography>
            <Typography>
              Last successful evidence:{' '}
              {value.analysis.verifiedAt ||
                'No successful verification recorded'}
            </Typography>
            <Typography className={c.text}>
              SHA-256: {value.analysis.sha256}
            </Typography>
            <Typography>
              Decode: {value.analysis.decode || 'Not verified'}
              {value.analysis.evidenceStaleOrIncomplete
                ? ' · Evidence is incomplete or catalog/content changed.'
                : ''}
            </Typography>
            <Typography>
              Measured codec: {value.analysis.measuredCodec || 'Unknown'} ·
              measured sample rate:{' '}
              {value.analysis.measuredSampleRate || 'Unknown'}
            </Typography>
            <Typography>
              {value.analysis.measuredSamples > 0
                ? `Near-full-scale samples: ${value.analysis.nearFullScaleSamples} of ${value.analysis.measuredSamples}. This is a clipping heuristic, not an audible-quality verdict.`
                : 'PCM measurements have not been collected.'}
            </Typography>
            <Typography variant="body2">
              Provenance and mastering remain unknown. A spectrum or high sample
              rate cannot establish them.
            </Typography>
          </CardContent>
        </Card>
      )}
      {value && (
        <Card>
          <CardContent>
            <Typography variant="h5">{value.title}</Typography>
            <Typography>{value.artist}</Typography>
            <Typography paragraph>
              {value.available
                ? 'Catalog available; no live source probe was performed.'
                : 'Source temporarily unavailable; metadata retained.'}
            </Typography>
            <dl className={c.facts}>
              {Object.entries(fields).map(([key, label]) => (
                <div key={key}>
                  <dt>
                    <strong>{label}</strong>
                  </dt>
                  <dd className={c.text}>
                    {value[key] === null ||
                    value[key] === undefined ||
                    value[key] === '' ||
                    value[key] === 0
                      ? 'Unknown'
                      : String(value[key])}
                  </dd>
                </div>
              ))}
            </dl>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
