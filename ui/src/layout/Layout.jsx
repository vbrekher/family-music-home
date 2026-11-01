import React, { useCallback, useEffect } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import { useLocation } from 'react-router-dom'
import { Layout as RALayout, toggleSidebar } from 'react-admin'
import { makeStyles } from '@material-ui/core/styles'
import { HotKeys } from 'react-hotkeys'
import Menu from './Menu'
import AppBar from './AppBar'
import Notification from './Notification'
import { AUTO_THEME_ID } from '../consts'
import { httpClient } from '../dataProvider'
import themes from '../themes'
import { changeTheme } from '../actions'
import { defaultTheme } from '../reducers/themeReducer'
import useCurrentTheme from '../themes/useCurrentTheme'
import { useSearchRefocus } from '../common'
import { setChildPolicy } from '../family/childPolicy'

const useStyles = makeStyles({
  root: {
    paddingBottom: (props) => (props.addPadding ? '80px' : 0),
    '&.layout': {
      minWidth: (props) => (props.familyLayout ? 0 : 'fit-content'),
    },
  },
  content: { minWidth: (props) => (props.familyLayout ? 0 : 'auto') },
})

const Layout = (props) => {
  const theme = useCurrentTheme()
  const queue = useSelector((state) => state.player?.queue)
  const { pathname } = useLocation()
  const familyLayout =
    pathname === '/' ||
    pathname.startsWith('/family') ||
    pathname === '/external-sources'
  const classes = useStyles({ addPadding: queue.length > 0, familyLayout })
  const dispatch = useDispatch()
  useSearchRefocus()
  useEffect(() => {
    let active = true
    httpClient('/api/family/me')
      .then(({ json }) => {
        if (active) setChildPolicy(json.child || null)
      })
      .catch(() => {})
    httpClient('/api/personal/theme')
      .then(({ json }) => {
        if (active)
          dispatch(
            changeTheme(
              themes[json.theme] || json.theme === AUTO_THEME_ID
                ? json.theme
                : defaultTheme(),
            ),
          )
      })
      .catch(() => {})
    return () => {
      active = false
    }
  }, [dispatch])

  const keyHandlers = {
    TOGGLE_MENU: useCallback(() => dispatch(toggleSidebar()), [dispatch]),
  }

  return (
    <HotKeys handlers={keyHandlers}>
      <RALayout
        {...props}
        className={classes.root}
        classes={{ content: classes.content }}
        menu={Menu}
        appBar={AppBar}
        theme={theme}
        notification={Notification}
      />
    </HotKeys>
  )
}

export default Layout
