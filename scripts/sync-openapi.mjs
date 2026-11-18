#!/usr/bin/env node
// Refreshes openapi/openapi.json from the full schema exported by the toktokdata.live backend
// (`python manage.py spectacular --format openapi-json --file openapi-full.json`).
// Keeps only operations marked `x-public: true`, the `ApiKey` security scheme and the
// `Error` / `Provenance` schemas, and sets the production server.
//
//   node scripts/sync-openapi.mjs path/to/openapi-full.json
import fs from 'node:fs'
import path from 'node:path'

const METHODS = ['get', 'post', 'put', 'patch', 'delete']
const source = process.argv[2]
if (!source) {
  console.error('usage: node scripts/sync-openapi.mjs <openapi-full.json>')
  process.exit(2)
}
const doc = JSON.parse(fs.readFileSync(source, 'utf8'))
const paths = {}
let count = 0
for (const [p, item] of Object.entries(doc.paths ?? {})) {
  for (const m of METHODS) {
    const op = item[m]
    if (op?.['x-public'] !== true) continue
    if (op.security) op.security = [{ ApiKey: [] }]
    paths[p] ??= {}
    paths[p][m] = op
    count += 1
  }
}
if (count === 0) {
  console.error('no operation is marked x-public: is this the toktokdata.live backend schema?')
  process.exit(1)
}
const schemas = doc.components?.schemas ?? {}
const out = {
  openapi: doc.openapi,
  info: { ...doc.info, title: 'toktokdata.live API' },
  servers: [{ url: 'https://api.toktokdata.live', description: 'Production' }],
  security: [{ ApiKey: [] }],
  tags: doc.tags ?? [],
  paths,
  components: {
    securitySchemes: { ApiKey: doc.components.securitySchemes.ApiKey },
    schemas: { Error: schemas.Error, ...(schemas.Provenance ? { Provenance: schemas.Provenance } : {}) },
  },
}
const target = path.resolve(import.meta.dirname, '../openapi/openapi.json')
fs.writeFileSync(target, `${JSON.stringify(out, null, 2)}\n`)
console.log(`wrote ${count} operations to openapi/openapi.json`)
