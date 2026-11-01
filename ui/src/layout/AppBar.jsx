import React, { createElement, forwardRef, Fragment } from 'react'
import {
  AppBar as RAAppBar,
  MenuItemLink,
  useTranslate,
  usePermissions,
  getResources,
} from 'react-admin'
import {
  MdInfo,
  MdPerson,
  MdPhonelink,
  MdSupervisorAccount,
} from 'react-icons/md'
import { useSelector } from 'react-redux'
import { makeStyles, MenuItem, ListItemIcon, Divider } from '@material-ui/core'
import ViewListIcon from '@material-ui/icons/ViewList'
import { Dialogs } from '../dialogs/Dialogs'
import { AboutDialog, QuickConnectDialog } from '../dialogs'
import PersonalMenu from './PersonalMenu'
import ActivityPanel from './ActivityPanel'
import NowPlayingPanel from './NowPlayingPanel'
import UserMenu from './UserMenu'
import config from '../config'
import { useChildPolicy } from '../family/childPolicy'
import { useLocation } from 'react-router-dom'

const useStyles = makeStyles(
  (theme) => ({
    root: {
      color: theme.palette.text.secondary,
    },
    family: {
      // React-admin refreshes its resource lists; family screens own their
      // refresh/retry controls. Keep one effective action on these pages.
      '& #react-admin-title + button': { display: 'none' },
    },
    active: {
      color: theme.palette.text.primary,
    },
    icon: { minWidth: theme.spacing(5) },
  }),
  {
    name: 'NDAppBar',
  },
)

const DialogMenuItem = forwardRef(
  ({ onClick, label, icon, dialog, ...rest }, ref) => {
    const classes = useStyles(rest)
    const [open, setOpen] = React.useState(false)

    const handleClose = () => {
      onClick && onClick()
      setOpen(false)
    }
    return (
      <>
        <MenuItem
          ref={ref}
          onClick={() => setOpen(true)}
          className={classes.root}
        >
          <ListItemIcon className={classes.icon}>
            {createElement(icon, { title: label, size: 24 })}
          </ListItemIcon>
          {label}
        </MenuItem>
        {createElement(dialog, { onClose: handleClose, open })}
      </>
    )
  },
)

DialogMenuItem.displayName = 'DialogMenuItem'

const settingsResources = (resource) =>
  resource.name !== 'user' &&
  resource.hasList &&
  resource.options &&
  resource.options.subMenu === 'settings'

const CustomUserMenu = ({ onClick, ...rest }) => {
  const child = useChildPolicy()
  const translate = useTranslate()
  const resources = useSelector(getResources)
  const classes = useStyles(rest)
  const { permissions } = usePermissions()

  const resourceDefinition = (resourceName) =>
    resources.find((r) => r?.name === resourceName)

  const renderUserMenuItemLink = () => {
    const userResource = resourceDefinition('user')
    if (!userResource) {
      return null
    }
    if (permissions !== 'admin') {
      if (!config.enableUserEditing) {
        return null
      }
      userResource.icon = MdPerson
    } else {
      userResource.icon = MdSupervisorAccount
    }
    return renderSettingsMenuItemLink(
      userResource,
      permissions !== 'admin' ? localStorage.getItem('userId') : null,
    )
  }

  const renderSettingsMenuItemLink = (resource, id) => {
    const label = translate(`resources.${resource.name}.name`, {
      smart_count: id ? 1 : 2,
    })
    const link = id ? `/${resource.name}/${id}` : `/${resource.name}`
    return (
      <MenuItemLink
        className={classes.root}
        activeClassName={classes.active}
        key={resource.name}
        to={link}
        primaryText={label}
        leftIcon={
          (resource.icon && createElement(resource.icon, { size: 24 })) || (
            <ViewListIcon />
          )
        }
        onClick={onClick}
        sidebarIsOpen={true}
      />
    )
  }

  return (
    <>
      {config.devActivityPanel &&
        permissions === 'admin' &&
        config.enableNowPlaying && <NowPlayingPanel />}
      {config.devActivityPanel && permissions === 'admin' && <ActivityPanel />}
      <UserMenu {...rest}>
        {!child && <PersonalMenu sidebarIsOpen={true} onClick={onClick} />}
        {!child && config.enableQuickConnect && (
          <DialogMenuItem
            label={translate('menu.quickConnect.name')}
            icon={MdPhonelink}
            dialog={QuickConnectDialog}
          />
        )}
        <Divider />
        {!child && renderUserMenuItemLink()}
        {!child &&
          resources
            .filter(settingsResources)
            .map((r) => renderSettingsMenuItemLink(r))}
        <Divider />
        <DialogMenuItem
          label={translate('menu.about')}
          icon={MdInfo}
          dialog={AboutDialog}
        />
      </UserMenu>
      <Dialogs />
    </>
  )
}

const AppBar = (props) => {
  const classes = useStyles()
  const { pathname } = useLocation()
  const family =
    pathname === '/' ||
    pathname.startsWith('/family') ||
    pathname.startsWith('/external-sources')
  return (
    <RAAppBar
      {...props}
      className={family ? classes.family : undefined}
      container={Fragment}
      userMenu={<CustomUserMenu />}
    />
  )
}

export default AppBar
