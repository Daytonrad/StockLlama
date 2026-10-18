const $ = id => document.getElementById(id);
const token = document.querySelector('meta[name=session]').content;
const clean = text => String(text).replace(/[.·:;→←↓…]/g, ' ').replace(/ +/g, ' ').trim();
let state, profile, boardData, mode = 'play', drillIndex = 0, selected = '', busy = false, sampleUser = '', hintVersion = 0, toastTimer;
const pieces = { K:'♚', Q:'♛', R:'♜', B:'♝', N:'♞', P:'♟', k:'♚', q:'♛', r:'♜', b:'♝', n:'♞', p:'♟' };
async function api(path, data) {
 const r = await fetch('/api/' + path, { method:data === undefined ? 'GET' : 'POST', headers:{'Content-Type':'application/json','X-StockLlama':token}, body:data === undefined ? undefined : JSON.stringify(data) });
 const text = await r.text(); let value;
 try { value = JSON.parse(text); } catch { throw Error('Service unavailable'); }
 if (!r.ok) throw Error(clean(value.error || 'Request failed'));
 return value;
}
function toast(text, error = false) { clearTimeout(toastTimer); $('toast').textContent = clean(text); $('toast').classList.toggle('error',error); $('toast').hidden = false; toastTimer = setTimeout(() => $('toast').hidden = true, 6000); }
async function action(fn) {
 if (busy) return; busy = true; document.body.setAttribute('aria-busy','true');
 for (const id of ['learnButton','importButton','analyseButton','moveButton','coachButton']) $(id).disabled = true;
 try { await fn(); } catch(e) { toast(e.message,true); }
 finally { busy = false; document.body.removeAttribute('aria-busy'); for (const id of ['learnButton','importButton','analyseButton','moveButton','coachButton']) $(id).disabled = false; }
}
async function refresh() {
 state = await api('state');
 $('engineNote').textContent = state.engine_installed ? 'Stockfish ready' : 'Open Settings to install Stockfish';
 $('savedArea').hidden = !state.profiles.length; const old = $('saved').value;
 $('saved').replaceChildren(...state.profiles.map(name => { const o = document.createElement('option'); o.value = name; o.textContent = name; return o; }));
 if (state.profiles.includes(old)) $('saved').value = old;
 coachLabel(); return state;
}
function coachLabel() {
 const p = state.settings.provider; $('providerTag').textContent = p === 'builtin' ? 'Stockfish' : p === 'ollama' ? 'Ollama' : 'API coach';
 $('cloudNote').textContent = p === 'api' ? 'Ask sends the position summary to your API provider' : p === 'ollama' ? 'Ask uses your local model Hints use Stockfish' : 'Move recommendations work offline without a language model';
}
async function fields() {
 const s = await api('settings'); for (const id of ['provider','endpoint','model','folder','engine']) $(id).value = s[id] || '';
 $('useGPU').checked = !!s.use_gpu; $('apiKey').value = ''; $('apiKey').placeholder = state.has_key ? 'Key loaded for this session' : 'Paste your API key'; $('clearKey').checked = false; showProvider(); hardware();
}
function showProvider() {
 const p = $('provider').value;
 document.querySelectorAll('.model-field').forEach(e => e.hidden = p === 'builtin');
 document.querySelectorAll('.ollama-field').forEach(e => e.hidden = p !== 'ollama');
 document.querySelectorAll('.api-field').forEach(e => e.hidden = p !== 'api');
 $('useGPU').disabled = p !== 'ollama';
}
async function hardware() {
 try { const g = await api('hardware'); $('gpuStatus').textContent = g.active ? '✓ GPU active for the selected model' : g.detected ? '✓ GPU detected ' + clean(g.name) : 'GPU not confirmed Ollama can still try automatic detection'; }
 catch { $('gpuStatus').textContent = 'GPU status unavailable'; }
}
function settingsPayload() {
 const s = {}; for (const id of ['provider','endpoint','model','folder','engine']) s[id] = $(id).value.trim();
 s.api_key = $('apiKey').value; s.clear_key = $('clearKey').checked; s.use_gpu = $('useGPU').checked; return s;
}
async function saveSettings() {
 await api('settings',settingsPayload()); $('apiKey').value = ''; $('clearKey').checked = false; await refresh(); $('settingsStatus').textContent = 'Saved';
}
async function detectModels() {
 await saveSettings(); $('modelStatus').textContent = 'Looking for models'; const r = await api('models',{});
 $('models').replaceChildren(...r.models.map(name => { const o = document.createElement('option'); o.value = name; return o; }));
 if (!$('model').value && r.models.length) $('model').value = r.models[0]; $('modelStatus').textContent = clean(r.message); hardware();
}
function showProgress(j) {
 const percent = j.percent || 0, text = clean(j.message) + ' ' + percent + '%';
 $('homeStatus').textContent = text; $('settingsStatus').textContent = text; $('progress').hidden = !j.busy; $('progress').value = percent;
}
async function waitForJob(open = true) {
 for (;;) { const j = await api('progress'); showProgress(j); if (j.error) throw Error(j.error);
  if (!j.busy) { await refresh(); if (open && state.profile) openRoom(); return; }
  await new Promise(r => setTimeout(r,250));
 }
}
async function preview(file) {
 const username = $('username').value.trim(), input = {username};
 $('homeStatus').textContent = 'Fetching recent games 0%'; $('progress').hidden = false; $('progress').value = 0;
 if (file) { if (file.size > 12 * 1024 * 1024) throw Error('Choose a file smaller than 12 MB'); input.filename = file.name; input.content = await file.text(); }
 const r = await api('preview',input); $('progress').hidden = true;
 if (r.profile) { await api('learn',input); await waitForJob(); return; }
 sampleUser = username; $('limit').max = r.recommended; $('limit').value = r.recommended;
 $('sampleText').textContent = `${r.available} games in this sample We recommend ${r.recommended} for a quick analysis`;
 $('samplePanel').hidden = false; $('homeStatus').textContent = r.cached ? 'Using saved games No download needed' : 'Games ready';
}
function openRoom() {
 profile = state.profile; mode = 'play'; drillIndex = 0; $('home').hidden = true; $('room').hidden = false;
 $('playerTitle').textContent = 'Prepare for ' + profile.username; $('gameCount').textContent = profile.games; $('moveCount').textContent = profile.analysed_moves; $('puzzleCount').textContent = profile.puzzle_count;
 $('openings').replaceChildren(...Object.entries(profile.openings).sort((a,b) => b[1]-a[1]).slice(0,3).map(([name,n]) => { const p = document.createElement('p'); p.textContent = clean(name) + ' in ' + n + ' games'; return p; }));
 $('toggleCoach').hidden=false;setCoachOpen(true);$('chatLog').replaceChildren(); $('drillTab').disabled = !profile.puzzle_count; boardData = state.board; setModeUI(); drawBoard(); window.scrollTo(0,0);
}
function setModeUI() { $('playTab').classList.toggle('active',mode === 'play'); $('drillTab').classList.toggle('active',mode === 'drill'); $('playControls').hidden = mode !== 'play'; $('drillControls').hidden = mode !== 'drill'; $('moveButton').textContent = mode === 'play' ? 'Play' : 'Check'; $('color').value = boardData.human || 'white'; selected = ''; }
function drawBoard() {
 selected = ''; renderSquares(); $('workStatus').textContent = clean(boardData.message || 'Your move');
 if (boardData.outcome && boardData.outcome !== '*') $('workStatus').textContent = 'Game finished ' + boardData.outcome;
 if (mode === 'drill') $('drillCount').textContent = `${drillIndex+1} / ${profile.puzzle_count}`;
 updateHint();
}
async function updateHint() {
 if($('coachPanel').classList.contains('closed'))return;
 const revision = ++hintVersion; $('coachText').textContent = 'Finding a reply';
 try { const r = await api('hint',{mode,index:drillIndex}); if (revision === hintVersion) $('coachText').textContent = r.text; }
 catch(e) { if (revision === hintVersion) $('coachText').textContent = clean(e.message); }
}
function renderSquares() {
 const cells = {}; boardData.fen.split(' ')[0].split('/').forEach((row,y) => { let x=0; for (const ch of row) { if (/[1-8]/.test(ch)) x += Number(ch); else { cells[String.fromCharCode(97+x)+(8-y)] = ch; x++; } } });
 const flip = mode === 'play' && boardData.human === 'black', buttons = [];
 for (let y=0;y<8;y++) for (let x=0;x<8;x++) {
  const file=flip?7-x:x, rank=flip?y+1:8-y, sq=String.fromCharCode(97+file)+rank, ch=cells[sq], b=document.createElement('button');
  b.type='button'; b.className='square '+((file+rank)%2?'light':'dark')+(ch?(ch===ch.toUpperCase()?' white':' black'):''); b.dataset.square=sq;
  b.setAttribute('aria-label',sq+(ch?' '+(ch===ch.toUpperCase()?'white ':'black ')+({k:'king',q:'queen',r:'rook',b:'bishop',n:'knight',p:'pawn'}[ch.toLowerCase()]):' empty'));
  if(ch){const span=document.createElement('span');span.textContent=pieces[ch];b.append(span);}
  if(x===0||y===7){const c=document.createElement('small');c.className='coord';c.textContent=y===7?sq[0]:sq[1];b.append(c);}
  b.onclick=()=>clickSquare(sq); if(sq===selected)b.classList.add('selected'); if(selected&&(boardData.legal||[]).some(m=>m.startsWith(selected+sq)))b.classList.add('target'); buttons.push(b);
 }
 $('board').replaceChildren(...buttons);
}
function clickSquare(sq) {
 if(busy)return; const legal=boardData.legal||[];
 if(selected){const candidates=legal.filter(m=>m.startsWith(selected+sq));if(candidates.length){const m=candidates.find(m=>m.length===4||m[4]===$('promotion').value)||candidates[0];action(()=>sendMove(m));return;}}
 selected=legal.some(m=>m.startsWith(sq))?sq:'';renderSquares();
}
async function sendMove(move) {
 boardData = mode === 'play' ? await api('play',{move}) : await api('drill',{index:drillIndex,move});
 if(boardData.solution)boardData.message+=' Play '+boardData.solution;drawBoard();$('move').value='';
}
async function loadDrill() { boardData=await api('drill',{index:drillIndex});drawBoard(); }
function chatMessage(text, user=false) { const p=document.createElement('p');p.className=user?'chat-user':'chat-reply';p.textContent=clean(text);$('chatLog').append(p);while($('chatLog').children.length>6)$('chatLog').firstChild.remove();p.scrollIntoView({block:'nearest'}); }
$('learnForm').onsubmit=e=>{e.preventDefault();action(()=>preview());};
$('username').oninput=()=>{$('samplePanel').hidden=true;sampleUser='';};
$('analyseButton').onclick=()=>action(async()=>{if(sampleUser!==$('username').value.trim())throw Error('Fetch games for this username first');const limit=Number($('limit').value);if(!Number.isInteger(limit)||limit<1||limit>Number($('limit').max))throw Error('Use the recommendation or a smaller number');await api('learn',{username:sampleUser,limit});await waitForJob();});
$('importButton').onclick=()=>action(async()=>{const file=$('importFile').files[0];if(!file)throw Error('Choose a file first');await preview(file);});
$('loadButton').onclick=()=>action(async()=>{state=await api('load',{username:$('saved').value});openRoom();});
$('back').onclick=()=>{hintVersion++;$('toggleCoach').hidden=true;$('room').hidden=true;$('home').hidden=false;refresh().catch(e=>toast(e.message,true));};
function setCoachOpen(open){$('coachPanel').classList.toggle('closed',!open);$('toggleCoach').setAttribute('aria-expanded',String(open));if(!open)hintVersion++;}
$('toggleCoach').onclick=()=>{const open=$('coachPanel').classList.contains('closed');setCoachOpen(open);if(open)updateHint();};
$('closeCoach').onclick=()=>setCoachOpen(false);
$('openSettings').onclick=()=>action(async()=>{await fields();$('settingsStatus').textContent='';$('settingsDialog').showModal();if(state.settings.provider!=='builtin')await detectModels();});
$('closeSettings').onclick=()=>{$('apiKey').value='';$('settingsDialog').close();};$('settingsDialog').addEventListener('close',()=>$('apiKey').value='');
$('provider').onchange=()=>{showProvider();if($('provider').value==='ollama')$('endpoint').value='http://127.0.0.1:11434';if($('provider').value==='api')$('endpoint').value='https://your-provider.example/v1';};
$('settingsForm').onsubmit=e=>{e.preventDefault();action(saveSettings);};$('detect').onclick=()=>action(detectModels);
$('browse').onclick=()=>action(async()=>{const r=await api('browse',{});if(r.folder)$('folder').value=r.folder;});
$('startOllama').onclick=()=>action(async()=>{$('endpoint').value='http://127.0.0.1:11435';await saveSettings();const r=await api('ollama/start',{});$('modelStatus').textContent=clean(r.message);});
$('installEngine').onclick=()=>action(async()=>{await api('engine/install',{});await waitForJob(false);await fields();});
$('chatForm').onsubmit=e=>{e.preventDefault();action(async()=>{const question=$('question').value.trim();if(question)chatMessage(question,true);$('question').value='';const r=await api('coach',{mode,index:drillIndex,question});chatMessage(r.text);hardware();});};
$('moveForm').onsubmit=e=>{e.preventDefault();if($('move').value.trim())action(()=>sendMove($('move').value.trim()));};
$('newGame').onclick=()=>action(async()=>{boardData=await api('play',{reset:true,color:$('color').value});drawBoard();});
$('playTab').onclick=()=>action(async()=>{mode='play';state=await api('state');boardData=state.board;setModeUI();drawBoard();});
$('drillTab').onclick=()=>action(async()=>{mode='drill';setModeUI();await loadDrill();});
$('nextDrill').onclick=()=>action(async()=>{drillIndex=(drillIndex+1)%profile.puzzle_count;await loadDrill();});
$('reveal').onclick=()=>action(async()=>{const r=await api('drill',{index:drillIndex,reveal:true});$('workStatus').textContent='Engine preferred '+r.solution;});
$('export').onclick=()=>action(async()=>{const p=await api('export');const url=URL.createObjectURL(new Blob([JSON.stringify(p)],{type:'application/json'}));const a=document.createElement('a');a.href=url;a.download=profile.username+'.json';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);});
refresh().then(()=>{if(state.job.busy)action(waitForJob);}).catch(e=>toast(e.message,true));
