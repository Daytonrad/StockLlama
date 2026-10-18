// Optional end-to-end checks. Start StockLlama first; then npm run test:gui.
const { chromium } = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

(async () => {
 const options = { headless: true };
 if (process.env.CHROME_PATH) options.executablePath = process.env.CHROME_PATH;
 else if (process.platform === 'win32') options.executablePath = path.join(process.env.PROGRAMFILES, 'Google', 'Chrome', 'Application', 'chrome.exe');
 const browser = await chromium.launch(options);
 let page, originalSettings;
 try {
  page = await browser.newPage({ viewport: { width: 1200, height: 900 } });
  const errors = [];
  const percentages = [];
  page.on('pageerror', e => errors.push(e.message));
  page.on('response', async r => { if(r.url().endsWith('/api/progress')) { try { percentages.push((await r.json()).percent); } catch {} } });
  await page.goto(process.env.STOCKLLAMA_URL || 'http://127.0.0.1:8765');
  originalSettings = await page.evaluate(() => fetch('/api/settings', {headers:{'X-StockLlama':document.querySelector('meta[name=session]').content}}).then(r=>r.json()));
  await page.waitForFunction(() => document.querySelector('#engineNote').textContent.includes('Stockfish ready'));
  await page.locator('#openSettings').click();
  await page.locator('#settingsDialog').waitFor({state:'visible'});
  await page.locator('#provider').selectOption('ollama');
  assert.equal(await page.locator('#useGPU').isEnabled(), true);
  await page.locator('#useGPU').uncheck();
  await page.locator('#provider').selectOption('api');
  await page.locator('#endpoint').fill('https://example.com/v1');
  await page.locator('#apiKey').fill('gui-test-secret');
  await page.locator('#settingsForm button[type=submit]').click();
  await page.waitForFunction(() => document.querySelector('#settingsStatus').textContent.startsWith('Saved'));
  const publicState = await page.evaluate(() => fetch('/api/state', {headers:{'X-StockLlama':document.querySelector('meta[name=session]').content}}).then(r => r.text()));
  assert(!publicState.includes('gui-test-secret'));
  assert(!publicState.includes(process.env.USERPROFILE));
  assert.equal(await page.evaluate(() => fetch('/api/settings').then(r=>r.status)), 403);
  await page.locator('#clearKey').check();
  await page.locator('#provider').selectOption('builtin');
  await page.locator('#settingsForm button[type=submit]').click();
  await page.waitForFunction(() => document.querySelector('#apiKey').value === '');
  await page.locator('#closeSettings').click();
  // Real offline PGN import through the file picker.
  await page.locator('#username').fill('PracticePartner');
  await page.locator('#importFile').setInputFiles('examples/demo.pgn');
  await page.locator('#importButton').click();
  await page.locator('#samplePanel').waitFor({state:'visible'});
  assert.equal(await page.locator('#limit').inputValue(), '1');
  await page.locator('#analyseButton').click();
  await page.locator('#room').waitFor({ state: 'visible', timeout: 120000 });
  assert.equal(await page.locator('#homeStatus').textContent(), 'Ready 100%');
  assert.equal(await page.locator('#board button').count(), 64);
  await page.locator('#closeCoach').click();
  assert.equal(await page.locator('#toggleCoach').getAttribute('aria-expanded'), 'false');
  await page.locator('#toggleCoach').click();
  assert.equal(await page.locator('#toggleCoach').getAttribute('aria-expanded'), 'true');
  assert(Number(await page.locator('#puzzleCount').textContent()) > 0);
  await page.locator('[data-square=e2]').click();
  await page.locator('[data-square=e4]').click();
  await page.waitForFunction(() => document.querySelector('#workStatus').textContent.includes('learned'));
  await page.locator('#drillTab').click();
  await page.locator('#reveal').click();
  await page.waitForFunction(() => document.querySelector('#workStatus').textContent.startsWith('Engine preferred'));
  await page.locator('#coachButton').click();
  await page.waitForFunction(() => document.querySelector('#chatLog').textContent.includes('PracticePartner'));
  assert((await page.locator('#chatLog').textContent()).split(/\s+/).length <= 40);
  // Save/export round-trip and responsive layout.
  const [download] = await Promise.all([page.waitForEvent('download'), page.locator('#export').click()]);
  fs.mkdirSync('test-results', { recursive: true });
  const exported = path.join('test-results', 'export.json'); await download.saveAs(exported);
  const saved = JSON.parse(fs.readFileSync(exported, 'utf8'));
  assert.equal(saved.username, 'PracticePartner');
  await page.locator('#back').click();
  await page.locator('#importFile').setInputFiles(exported);
  await page.locator('#importButton').click();
  await page.locator('#room').waitFor({ state: 'visible', timeout: 10000 });
  await page.setViewportSize({ width: 390, height: 844 });
  assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
  await page.screenshot({ path: 'test-results/mobile.png', fullPage: true });
  await page.setViewportSize({ width: 1200, height: 900 });
  await page.screenshot({ path: 'test-results/workspace.png', fullPage: true });
  if (process.env.STOCKLLAMA_LIVE_TEST === '1') {
   await page.locator('#back').click();
   await page.locator('#username').fill(process.env.CHESS_USERNAME || 'Hikaru');
   await page.locator('#learnButton').click();
   await page.locator('#samplePanel').waitFor({state:'visible',timeout:30000});
   const recommended = Number(await page.locator('#limit').inputValue());
   assert(recommended >= 1 && recommended <= 3);
   await page.locator('#limit').fill('1');
   await page.locator('#analyseButton').click();
   await page.locator('#room').waitFor({ state: 'visible', timeout: 180000 });
   assert((await page.locator('#playerTitle').textContent()).toLowerCase().includes((process.env.CHESS_USERNAME || 'Hikaru').toLowerCase()));
   await page.screenshot({ path: 'test-results/live-profile.png', fullPage: true });
  }
  assert(percentages.includes(100));
  assert(percentages.every(p=>p>=0&&p<=100));
  assert.deepEqual(errors, []);
  console.log('GUI PASS: settings, secret redaction, PGN import, board moves, drills, coaching, export/import, mobile layout' + (process.env.STOCKLLAMA_LIVE_TEST === '1' ? ', live username import' : ''));
 } finally {
  if(page&&originalSettings)await page.evaluate(s=>fetch('/api/settings',{method:'POST',headers:{'Content-Type':'application/json','X-StockLlama':document.querySelector('meta[name=session]').content},body:JSON.stringify({...s,clear_key:true})}),originalSettings).catch(()=>{});
  await browser.close();
 }
})().catch(e => { console.error(e); process.exitCode = 1; });
