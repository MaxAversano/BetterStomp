'use strict';
/* ================= BetterStomp – interfaccia ================= */
const $ = s => document.querySelector(s);
const esc = s => String(s ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const S = { settings:null, st:null, v:'', champs:{}, champList:[], items:{}, perk:{}, style:{}, styles:[], spells:{}, profile:null, ui:{}, lastPhase:'' };
const DD = 'https://ddragon.leagueoflegends.com/cdn/';
const CD = 'https://raw.communitydragon.org/latest/plugins/rcp-fe-lol-static-assets/global/default/images/ranked-emblem/emblem-';

const MODES = [['ranked','Classificata Solo/Duo'],['flex','Classificata Flex'],['normal','Normali'],['aram','ARAM'],['mayhem','ARAM Mayhem'],['arena','Arena'],['urf','URF']];
const TIERS = ['IRON','BRONZE','SILVER','GOLD','PLATINUM','EMERALD','DIAMOND','MASTER','GRANDMASTER','CHALLENGER'];
const TIER_IT = {IRON:'Ferro',BRONZE:'Bronzo',SILVER:'Argento',GOLD:'Oro',PLATINUM:'Platino',EMERALD:'Smeraldo',DIAMOND:'Diamante',MASTER:'Maestro',GRANDMASTER:'Gran Maestro',CHALLENGER:'Challenger'};
const POS = {TOP:'Top',JUNGLE:'Jungle',MIDDLE:'Mid',BOTTOM:'ADC',UTILITY:'Support',ALL:'Tutti'};
const DIVS = ['IV','III','II','I'];
const REGIONS = [['euw1','EUW'],['eun1','EUNE'],['na1','NA'],['kr','KR'],['br1','BR'],['la1','LAN'],['la2','LAS'],['oc1','OCE'],['tr1','TR'],['ru','RU'],['jp1','JP'],['me1','ME'],['sg2','SEA'],['tw2','TW'],['vn2','VN']];
const SHARDS = {5008:['Forza adattiva','StatModsAdaptiveForceIcon.png'],5005:['Velocità d\'attacco','StatModsAttackSpeedIcon.png'],5007:['Velocità abilità','StatModsCDRScalingIcon.png'],
  5001:['Salute crescente','StatModsHealthScalingIcon.png'],5011:['Salute','StatModsHealthPlusIcon.png'],5010:['Velocità movimento','StatModsMovementSpeedIcon.png'],
  5013:['Tenacia e res. rallentamenti','StatModsTenacityIcon.png'],5002:['Armatura','StatModsArmorIcon.png'],5003:['Resistenza magica','StatModsMagicResIcon.MagicResist_Fix.png']};

/* ---------- utilità ---------- */
async function api(path, body){
  const o = body !== undefined ? {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)} : {};
  const r = await fetch(path, o);
  if(!r.ok) throw new Error(await r.text());
  return r.json();
}
function toast(msg, ms=3200){ const t=$('#toast'); t.textContent=msg; t.classList.add('on'); clearTimeout(t._h); t._h=setTimeout(()=>t.classList.remove('on'),ms); }
const pct = (x,d=1) => (x*100).toFixed(d).replace('.',',')+'%';
const num = n => Number(n||0).toLocaleString('it-IT');
function fmtPts(n){ n=Number(n||0); if(n>=1e6) return (n/1e6).toFixed(n>=1e7?1:2).replace('.',',').replace(/,0+$/,'')+'M'; if(n>=1e3) return Math.round(n/1e3)+'K'; return String(n); }
function ago(ms){ const s=(Date.now()-ms)/1000; if(s<3600) return Math.max(1,Math.round(s/60))+' min fa'; if(s<86400) return Math.round(s/3600)+' h fa'; const d=Math.round(s/86400); return d===1?'ieri':d+' giorni fa'; }
const dur = s => Math.floor(s/60)+':'+String(Math.floor(s%60)).padStart(2,'0');
const champ = k => S.champs[k] || {id:'',name:'#'+k};
const cImg = k => S.champs[k] ? `${DD}${S.v}/img/champion/${S.champs[k].id}.png` : '';
const iImg = id => `${DD}${S.v}/img/item/${id}.png`;
const sImg = k => S.spells[k] ? `${DD}${S.v}/img/spell/${S.spells[k].image.full}` : '';
const pImg = id => S.perk[id] ? 'https://ddragon.leagueoflegends.com/cdn/img/'+S.perk[id].icon : (SHARDS[id] ? 'https://ddragon.leagueoflegends.com/cdn/img/perk-images/StatMods/'+SHARDS[id][1] : '');
const stImg = id => S.style[id] ? 'https://ddragon.leagueoflegends.com/cdn/img/'+S.style[id].icon : '';
const profImg = i => `${DD}${S.v}/img/profileicon/${i||29}.png`;
const emblem = t => t && t!=='NONE' ? `<img class="emblem" src="${CD}${t.toLowerCase()}.png" onerror="this.style.display='none'">` : '';
const rankTxt = q => !q || !q.tier || q.tier==='NONE' ? 'Non classificato' : TIER_IT[q.tier]+(['MASTER','GRANDMASTER','CHALLENGER'].includes(q.tier)?'':' '+q.division);
const ci = (k,cls='') => k ? `<img class="ic ${cls}" src="${cImg(k)}" title="${esc(champ(k).name)}" loading="lazy">` : `<span class="ic ${cls}"></span>`;
const tierBadge = t => t ? `<span class="tier ${t==='S+'?'Sp':t}">${t}</span>` : '';
const opt = (list, cur) => list.map(([v,l]) => `<option value="${v}" ${String(v)===String(cur)?'selected':''}>${esc(l)}</option>`).join('');
function tierOptions(cur){
  const l = [['auto','Il mio rango'],['ALL','Tutti i ranghi'],['PLATINUM+','Platino+'],['EMERALD+','Smeraldo+'],['DIAMOND+','Diamante+'],['MASTER+','Maestro+']].concat(TIERS.map(t=>[t,TIER_IT[t]]));
  return opt(l, cur||'auto');
}
const modeName = m => (MODES.find(x=>x[0]===m)||[0,m])[1];
const isPos = m => ['ranked','flex','normal'].includes(m);
function sw(path, on){ return `<label class="switch"><input type="checkbox" data-set="${path}" ${on?'checked':''}><span></span></label>`; }
function getPath(o, p){ return p.split('.').reduce((a,k)=>a?.[k], o); }
function setPath(o, p, v){ const k=p.split('.'); let x=o; k.slice(0,-1).forEach(z=>x=x[z]=x[z]||{}); x[k.at(-1)]=v; }

/* ---------- impostazioni (salvate al volo, senza ricaricare) ---------- */
let saveTimer;
function setSetting(path, value){
  setPath(S.settings, path, value);
  applyTheme();
  clearTimeout(saveTimer);
  saveTimer = setTimeout(()=>api('/api/settings', S.settings).catch(e=>toast('Errore salvataggio: '+e.message)), 250);
  document.dispatchEvent(new CustomEvent('settings', {detail:path}));
}
function applyTheme(){
  const s = S.settings||{};
  document.body.classList.toggle('light', s.theme==='light');
  document.body.classList.toggle('premium', !!s.premium);
  document.body.classList.toggle('noanim', s.animations===false);
  const tb=document.getElementById('themebtn'); if(tb) tb.textContent = s.theme==='light' ? '☾' : '☀';
}

/* ---------- avvio ---------- */
async function loadStatic(){
  for(let i=0;;i++){
    try{
      const [c, it, ru, sp] = await Promise.all(['champion.json','item.json','runesReforged.json','summoner.json'].map(f=>api('/api/static/'+f)));
      S.v = c.version;
      for(const x of Object.values(c.data)) S.champs[+x.key] = x;
      S.champList = Object.values(c.data).sort((a,b)=>a.name.localeCompare(b.name,'it'));
      for(const [id,x] of Object.entries(it.data)) S.items[+id] = x;
      S.styles = ru;
      for(const st of ru){ S.style[st.id]=st; st.slots.forEach(sl=>sl.runes.forEach(r=>S.perk[r.id]=r)); }
      for(const x of Object.values(sp.data)) S.spells[+x.key] = x;
      return;
    }catch(e){ if(i===0) $('#main').innerHTML = `<div class="loading"><span class="spinner"></span>Scarico i dati di gioco da Riot…</div>`; await new Promise(r=>setTimeout(r,1500)); }
  }
}

async function boot(){
  S.settings = await api('/api/settings');
  applyTheme();
  await loadStatic();
  await poll();
  setInterval(poll, 2000);
  window.addEventListener('hashchange', render);
  render();
}

async function poll(){
  try{ S.st = await api('/api/state'); }catch(e){ return; }
  const c = S.st.client;
  $('#cdot').classList.toggle('on', c.connected);
  const ph = {None:'Client connesso',Lobby:'In lobby',Matchmaking:'In coda',ReadyCheck:'Partita trovata',ChampSelect:'Champion select',InProgress:'In partita',EndOfGame:'Fine partita',WaitingForStats:'Fine partita',PreEndOfGame:'Fine partita'};
  $('#ctext').textContent = c.connected ? (ph[c.phase]||'Client connesso') : (S.st.inGame ? 'In partita' : 'Client non connesso');
  const navSel = document.querySelector('.nav a[data-p=select]');
  navSel.innerHTML = 'Champion Select' + (c.phase==='ChampSelect'?'<span class="live"></span>':'');
  // in champion select l'app si porta da sola sulla schermata di aiuto
  if(c.phase==='ChampSelect' && S.lastPhase!=='ChampSelect' && route()[0]!=='select'){ location.hash='#/select'; }
  if(c.phase!==S.lastPhase && c.phase==='EndOfGame'){ S.profile=null; }
  S.lastPhase = c.phase;
  if(c.connected && !S.profile && !S._profLoading){ S._profLoading=true; api('/api/profile').then(p=>{S.profile=p; renderSide(); if(route()[0]==='profilo'&&!S.ui.search) render();}).finally(()=>S._profLoading=false); }
  renderSide();
  if(route()[0]==='select') updateSelect();
  if(route()[0]==='impostazioni') updateEngineBox();
  if(route()[0]==='tier' && S.ui.tierEmpty && S.st.engine.total>0) render();
}

