import React from 'react'
import { Route } from 'react-router-dom'
import FamilyHome from './family/FamilyHome'
import Shelves from './family/Shelves'
import Passport from './family/Passport'
import Journal, { Letters } from './family/Journal'
import Requests from './family/Requests'
import Versions from './family/Versions'
import Doctor from './family/Doctor'
import Kids from './family/Kids'
import Children from './family/Children'
import Legacy from './family/Legacy'
import Personal from './personal/Personal'
import ExternalSources from './library/ExternalSources'

const routes = [
  <Route exact path="/family/legacy" component={Legacy} key="family-legacy" />,
  <Route exact path="/family/kids" component={Kids} key="family-kids" />,
  <Route
    exact
    path="/family/children"
    component={Children}
    key="family-children"
  />,
  ...[
    ['capsules', Journal],
    ['letters', Letters],
    ['requests', Requests],
    ['versions', Versions],
    ['doctor', Doctor],
  ].flatMap(([name, component]) => [
    <Route
      exact
      path={`/family/${name}`}
      component={component}
      key={`family-${name}`}
    />,
    <Route
      exact
      path={`/family/${name}/:id`}
      component={component}
      key={`family-${name}-detail`}
    />,
  ]),
  <Route exact path="/family" component={FamilyHome} key="family" />,
  <Route
    exact
    path="/family/shelves"
    component={Shelves}
    key="family-shelves"
  />,
  <Route
    exact
    path="/family/shelves/:id"
    component={Shelves}
    key="family-shelf"
  />,
  <Route
    exact
    path="/family/passport"
    component={Passport}
    key="family-passport"
  />,
  <Route
    exact
    path="/external-sources"
    component={ExternalSources}
    key="external-sources"
  />,
  <Route exact path="/personal" render={() => <Personal />} key={'personal'} />,
]

export default routes
