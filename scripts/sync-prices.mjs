#!/usr/bin/env node
/**
 * Sync model prices into odotoken prices.json.
 *
 * Sources (later entries win for the same model id):
 *   1. pricetoken npm package — official models (bundled static pricing)
 *   2. Pegasus pricing API (default http://154.29.76.173, override PEGASUS_PRICING_API)
 *      NOTE: the API reports per-TOKEN amounts even though the unit field says
 *      "per_1m_tokens" — this script multiplies by 1M to normalize.
 *   3. crush providers.json (~/.local/share/crush/providers.json) — exact prices
 *      your local routers charge, highest priority.
 *
 * Entries with "override": true in prices.json are never touched.
 *
 * Usage: node scripts/sync-prices.mjs [--output prices.json]
 */
import { STATIC_PRICING } from 'pricetoken';
import { readFileSync, writeFileSync } from 'fs';
import { homedir } from 'os';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const PRICING_API = process.env.PEGASUS_PRICING_API ?? 'http://154.29.76.173';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');
const outputPath = process.argv.includes('--output')
  ? process.argv[process.argv.indexOf('--output') + 1]
  : join(root, 'prices.json');

// input/output/cacheRead are per-1M-token USD rates (values are stored as-is).
const setPrice = (prices, id, input, output, cacheRead = 0) => {
  prices[id] = {
    input: Math.round(input * 1e6) / 1e6,
    output: Math.round(output * 1e6) / 1e6,
    cache_read: Math.round(cacheRead * 1e6) / 1e6,
    cache_write: prices[id]?.cache_write ?? 0,
  };
};

const parsePrice = (model) => {
  // The pricing API mixes conventions: most providers (openrouter etc.)
  // report per-TOKEN amounts under a "per_1m_tokens" label, while others
  // (snowflake, databricks…) already report per-1M. Disambiguate by
  // magnitude: paid per-token prices are < $0.001; per-1M prices for real
  // models are >= $0.01. Zero stays zero (free model).
  const pin = model.prices?.find((p) => p.unit === 'per_1m_input_tokens')?.amount;
  const pout = model.prices?.find((p) => p.unit === 'per_1m_output_tokens')?.amount;
  if (pin === undefined || pout === undefined) return null;
  let input = parseFloat(pin);
  let output = parseFloat(pout);
  if (!Number.isFinite(input) || !Number.isFinite(output)) return null;
  const isPerToken = (input > 0 && input < 0.001) || (output > 0 && output < 0.001);
  if (isPerToken) {
    input *= 1_000_000;
    output *= 1_000_000;
  }
  return { input, output };
};

const bareId = (externalId) => {
  const id = externalId.split('/').pop();
  return id.replace(/:0$/, '');
};

async function main() {
  let prices = {};
  try {
    prices = JSON.parse(readFileSync(outputPath, 'utf8'));
  } catch {
    // start fresh
  }
  const overridden = new Set(
    Object.entries(prices)
      .filter(([id, r]) => id !== 'default' && r.override === true)
      .map(([id]) => id)
  );
  const isFree = (id) => {
    const r = prices[id];
    return r && r.input === 0 && r.output === 0;
  };

  let stats = { pricetoken: 0, api: 0, crush: 0, kept: overridden.size };

  // 1. pricetoken official static pricing (per-1M rates)
  for (const m of Object.values(STATIC_PRICING)) {
    if (overridden.has(m.modelId)) continue;
    setPrice(prices, m.modelId, m.inputPerMTok, m.outputPerMTok, prices[m.modelId]?.cache_read ?? 0);
    stats.pricetoken++;
  }

  // 2. Pegasus pricing API (retries once on rate limit)
  const fetchWithRetry = async (url, tries = 3) => {
    for (let i = 0; i < tries; i++) {
      const res = await fetch(url);
      if (res.status === 429 && i < tries - 1) {
        const wait = parseInt(res.headers.get('retry-after') ?? '65', 10);
        console.error(`rate limited, waiting ${wait}s…`);
        await new Promise((r) => setTimeout(r, wait * 1000));
        continue;
      }
      return res;
    }
  };
  try {
    const res = await fetchWithRetry(`${PRICING_API}/v1/models`);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const models = await res.json();
    // Deduplicate by bare id keeping the cheapest input price
    const byBare = new Map();
    for (const m of models) {
      const p = parsePrice(m);
      if (!p) continue;
      const id = bareId(m.external_id);
      const prev = byBare.get(id);
      if (!prev || p.input < prev.input || (p.input === prev.input && p.output < prev.output)) {
        byBare.set(id, p);
      }
    }
    for (const [id, p] of byBare) {
      if (overridden.has(id)) continue;
      // Don't downgrade a known non-free price to a free one
      if (isFree(id) || !(id in prices)) {
        setPrice(prices, id, p.input, p.output, prices[id]?.cache_read ?? 0);
        stats.api++;
      }
    }
  } catch (e) {
    console.error(`warning: pricing API unreachable (${e.message}), continuing without it`);
  }

  // 3. crush providers.json — exact router prices (highest priority)
  try {
    const crush = JSON.parse(
      readFileSync(join(homedir(), '.local/share/crush/providers.json'), 'utf8')
    );
    for (const prov of crush) {
      for (const m of prov.models || []) {
        const cin = m.cost_per_1m_in ?? 0;
        const cout = m.cost_per_1m_out ?? 0;
        if (cin === 0 && cout === 0) continue;
        if (overridden.has(m.id)) continue;
        setPrice(prices, m.id, cin, cout, m.cost_per_1m_in_cached ?? 0);
        stats.crush++;
      }
    }
  } catch {
    console.log('note: crush providers.json not found, skipped');
  }

  // 4. Pegasus Relay wrapper models (pegasus-claude-*) wrap official Claude
  //    models — priced at the underlying Claude rates until the relay
  //    publishes its own pricing. Override manually in prices.json if needed.
  const pegasusWrappers = {
    'pegasus-claude-speed':    [3.0, 15.0, 3.75, 18.75],  // sonnet-4-6
    'pegasus-claude-ultra':    [5.0, 25.0, 6.25, 31.25],  // opus-4-6
    'pegasus-claude-thinking': [5.0, 25.0, 6.25, 31.25],  // opus-4-6
    'pegasus-haiku-flash':     [1.0, 5.0, 1.25, 6.25],   // haiku-4-5
  };
  for (const [id, [cin, cout, cr, cw]] of Object.entries(pegasusWrappers)) {
    if (overridden.has(id)) continue;
    prices[id] = { input: cin, output: cout, cache_read: cr, cache_write: cw };
  }

  prices['default'] = prices['default'] ?? { input: 0, output: 0, cache_read: 0, cache_write: 0 };

  writeFileSync(outputPath, JSON.stringify(prices, null, 2));
  console.log(
    `prices.json updated: ${Object.keys(prices).length} models ` +
    `(pricetoken: ${stats.pricetoken}, pricing-api: ${stats.api}, crush: ${stats.crush}, overrides: ${stats.kept})`
  );
}

main();