/* ---------- sidebar: riepilogo account ---------- */
function renderSide(){
  const c = S.st?.client; const el = $('#side');
  let h = '';
  if(c && c.connected && c.summoner){
    const s = c.summoner, solo = c.ranked?.RANKED_SOLO_5x5, flex = c.ranked?.RANKED_FLEX_SR;
    const p = S.profile;
    const games = solo ? solo.wins+solo.losses : 0;
    h += `<div class="card acc"><div class="av"><img src="${profImg(s.profileIconId)}"><span class="lvl">${s.summonerLevel}</span></div>
      <div class="name">${esc(s.gameName)} <span class="tag">#${esc(s.tagLine)}</span></div>
      <div class="col mt">
        ${[['Solo/Duo',solo],['Flex',flex]].map(([n,q])=>`<div class="rankmini">${q&&q.tier&&q.tier!=='NONE'?`<img src="${CD}${q.tier.toLowerCase()}.png" onerror="this.style.visibility='hidden'">`:'<span style="width:46px"></span>'}
          <div class="sp"><div class="tiny dim b">${n.toUpperCase()}</div><div class="b">${rankTxt(q)}</div>
          ${q&&q.tier&&q.tier!=='NONE'?`<div class="small muted">${q.leaguePoints} LP · ${q.wins}V ${q.losses}S · ${pct(q.wins/Math.max(1,q.wins+q.losses),0)}</div>`:''}</div></div>`).join('')}
      </div>
      <div class="mt" style="text-align:left">
        <div class="kv"><span class="muted">Ruolo principale</span><b>${p? (p.roles.enough? POS[p.roles.main]+(p.roles.second?' / '+POS[p.roles.second]:'') : `<span class="dim">${p.roles.games}/${p.roles.needed} partite</span>`) : '…'}</b></div>
        <div class="kv"><span class="muted">Oggi (Solo/Duo)</span><b>${p? `<span class="good">${p.today.w}V</span> <span class="bad">${p.today.l}S</span>`:'…'}</b></div>
        <div class="kv"><span class="muted">Campione più giocato</span><b>${p&&p.champs?.[0]? esc(champ(p.champs[0].champion).name):'…'}</b></div>
      </div></div>`;
  } else {
    h += `<div class="card acc"><div class="av"><img src="logo.svg" style="background:none;border:0"></div><div class="name">Benvenuto</div>
      <p class="muted small">Apri il client di League of Legends: BetterStomp lo rileva da solo e mostra qui il riepilogo del tuo account.</p></div>`;
  }
  const e = S.st?.engine;
  if(e){
    h += `<div class="card flat"><div class="kicker">Motore statistiche</div>
      ${!e.hasKey?`<p class="small muted">Inserisci una chiave Riot API in <a href="#/impostazioni">Impostazioni</a> per sbloccare tier list, win rate e build per rango.</p>`
      : `<div class="row mt"><span class="dot ${e.running&&!e.error?'on':''}"></span><b>${num(e.total)}</b><span class="muted small">partite analizzate · patch ${esc(e.patch||'-')}</span></div>
         ${e.error?`<div class="small bad mt">${esc(e.error)}</div>`:''}`}
    </div>`;
  }
  h += `<div class="ad short">Spazio pubblicitario</div>`;
  if(el._h !== h){ el.innerHTML = h; el._h = h; }
}

