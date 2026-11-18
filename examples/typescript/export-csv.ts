// Export an official board as CSV and save it to a file.
// Usage: npx tsx export-csv.ts [REGION] [BOARD] [OUT_FILE]     e.g. npx tsx export-csv.ts VN daily
// Scope: export
import { writeFile } from 'node:fs/promises';
import { TokTokData } from '@toktokdata/sdk';
import { exitWithError } from './common.js';

interface ExportJob {
  id: string;
  status: 'pending' | 'running' | 'ready' | 'failed' | string;
  rowCount: number | null;
  filename: string | null;
  error: string | null;
}

const region = process.argv[2] ?? 'VN';
const board = process.argv[3] ?? 'daily';

const client = new TokTokData();

try {
  // 201 when the file is ready at once, 202 while it is being built.
  let job = (await client.exports.create({ dataset: 'rankings', board, region, format: 'csv' })) as unknown as ExportJob;
  console.log(`export ${job.id}: ${job.status}`);

  if (job.status !== 'ready' && job.status !== 'failed') {
    job = (await client.exports.waitUntilReady(job.id)) as unknown as ExportJob;
  }
  if (job.status === 'failed') {
    console.error(`export failed: ${job.error ?? 'unknown error'}`);
    process.exit(1);
  }

  const file = (await client.exports.download(job.id)) as string | ArrayBuffer | Uint8Array;
  const out = process.argv[4] ?? job.filename ?? `toktokdata-${board}-${region}.csv`;
  await writeFile(out, typeof file === 'string' ? file : new Uint8Array(file));
  console.log(`saved ${job.rowCount ?? '?'} rows to ${out}`);
} catch (err) {
  exitWithError(err);
}
