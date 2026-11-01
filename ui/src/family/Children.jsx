import React, { useEffect, useState } from 'react'
import { Title } from 'react-admin'
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
} from '@material-ui/core'
import { httpClient } from '../dataProvider'
import { call, useFamilyStyles } from './common'
import FamilyNav from './FamilyNav'
const fresh = () => ({
  userName: '',
  name: '',
  password: '',
  libraryIds: [],
  policy: {
    interface: 'kids',
    density: 'large',
    storyLibraries: [],
    allowFamilyMessages: false,
    enabled: true,
  },
  confirmed: false,
})
export default function Children() {
  const c = useFamilyStyles(),
    [me, setMe] = useState(null),
    [profiles, setProfiles] = useState([]),
    [libraries, setLibraries] = useState([]),
    [edit, setEdit] = useState(null),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(false)
  const load = () =>
    call('/children')
      .then(setProfiles)
      .catch(() =>
        setError(
          'Family profiles could not be loaded. Existing accounts are unchanged.',
        ),
      )
  useEffect(() => {
    call('/me')
      .then(setMe)
      .catch(() => setError('Sign in again.'))
  }, [])
  useEffect(() => {
    if (!me?.admin) return
    load()
    httpClient('/api/library')
      .then(({ json }) => setLibraries(json))
      .catch(() => setError('Libraries could not be loaded.'))
  }, [me?.admin])
  const policy = (key, value) =>
    setEdit({ ...edit, policy: { ...edit.policy, [key]: value } })
  const save = async () => {
    setBusy(true)
    setError('')
    try {
      const { userId, ...body } = edit
      await call(
        '/children' + (userId ? '/' + userId : ''),
        body,
        userId ? 'PUT' : 'POST',
      )
      setEdit(null)
      await load()
    } catch (_) {
      setError(
        'Profile could not be saved. Check the fields, reload after a conflict, and try again.',
      )
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className={c.page}>
      <Title title="Family child accounts" />
      <FamilyNav />
      <Typography variant="h4" gutterBottom>
        Family child accounts
      </Typography>
      <Typography paragraph>
        Create a protected account when you choose. Existing adult accounts are
        preserved. Library permissions stay in Navidrome; switching the
        interface keeps the same account, favorites and history.
      </Typography>
      <Typography role="status">{error}</Typography>
      {me && !me.admin && (
        <Typography>
          Only the family Owner can manage child accounts.
        </Typography>
      )}
      {me?.admin && (
        <>
          <Button
            color="primary"
            variant="contained"
            onClick={() => setEdit(fresh())}
          >
            Create child account
          </Button>
          {!profiles.length && (
            <Typography>
              No child accounts yet. None are created automatically.
            </Typography>
          )}
          {profiles.map((p) => (
            <Card key={p.userId}>
              <CardContent>
                <Typography variant="h6">{p.name}</Typography>
                <Typography>
                  {p.userName} · {p.policy.interface} interface ·{' '}
                  {p.libraryIds.length} allowed libraries ·{' '}
                  {p.policy.enabled ? 'enabled' : 'paused'}
                </Typography>
                <Button
                  onClick={() =>
                    setEdit({ ...p, password: '', confirmed: false })
                  }
                >
                  Manage {p.name}
                </Button>
              </CardContent>
            </Card>
          ))}
        </>
      )}
      <Dialog
        className={c.dialog}
        open={!!edit}
        onClose={() => !busy && setEdit(null)}
        fullWidth
        maxWidth="sm"
      >
        <DialogTitle>
          {edit?.userId
            ? 'Manage protected account'
            : 'Create a protected account'}
        </DialogTitle>
        <DialogContent>
          {edit && (
            <>
              <Typography paragraph>
                Choose only approved libraries. External metadata requests,
                external scrobbling, public playlists and storage administration
                remain unavailable to this account. This is family access
                control, not an age certification.
              </Typography>
              <TextField
                fullWidth
                label="Account username"
                id="family-child-username"
                value={edit.userName}
                disabled={!!edit.userId}
                onChange={(e) => setEdit({ ...edit, userName: e.target.value })}
              />
              <TextField
                fullWidth
                label="Display name"
                id="family-child-name"
                value={edit.name}
                onChange={(e) => setEdit({ ...edit, name: e.target.value })}
              />
              {!edit.userId && (
                <TextField
                  fullWidth
                  type="password"
                  autoComplete="new-password"
                  label="Account password (at least 12 characters)"
                  id="family-child-password"
                  value={edit.password}
                  onChange={(e) =>
                    setEdit({ ...edit, password: e.target.value })
                  }
                />
              )}
              <TextField
                fullWidth
                select
                SelectProps={{ multiple: true }}
                label="Allowed libraries"
                id="family-child-libraries"
                value={edit.libraryIds}
                onChange={(e) =>
                  setEdit({
                    ...edit,
                    libraryIds: e.target.value,
                    policy: {
                      ...edit.policy,
                      storyLibraries: edit.policy.storyLibraries.filter((id) =>
                        e.target.value.includes(id),
                      ),
                    },
                  })
                }
              >
                {libraries.map((l) => (
                  <MenuItem key={l.id} value={l.id}>
                    {l.name}
                  </MenuItem>
                ))}
              </TextField>
              <TextField
                fullWidth
                select
                SelectProps={{ multiple: true }}
                label="Libraries shown as Stories"
                id="family-child-stories"
                value={edit.policy.storyLibraries}
                onChange={(e) => policy('storyLibraries', e.target.value)}
              >
                {libraries
                  .filter((l) => edit.libraryIds.includes(l.id))
                  .map((l) => (
                    <MenuItem key={l.id} value={l.id}>
                      {l.name}
                    </MenuItem>
                  ))}
              </TextField>
              <TextField
                fullWidth
                select
                label="Interface"
                id="family-child-interface"
                value={edit.policy.interface}
                onChange={(e) => policy('interface', e.target.value)}
              >
                <MenuItem value="kids">
                  Large covers, songs, stories and collections
                </MenuItem>
                <MenuItem value="full">
                  Full interface, same account and parental restrictions
                </MenuItem>
              </TextField>
              <TextField
                fullWidth
                select
                label="Reading and touch size"
                id="family-child-density"
                value={edit.policy.density}
                onChange={(e) => policy('density', e.target.value)}
              >
                <MenuItem value="large">Large</MenuItem>
                <MenuItem value="comfortable">Comfortable</MenuItem>
              </TextField>
              <FormControlLabel
                control={
                  <Checkbox
                    checked={edit.policy.enabled}
                    onChange={(e) => policy('enabled', e.target.checked)}
                  />
                }
                label="Account access enabled"
              />
              <FormControlLabel
                control={
                  <Checkbox
                    checked={edit.policy.allowFamilyMessages}
                    onChange={(e) =>
                      policy('allowFamilyMessages', e.target.checked)
                    }
                  />
                }
                label="Allow explicit family sharing from other members (parent only by default)"
              />
              <FormControlLabel
                control={
                  <Checkbox
                    checked={edit.confirmed}
                    onChange={(e) =>
                      setEdit({ ...edit, confirmed: e.target.checked })
                    }
                  />
                }
                label={
                  edit.userId
                    ? 'I confirm these access changes'
                    : 'I choose to create this child account now'
                }
              />
            </>
          )}
        </DialogContent>
        <DialogActions>
          <Button disabled={busy} onClick={() => setEdit(null)}>
            Cancel
          </Button>
          <Button
            disabled={
              busy ||
              !edit?.confirmed ||
              !edit?.userName ||
              !edit?.name ||
              (!edit?.userId && edit?.password.length < 12)
            }
            onClick={save}
          >
            Save protected account
          </Button>
        </DialogActions>
      </Dialog>
    </div>
  )
}