/* ---------- router ---------- */
function route(){ return (location.hash.replace(/^#\/?/,'')||'profilo').split('/').map(decodeURIComponent); }
function render(){
  const r = route();
  document.querySelectorAll('.nav a').forEach(a=>a.classList.toggle('on', a.dataset.p===r[0] || (r[0]==='campione'&&a.dataset.p==='campioni')));
  const m = $('#main');
  const pages = {profilo:pageProfile, campioni:pageChampions, campione:pageChampion, tier:pageTier, select:pageSelect, patch:pagePatch, overlay:pageOverlay, streaming:pageStreaming, premium:pagePremium, impostazioni:pageSettings};
  (pages[r[0]]||pageProfile)(m, r);
  m.scrollTop = 0;
}

/* ================= PROFILO ================= */
async function pageProfile(m){
  S.ui.ptab = S.ui.ptab || 'partite';
  const reg = S.ui.searchRegion || S.settings.region;
  m.innerHTML = `<div class="page"><div class="card flat mb"><div class="search">
      <input type="search" id="sid" placeholder="Cerca un giocatore: Nome#TAG" value="${esc(S.ui.search||'')}">
      <select id="sreg">${opt(REGIONS, reg)}</select>
      <button class="btn" data-act="search">Cerca</button>
      ${S.ui.search?`<button class="btn ghost" data-act="myprofile">Il mio profilo</button>`:''}
    </div></div><div id="pbody"><div class="loading"><span class="spinner"></span>Carico il profilo…</div></div></div>`;
  $('#sid').addEventListener('keydown', e=>{ if(e.key==='Enter') actions.search(); });
  let p;
  try{
    if(S.ui.search) p = await api(`/api/search?riotId=${encodeURIComponent(S.ui.search)}&region=${reg}`);
    else { p = S.profile && !S.ui.forceProf ? S.profile : await api('/api/profile'+(S.ui.forceProf?'?force=1':'')); S.ui.forceProf=false; if(!p.error) S.profile=p; }
  }catch(e){ p = {error:e.message}; }
  if(route()[0]!=='profilo') return;
  S.ui.prof = p;
  $('#pbody').innerHTML = profileHTML(p);
}

function profileHTML(p){
  if(p.error) return `<div class="card"><div class="empty"><div class="big">🔎</div>${esc(p.error)}
    ${p.error.includes('chiave')?'<div class="mt"><a class="btn" href="#/impostazioni">Vai alle impostazioni</a></div>':''}</div></div>`;
  const solo = p.ranked?.RANKED_SOLO_5x5, flex = p.ranked?.RANKED_FLEX_SR;
  const rc = (n,q) => `<div class="card flat row" style="flex:1;min-width:230px">${emblem(q?.tier)||'<div style="width:72px"></div>'}
     <div><div class="kicker">${n}</div><div style="font-size:19px;font-weight:750">${rankTxt(q)}</div>
     ${q&&q.tier&&q.tier!=='NONE'?`<div class="muted">${q.leaguePoints} LP · ${q.wins}V ${q.losses}S · <b>${pct(q.wins/Math.max(1,q.wins+q.losses))}</b></div>`:''}</div></div>`;
  const tabs = [['partite','Partite'],['campioni','Campioni'],['maestrie','Maestrie'],['premade','Premade']];
  return `<div class="card glow"><div class="hero">
      <div class="acc"><div class="av" style="margin:0"><img src="${profImg(p.icon)}"><span class="lvl">${p.level||''}</span></div></div>
      <div class="sp"><h1>${esc(p.name)} <span class="dim">#${esc(p.tag)}</span></h1>
        <div class="row mt wrap">
          <span class="badge">${p.source==='client'?'Dal client':'Riot API · '+esc((REGIONS.find(r=>r[0]===p.region)||['',''])[1])}</span>
          <span class="badge">Ruolo: ${p.roles.enough? POS[p.roles.main]+(p.roles.second?' / '+POS[p.roles.second]:'') : `servono 20 partite (${p.roles.games}/20)`}</span>
          ${p.source==='client'?`<button class="btn ghost sm" data-act="refreshprof">↻ Aggiorna</button>`:''}
        </div></div></div>
      <div class="row wrap mt">${rc('Classificata Solo/Duo',solo)}${rc('Classificata Flex',flex)}</div></div>
    <div class="row mt2 mb"><div class="seg" id="ptabs">${tabs.map(([k,l])=>`<button data-act="ptab" data-k="${k}" class="${S.ui.ptab===k?'on':''}">${l}</button>`).join('')}</div></div>
    <div id="ptab">${profileTab(p)}</div>`;
}

function profileTab(p){
  const t = S.ui.ptab;
  if(t==='maestrie'){
    if(!p.mastery?.length) return `<div class="card"><div class="empty">Nessuna maestria disponibile.</div></div>`;
    return `<div class="mast">${p.mastery.slice(0,30).map(m=>`<div class="m"><img src="${cImg(m.champion)}"><div class="small b mt" style="margin-top:6px">${esc(champ(m.champion).name)}</div>
      <div class="pts">${fmtPts(m.points)}</div><div class="tiny dim">Livello ${m.level}</div></div>`).join('')}</div>`;
  }
  if(t==='campioni'){
    if(!p.champs?.length) return `<div class="card"><div class="empty">Nessuna partita recente.</div></div>`;
    return `<table class="tl"><thead><tr><th>Campione</th><th>Partite</th><th>Win rate</th><th>KDA</th><th>CS/min</th></tr></thead><tbody>
      ${p.champs.map(c=>`<tr data-act="openchamp" data-id="${champ(c.champion).id}"><td><div class="row">${ci(c.champion,'s')}<b>${esc(champ(c.champion).name)}</b></div></td>
      <td>${c.games}</td><td class="${c.wins/c.games>=.5?'good':'bad'} b">${pct(c.wins/c.games,0)}</td><td>${c.kda.toFixed(2).replace('.',',')}</td><td>${c.csMin.toFixed(1).replace('.',',')}</td></tr>`).join('')}</tbody></table>`;
  }
  if(t==='premade'){
    if(!p.premades?.length) return `<div class="card"><div class="empty"><div class="big">👥</div>Nessun compagno ricorrente nelle ultime partite.<div class="small dim mt">Vengono mostrati i giocatori trovati con te in squadra almeno 2 volte.</div></div></div>`;
    return `<div class="card"><div class="ctitle"><h2>Con chi hai giocato</h2><span class="muted small">compagni ricorrenti nelle ultime ${p.matches.length} partite</span></div>
      ${p.premades.map(x=>`<div class="kv" style="align-items:center"><div><b>${esc(x.name)}</b> <span class="dim">#${esc(x.tag)}</span><div class="tiny dim">ultima ${ago(x.last)}</div></div>
      <div class="row"><span class="badge">${x.games} partite</span><b class="${x.wins/x.games>=.5?'good':'bad'}">${pct(x.wins/x.games,0)}</b>
      <button class="btn ghost sm" data-act="lookup" data-id="${esc(x.name+'#'+x.tag)}">Profilo</button></div></div>`).join('')}</div>`;
  }
  if(!p.matches?.length) return `<div class="card"><div class="empty">Nessuna partita recente.</div></div>`;
  return p.matches.map((g,i)=>{
    const me = g.me, cls = g.remake?'remake':me.win?'win':'loss';
    const mins = Math.max(1,g.duration/60);
    const res = g.remake?'Remake':(me.place? me.place+'° posto' : me.win?'Vittoria':'Sconfitta');
    return `<div class="match ${cls}"><div class="stripe"></div>${ci(me.champ,'l')}
      <div><div class="res">${res}</div><div class="small muted">${esc(qName(g.queue))} · ${dur(g.duration)} · ${ago(g.start)}</div>
        <div class="row gap6 mt" style="margin-top:6px">${me.spells.map(s=>s?`<img class="ic xs" src="${sImg(s)}">`:'').join('')}</div></div>
      <div><div class="b" style="font-size:16px">${me.k} / <span class="bad">${me.d}</span> / ${me.a}</div><div class="small muted">${((me.k+me.a)/Math.max(1,me.d)).toFixed(2).replace('.',',')} KDA · ${me.cs} CS (${(me.cs/mins).toFixed(1).replace('.',',')})</div></div>
      <div class="items">${me.items.map(x=>x?`<img src="${iImg(x)}" title="${esc(S.items[x]?.name||'')}">`:'<span class="eslot"></span>').join('')}</div>
      <div class="row gap6"><button class="btn ghost sm" data-act="analyze" data-i="${i}">Analizza</button>
        <button class="coach" data-act="coach"><span class="sparkle">✦</span> Coach AI</button></div>
      <div class="analysis" id="an${i}" style="display:none"></div></div>`;
  }).join('');
}
function qName(q){ return {420:'Solo/Duo',440:'Flex',400:'Normale',430:'Blind',490:'Rapida',450:'ARAM',2400:'ARAM Mayhem',1700:'Arena',1710:'Arena',900:'URF',1900:'URF',0:'Personalizzata'}[q] || 'Altra modalità'; }

/* ================= CAMPIONI ================= */
async function pageChampions(m){
  const u = S.ui; u.cmode = u.cmode||S.settings.defaultMode||'ranked'; u.ctier = u.ctier||'auto'; u.cpos = u.cpos||'ALL'; u.cq = u.cq||'';
  m.innerHTML = `<div class="page"><div class="filters">
      <input type="search" id="cq" placeholder="Cerca campione…" value="${esc(u.cq)}" style="width:220px">
      <select data-ui="cmode">${opt(MODES,u.cmode)}</select>
      <select data-ui="ctier">${tierOptions(u.ctier)}</select>
      <div class="chips">${isPos(u.cmode)?['ALL','TOP','JUNGLE','MIDDLE','BOTTOM','UTILITY'].map(p=>`<span class="chip ${u.cpos===p?'on':''}" data-act="cpos" data-p="${p}">${POS[p]}</span>`).join(''):''}</div>
    </div><div id="cgrid"><div class="loading"><span class="spinner"></span></div></div></div>`;
  $('#cq').addEventListener('input', e=>{ u.cq=e.target.value; drawChampGrid(); });
  try{ S.ui.ctl = await api(`/api/tierlist?mode=${u.cmode}&tier=${u.ctier}&pos=`); }catch(e){ S.ui.ctl = {rows:[]}; }
  drawChampGrid();
}
function drawChampGrid(){
  const u = S.ui, rows = u.ctl?.rows||[];
  const best = {};
  rows.forEach(r=>{ if(u.cpos!=='ALL' && isPos(u.cmode) && r.position!==u.cpos) return; if(!best[r.champion]||best[r.champion].games<r.games) best[r.champion]=r; });
  const q = u.cq.trim().toLowerCase();
  let list = S.champList.filter(c=>!q || c.name.toLowerCase().includes(q) || c.id.toLowerCase().includes(q));
  if(u.cpos!=='ALL' && isPos(u.cmode) && rows.length) list = list.filter(c=>best[+c.key]);
  const el = $('#cgrid'); if(!el) return;
  el.innerHTML = (rows.length?'':`<div class="card flat mb small muted">Le statistiche per rango compariranno qui quando il motore avrà analizzato abbastanza partite (serve la chiave Riot API).</div>`) +
    `<div class="cgrid">${list.map(c=>{ const r=best[+c.key]; return `<div class="ccard" data-act="openchamp" data-id="${c.id}">
      <span class="t">${r?tierBadge(r.tier):''}</span><img src="${DD}${S.v}/img/champion/${c.id}.png" loading="lazy"><div class="n">${esc(c.name)}</div>
      <div class="wr">${r?pct(r.winRate)+' WR':esc(c.tags?.[0]||'')}</div></div>`; }).join('')}</div>`;
}

/* ================= CAMPIONE ================= */
async function pageChampion(m, r){
  const c = S.champList.find(x=>x.id===r[1]); if(!c){ location.hash='#/campioni'; return; }
  const key = +c.key, u = S.ui;
  if(u.chId!==c.id){ u.chId=c.id; u.chPos=''; u.editing=false; }
  u.chMode = u.chMode||u.cmode||S.settings.defaultMode||'ranked'; u.chTier = u.chTier||u.ctier||'auto';
  m.innerHTML = `<div class="page"><div class="champhero"><div class="bg" style="background-image:url('https://ddragon.leagueoflegends.com/cdn/img/champion/splash/${c.id}_0.jpg')"></div>
    <div class="in"><img class="ic xl" src="${cImg(key)}"><div class="sp"><div class="kicker" style="color:#c6cbe6">${esc(c.title)}</div><h1 style="font-size:34px">${esc(c.name)}</h1>
    <div class="row wrap mt" id="chstats"></div></div>
    <div class="col" style="align-items:flex-end"><a class="btn ghost sm" href="#/campioni">← Tutti i campioni</a></div></div></div>
    <div class="filters mt"><select data-ui="chMode">${opt(MODES,u.chMode)}</select><select data-ui="chTier">${tierOptions(u.chTier)}</select><div class="chips" id="chpos"></div><span class="sp"></span>
      <span class="small muted" id="chsrc"></span></div>
    <div id="chbody"><div class="loading"><span class="spinner"></span>Calcolo la build…</div></div></div>`;
  let data, detail;
  try{
    [data, detail] = await Promise.all([api(`/api/champion?id=${key}&mode=${u.chMode}&tier=${u.chTier}&pos=${u.chPos}`), api(`/api/static/champion/${c.id}.json`).catch(()=>null)]);
  }catch(e){ $('#chbody').innerHTML = `<div class="card"><div class="empty">${esc(e.message)}</div></div>`; return; }
  if(route()[1]!==c.id) return;
  u.chData = data; u.chKey = key;
  const b = data.build;
  $('#chstats').innerHTML = b.games ? `${tierBadge(b.tierLabel)}<span class="badge">${pct(b.winRate)} WR</span><span class="badge">${pct(b.pickRate)} pick</span><span class="badge">${pct(b.banRate)} ban</span><span class="badge">${num(b.games)} partite</span>`
    : `<span class="badge">Nessuna statistica ancora per questo filtro</span>`;
  $('#chpos').innerHTML = isPos(u.chMode) ? (b.positions||[]).filter(p=>p.position!=='ALL'&&p.share>=0.04).map(p=>`<span class="chip ${b.position===p.position?'on':''}" data-act="chpos" data-p="${p.position}">${POS[p.position]} <span class="tiny">${pct(p.share,0)}</span></span>`).join('') : '';
  $('#chsrc').innerHTML = b.source==='stats' ? `Patch ${esc(b.patch)}${b.widened?' · campione ampliato a tutti i ranghi':''}` : '';
  const prof = S.profile?.champs?.find(x=>x.champion===key);
  $('#chbody').innerHTML = `<div class="grid2" style="grid-template-columns:1.4fr 1fr">
    <div class="col gap16">
      <div class="card"><div class="ctitle"><h2>Build</h2>${b.personal?'<span class="badge goldc">Build personale</span>':''}<span class="sp"></span>
        <button class="btn sm" data-act="import" ${S.st?.client?.connected?'':'disabled title="Apri il client di League"'}>Importa nel client</button>
        <button class="btn ghost sm" data-act="editbuild">${u.editing?'Chiudi editor':'Modifica build personale'}</button></div>
        <div id="buildbox">${buildHTML(b, data.personal)}</div></div>
      <div class="card"><h2>Abilità</h2>${detail? abilitiesHTML(detail.data[c.id]) : '<div class="muted">Non disponibili.</div>'}</div>
    </div>
    <div class="col gap16">
      <div class="card"><h2>Rune</h2>${runesHTML(b.runes)}${b.runes?`<div class="row mt"><span class="small muted sp">${esc(b.runes.source||'')}${b.runes.games?` · ${pct(b.runes.winRate)} WR su ${num(b.runes.games)}`:''}</span><button class="btn ghost sm" data-act="saverunes">Salva come personali</button></div>`:''}
        ${b.runesAlt?`<details class="mt"><summary class="small muted" style="cursor:pointer">Alternativa (${pct(b.runesAlt.winRate)} WR)</summary><div class="mt">${runesHTML(b.runesAlt)}</div></details>`:''}</div>
      <div class="card"><h2>Matchup</h2>${pairsHTML('Più facili',b.best,'good')}${pairsHTML('Più difficili',b.worst,'bad')}${pairsHTML('Sinergie migliori',b.synergy,'good')}
        ${!b.best?.length&&!b.worst?.length?'<div class="muted small">Servono più partite analizzate per i matchup.</div>':''}</div>
      ${prof?`<div class="card"><h2>Le tue partite</h2><div class="stats"><div class="stat"><div class="k">Partite</div><div class="v">${prof.games}</div></div>
        <div class="stat"><div class="k">Win rate</div><div class="v ${prof.wins/prof.games>=.5?'good':'bad'}">${pct(prof.wins/prof.games,0)}</div></div><div class="stat"><div class="k">KDA</div><div class="v">${prof.kda.toFixed(2).replace('.',',')}</div></div></div></div>`:''}
      <div class="card flat"><div class="kicker mb">Approfondisci</div><div class="row wrap">
        ${[['u.gg',`https://u.gg/lol/champions/${c.id.toLowerCase()}/build`],['op.gg',`https://op.gg/lol/champions/${c.id.toLowerCase()}/build`],['lolalytics',`https://lolalytics.com/lol/${c.id.toLowerCase()}/build/`]].map(([n,l])=>`<a class="btn ghost sm" href="${l}" target="_blank">${n} ↗</a>`).join('')}</div></div>
    </div></div>`;
}

function slotsHTML(ids, n, block, editable, wr){
  let h = '';
  for(let i=0;i<n;i++){
    const id = ids?.[i];
    h += `<div class="slot ${id?'filled':''} ${editable?'edit':''}" ${editable?`data-act="pickitem" data-b="${block}" data-i="${i}"`:''} title="${id?esc(S.items[id]?.name||''):'Slot vuoto'}">
      ${id?`<img src="${iImg(id)}">`:(editable?'+':'')}${id&&editable?`<span class="x" data-act="clearitem" data-b="${block}" data-i="${i}">×</span>`:''}
      ${id&&wr&&wr[id]!==undefined&&!editable?`<span class="wr">${pct(wr[id],0)}</span>`:''}</div>`;
  }
  return `<div class="slots" style="margin-bottom:${wr?14:0}px">${h}</div>`;
}
function buildHTML(b, personal){
  const ed = S.ui.editing;
  const src = ed ? (S.ui.edit ||= {starting:[...(personal?.starting||b.starting||[])], core:[...(personal?.core||b.core||[])], situational:[...(personal?.situational||b.situational||[])]}) : b;
  return `${!ed && b.spells?.length?`<div class="blocklbl"><span>Incantesimi</span><span class="small muted">${b.spellsWR?pct(b.spellsWR)+' WR':''}</span></div><div class="row">${b.spells.map(s=>`<img class="ic" src="${sImg(s)}" title="${esc(S.spells[s]?.name||'')}">`).join('')}
      ${b.skills?`<span class="sp"></span><div class="skills">${b.skills.split('>').map((k,i)=>`${i?'<span class="arr">›</span>':''}<span class="skey">${k}</span>`).join('')}</div>`:''}</div>
      ${b.skillFirst?`<div class="small muted right" style="margin-top:6px">Primi livelli: ${b.skillFirst.split('').join(' → ')}</div>`:''}`:''}
    <div class="blocklbl"><span>Inizio partita</span><span class="small dim">5 slot</span></div>${slotsHTML(src.starting,5,'starting',ed)}
    <div class="blocklbl"><span>Build principale</span><span class="small dim">10 slot</span></div>${slotsHTML(src.core,10,'core',ed,ed?null:b.itemWR)}
    <div class="blocklbl"><span>Situazionali</span><span class="small dim">10 slot</span></div>${slotsHTML(src.situational,10,'situational',ed,ed?null:b.itemWR)}
    ${ed?`<div class="row mt"><button class="btn" data-act="savebuild">Salva build personale</button><button class="btn ghost" data-act="resetbuild">Elimina build personale</button>
      <span class="small muted">La build personale viene importata al posto di quella statistica quando "Preferisci le build personali" è attivo.</span></div>`:''}
    ${!ed && !b.core?.length?`<p class="small muted mt">Nessuna build statistica ancora: puoi crearne una personale con l'editor, verrà importata in champion select.</p>`:''}`;
}
function runesHTML(r){
  if(!r || !r.perks?.length) return `<div class="muted small">Nessuna pagina disponibile. Con il client aperto vengono usate le rune consigliate da Riot.</div>`;
  const tree = (styleId, chosen, primary) => { const st = S.style[styleId]; if(!st) return '';
    const slots = primary ? st.slots : st.slots.slice(1);
    return `<div class="tree"><div class="row gap6"><img src="${stImg(styleId)}" style="width:22px;height:22px;opacity:1;filter:none"><span class="tname">${esc(st.name)}</span></div>
      ${slots.map((sl,i)=>`<div class="rrow">${sl.runes.map(x=>`<img class="${chosen.includes(x.id)?'on':''} ${primary&&i===0?'key':''}" src="https://ddragon.leagueoflegends.com/cdn/img/${x.icon}" title="${esc(x.name)}">`).join('')}</div>`).join('')}</div>`; };
  const p = r.perks;
  return `<div class="runes">${tree(r.primary,p.slice(0,4),true)}${tree(r.secondary,p.slice(4,6),false)}
    <div class="shards">${p.slice(6,9).map(s=>`<div class="shard"><img src="${pImg(s)}" onerror="this.style.visibility='hidden'">${esc(SHARDS[s]?.[0]||s)}</div>`).join('')}</div></div>`;
}
function pairsHTML(t, list, cls){
  if(!list?.length) return '';
  return `<div class="blocklbl"><span>${t}</span></div><div class="pairs">${list.map(p=>`<div class="pair" data-act="openchamp" data-id="${champ(p.champion).id}">${ci(p.champion,'s')}<b class="sp">${esc(champ(p.champion).name)}</b>
    <span class="${cls} b">${pct(p.winRate)}</span><span class="tiny dim">${p.games} p.</span></div>`).join('')}</div>`;
}
function abilitiesHTML(d){
  if(!d) return '';
  const clean = s => esc(String(s||'').replace(/<br\s*\/?>/g,' ').replace(/<[^>]+>/g,''));
  const rows = [[`${DD}${S.v}/img/passive/${d.passive.image.full}`,'Passiva',d.passive.name,d.passive.description]]
    .concat(d.spells.map((s,i)=>[`${DD}${S.v}/img/spell/${s.image.full}`,'QWER'[i],s.name,s.description+(s.cooldownBurn?` (Ricarica: ${s.cooldownBurn}s)`:'')]));
  return rows.map(([img,k,n,desc])=>`<div class="ability"><img src="${img}"><div><b>${k} · ${esc(n)}</b><div class="d">${clean(desc)}</div></div></div>`).join('');
}

/* ================= TIER LIST ================= */
async function pageTier(m){
  const u = S.ui; u.tmode = u.tmode||S.settings.defaultMode||'ranked'; u.ttier=u.ttier||'auto'; u.tpos=u.tpos||'ALL'; u.tsort=u.tsort||'score';
  m.innerHTML = `<div class="page"><div class="row mb"><h1>Tier list</h1><span class="sp"></span><span class="small muted" id="tinfo"></span></div>
    <div class="filters"><select data-ui="tmode">${opt(MODES,u.tmode)}</select><select data-ui="ttier">${tierOptions(u.ttier)}</select>
    <div class="chips">${isPos(u.tmode)?['ALL','TOP','JUNGLE','MIDDLE','BOTTOM','UTILITY'].map(p=>`<span class="chip ${u.tpos===p?'on':''}" data-act="tpos" data-p="${p}">${POS[p]}</span>`).join(''):''}</div></div>
    <div id="tbody"><div class="loading"><span class="spinner"></span></div></div></div>`;
  let d; try{ d = await api(`/api/tierlist?mode=${u.tmode}&tier=${u.ttier}&pos=${isPos(u.tmode)?u.tpos:''}`); }catch(e){ d={rows:[]}; }
  if(route()[0]!=='tier') return;
  u.tdata = d; u.tierEmpty = !d.rows?.length;
  drawTier();
}
function drawTier(){
  const u = S.ui, d = u.tdata; const el = $('#tbody'); if(!el) return;
  const e = S.st?.engine||{};
  if(!d.rows?.length){
    el.innerHTML = `<div class="card"><div class="empty"><div class="big">📊</div><h2>Ancora nessun dato per questo filtro</h2>
      <p class="muted">La tier list è calcolata da BetterStomp analizzando partite pubbliche tramite la Riot API ufficiale.<br>
      ${!e.hasKey?'Inserisci la tua chiave Riot API nelle impostazioni per avviare il motore statistiche.':`Motore attivo: ${num(e.total)} partite analizzate finora${e.perHour?` (~${num(e.perHour)}/ora)`:''}. I dati crescono nel tempo e restano salvati sul PC.`}</p>
      ${!e.hasKey?'<a class="btn" href="#/impostazioni">Apri impostazioni</a>':''}</div></div>`;
    return;
  }
  $('#tinfo').textContent = `${num(d.matches)} partite · patch ${d.patch} · ${d.tier.includes('+')?TIER_IT[d.tier.replace('+','')]+'+':TIER_IT[d.tier]||'Tutti i ranghi'}`;
  const rows = [...d.rows];
  const k = u.tsort, dir = u.tdir||-1;
  if(k!=='score') rows.sort((a,b)=> k==='name' ? champ(a.champion).name.localeCompare(champ(b.champion).name)*-dir : (a[k]-b[k])*dir);
  const th = (key,l) => `<th class="${u.tsort===key?'on':''}" data-act="tsort" data-k="${key}">${l}${u.tsort===key?(dir<0?' ↓':' ↑'):''}</th>`;
  el.innerHTML = `<table class="tl"><thead><tr><th>#</th>${th('name','Campione')}${isPos(u.tmode)?'<th>Ruolo</th>':''}${th('score','Tier')}${th('winRate','Win rate')}${th('pickRate','Pick rate')}${th('banRate','Ban rate')}${th('games','Partite')}</tr></thead><tbody>
    ${rows.map((r,i)=>`<tr data-act="openchamp" data-id="${champ(r.champion).id}" data-pos="${r.position}"><td class="dim b">${i+1}</td><td><div class="row">${ci(r.champion,'s')}<b>${esc(champ(r.champion).name)}</b></div></td>
    ${isPos(u.tmode)?`<td class="muted">${POS[r.position]||''}</td>`:''}<td>${tierBadge(r.tier)}</td><td class="b ${r.winRate>=.5?'good':'bad'}">${pct(r.winRate)}</td><td>${pct(r.pickRate)}</td><td>${pct(r.banRate)}</td><td class="muted">${num(r.games)}</td></tr>`).join('')}</tbody></table>`;
}

/* ================= CHAMPION SELECT ================= */
function pageSelect(m){
  const u = S.ui; u.skind = u.skind||'matchup'; u.over = u.over||{}; u.sbuildKey='';
  m.innerHTML = `<div class="page" id="selpage"><div class="card glow"><div class="row mb"><h1>Champion Select</h1><span class="sp"></span><span id="selphase" class="badge"></span></div>
      <div class="teams" id="teams"></div><div class="row mt small muted" id="bans"></div></div>
    <div class="grid2 mt2" style="grid-template-columns:1fr 1fr">
      <div class="card"><div class="ctitle"><h2>Scegli il campione</h2><span class="sp"></span>
        <div class="seg"><button data-act="skind" data-k="matchup" class="${u.skind==='matchup'?'on':''}">Per matchup</button><button data-act="skind" data-k="team" class="${u.skind==='team'?'on':''}">Per team comp</button></div></div>
        <div class="filters" id="sfilters"></div><div id="sugg"></div></div>
      <div class="card"><div class="ctitle"><h2>La tua build</h2><span class="sp"></span><span id="impmsg" class="small muted"></span></div><div id="selbuild"></div></div>
    </div></div>`;
  updateSelect(true);
}
function selState(){
  const c = S.st?.client, cs = c?.champSelect, u = S.ui;
  const mine = [], theirs = [];
  let myPos = '', myChamp = 0, myCell = -1;
  if(cs){
    cs.myTeam.forEach((p,i)=>{ const ch = p.championId||p.championPickIntent; mine.push({champ:u.over['a'+i]||ch, pos:p.assignedPosition?.toUpperCase()||'', name:p.gameName||'', me:p.cellId===cs.localPlayerCellId}); });
    cs.theirTeam.forEach((p,i)=>theirs.push({champ:u.over['e'+i]||p.championId, pos:''}));
    myPos = cs.myPosition; myChamp = cs.myChampion; myCell = cs.localPlayerCellId;
  } else {
    for(let i=0;i<5;i++){ mine.push({champ:u.over['a'+i]||0,pos:'',me:i===0}); theirs.push({champ:u.over['e'+i]||0}); }
  }
  const me = mine.find(x=>x.me); if(me && u.over['a'+mine.indexOf(me)]) myChamp = me.champ;
  const posMap = {top:'TOP',jungle:'JUNGLE',middle:'MIDDLE',bottom:'BOTTOM',utility:'UTILITY'};
  return {cs, mine, theirs, myPos: u.spos || posMap[(myPos||'').toLowerCase()] || (S.profile?.roles?.main) || 'MIDDLE', myChamp, mode: queueMode(c?.queueId)};
}
function queueMode(q){ return {420:'ranked',440:'flex',400:'normal',430:'normal',490:'normal',450:'aram',2400:'mayhem',1700:'arena',1710:'arena',900:'urf',1900:'urf'}[q] || S.ui.smode || 'ranked'; }
function updateSelect(force){
  if(!$('#selpage')) return;
  const s = selState(), u = S.ui;
  const sig = JSON.stringify([s.mine,s.theirs,s.myPos,s.mode,u.skind,u.sopp,s.cs?.bans,s.myChamp]);
  const c = S.st?.client;
  $('#selphase').textContent = s.cs ? (s.cs.timer?.phase==='FINALIZATION'?'Finalizzazione':'In corso')+(s.cs.timer?.adjustedTimeLeftInPhase>0?` · ${Math.ceil(s.cs.timer.adjustedTimeLeftInPhase/1000)}s`:'') : 'Modalità manuale';
  $('#impmsg').textContent = c?.imported||'';
  if(!force && sig===u.ssig) return;
  u.ssig = sig;
  const pick = (p,i,side) => `<div class="pick ${p.me?'me':''}" data-act="override" data-k="${side}${i}" title="Clicca per correggere il campione">
      <div class="p">${p.champ?`<img src="${cImg(p.champ)}">`:'?'}</div><div class="pl">${p.champ?esc(champ(p.champ).name):(side==='a'?(POS[p.pos]||'Alleato'):'Nemico')}</div></div>`;
  $('#teams').innerHTML = `<div class="team">${s.mine.map((p,i)=>pick(p,i,'a')).join('')}</div><div class="vs">VS</div><div class="team enemy">${s.theirs.map((p,i)=>pick(p,i,'e')).join('')}</div>`;
  const bans = [...(s.cs?.bans?.myTeamBans||[]),...(s.cs?.bans?.theirTeamBans||[])].filter(Boolean);
  $('#bans').innerHTML = s.cs ? `Ban: <span class="bans">${bans.map(b=>`<img src="${cImg(b)}">`).join('')||'<span class="dim">nessuno</span>'}</span><span class="sp"></span><span class="dim">Clicca un ritratto per correggerlo se è rilevato male</span>`
    : `<span>Il client non è in champion select: puoi inserire i campioni a mano cliccando sui ritratti.</span>`;
  const enemies = s.theirs.map(x=>x.champ).filter(Boolean);
  if(u.sopp && !enemies.includes(u.sopp) && !u.soppManual) u.sopp = 0;
  $('#sfilters').innerHTML = `${!s.cs?`<select data-ui="smode">${opt(MODES,s.mode)}</select>`:''}
    ${isPos(s.mode)?`<select data-ui="spos">${opt(['TOP','JUNGLE','MIDDLE','BOTTOM','UTILITY'].map(p=>[p,POS[p]]), s.myPos)}</select>`:''}
    ${u.skind==='matchup'?`<select data-ui="sopp" data-num="1"><option value="0">Avversario di corsia…</option>${enemies.map(e=>`<option value="${e}" ${u.sopp===e?'selected':''}>${esc(champ(e).name)}</option>`).join('')}${u.sopp&&!enemies.includes(u.sopp)?`<option value="${u.sopp}" selected>${esc(champ(u.sopp).name)}</option>`:''}</select>
      <button class="btn ghost sm" data-act="pickopp">Scegli avversario</button>`:''}`;
  loadSuggestions(s);
  // build del campione scelto
  const bk = `${s.myChamp}|${s.myPos}|${s.mode}`;
  if(bk!==u.sbuildKey){ u.sbuildKey=bk; loadSelBuild(s); }
}
async function loadSuggestions(s){
  const u = S.ui, el = $('#sugg');
  const allies = s.mine.filter(x=>!x.me).map(x=>x.champ).filter(Boolean), enemies = s.theirs.map(x=>x.champ).filter(Boolean);
  const bans = [...(s.cs?.bans?.myTeamBans||[]),...(s.cs?.bans?.theirTeamBans||[])].filter(Boolean);
  if(u.skind==='matchup' && !u.sopp){ el.innerHTML = `<div class="empty small">Seleziona il tuo avversario di corsia per vedere i campioni migliori contro di lui.<br><span class="dim">In champion select i ruoli nemici non sono visibili: sceglilo dall'elenco o con "Scegli avversario".</span></div>`; return; }
  el.innerHTML = `<div class="loading"><span class="spinner"></span></div>`;
  let list = [];
  try{ list = await api('/api/suggest', {mode:s.mode, tier:'auto', position:s.myPos, kind:u.skind, allies, enemies, opponent:u.sopp||0, exclude:bans}); }catch(e){}
  if(!$('#sugg')) return;
  if(!list?.length){ el.innerHTML = `<div class="empty small">${S.st?.engine?.total? 'Non ci sono ancora abbastanza partite per questo filtro.' : 'I suggerimenti si basano sul motore statistiche: aggiungi la chiave Riot API nelle impostazioni.'}</div>`; return; }
  el.innerHTML = `<div class="sugg">${list.map(x=>`<div class="sg" data-act="openchamp" data-id="${champ(x.champion).id}">${ci(x.champion)}<div class="sp"><b>${esc(champ(x.champion).name)}</b><div class="tiny muted">${esc(x.detail)}</div></div>
    <span class="sc ${x.score>=.5?'good':'bad'}">${pct(x.score,1)}</span></div>`).join('')}</div>`;
}
async function loadSelBuild(s){
  const el = $('#selbuild'); if(!el) return;
  if(!s.myChamp){ el.innerHTML = `<div class="empty small">Quando scegli (o passi sopra a) un campione, qui compare la build con rune, incantesimi e oggetti pronti da importare.</div>${autoBox()}`; return; }
  el.innerHTML = `<div class="loading"><span class="spinner"></span></div>`;
  let d; try{ d = await api(`/api/champion?id=${s.myChamp}&mode=${s.mode}&tier=auto&pos=${isPos(s.mode)?s.myPos:''}`); }catch(e){ el.innerHTML = esc(e.message); return; }
  if(!$('#selbuild')) return;
  const b = d.build; S.ui.selBuild = {champ:s.myChamp, pos:b.position||s.myPos, mode:s.mode};
  el.innerHTML = `<div class="row mb">${ci(s.myChamp,'l')}<div class="sp"><h2>${esc(champ(s.myChamp).name)}</h2><div class="small muted">${b.games?`${pct(b.winRate)} WR · ${num(b.games)} partite · ${POS[b.position]||modeName(s.mode)}`:'Build personale / consigliata da Riot'}</div></div>
      <button class="btn" data-act="selimport" ${S.st?.client?.connected?'':'disabled'}>Importa tutto</button></div>
    ${runesHTML(b.runes)}<div class="mt">${buildHTML(b, null)}</div>${autoBox()}`;
}
function autoBox(){
  const s = S.settings;
  return `<div class="card flat mt"><div class="kicker">Import automatico quando blocchi il campione</div>
    <div class="setting"><div class="lbl"><b>Rune</b></div>${sw('autoRunes',s.autoRunes)}</div>
    <div class="setting"><div class="lbl"><b>Incantesimi</b><small>Flash su ${s.flashKey}</small></div>${sw('autoSpells',s.autoSpells)}</div>
    <div class="setting"><div class="lbl"><b>Oggetti (5 - 10 - 10)</b></div>${sw('autoItems',s.autoItems)}</div></div>`;
}

/* ================= PATCH ================= */
async function pagePatch(m, r, force){
  const u = S.ui; u.pf = u.pf||'all'; u.pq = u.pq||'';
  m.innerHTML = `<div class="page"><div id="phero"><div class="loading"><span class="spinner"></span>Carico le note della patch…</div></div><div id="pbody2"></div></div>`;
  let d; try{ d = await api('/api/patch'+(force?'?force=1':'')); }catch(e){ d={error:e.message}; }
  if(route()[0]!=='patch') return;
  const rep = d.report;
  if(!rep){ $('#phero').innerHTML = `<div class="card"><div class="empty"><div class="big">📰</div>Note della patch non ancora disponibili.${d.error?`<div class="small dim mt">${esc(d.error)}</div>`:''}<div class="mt"><button class="btn" data-act="patchrefresh">Riprova</button></div></div></div>`; return; }
  u.patch = rep;
  const all = [...(rep.champions||[]),...(rep.items||[]),...(rep.other||[])];
  const cnt = v => all.filter(e=>e.verdict===v).length;
  $('#phero').innerHTML = `<div class="patchhero"><div class="bg" style="background-image:url('${esc(rep.image||'')}')"></div><div class="in"><div class="row">
      <div class="sp"><div class="kicker" style="color:#c6cbe6">${rep.source==='official'?'Note ufficiali Riot':'Modifiche rilevate dai dati di gioco'}</div><h1 style="font-size:32px">Patch ${esc(rep.patch)}</h1>
      <div class="row mt wrap"><span class="tag buff">${cnt('buff')} buff</span><span class="tag nerf">${cnt('nerf')} nerf</span><span class="tag modifica">${cnt('modifica')} modifiche</span>${cnt('nuovo')?`<span class="tag nuovo">${cnt('nuovo')} novità</span>`:''}</div></div>
      <div class="col" style="align-items:flex-end">${rep.url?`<a class="btn ghost sm" href="${esc(rep.url)}" target="_blank">Pagina ufficiale ↗</a>`:''}<button class="btn ghost sm" data-act="patchrefresh">↻ Aggiorna</button>
      <span class="tiny" style="color:#c6cbe6">aggiornato ${ago(rep.fetched*1000)}</span></div></div></div></div>
    <div class="filters mt"><div class="chips">${[['all','Tutto'],['buff','Buff'],['nerf','Nerf'],['modifica','Modifiche']].map(([k,l])=>`<span class="chip ${u.pf===k?'on':''}" data-act="pf" data-k="${k}">${l}</span>`).join('')}</div>
      <input type="search" id="pq" placeholder="Cerca campione o oggetto…" value="${esc(u.pq)}" style="width:260px"><span class="sp"></span>
      <label class="row small muted">Mostra i motivi ${sw('_why', u.why)}</label></div>`;
  $('#pq').addEventListener('input', e=>{ u.pq=e.target.value; drawPatch(); });
  drawPatch();
}
function drawPatch(){
  const u = S.ui, rep = u.patch, el = $('#pbody2'); if(!el||!rep) return;
  const q = u.pq.trim().toLowerCase();
  const f = e => (u.pf==='all'||e.verdict===u.pf) && (!q || e.name.toLowerCase().includes(q));
  const card = e => {
    const icon = e.kind==='champion'&&e.key? `<img class="ic" src="${cImg(e.key)}">` : e.kind==='item'&&e.key? `<img class="ic" src="${iImg(e.key)}">` : '';
    return `<div class="pc"><div class="hd">${icon}<b class="sp">${esc(e.name)}</b><span class="tag ${e.verdict}">${e.verdict}</span></div>
      ${e.summary?`<div class="small muted">${esc(e.summary)}</div>`:''}
      ${(e.groups||[]).map(g=>`${g.title?`<div class="gt">${esc(g.title)}</div>`:''}${g.changes.map(c=>c.new? `<div class="chg ${c.good>0?'g':c.good<0?'b':''}"><span class="l">${esc(c.label)}</span><span class="o">${esc(c.old)}</span><span class="dim">→</span><span class="n">${esc(c.new)}</span></div>`
        : `<div class="chg"><span class="l">• ${esc(c.label)}</span></div>`).join('')}`).join('')}
      ${u.why&&e.context?`<div class="why">${esc(e.context)}</div>`:''}</div>`; };
  const sec = (t, list) => { const l = (list||[]).filter(f); return l.length? `<div class="row mt2 mb"><h2>${t}</h2><span class="badge">${l.length}</span></div><div class="pgrid">${l.map(card).join('')}</div>` : ''; };
  el.innerHTML = sec('Campioni', rep.champions) + sec('Oggetti', rep.items) + sec('Altre modifiche', rep.other) || `<div class="card mt"><div class="empty">Nessun risultato.</div></div>`;
}

/* ================= OVERLAY ================= */
const WIDGETS = [['cs','Contatore CS in tempo reale','CS e CS al minuto, aggiornati ogni secondo'],['objectives','Timer obiettivi','Drago, Larve, Araldo, Atakhan, Barone (dagli eventi di gioco)'],
  ['camps','Timer campi della giungla','Blu, Gromp, Lupi, Pappagalli, Rosso, Sassi (in ARAM: le vite)'],['buffs','Promemoria buff','Scadenza buff del Barone e del Drago Antico'],
  ['deaths','Timer di respawn','Quanto manca al respawn dei giocatori morti'],['itemDiff','Differenza valore oggetti','Tieni premuto TAB: squadra e avversario diretto'],['loading','Statistiche squadre','Rango e win rate di tutti i giocatori (caricamento e Ctrl+T)']];
const TIMERS = [['dragonFirst','Primo drago'],['dragonRespawn','Respawn drago'],['elderRespawn','Respawn Drago Antico'],['grubsFirst','Larve del Vuoto'],['heraldFirst','Araldo'],['atakhanFirst','Atakhan (0 = disattivo)'],
  ['baronFirst','Primo Barone'],['baronRespawn','Respawn Barone'],['campFirst','Primo spawn campi'],['buffFirst','Primo spawn Blu/Rosso'],['campRespawn','Respawn campi piccoli'],['buffRespawn','Respawn Blu/Rosso'],
  ['baronBuff','Durata buff Barone'],['elderBuff','Durata buff Antico'],['relicFirst','ARAM: prime vite'],['relicRespawn','ARAM: respawn vite']];
function pageOverlay(m){
  const o = S.settings.overlay;
  m.innerHTML = `<div class="page"><div class="row mb"><h1>Overlay in-game</h1><span class="sp"></span>${sw('overlay.enabled',o.enabled)}<b>Attivo</b></div>
    <div class="card flat mb small muted">ℹ️ Imposta League in modalità <b>Senza bordi</b> (Impostazioni di gioco → Video → Modalità finestra): in "Schermo intero" Windows non permette a nessun overlay di comparire sopra il gioco.
      L'overlay resta visibile anche quando fai alt-tab e torna da solo a ogni nuova partita.</div>
    <div class="grid2" style="grid-template-columns:1fr 1.15fr">
      <div class="col gap16">
        <div class="card"><h2>Elementi</h2>${WIDGETS.map(([k,n,d])=>`<div class="wrow"><div><b>${n}</b><div class="small muted">${d}</div></div>${sw('overlay.'+k+'.on',o[k].on)}
          <div class="pos"><span>X</span><input type="range" min="0" max="1" step="0.005" value="${o[k].x}" data-setnum="overlay.${k}.x"><span>Y</span><input type="range" min="0" max="1" step="0.005" value="${o[k].y}" data-setnum="overlay.${k}.y"></div></div>`).join('')}
          <div class="setting"><div class="lbl"><b>Dimensione</b></div><input type="range" min="0.7" max="1.6" step="0.05" value="${o.scale}" data-setnum="overlay.scale" style="width:160px"></div></div>
        <div class="card"><div class="ctitle"><h2>Timer sulla minimappa</h2><span class="sp"></span>${sw('overlay.minimap',o.minimap)}</div>
          <p class="small muted">I timer compaiono direttamente sopra i campi. Regola la dimensione finché le etichette non combaciano con la tua minimappa.</p>
          <div class="setting"><div class="lbl"><b>Lato minimappa</b><small>${o.minimapSize}px</small></div><input type="range" min="160" max="520" step="2" value="${o.minimapSize}" data-setnum="overlay.minimapSize" style="width:180px"></div>
          <div class="setting"><div class="lbl"><b>Distanza dal bordo destro</b><small>${o.minimapRight}px</small></div><input type="range" min="0" max="80" step="1" value="${o.minimapRight}" data-setnum="overlay.minimapRight" style="width:180px"></div>
          <div class="setting"><div class="lbl"><b>Distanza dal bordo inferiore</b><small>${o.minimapBottom}px</small></div><input type="range" min="0" max="80" step="1" value="${o.minimapBottom}" data-setnum="overlay.minimapBottom" style="width:180px"></div></div>
        <div class="card"><h2>Scorciatoie (solo durante la partita)</h2><div class="hk">
          <span><span class="kbd">Ctrl</span>+<span class="kbd">Shift</span>+<span class="kbd">1…6</span></span><span>Campo del tuo lato ripulito: 1 Blu · 2 Gromp · 3 Lupi · 4 Pappagalli · 5 Rosso · 6 Sassi (premi di nuovo per annullare)</span>
          <span><span class="kbd">Ctrl</span>+<span class="kbd">Alt</span>+<span class="kbd">1…6</span></span><span>Stesso campo, ma del lato nemico</span>
          <span><span class="kbd">Ctrl</span>+<span class="kbd">Shift</span>+<span class="kbd">1…4</span></span><span>In ARAM: vita raccolta (avvia il timer di respawn)</span>
          <span><span class="kbd">TAB</span> (tenuto)</span><span>Differenza valore oggetti</span>
          <span><span class="kbd">Ctrl</span>+<span class="kbd">T</span></span><span>Mostra/nascondi statistiche delle squadre</span>
          <span><span class="kbd">Ctrl</span>+<span class="kbd">Shift</span>+<span class="kbd">O</span></span><span>Nascondi/mostra tutto l'overlay (si riattiva da solo alla partita successiva)</span></div></div>
        <div class="card"><div class="ctitle"><h2>Durate dei timer</h2><span class="sp"></span><button class="btn ghost sm" data-act="resetoverlay">Ripristina predefiniti</button></div>
          <p class="small muted">In secondi. Riot cambia spesso questi tempi tra una stagione e l'altra: se non tornano, correggili qui.</p>
          <div class="grid2" style="gap:8px 16px">${TIMERS.map(([k,l])=>`<label class="row small"><span class="sp muted">${l}</span><input type="number" min="0" step="5" value="${o.timers[k]}" data-setnum="overlay.timers.${k}" style="width:80px"></label>`).join('')}</div></div>
      </div>
      <div><div class="card" style="position:sticky;top:0"><div class="ctitle"><h2>Anteprima</h2><span class="sp"></span><span class="small muted" id="ovstate"></span></div>
        <div class="screen" id="screen"></div><p class="small muted mt">Anteprima con dati di esempio quando non sei in partita. La differenza oggetti appare in gioco solo tenendo premuto TAB.</p></div></div>
    </div></div>`;
  drawOverlayPreview();
  clearInterval(S.ovTimer); S.ovTimer = setInterval(()=>{ if(route()[0]!=='overlay'){clearInterval(S.ovTimer);return;} drawOverlayPreview(); }, 1000);
}
async function drawOverlayPreview(){
  let md; try{ md = await api('/api/overlay/model?preview=1'); }catch(e){ return; }
  const el = $('#screen'); if(!el) return;
  $('#ovstate').textContent = S.st?.inGame ? '● Partita in corso' : 'Dati di esempio';
  const W = el.clientWidth, H = el.clientHeight, k = H/1080*(S.settings.overlay.scale||1), o = S.settings.overlay;
  const mmS = o.minimapSize*H/1080, mmR = o.minimapRight*H/1080, mmB = o.minimapBottom*H/1080;
  let h = `<div class="mm" style="width:${mmS}px;height:${mmS}px;right:${mmR}px;bottom:${mmB}px"></div>`;
  for(const w of md.widgets){
    if(w.pill){ h += `<div class="ow pill" style="--acc:${w.accent};left:${W - w.x*H/1080}px;top:${H - w.y*H/1080}px;color:${w.lines[0].color}">${esc(w.lines[0].text)}</div>`; continue; }
    const ww = w.width*k, pad = 10*k, fs = Math.max(5, 13*k);
    let x = w.x*W - (w.anchor==='center'?ww/2:0);
    h += `<div class="ow" style="--acc:${w.accent};left:${Math.max(0,x)}px;top:${w.y*H}px;width:${ww}px;padding:${pad}px ${12*k}px ${pad}px ${16*k}px;font-size:${fs}px">
      ${w.title?`<div class="t" style="height:${18*k}px;font-size:${Math.max(4.5,11*k)}px">${esc(w.title)}</div>`:''}${w.lines.map(l=>`<div class="ln" style="height:${(l.size>=16?28:21)*k}px;color:${l.color};font-weight:${l.bold?800:500}${l.size>=16?';font-size:'+Math.max(6,l.size*k)+'px':''}"><span>${esc(l.text)}</span><span>${esc(l.right||'')}</span></div>`).join('')}</div>`;
  }
  el.innerHTML = h;
}

/* ================= STREAMING ================= */
function pageStreaming(m){
  const s = S.settings.stream;
  const url = location.origin + '/obs';
  const drag = `${url}?layer-name=BetterStomp&layer-width=1920&layer-height=1080`;
  m.innerHTML = `<div class="page"><h1 class="mb">Overlay per lo streaming</h1>
    <div class="grid2" style="grid-template-columns:1fr 1.1fr">
      <div class="col gap16">
        <div class="card"><h2>Aggiungilo a OBS</h2>
          <p class="small muted">Trascina questo pulsante dentro la scena di OBS: crea da solo una fonte <b>Browser</b> già impostata a <b>1920×1080</b>.</p>
          <a class="btn" href="${drag}" draggable="true" onclick="event.preventDefault()">⠿ Trascinami in OBS</a>
          <p class="small muted mt">In alternativa: Fonti → + → Browser, incolla l'indirizzo e imposta larghezza 1920 e altezza 1080.</p>
          <div class="row"><input type="text" readonly value="${url}" style="flex:1" id="obsurl"><button class="btn ghost" data-act="copyobs">Copia</button></div>
          <p class="tiny dim mt">BetterStomp deve restare aperto mentre sei in live.</p></div>
        <div class="card"><h2>Obiettivo della stagione</h2>
          <div class="row"><select data-set="stream.goalTier" style="flex:1">${opt(TIERS.map(t=>[t,TIER_IT[t]]),s.goalTier)}</select>
          ${['MASTER','GRANDMASTER','CHALLENGER'].includes(s.goalTier)?'':`<select data-set="stream.goalDivision">${opt(DIVS.map(d=>[d,d]),s.goalDivision)}</select>`}</div>
          <div class="setting mt"><div class="lbl"><b>Mostra obiettivo e progresso</b></div>${sw('stream.showGoal',s.showGoal)}</div>
          <div class="setting"><div class="lbl"><b>Vittorie/sconfitte di oggi</b></div>${sw('stream.showSession',s.showSession)}</div>
          <div class="setting"><div class="lbl"><b>Win rate stagionale</b></div>${sw('stream.showWinrate',s.showWinrate)}</div>
          <div class="setting"><div class="lbl"><b>Coda</b></div><select data-set="stream.queue">${opt([['RANKED_SOLO_5x5','Solo/Duo'],['RANKED_FLEX_SR','Flex']],s.queue)}</select></div>
          <div class="setting"><div class="lbl"><b>Posizione</b></div><select data-set="stream.corner">${opt([['top-left','In alto a sinistra'],['top-right','In alto a destra'],['bottom-left','In basso a sinistra'],['bottom-right','In basso a destra']],s.corner)}</select></div>
          <div class="setting"><div class="lbl"><b>Stile</b></div><select data-set="stream.style">${opt([['glass','Vetro'],['solid','Pieno'],['minimal','Minimal']],s.style)}</select></div></div>
      </div>
      <div class="card" style="align-self:start"><h2>Anteprima 1920×1080</h2><div id="obsprev" style="position:relative;width:100%;aspect-ratio:16/9;border-radius:14px;overflow:hidden;background:linear-gradient(135deg,#2a3350,#151a2e)">
        <iframe src="/obs?preview=1" style="position:absolute;top:0;left:0;width:1920px;height:1080px;border:0;transform-origin:0 0;background:transparent" id="obsframe"></iframe></div></div>
    </div></div>`;
  const fit = () => { const p=$('#obsprev'), f=$('#obsframe'); if(p&&f) f.style.transform=`scale(${p.clientWidth/1920})`; };
  fit(); window.addEventListener('resize', fit);
}

/* ================= PREMIUM ================= */
function pagePremium(m){
  m.innerHTML = `<div class="page"><div class="premhero"><div class="kicker" style="color:var(--gold)">BetterStomp</div><h1>✦ Premium</h1>
      <p class="muted" style="max-width:560px">Tutte le funzioni principali di BetterStomp sono e resteranno gratuite. Premium aggiunge il coach personale e toglie la pubblicità.</p>
      <div class="row mt2"><button class="coach" style="padding:12px 22px;font-size:14px" data-act="coach"><span class="sparkle">✦</span> Passa a Premium</button><span class="muted small">In arrivo</span></div></div>
    <div class="grid2 mt2">
      <div class="card"><div class="feat"><div class="fi">🧠</div><div><h2>Coach AI</h2><p class="muted">Un coach che ti spiega la corsia prima di iniziare e come giocare la partita al meglio in base ai campioni in campo. Dopo ogni game, il pulsante dorato accanto ad "Analizza" ti dà una revisione approfondita.</p></div></div></div>
      <div class="card"><div class="feat"><div class="fi">🚫</div><div><h2>Niente pubblicità</h2><p class="muted">Rimuove tutti gli spazi pubblicitari dall'app per un'interfaccia più pulita e più spazio per i contenuti.</p></div></div></div>
    </div>
    <div class="card flat mt2"><div class="kicker mb">Sempre gratis</div><div class="row wrap gap6">${['Statistiche e tier list','Build per rango','Champion select','Import automatico','Analisi delle partite','Overlay in-game','Overlay streaming','Note della patch','Profilo e premade'].map(x=>`<span class="badge">✓ ${x}</span>`).join('')}</div></div></div>`;
}

/* ================= IMPOSTAZIONI ================= */
function pageSettings(m){
  const s = S.settings;
  m.innerHTML = `<div class="page"><h1 class="mb">Impostazioni</h1><div class="grid2">
    <div class="col gap16">
      <div class="card"><h2>Aspetto</h2>
        <div class="setting"><div class="lbl"><b>Tema</b></div><div class="seg"><button data-act="theme" data-k="dark" class="${s.theme!=='light'?'on':''}">Scuro</button><button data-act="theme" data-k="light" class="${s.theme==='light'?'on':''}">Chiaro</button></div></div>
        <div class="setting"><div class="lbl"><b>Animazioni</b></div>${sw('animations',s.animations!==false)}</div></div>
      <div class="card"><h2>Automatismi in champion select</h2>
        <div class="setting"><div class="lbl"><b>Importa rune</b><small>Crea una pagina "BetterStomp" quando blocchi il campione</small></div>${sw('autoRunes',s.autoRunes)}</div>
        <div class="setting"><div class="lbl"><b>Importa incantesimi dell'evocatore</b></div>${sw('autoSpells',s.autoSpells)}</div>
        <div class="setting"><div class="lbl"><b>Importa set di oggetti</b><small>Blocchi da 5 - 10 - 10 slot</small></div>${sw('autoItems',s.autoItems)}</div>
        <div class="setting"><div class="lbl"><b>Posizione del Flash</b></div><div class="seg"><button data-act="flash" data-k="D" class="${s.flashKey==='D'?'on':''}">D</button><button data-act="flash" data-k="F" class="${s.flashKey==='F'?'on':''}">F</button></div></div>
        <div class="setting"><div class="lbl"><b>Preferisci le build personali</b><small>Se hai salvato una build per un campione, usa quella</small></div>${sw('preferPersonal',s.preferPersonal)}</div></div>
      <div class="card"><h2>Preferenze</h2>
        <div class="setting"><div class="lbl"><b>Modalità predefinita</b></div><select data-set="defaultMode">${opt(MODES,s.defaultMode)}</select></div>
        <div class="setting"><div class="lbl"><b>Rango predefinito per le statistiche</b><small>"Il mio rango" usa quello del tuo account</small></div><select data-set="defaultTier">${tierOptions(s.defaultTier)}</select></div></div>
    </div>
    <div class="col gap16">
      <div class="card"><h2>Riot API e statistiche</h2>
        <div class="setting"><div class="lbl"><b>Regione</b></div><select data-set="region">${opt(REGIONS,s.region)}</select></div>
        <div class="setting" style="flex-wrap:wrap"><div class="lbl"><b>Chiave Riot API</b><small>Gratuita su <a href="https://developer.riotgames.com" target="_blank">developer.riotgames.com</a>. La chiave Development scade ogni 24 ore.</small></div>
          <div class="row" style="width:100%"><input type="password" id="apikey" placeholder="RGAPI-…" value="${esc(s.riotApiKey||'')}" style="flex:1"><button class="btn ghost sm" data-act="showkey">👁</button><button class="btn sm" data-act="savekey">Salva</button></div></div>
        <div class="setting"><div class="lbl"><b>Motore statistiche</b><small>Analizza partite pubbliche in background per tier list, win rate, build e matchup</small></div>${sw('engineEnabled',s.engineEnabled)}</div>
        <div id="enginebox"></div></div>
      <div class="card"><h2>Client di League</h2>
        <div class="setting" style="flex-wrap:wrap"><div class="lbl"><b>Cartella di installazione</b><small>Lascia vuoto per il rilevamento automatico</small></div>
          <input type="text" data-set="leaguePath" placeholder="C:\\Riot Games\\League of Legends" value="${esc(s.leaguePath||'')}" style="width:100%"></div></div>
      <div class="card flat"><h2>Sviluppo</h2><div class="setting"><div class="lbl"><b>Simula Premium</b><small>Nasconde gli spazi pubblicitari per vedere l'app senza annunci</small></div>${sw('premium',s.premium)}</div>
        <p class="tiny dim">BetterStomp isn't endorsed by Riot Games and doesn't reflect the views or opinions of Riot Games or anyone officially involved in producing or managing Riot Games properties.</p></div>
    </div></div></div>`;
  updateEngineBox();
}
function updateEngineBox(){
  const el = $('#enginebox'), e = S.st?.engine; if(!el||!e) return;
  const ks = {ok:'<span class="good">valida</span>',invalid:'<span class="bad">non valida o scaduta</span>'}[e.keyState]||'<span class="dim">non verificata</span>';
  const h = `<div class="stats mt"><div class="stat"><div class="k">Partite (patch ${esc(e.patch||'-')})</div><div class="v">${num(e.total)}</div></div>
    <div class="stat"><div class="k">Velocità</div><div class="v">${e.perHour?num(e.perHour)+'/h':'—'}</div></div><div class="stat"><div class="k">Chiave</div><div class="v" style="font-size:14px">${e.hasKey?ks:'<span class="dim">assente</span>'}</div></div></div>
    ${e.error?`<div class="small bad mt">${esc(e.error)}</div>`:''}`;
  if(el._h!==h){ el.innerHTML=h; el._h=h; }
}

/* ================= MODALI ================= */
function modal(title, inner, onReady){
  const d = document.createElement('div'); d.className='modal';
  d.innerHTML = `<div class="box"><div class="row"><h2 class="sp">${title}</h2><button class="iconbtn" data-close>✕</button></div><div class="body">${inner}</div></div>`;
  d.addEventListener('click', e=>{ if(e.target===d || e.target.closest('[data-close]')) d.remove(); });
  document.body.appendChild(d); onReady?.(d); return d;
}
function champPicker(cb){
  const grid = q => S.champList.filter(c=>!q||c.name.toLowerCase().includes(q)).map(c=>`<div class="pitem" data-k="${c.key}"><img src="${DD}${S.v}/img/champion/${c.id}.png" loading="lazy">${esc(c.name)}</div>`).join('');
  modal('Scegli campione', `<input type="search" id="mq" placeholder="Cerca…" style="width:100%"><div class="pgrid2 mt" id="mg"><div class="pitem" data-k="0"><span class="ic l" style="display:block;margin:0 auto 4px"></span>Nessuno</div>${grid('')}</div>`, d=>{
    const q = d.querySelector('#mq'); q.focus();
    q.addEventListener('input', ()=>d.querySelector('#mg').innerHTML = grid(q.value.toLowerCase()));
    d.querySelector('#mg').addEventListener('click', e=>{ const it=e.target.closest('.pitem'); if(it){ cb(+it.dataset.k); d.remove(); } });
  });
}
function itemPicker(cb){
  const list = Object.entries(S.items).filter(([id,x])=>x.maps?.['11'] && x.gold?.purchasable && x.inStore!==false && !x.requiredChampion && !x.hideFromAll && (x.gold.total>0||x.tags?.includes('Consumable')))
    .sort((a,b)=>b[1].gold.total-a[1].gold.total);
  const grid = q => list.filter(([id,x])=>!q||x.name.toLowerCase().includes(q)).map(([id,x])=>`<div class="pitem" data-id="${id}"><img src="${iImg(id)}" loading="lazy">${esc(x.name)}<div class="dim">${x.gold.total}</div></div>`).join('');
  modal('Scegli oggetto', `<input type="search" id="mq" placeholder="Cerca oggetto…" style="width:100%"><div class="pgrid2 mt" id="mg">${grid('')}</div>`, d=>{
    const q = d.querySelector('#mq'); q.focus();
    q.addEventListener('input', ()=>d.querySelector('#mg').innerHTML = grid(q.value.toLowerCase()));
    d.querySelector('#mg').addEventListener('click', e=>{ const it=e.target.closest('.pitem'); if(it){ cb(+it.dataset.id); d.remove(); } });
  });
}

/* ================= AZIONI ================= */
const actions = {
  search(){ const v=$('#sid').value.trim(); if(!v) return; if(!v.includes('#')){ toast('Scrivi il Riot ID completo, es. Nome#EUW'); return; } S.ui.search=v; S.ui.searchRegion=$('#sreg').value; S.ui.ptab='partite'; render(); },
  myprofile(){ S.ui.search=''; render(); },
  lookup(t){ S.ui.search=t.dataset.id; S.ui.ptab='partite'; location.hash='#/profilo'; render(); },
  refreshprof(){ S.ui.forceProf=true; S.profile=null; render(); },
  ptab(t){ S.ui.ptab=t.dataset.k; document.querySelectorAll('#ptabs button').forEach(b=>b.classList.toggle('on',b===t)); $('#ptab').innerHTML=profileTab(S.ui.prof); },
  analyze(t){
    const i=+t.dataset.i, el=$('#an'+i), g=S.ui.prof.matches[i];
    if(el.style.display!=='none'){ el.style.display='none'; return; }
    el.innerHTML = (g.analysis||[]).map(a=>`<div class="ins ${a.kind}"><span class="i">${a.kind==='good'?'✓':a.kind==='bad'?'!':'i'}</span><span>${esc(a.text)}</span></div>`).join('');
    el.style.display='block';
  },
  coach(){ modal('✦ Coach AI', `<div class="feat"><div class="fi">🧠</div><div><p>Il Coach AI fa parte di <b class="goldc">BetterStomp Premium</b>, in arrivo a breve.</p>
    <p class="muted">Ti spiegherà la corsia prima della partita e rivedrà i tuoi game in dettaglio. Nel frattempo "Analizza" resta gratuito per ogni partita.</p><a class="btn" href="#/premium" data-close>Scopri Premium</a></div></div>`); },
  openchamp(t){ if(!t.dataset.id) return; if(t.dataset.pos && t.dataset.pos!=='ALL'){ S.ui.chPos=t.dataset.pos; S.ui.chId=t.dataset.id; }
    if(route()[0]==='tier'){ S.ui.chMode=S.ui.tmode; S.ui.chTier=S.ui.ttier; } if(route()[0]==='campioni'){ S.ui.chMode=S.ui.cmode; S.ui.chTier=S.ui.ctier; }
    location.hash='#/campione/'+t.dataset.id; },
  cpos(t){ S.ui.cpos=t.dataset.p; document.querySelectorAll('[data-act=cpos]').forEach(c=>c.classList.toggle('on',c===t)); drawChampGrid(); },
  chpos(t){ S.ui.chPos=t.dataset.p; render(); },
  tpos(t){ S.ui.tpos=t.dataset.p; render(); },
  tsort(t){ const k=t.dataset.k; if(S.ui.tsort===k) S.ui.tdir=-(S.ui.tdir||-1); else { S.ui.tsort=k; S.ui.tdir=k==='name'?1:-1; } drawTier(); },
  async import(){ const d=S.ui.chData; try{ const r=await api('/api/import',{champ:S.ui.chKey,pos:d.build.position,mode:S.ui.chMode,runes:true,spells:true,items:true}); toast(r.message||'Fatto'); }catch(e){ toast(e.message); } },
  async selimport(){ const b=S.ui.selBuild; if(!b) return; try{ const r=await api('/api/import',{champ:b.champ,pos:b.pos,mode:b.mode,runes:true,spells:true,items:true}); toast(r.message||'Fatto'); }catch(e){ toast(e.message); } },
  editbuild(){ S.ui.editing=!S.ui.editing; S.ui.edit=null; $('#buildbox').innerHTML=buildHTML(S.ui.chData.build, S.ui.chData.hasPersonal?S.ui.chData.personal:null); },
  pickitem(t,e){ if(e.target.closest('.x')) return; itemPicker(id=>{ const b=t.dataset.b, i=+t.dataset.i; const arr=S.ui.edit[b]; arr[i]=id; S.ui.edit[b]=arr.map(x=>x||0); refreshEditor(); }); },
  clearitem(t){ const b=t.dataset.b, i=+t.dataset.i; S.ui.edit[b].splice(i,1); refreshEditor(); },
  async savebuild(){ const e=S.ui.edit; const cur=S.ui.chData.hasPersonal?S.ui.chData.personal:{};
    await api('/api/personal?id='+S.ui.chKey, {...cur, starting:e.starting.filter(Boolean), core:e.core.filter(Boolean), situational:e.situational.filter(Boolean)});
    toast('Build personale salvata'); S.ui.editing=false; render(); },
  async resetbuild(){ await api('/api/personal?id='+S.ui.chKey, {starting:[],core:[],situational:[]}); toast('Build personale eliminata'); S.ui.editing=false; render(); },
  async saverunes(){ const d=S.ui.chData; const cur=d.hasPersonal?d.personal:{}; await api('/api/personal?id='+S.ui.chKey, {...cur, runes:d.build.runes, spells:d.build.spells}); toast('Rune e incantesimi salvati come personali'); },
  skind(t){ S.ui.skind=t.dataset.k; document.querySelectorAll('[data-act=skind]').forEach(b=>b.classList.toggle('on',b===t)); S.ui.ssig=''; updateSelect(true); },
  override(t){ champPicker(k=>{ S.ui.over[t.dataset.k]=k; S.ui.ssig=''; S.ui.sbuildKey=''; updateSelect(true); }); },
  pickopp(){ champPicker(k=>{ S.ui.sopp=k; S.ui.soppManual=true; S.ui.ssig=''; updateSelect(true); }); },
  pf(t){ S.ui.pf=t.dataset.k; document.querySelectorAll('[data-act=pf]').forEach(c=>c.classList.toggle('on',c===t)); drawPatch(); },
  patchrefresh(){ pagePatch($('#main'), route(), true); },
  async resetoverlay(){ S.settings = await api('/api/settings/reset-overlay'); render(); toast('Overlay ripristinato'); },
  copyobs(){ navigator.clipboard.writeText($('#obsurl').value).then(()=>toast('Indirizzo copiato')); },
  theme(t){ setSetting('theme', t.dataset.k); document.querySelectorAll('[data-act=theme]').forEach(b=>b.classList.toggle('on',b===t)); },
  flash(t){ setSetting('flashKey', t.dataset.k); document.querySelectorAll('[data-act=flash]').forEach(b=>b.classList.toggle('on',b===t)); },
  showkey(){ const i=$('#apikey'); i.type = i.type==='password'?'text':'password'; },
  savekey(){ setSetting('riotApiKey', $('#apikey').value.trim()); toast('Chiave salvata: il motore statistiche parte in pochi secondi'); },
};
function refreshEditor(){ $('#buildbox').innerHTML = buildHTML(S.ui.chData.build, S.ui.chData.personal); }

document.addEventListener('click', e=>{
  const t = e.target.closest('[data-act]'); if(!t) return;
  const f = actions[t.dataset.act]; if(f){ e.preventDefault(); f(t, e); }
});
document.addEventListener('change', e=>{
  const t = e.target;
  if(t.dataset.set){
    if(t.dataset.set==='_why'){ S.ui.why=t.checked; drawPatch(); return; }
    setSetting(t.dataset.set, t.type==='checkbox' ? t.checked : t.value);
    if(t.dataset.set==='stream.goalTier' || t.dataset.set.startsWith('overlay.minimap')) render();
    if(['autoRunes','autoSpells','autoItems'].includes(t.dataset.set)) document.querySelectorAll(`[data-set=${t.dataset.set}]`).forEach(x=>x.checked=t.checked);
  }
  if(t.dataset.ui){ S.ui[t.dataset.ui] = t.dataset.num ? +t.value : t.value; if(t.dataset.ui==='sopp'){ S.ui.soppManual=false; } if(t.dataset.ui==='chMode') S.ui.chPos=''; if(['sopp','spos','smode'].includes(t.dataset.ui)){ S.ui.ssig=''; S.ui.sbuildKey=''; updateSelect(true); } else render(); }
});
document.addEventListener('input', e=>{
  const t = e.target;
  if(t.dataset.setnum){ setSetting(t.dataset.setnum, parseFloat(t.value)); if(route()[0]==='overlay'){ const sm=t.closest('.setting')?.querySelector('small'); if(sm) sm.textContent=t.value+'px'; drawOverlayPreview(); } }
});
$('#themebtn').addEventListener('click', ()=>{ setSetting('theme', S.settings.theme==='light'?'dark':'light'); if(route()[0]==='impostazioni') render(); });

boot();
