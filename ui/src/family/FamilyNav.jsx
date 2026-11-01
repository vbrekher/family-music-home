import React, { useState } from 'react'
import {
  Button,
  TextField,
  Menu,
  MenuItem,
  makeStyles,
} from '@material-ui/core'
import { useLocation } from 'react-router-dom'
import { useChildPolicy } from './childPolicy'
const useNavStyles = makeStyles((theme) => ({
  nav: {
    display: 'flex',
    minWidth: 0,
    maxWidth: '100%',
    gap: theme.spacing(1),
    marginBottom: theme.spacing(2),
    flexWrap: 'wrap',
    padding: theme.spacing(1),
    borderRadius: 16,
    background: theme.palette.background.paper,
    border: `1px solid ${theme.palette.divider}`,
    '& .MuiButton-root': {
      minHeight: 44,
      borderRadius: 10,
      textTransform: 'none',
      padding: theme.spacing(1, 1.5),
      background: 'transparent',
      boxShadow: 'none',
      color: theme.palette.text.secondary,
    },
    '& .MuiButton-root[aria-current="page"]': {
      background: theme.palette.action.selected,
      color: theme.palette.text.primary,
    },
    [theme.breakpoints.down('sm')]: { display: 'none' },
  },
  mobile: {
    display: 'none',
    [theme.breakpoints.down('sm')]: {
      display: 'block',
      marginBottom: theme.spacing(2),
    },
  },
}))
export default function FamilyNav() {
  const c = useNavStyles(),
    child = useChildPolicy(),
    location = useLocation(),
    [anchor, setAnchor] = useState(null)
  const routes =
    child && !child.enabled
      ? [['kids', 'Your music']]
      : child?.interface === 'kids'
        ? [
            ['kids', 'Your music'],
            ['requests', 'Ask for music'],
            ['legacy', 'Save your history'],
          ]
        : [
            ['', 'Home'],
            ['shelves', 'Shelves'],
            ['requests', 'Music requests'],
            ['letters', 'Music letters'],
            ['capsules', 'Memories'],
            ['versions', 'Editions'],
            ...(localStorage.getItem('role') === 'admin'
              ? [['doctor', 'Library Doctor']]
              : []),
            ['passport', 'Quality passport'],
            ['legacy', 'Legacy archive'],
            ...(localStorage.getItem('role') === 'admin'
              ? [['children', 'Child accounts']]
              : child
                ? [['kids', 'Your music']]
                : []),
          ]
  const current =
    routes.find(
      ([path]) =>
        location.pathname === `/family${path ? '/' + path : ''}` ||
        (path && location.pathname.startsWith(`/family/${path}/`)),
    )?.[0] || ''
  return (
    <>
      <TextField
        className={c.mobile}
        fullWidth
        select
        variant="outlined"
        label="Family music navigation"
        InputLabelProps={{ shrink: true }}
        value={current}
        SelectProps={{ native: true }}
        onChange={(e) => {
          window.location.hash = `/family${e.target.value ? '/' + e.target.value : ''}`
        }}
      >
        {routes.map(([path, label]) => (
          <option key={path} value={path}>
            {label}
          </option>
        ))}
      </TextField>
      <nav className={c.nav} aria-label="Family music navigation">
        {routes.slice(0, 5).map(([path, label]) => (
          <Button
            key={path}
            aria-current={current === path ? 'page' : undefined}
            href={`#/family${path ? '/' + path : ''}`}
          >
            {label}
          </Button>
        ))}
        {routes.length > 5 && (
          <>
            <Button
              aria-current={
                routes.slice(5).some(([path]) => path === current)
                  ? 'page'
                  : undefined
              }
              aria-haspopup="menu"
              aria-expanded={Boolean(anchor)}
              onClick={(event) => setAnchor(event.currentTarget)}
            >
              {routes.slice(5).find(([path]) => path === current)?.[1] ||
                'More'}
            </Button>
            <Menu
              anchorEl={anchor}
              open={Boolean(anchor)}
              onClose={() => setAnchor(null)}
              PaperProps={{ style: { maxWidth: 'calc(100vw - 32px)' } }}
            >
              {routes.slice(5).map(([path, label]) => (
                <MenuItem
                  key={path}
                  component="a"
                  href={`#/family/${path}`}
                  selected={path === current}
                  onClick={() => setAnchor(null)}
                  style={{ minHeight: 44, whiteSpace: 'normal' }}
                >
                  {label}
                </MenuItem>
              ))}
            </Menu>
          </>
        )}
      </nav>
    </>
  )
}
