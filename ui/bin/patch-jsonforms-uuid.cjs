// JSONForms 2.5.2's CommonJS build uses a UUID subpath removed in UUID 11.
// Keep the security update and call the supported named v1 export instead.
// This is a deterministic install-time compatibility patch, not a UUID fork.
const fs = require('node:fs')
const crypto = require('node:crypto')
const path = require('node:path')
const file = path.join(__dirname, '../node_modules/@jsonforms/material-renderers/lib/layouts/ExpandPanelRenderer.js')
const original = fs.readFileSync(file, 'utf8')
const old = 'var v1_1 = tslib_1.__importDefault(require("uuid/v1"));'
const replacement = 'var v1_1 = { default: require("uuid").v1 };'
const checksum = (s) => crypto.createHash('sha256').update(s).digest('hex')
const expected = '15ec6eea2c5bf2c9930c71884bff13fc40cf2835245e3ce77299f0e9442000bc'
if (original.includes(replacement) && checksum(original.replace(replacement, old)) === expected) process.exit(0)
if (checksum(original) !== expected || !original.includes(old)) throw new Error('JSONForms UUID compatibility patch: unexpected upstream source; review required')
fs.writeFileSync(file, original.replace(old, replacement))
