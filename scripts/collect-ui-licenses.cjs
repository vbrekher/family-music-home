#!/usr/bin/env node
// Collect declared runtime npm licenses and their installed notice texts.
// Run inside the local, lockfile-built UI Docker image. No network requests.
const fs = require('node:fs')
const path = require('node:path')
const crypto = require('node:crypto')
const [rootArg, jsonOutput, textOutput, fallbackRoot] = process.argv.slice(2)
if (!rootArg || !jsonOutput || !textOutput) throw new Error('Usage: node collect-ui-licenses.cjs UI_ROOT INVENTORY_JSON NOTICES_TXT')
const root = fs.realpathSync(rootArg)
const lock = JSON.parse(fs.readFileSync(path.join(root, 'package-lock.json'), 'utf8'))
const packages = [], notices = [], missing = []
const sourceLicenses = fallbackRoot ? JSON.parse(fs.readFileSync(path.join(fallbackRoot, 'sources.lock.json'), 'utf8')) : {}
for (const [relative, item] of Object.entries(lock.packages)) {
  if (!relative.startsWith('node_modules/') || item.dev) continue
  const folder = fs.realpathSync(path.join(root, relative))
  if (!folder.startsWith(root + path.sep)) throw new Error('Package escapes UI root')
  const pkg = JSON.parse(fs.readFileSync(path.join(folder, 'package.json'), 'utf8'))
  const licenseFiles = fs.readdirSync(folder).filter(name => /^(licen[cs]e|copying|notice)([._-].*)?$/i.test(name) && fs.statSync(path.join(folder, name)).isFile())
  let license = pkg.license || item.license || 'UNKNOWN_REVIEW_REQUIRED'
  const texts = licenseFiles.map(name => fs.readFileSync(path.join(folder, name), 'utf8'))
  const sourceLicense = sourceLicenses[`${pkg.name}@${pkg.version}`]
  if (!texts.length && sourceLicense) {
    if (path.basename(sourceLicense.file) !== sourceLicense.file) throw new Error('Invalid source-license filename')
    const text = fs.readFileSync(path.join(fallbackRoot, sourceLicense.file), 'utf8')
    if (crypto.createHash('sha256').update(text).digest('hex') !== sourceLicense.sha256) throw new Error('Source-license checksum mismatch')
    texts.push(text)
    licenseFiles.push(sourceLicense.source)
  }
  if (pkg.name === 'css-mediaquery' && license === 'BSD' && texts.some(text => text.includes('Neither the name of the Yahoo! Inc.'))) license = 'BSD-3-Clause'
  packages.push({ name: pkg.name, version: pkg.version, license, integrity: item.integrity || null, licenseFiles, licenseTextSHA256: texts.map(text => crypto.createHash('sha256').update(text).digest('hex')) })
  notices.push(`${pkg.name}@${pkg.version}\nDeclared license: ${license}\n${texts.join('\n\n') || 'No standalone license text located; review required.'}\n`)
  if (!texts.length || license === 'UNKNOWN_REVIEW_REQUIRED') missing.push(`${pkg.name}@${pkg.version}`)
}
fs.writeFileSync(jsonOutput, JSON.stringify({ scope: 'Installed runtime packages from the reviewed npm lock; original component licenses apply', packages, missingLicenseReview: missing }, null, 2) + '\n')
fs.writeFileSync(textOutput, 'Family Music Home — retained runtime npm license notices\n\n' + notices.join('\n' + '='.repeat(72) + '\n\n'))
console.log(JSON.stringify({ packages: packages.length, missingLicenseReview: missing }))
