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

/* ---------- aggiornamento "morbido": cambia solo ciò che è diverso, senza sfarfallii ---------- */
function morphChildren(a, b){
  const an=[...a.childNodes], bn=[...b.childNodes];
  bn.forEach((y,i)=>{
    const x = an[i];
    if(!x){ a.appendChild(y); return; }
    if(x.nodeType!==y.nodeType || x.nodeName!==y.nodeName){ a.replaceChild(y,x); return; }
    if(x.nodeType!==1){ if(x.nodeValue!==y.nodeValue) x.nodeValue=y.nodeValue; return; }
    for(const at of [...x.attributes]) if(!y.hasAttribute(at.name)) x.removeAttribute(at.name);
    for(const at of [...y.attributes]) if(x.getAttribute(at.name)!==at.value) x.setAttribute(at.name, at.value);
    if(x===document.activeElement && (x.tagName==='INPUT'||x.tagName==='SELECT')) return;
    morphChildren(x,y);
  });
  for(let i=an.length-1;i>=bn.length;i--) a.removeChild(an[i]);
}
function put(el, html, soft){
  if(!el) return;
  if(!soft){ el.innerHTML = html; return; }
  if(el._last === html) return;
  const t = document.createElement('div'); t.innerHTML = html;
  morphChildren(el, t);
  el._last = html;
}

/* ---------- impostazioni (salvate al volo, senza ricaricare) ---------- */
let saveTimer, pending = {};
function setSetting(path, value){
  setPath(S.settings, path, value);
  pending[path] = value;
  applyTheme();
  clearTimeout(saveTimer);
  saveTimer = setTimeout(async ()=>{
    const send = pending; pending = {};
    try{ const ns = await api('/api/settings/patch', send); if(!Object.keys(pending).length) S.settings = ns; }
    catch(e){ toast('Errore salvataggio: '+e.message); }
  }, 200);
}
function applyTheme(){
  const s = S.settings||{};
  document.body.classList.toggle('light', s.theme==='light');
  document.body.classList.add('premium');
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
  window.addEventListener('hashchange', ()=>render());
  render();
  // ogni 5 minuti controlla dati nuovi e aggiorna la pagina senza ricaricarla
  setInterval(()=>softRefresh(), 5*60*1000);
}
function softRefresh(){
  if(document.querySelector('.modal') || S.ui.editing) return;
  render(true);
}

// barra della finestra disegnata dall'app (finestra senza bordi di Windows)
document.addEventListener('mousedown', e=>{
  if(e.button!==0 || !document.body.classList.contains('frameless')) return;
  const z = e.target.closest('[data-rz]'); if(z){ e.preventDefault(); fetch('/api/win?a=size'+z.dataset.rz); return; }
  const b = e.target.closest('[data-win]'); if(b){ e.preventDefault(); fetch('/api/win?a='+b.dataset.win); return; }
  if(e.target.closest('header.top') && !e.target.closest('a,button,input,select,.status,.gamechip') && e.detail===1) fetch('/api/win?a=drag');
});
document.addEventListener('dblclick', e=>{ if(document.body.classList.contains('frameless') && e.target.closest('header.top') && !e.target.closest('a,button,input,select')) fetch('/api/win?a=max'); });
function sendInner(){ fetch(`/api/win?a=inner&h=${innerHeight}&d=${devicePixelRatio}`).catch(()=>{}); }
addEventListener('resize', sendInner); setTimeout(sendInner, 300);
// barra di caricamento dei dati OP.GG all'avvio (solo quando c'è qualcosa da scaricare)
function syncBar(st){
  let el = $('#syncbar');
  const on = st && st.running && st.step!=='check';
  if(!on){ if(el){ el.classList.add('done'); setTimeout(()=>el.remove(), 600); if(!S._syncReload){ S._syncReload=true; render(true); } } return; }
  S._syncReload = false;
  if(!el){ el = document.createElement('div'); el.id='syncbar'; el.innerHTML='<div class="sbtxt"></div><div class="sbtrack"><i></i></div>'; document.body.appendChild(el); }
  const pctAll = st.step==='elo' && st.total ? Math.max(1, Math.round(st.done/st.total*100)) : 0;
  const what = st.step==='elo' ? `Tier list e build · ${esc(st.label||'')}` : 'Controllo il meta delle corsie';
  el.querySelector('.sbtxt').innerHTML = `<b>Aggiorno i dati OP.GG</b> · ${what}<span class="sp"></span><b>${pctAll}%</b>`;
  el.querySelector('.sbtrack i').style.width = pctAll+'%';
}
// nuova versione scaricata dal server: un clic e si riavvia aggiornata
function updBar(u){
  let el = $('#updbar');
  if(!u?.ready){ if(el) el.remove(); return; }
  if(el) return;
  el = document.createElement('div'); el.id='updbar';
  el.innerHTML = `<span>✨ <b>Nuova versione di BetterStomp</b> pronta</span><button class="btn sm">Aggiorna ora</button>`;
  el.querySelector('button').onclick = ()=>{ el.querySelector('button').textContent='Riavvio…'; fetch('/api/win?a=update'); };
  document.body.appendChild(el);
}
async function poll(){
  try{ S.st = await api('/api/state'); }catch(e){ return; }
  document.body.classList.toggle('frameless', !!S.st.frameless);
  if(Date.now()-(S._inSent||0) > 5000){ S._inSent=Date.now(); sendInner(); }
  const c = S.st.client;
  $('#cdot').classList.toggle('on', c.connected);
  const ph = {None:'Client connesso',Lobby:'In lobby',Matchmaking:'In coda',ReadyCheck:'Partita trovata',ChampSelect:'Champion select',InProgress:'In partita',EndOfGame:'Fine partita',WaitingForStats:'Fine partita',PreEndOfGame:'Fine partita'};
  $('#ctext').textContent = c.connected ? (ph[c.phase]||'Client connesso') : (S.st.inGame ? 'In partita' : 'Client non connesso');
  const navSel = document.querySelector('.nav a[data-p=select]');
  navSel.innerHTML = 'Champion Select' + (c.phase==='ChampSelect'?'<span class="live"></span>':'');
  // in champion select l'app si porta da sola sulla schermata di aiuto
  if(c.phase==='ChampSelect' && S.lastPhase!=='ChampSelect' && route()[0]!=='select'){ location.hash='#/select'; }
  if(c.phase!==S.lastPhase && c.phase==='EndOfGame'){ S.ui.postAt=Date.now(); S._profAt=0; }
  S.lastPhase = c.phase;
  if(S.st.profileVer!==undefined && S._pv!==S.st.profileVer){ if(S._pv!==undefined) S._profAt=0; S._pv=S.st.profileVer; } // partita finita o nuovi dati dal server: aggiorno subito
  if(c.connected && S.profile && Date.now()-(S._profAt||0) > ((S.profile.seasonLoading||S.profile.seasonPending)?8000:60000) && !S._profLoading){ S._profLoading=true; api('/api/profile').then(p=>{ if(!p.error){ S.profile=p; S._profAt=Date.now(); renderSide(); if(route()[0]==='profilo'&&!S.ui.search&&!S.ui.openAnAny) render(true); maybePostGame(p); } }).finally(()=>S._profLoading=false); }
  if(c.connected && !S.profile && !S._profLoading){ S._profLoading=true; api('/api/profile').then(p=>{S.profile=p; S._profAt=Date.now(); renderSide(); if(route()[0]==='profilo'&&!S.ui.search) render(true);}).finally(()=>S._profLoading=false); }
  syncBar(S.st.opggSync);
  updBar(S.st.update);
  renderSide();
  if(route()[0]==='select') updateSelect();
  if(route()[0]==='impostazioni') updateEngineBox();
  if(route()[0]==='tier' && S.ui.tierEmpty && S.st.engine.total>0){ S.ui.tierEmpty=false; render(true); }
}

/* ---------- sidebar: riepilogo account (stile Blitz) ---------- */
const TIER_COL = {IRON:'#9a8f8b',BRONZE:'#c8865a',SILVER:'#b4c0cf',GOLD:'#f0c25a',PLATINUM:'#3fd0c9',EMERALD:'#3ee6a0',DIAMOND:'#7aa7ff',MASTER:'#c77dff',GRANDMASTER:'#ff6b81',CHALLENGER:'#ffd76a'};
function rankLine(n, q){
  const ok = q && q.tier && q.tier!=='NONE';
  const g = ok ? q.wins+q.losses : 0, wr = g ? q.wins/g : 0;
  return `<div class="rkline">${ok?`<img src="${CD}${q.tier.toLowerCase()}.png" onerror="this.style.visibility='hidden'">`:'<span class="rkph"></span>'}<span class="muted sp">${n}</span>
    ${ok?`<div class="right"><b style="color:${TIER_COL[q.tier]}">${rankTxt(q)} ${q.leaguePoints} LP</b><div class="tiny"><b class="${wr>=.5?'good':'bad'}">${pct(wr,0)} WR</b> <span class="dim">${q.wins}V - ${q.losses}S</span></div></div>`:'<span class="dim">Non classificato</span>'}</div>`;
}
function recentStrip(p){
  const ms = (p.matches||[]).filter(m=>!m.remake);
  if(!ms.length) return '';
  const mode = ms[0].queue, list = ms.filter(m=>m.queue===mode).slice(0,10);
  const w = list.filter(m=>m.me.win).length, l = list.length-w;
  return `<div class="card side-card"><h3>${esc(qName(mode, ms[0].mode))} <span class="dim small">(ultime ${list.length})</span></h3>
    <div class="row small" style="margin-top:6px"><b class="${w/list.length>=.5?'good':'bad'}">${pct(w/list.length)} WR</b><span class="muted">${w}V - ${l}S</span></div>
    <div class="bar" style="margin:6px 0 10px"><i style="width:${w/list.length*100}%"></i></div>
    <div class="recent">${list.map(m=>`<span class="rc"><img src="${cImg(m.me.champ)}" title="${esc(champ(m.me.champ).name)} · ${m.me.k}/${m.me.d}/${m.me.a}"><i class="${m.me.win?'w':'l'}"></i></span>`).join('')}</div></div>`;
}
function renderSide(){
  const c = S.st?.client; const el = $('#side');
  const more = S.ui.sideMore||{};
  let h = '';
  if(c && c.connected && c.summoner){
    const s = c.summoner, p = S.profile;
    h += `<div class="side-head"><div class="av3"><img src="${profImg(s.profileIconId)}"><span class="lvl">${s.summonerLevel}</span></div>
      <div><div class="name">${esc(s.gameName)} <span class="dim">#${esc(s.tagLine)}</span></div>
      <div class="tiny muted">${p? (p.roles.enough? 'Ruolo: '+POS[p.roles.main]+(p.roles.second?' / '+POS[p.roles.second]:'') : `Ruolo: servono 20 partite (${p.roles.games}/20)`) : 'Carico il profilo…'}</div></div></div>`;
    h += `<div class="card side-card">${rankLine('Solo/Duo', c.ranked?.RANKED_SOLO_5x5)}${rankLine('Flex', c.ranked?.RANKED_FLEX_SR)}
      ${p?`<div class="rkline"><span class="rkph">📅</span><span class="muted sp">Oggi (Solo/Duo)</span><b><span class="good">${p.today.w}V</span> - <span class="bad">${p.today.l}S</span></b></div>`:''}</div>`;
    const champsSrc = p?.season || [];
    if(p && !champsSrc.length){
      h += `<div class="card side-card"><h3>Rendimento campioni</h3><div class="small muted" style="margin-top:6px">${p.seasonLocal?'Nessuna Solo/Duo tra le ultime 20 partite del client. Collega il server statistiche per vedere tutta la stagione.':'Carico le tue partite Solo/Duo dall\'inizio della stagione…'}${p.seasonPending?` (${p.seasonPending} in arrivo)`:''}</div></div>`;
    }
    if(champsSrc?.length){
      const list = champsSrc.slice(0, more.champs?12:4);
      h += `<div class="card side-card"><h3>Rendimento campioni <span class="tiny dim">${p.seasonLocal?'· Solo/Duo · ultime 20 partite del client':`· stagione Solo/Duo · ${p.seasonGames} partite${p.seasonPending?` · altre ${p.seasonPending} in caricamento`:''}`}</span></h3>${list.map(x=>{ const wr=x.wins/x.games; return `<div class="pl2" data-act="openchamp" data-id="${champ(x.champion).id}"><img src="${cImg(x.champion)}">
        <div class="sp"><b>${esc(champ(x.champion).name)}</b><div class="tiny dim">${x.kda.toFixed(1).replace('.',',')} KDA</div></div>
        <div class="right"><b class="${wr>=.5?'good':'bad'} small">${pct(wr)} WR</b><div class="tiny dim">${x.wins}V - ${x.games-x.wins}S</div></div></div>`; }).join('')}
        ${champsSrc.length>4?`<button class="showmore" data-act="sidemore" data-k="champs">${more.champs?'Mostra meno ▴':'Mostra altro ▾'}</button>`:''}</div>`;
    }
    if(p?.premades?.length){
      const list = p.premades.slice(0, more.mates?12:4);
      h += `<div class="card side-card"><h3>Giocato di recente con</h3>${list.map(x=>{ const wr=x.wins/x.games; return `<div class="pl2" data-act="lookup" data-id="${esc(x.name+'#'+x.tag)}">${x.icon?`<img src="${profImg(x.icon)}" style="border-radius:50%">`:`<span class="av2">${esc((x.name||'?').slice(0,2).toUpperCase())}</span>`}
        <div class="sp" style="min-width:0"><b class="ell">${esc(x.name)}<span class="dim">#${esc(x.tag)}</span></b><div class="tiny dim">${x.games} partite</div></div>
        <div class="right"><b class="${wr>=.5?'good':'bad'} small">${pct(wr,0)} WR</b><div class="tiny dim">${x.wins}V - ${x.games-x.wins}S</div></div></div>`; }).join('')}
        ${p.premades.length>4?`<button class="showmore" data-act="sidemore" data-k="mates">${more.mates?'Mostra meno ▴':'Mostra altro ▾'}</button>`:''}</div>`;
    }
  } else {
    h += `<div class="card acc"><img class="welcome-logo" src="logo.png" alt="BetterStomp"><div class="name">Benvenuto</div>
      <p class="muted small">Apri il client di League of Legends: BetterStomp lo rileva da solo e mostra qui il riepilogo del tuo account.</p></div>`;
  }
  const e = S.st?.engine;
  if(e){
    h += `<div class="card side-card"><div class="kicker">Motore statistiche</div>
      ${!e.hasKey&&!e.hasRemote&&!e.legacy?`<p class="small muted">Inserisci una chiave Riot API in <a href="#/impostazioni">Impostazioni</a> per sbloccare tier list, win rate e build per rango.</p>`
      : `<div class="row" style="margin-top:8px"><span class="dot ${e.running&&!e.error?'on':''}"></span><b>${num(e.total+(e.remote||0))}</b><span class="muted small">partite · patch ${esc(e.patch||'-')}</span></div>${e.hasRemote?`<div class="tiny ${e.remoteErr?'bad':'dim'}" style="margin-top:4px">${e.remoteErr?esc(e.remoteErr):`di cui ${num(e.remote||0)} dal server`}</div>`:''}${e.legacy?`<div class="tiny dim" style="margin-top:4px">+ archivio vecchia versione: ${num(e.legacy)} partite (patch ${esc(e.legacyPatch.replace('+',', '))})</div>`:''}
         ${e.error?`<div class="small bad mt">${esc(e.error)}</div>`:''}`}</div>`;
  }
  if(el._h !== h){ put(el, h, !!el._h); el._h = h; }
}

/* ---------- router ---------- */
function route(){ return (location.hash.replace(/^#\/?/,'')||'profilo').split('/').map(decodeURIComponent); }
function render(soft){
  const r = route();
  document.querySelectorAll('.nav a').forEach(a=>a.classList.toggle('on', a.dataset.p===r[0] || (r[0]==='campione'&&a.dataset.p==='campioni')));
  const m = $('#main');
  const pages = {profilo:pageProfile, campioni:pageChampions, campione:pageChampion, tier:pageTier, select:pageSelect, patch:pagePatch, overlay:pageOverlay, streaming:pageStreaming, impostazioni:pageSettings};
  if(soft && !(pages[r[0]]||pageProfile).soft) return;
  (pages[r[0]]||pageProfile)(m, r, soft);
  if(!soft) m.scrollTop = 0;
}

/* ================= PROFILO ================= */
async function pageProfile(m, r, soft){
  S.ui.ptab = S.ui.ptab || 'partite';
  const reg = S.ui.searchRegion || S.settings.region;
  if(soft && !$('#pbody')) soft = false;
  if(soft && S.ui.search) return;
  if(soft){ S.ui.forceProf = true; }
  else m.innerHTML = `<div class="page"><div class="card flat mb"><div class="search">
      <input type="search" id="sid" placeholder="Cerca un giocatore: Nome#TAG" value="${esc(S.ui.search||'')}">
      <select id="sreg">${opt(REGIONS, reg)}</select>
      <button class="btn" data-act="search">Cerca</button>
      ${S.ui.search?`<button class="btn ghost" data-act="myprofile">Il mio profilo</button>`:''}
    </div></div><div id="pbody"><div class="loading"><span class="spinner"></span>Carico il profilo…</div></div></div>`;
  if(!soft) $('#sid').addEventListener('keydown', e=>{ if(e.key==='Enter') actions.search(); });
  let p;
  try{
    if(S.ui.search) p = await api(`/api/search?riotId=${encodeURIComponent(S.ui.search)}&region=${reg}`);
    else { p = S.profile && !S.ui.forceProf ? S.profile : await api('/api/profile'+(S.ui.forceProf?'?force=1':'')); S.ui.forceProf=false; if(!p.error) S.profile=p; }
  }catch(e){ p = {error:e.message}; }
  if(route()[0]!=='profilo') return;
  if(soft && p.error && S.ui.prof && !S.ui.prof.error) return;
  S.ui.prof = p;
  put($('#pbody'), profileHTML(p), soft);
}
pageProfile.soft = true;

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
    ${p.source==='client'&&!S.ui.search?`<div class="grid2 mt2">${trendCard(p)}${eloCard(p)}</div>${metaCard(p)}`:''}
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
    const cq = S.ui.cq2||'420', cr = S.ui.cr2||'ALL';
    const ms = (p.matches||[]).filter(m=>!m.remake && (cq==='all'||String(m.queue)===cq) && (cr==='ALL'||m.me.pos===cr));
    const agg = {};
    for(const m of ms){ const c = agg[m.me.champ] ||= {champion:m.me.champ,games:0,wins:0,k:0,d:0,a:0,cs:0,mins:0}; c.games++; if(m.me.win) c.wins++; c.k+=m.me.k; c.d+=m.me.d; c.a+=m.me.a; c.cs+=m.me.cs; c.mins+=m.duration/60; }
    const rows = Object.values(agg).map(c=>({...c, kda:(c.k+c.a)/Math.max(1,c.d), csMin:c.cs/Math.max(1,c.mins)})).sort((a,b)=>b.games-a.games);
    const filt = `<div class="row wrap gap6 mb"><div class="chips">${[['420','Solo/Duo'],['440','Flex'],['all','Tutte']].map(([k,l])=>`<span class="chip ${cq===k?'on':''}" data-act="cq2" data-k="${k}">${l}</span>`).join('')}</div>
      <div class="chips">${['ALL','TOP','JUNGLE','MIDDLE','BOTTOM','UTILITY'].map(k=>`<span class="chip ${cr===k?'on':''}" data-act="cr2" data-k="${k}">${POS[k]}</span>`).join('')}</div><span class="tiny dim">${ms.length} partite</span></div>`;
    if(!rows.length) return filt+`<div class="card"><div class="empty">Nessuna partita con questi filtri.</div></div>`;
    return filt+`<table class="tl"><thead><tr><th>Campione</th><th>Partite</th><th>Win rate</th><th>KDA</th><th>CS/min</th></tr></thead><tbody>
      ${rows.map(c=>`<tr data-act="openchamp" data-id="${champ(c.champion).id}"><td><div class="row">${ci(c.champion,'s')}<b>${esc(champ(c.champion).name)}</b></div></td>
      <td>${c.games}</td><td class="${c.wins/c.games>=.5?'good':'bad'} b">${pct(c.wins/c.games,0)}</td><td>${c.kda.toFixed(2).replace('.',',')}</td><td>${c.csMin.toFixed(1).replace('.',',')}</td></tr>`).join('')}</tbody></table>`;
  }
  if(t==='premade'){
    if(!p.premades?.length) return `<div class="card"><div class="empty"><div class="big">👥</div>Nessun compagno ricorrente nelle ultime partite.<div class="small dim mt">Vengono mostrati i giocatori trovati con te in squadra almeno 2 volte.</div></div></div>`;
    return `<div class="card"><div class="ctitle"><h2>Con chi hai giocato</h2><span class="muted small">compagni ricorrenti nelle ultime ${p.matches.length} partite</span></div>
      ${p.premades.map(x=>`<div class="kv" style="align-items:center"><div class="row">${x.icon?`<img class="ic s round" src="${profImg(x.icon)}">`:''}<div><b>${esc(x.name)}</b> <span class="dim">#${esc(x.tag)}</span><div class="tiny dim">ultima ${ago(x.last)}</div></div></div>
      <div class="row"><span class="badge">${x.games} partite</span><b class="${x.wins/x.games>=.5?'good':'bad'}">${pct(x.wins/x.games,0)}</b>
      <button class="btn ghost sm" data-act="lookup" data-id="${esc(x.name+'#'+x.tag)}">Profilo</button></div></div>`).join('')}</div>`;
  }
  if(!p.matches?.length) return `<div class="card"><div class="empty">Nessuna partita recente.</div></div>`;
  const qf = S.ui.mq||'all', shown = S.ui.mShow||20;
  const list = p.matches.map((g,i)=>[g,i]).filter(([g])=> qf==='all' || (qf==='420'? g.queue===420 : qf==='440'? g.queue===440 : g.queue!==420 && g.queue!==440));
  const chips = `<div class="chips mb">${[['all','Tutte'],['420','Solo/Duo'],['440','Flex'],['other','Altre']].map(([k,l])=>`<span class="chip ${qf===k?'on':''}" data-act="mq" data-q="${k}">${l}</span>`).join('')}<span class="tiny dim" style="margin-left:6px">${list.length} partite${p.seasonPending?` · altre in arrivo dal server`:''}</span></div>`;
  return chips + list.slice(0,shown).map(([g,i])=>{
    const me = g.me, cls = g.remake?'remake':me.win?'win':'loss';
    const mins = Math.max(1,g.duration/60);
    const res = g.remake?'Remake':(me.place? me.place+'° posto' : me.win?'Vittoria':'Sconfitta');
    return `<div class="match ${cls}"><div class="stripe"></div>${ci(me.champ,'l')}
      <div><div class="res">${res}</div><div class="small muted">${esc(qName(g.queue,g.mode))} · ${dur(g.duration)} · ${ago(g.start)}</div>
        <div class="row gap6 mt" style="margin-top:6px">${me.spells.map(s=>s?`<img class="ic xs" src="${sImg(s)}">`:'').join('')}</div></div>
      <div><div class="b" style="font-size:16px">${me.k} / <span class="bad">${me.d}</span> / ${me.a}</div><div class="small muted">${((me.k+me.a)/Math.max(1,me.d)).toFixed(2).replace('.',',')} KDA · ${me.cs} CS (${(me.cs/mins).toFixed(1).replace('.',',')})</div></div>
      <div class="items">${me.items.map(x=>x?`<img src="${iImg(x)}" title="${esc(S.items[x]?.name||'')}">`:'<span class="eslot"></span>').join('')}</div>
      <div class="row gap6"><button class="btn ghost sm" data-act="analyze" data-i="${i}">Analizza</button></div>
      <div class="analysis" id="an${i}" style="display:${S.ui.openAn?.[g.id]?'block':'none'}">${S.ui.openAn?.[g.id]?analysisHTML(g):''}</div></div>`;
  }).join('') + (list.length>shown?`<div class="row mt" style="justify-content:center"><button class="btn ghost" data-act="mmore">Mostra altre ${Math.min(20,list.length-shown)} partite</button></div>`:'');
}
/* ----- riepilogo a fine partita ----- */
async function maybePostGame(p){
  const u = S.ui; if(!u.postAt || Date.now()-u.postAt > 5*60000) return;
  const m = (p.matches||[]).find(x=>x.start + x.duration*1000 > u.postAt - 20*60000);
  if(!m || u.postShown===m.id) return;
  u.postShown = m.id; u.postAt = 0;
  let lp = null;
  if(m.queue===420 || m.queue===440){ try{ const h = await api('/api/lp'); const q = m.queue===420?'RANKED_SOLO_5x5':'RANKED_FLEX_SR'; const pts = h.filter(x=>x.queue===q); if(pts.length>=2 && pts.at(-1).t > m.start) lp = pts.at(-1).value - pts.at(-2).value; }catch(e){} }
  const me = m.me, mins = Math.max(1, m.duration/60), csm = me.cs/mins;
  const target = me.pos==='JUNGLE'?5.5:me.pos==='UTILITY'?1.5:7;
  const kp = (()=>{ const tk = m.parts.filter(x=>x.team===me.team).reduce((a,x)=>a+x.k,0); return tk? (me.k+me.a)/tk : 0; })();
  const grade = (()=>{ let s = 0; s += Math.min(1.2, ((me.k+me.a)/Math.max(1,me.d))/4); s += me.pos==='UTILITY'?0.5:Math.min(1.2, csm/target); s += Math.min(1, kp/0.55); s += me.win?0.3:0; const g = s/3.7; return g>=.95?'S+':g>=.85?'S':g>=.75?'A':g>=.62?'B':g>=.5?'C':'D'; })();
  modal(m.remake?'Remake':me.win?'Vittoria':'Sconfitta', `<div class="pg">
    <div class="row" style="gap:14px">${ci(me.champ,'l')}<div class="sp"><b style="font-size:16px">${esc(champ(me.champ).name)}</b><div class="small muted">${esc(qName(m.queue,m.mode))} · ${dur(m.duration)}</div></div>
      <div class="pggrade">${grade}</div>${lp!==null?`<div class="pglp ${lp>=0?'good':'bad'}">${lp>=0?'+':''}${lp} LP</div>`:''}</div>
    <div class="stats mt12"><div class="stat"><div class="k">KDA</div><div class="v">${me.k}/${me.d}/${me.a}</div></div>
      <div class="stat"><div class="k">CS/min</div><div class="v ${me.pos==='UTILITY'||csm>=target?'good':'bad'}">${csm.toFixed(1).replace('.',',')}</div><div class="tiny dim">obiettivo ${String(target).replace('.',',')}</div></div>
      <div class="stat"><div class="k">Partecipazione</div><div class="v">${pct(kp,0)}</div></div><div class="stat"><div class="k">Visione</div><div class="v">${me.vision}</div></div></div>
    <div class="mt12">${analysisHTML(m)}</div></div>`);
}

/* ----- campioni forti nel tuo ruolo in questa patch ----- */
function metaCard(p){
  const u = S.ui, pos = p.roles?.main; if(!pos) return '';
  if(u.metaPos!==pos && !u.metaLoading){ u.metaLoading=true; api(`/api/tierlist?mode=ranked&tier=auto&pos=`).then(d=>{ u.meta=(d.rows||[]).filter(r=>r.position===pos).slice(0,8); u.metaPos=pos; if(route()[0]==='profilo') render(true); }).finally(()=>u.metaLoading=false); }
  if(!u.meta?.length || u.metaPos!==pos) return '';
  const mine = new Set((p.champs||[]).map(c=>c.champion));
  return `<div class="card mb"><div class="ctitle"><h2>I più forti in ${POS[pos]} questa patch</h2><span class="sp"></span><a class="small" href="#/tier">Tier list completa →</a></div>
    <div class="metarow">${u.meta.map(r=>`<div class="metac" data-act="openchamp" data-id="${champ(r.champion).id}" data-pos="${r.position}">${ci(r.champion)}<b class="small">${esc(champ(r.champion).name)}</b><span class="tiny"><b class="${r.winRate>=.5?'good':'bad'}">${pct(r.winRate)}</b> ${tierBadge(r.tier)}</span>${mine.has(r.champion)?'<span class="tiny dim">lo giochi</span>':''}</div>`).join('')}</div></div>`;
}

/* ----- confronto con la media dell'elo, sulle tue Solo/Duo nel ruolo principale ----- */
function eloCard(p){
  const u = S.ui, pos = p.roles?.main, solo = p.ranked?.RANKED_SOLO_5x5;
  if(!pos) return '';
  const myTier = solo?.tier && solo.tier!=='NONE' ? solo.tier : 'GOLD';
  const next = TIERS[Math.min(TIERS.length-1, TIERS.indexOf(myTier)+1)];
  const tier = u.btier==='next' ? next : myTier;
  const key = pos+'|'+tier;
  if(u.benchKey!==key && !u.benchLoading){ u.benchLoading=true; api(`/api/bench?pos=${pos}&tier=${tier}`).then(b=>{ u.bench=b; u.benchKey=key; if(route()[0]==='profilo') render(true); }).finally(()=>u.benchLoading=false); }
  const b = u.benchKey===key ? u.bench : null;
  const ms = (p.matches||[]).filter(m=>m.queue===420 && !m.remake && m.me.pos===pos);
  if(ms.length < 3 || !b) return `<div class="card mb"><div class="ctitle"><h2>Confronto con l'elo</h2></div><div class="small dim">${ms.length<3?'Servono almeno 3 Solo/Duo nel tuo ruolo principale.':'Carico le medie…'}</div></div>`;
  let mins=0, cs=0, gold=0, k=0, d=0, a=0, kp=0;
  for(const m of ms){ const mm=m.duration/60; mins+=mm; cs+=m.me.cs; gold+=m.me.gold; k+=m.me.k; d+=m.me.d; a+=m.me.a; const tk=m.parts.filter(x=>x.team===m.me.team).reduce((s,x)=>s+x.k,0); kp += tk?(m.me.k+m.me.a)/tk:0; }
  const mine = {csm: cs/mins, gpm: gold/mins, kp: kp/ms.length, kda: (k+a)/Math.max(1,d)};
  const row = (l, y, r, f) => { const ok = y>=r; const w = Math.min(100, y/(r*1.5)*100), rw = Math.min(100, r/(r*1.5)*100);
    return `<div class="cmp"><span class="small">${l}</span><div class="cbar"><i class="${ok?'g':'b'}" style="width:${w}%"></i><em style="left:${rw}%"></em></div><b class="small ${ok?'good':'bad'}">${f(y)}</b><span class="tiny dim">/ ${f(r)}</span></div>`; };
  const f1 = v=>v.toFixed(1).replace('.',','), f0 = v=>Math.round(v), fp = v=>pct(v,0);
  return `<div class="card mb"><div class="ctitle"><h2>Confronto con l'elo</h2><span class="sp"></span>
      <div class="seg sm"><button data-act="benchtier" data-k="mine" class="${u.btier!=='next'?'on':''}">${TIER_IT[myTier]}</button><button data-act="benchtier" data-k="next" class="${u.btier==='next'?'on':''}">${TIER_IT[next]}</button></div></div>
    <div class="tiny dim mb">${POS[pos]} · ${ms.length} Solo/Duo della stagione · la tacca è la media di ${TIER_IT[tier]}</div>
    ${pos!=='UTILITY'?row('CS al minuto', mine.csm, b.csm, f1):''}${row('Oro al minuto', mine.gpm, b.gpm, f0)}${row('Partecipazione', mine.kp, b.kp, fp)}${row('KDA', mine.kda, b.kda, f1)}</div>`;
}

/* ----- andamento: LP nel tempo, o vittorie e sconfitte delle ultime partite ----- */
function trendCard(p){
  const u = S.ui;
  if(!u.lpLoaded){ u.lpLoaded = true; api('/api/lp').then(h=>{ u.lp = h; if(route()[0]==='profilo') render(true); }).catch(()=>{}); }
  const pts = (u.lp||[]).filter(x=>x.queue==='RANKED_SOLO_5x5');
  const W = 560, H = 120;
  let svg = '', title = '', sub = '';
  if(pts.length >= 2){
    const vs = pts.map(x=>x.value), mn = Math.min(...vs), mx = Math.max(...vs), span = Math.max(50, mx-mn);
    const xy = pts.map((x,i)=>[i/(pts.length-1)*W, H-8-((x.value-mn)/span)*(H-16)]);
    svg = `<polyline fill="none" stroke="url(#lg)" stroke-width="2.5" points="${xy.map(a=>a.join(',')).join(' ')}"/>`;
    const d = vs.at(-1)-vs[0];
    title = 'Andamento LP · Solo/Duo'; sub = `<b class="${d>=0?'good':'bad'}">${d>=0?'+':''}${d} LP</b> <span class="dim">in ${pts.length-1} variazioni</span>`;
  } else {
    const ms = (p.matches||[]).filter(m=>m.queue===420 && !m.remake).slice(0,40).reverse();
    if(ms.length < 3) return '';
    let v = 0; const vals = [0, ...ms.map(m=> v += m.me.win?1:-1)];
    const mn = Math.min(...vals), mx = Math.max(...vals), span = Math.max(4, mx-mn);
    const xy = vals.map((x,i)=>[i/(vals.length-1)*W, H-8-((x-mn)/span)*(H-16)]);
    const zero = H-8-((0-mn)/span)*(H-16);
    svg = `<line x1="0" x2="${W}" y1="${zero}" y2="${zero}" stroke="var(--line2)" stroke-dasharray="4 4"/><polyline fill="none" stroke="url(#lg)" stroke-width="2.5" points="${xy.map(a=>a.join(',')).join(' ')}"/>`;
    const w = ms.filter(m=>m.me.win).length;
    title = `Ultime ${ms.length} Solo/Duo`; sub = `<b class="${w/ms.length>=.5?'good':'bad'}">${w}V - ${ms.length-w}S</b> <span class="dim">· i LP vengono registrati da ora in poi</span>`;
  }
  return `<div class="card mb"><div class="ctitle"><h2>${title}</h2><span class="sp"></span><span class="small">${sub}</span></div>
    <svg viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" style="width:100%;height:${H}px"><defs><linearGradient id="lg" x1="0" x2="1"><stop offset="0" stop-color="#8B6CFF"/><stop offset="1" stop-color="#38D6FF"/></linearGradient></defs>${svg}</svg></div>`;
}
function analysisHTML(g){ return (g.analysis||[]).map(a=>`<div class="ins ${a.kind}"><span class="i">${a.kind==='good'?'✓':a.kind==='bad'?'!':'i'}</span><span>${esc(a.text)}</span></div>`).join(''); }
function qName(q, mode){
  const Q = {420:'Solo/Duo',440:'Flex',400:'Normale',430:'Normale',490:'Partita rapida',480:'Partita rapida',450:'ARAM',2400:'ARAM: Caos',100:'ARAM',1700:'Arena',1710:'Arena',1750:'Arena',1810:'Arena',900:'URF',1900:'URF',1010:'URF',1020:'Uno per tutti',1300:'Nexus Blitz',700:'Clash',720:'Clash ARAM',830:'Contro IA',840:'Contro IA',850:'Contro IA',870:'Contro IA',880:'Contro IA',890:'Contro IA',2000:'Tutorial',2010:'Tutorial',2020:'Tutorial',3100:'Personalizzata',0:'Personalizzata'};
  if(Q[q]) return Q[q];
  const M = {CLASSIC:'Normale',ARAM:'ARAM',CHERRY:'Arena',URF:'URF',ARURF:'URF',ONEFORALL:'Uno per tutti',NEXUSBLITZ:'Nexus Blitz',ULTBOOK:'Libro delle Ulti',TUTORIAL:'Tutorial',PRACTICETOOL:'Allenamento',KIWI:'ARAM: Caos',SWIFTPLAY:'Partita rapida',STRAWBERRY:'Swarm',BRAWL:'Brawl'};
  return M[String(mode||'').toUpperCase()] || (mode ? mode.charAt(0)+mode.slice(1).toLowerCase() : 'Altra modalità');
}

/* ================= CAMPIONI ================= */
async function pageChampions(m, r, soft){
  const u = S.ui; u.cmode = u.cmode||S.settings.defaultMode||'ranked'; u.ctier = u.ctier||'auto'; u.cpos = u.cpos||'ALL'; u.cq = u.cq||'';
  if(soft && !$('#cgrid')) soft = false;
  if(!soft) m.innerHTML = `<div class="page"><div class="filters">
      <input type="search" id="cq" placeholder="Cerca campione…" value="${esc(u.cq)}" style="width:220px">
      <select data-ui="cmode">${opt(MODES,u.cmode)}</select>
      <select data-ui="ctier">${tierOptions(u.ctier)}</select>
      <div class="chips">${isPos(u.cmode)?['ALL','TOP','JUNGLE','MIDDLE','BOTTOM','UTILITY'].map(p=>`<span class="chip ${u.cpos===p?'on':''}" data-act="cpos" data-p="${p}">${POS[p]}</span>`).join(''):''}</div>
    </div><div id="cgrid"><div class="loading"><span class="spinner"></span></div></div></div>`;
  if(!soft) $('#cq').addEventListener('input', e=>{ u.cq=e.target.value; drawChampGrid(); });
  try{ S.ui.ctl = await api(`/api/tierlist?mode=${u.cmode}&tier=${u.ctier}&pos=`); }catch(e){ if(soft) return; S.ui.ctl = {rows:[]}; }
  drawChampGrid(soft);
}
pageChampions.soft = true;
function drawChampGrid(soft){
  const u = S.ui, rows = u.ctl?.rows||[];
  const best = {};
  rows.forEach(r=>{ if(u.cpos!=='ALL' && isPos(u.cmode) && r.position!==u.cpos) return; if(!best[r.champion]||best[r.champion].games<r.games) best[r.champion]=r; });
  const q = u.cq.trim().toLowerCase();
  let list = S.champList.filter(c=>!q || c.name.toLowerCase().includes(q) || c.id.toLowerCase().includes(q));
  if(u.cpos!=='ALL' && isPos(u.cmode) && rows.length) list = list.filter(c=>best[+c.key]);
  const el = $('#cgrid'); if(!el) return;
  put(el, (rows.length?'':`<div class="card flat mb small muted">Le statistiche per rango compariranno qui quando il motore avrà analizzato abbastanza partite (serve la chiave Riot API).</div>`) +
    `<div class="cgrid">${list.map(c=>{ const r=best[+c.key]; return `<div class="ccard" data-act="openchamp" data-id="${c.id}">
      <span class="t">${r?tierBadge(r.tier):''}</span><img src="${DD}${S.v}/img/champion/${c.id}.png" loading="lazy"><div class="n">${esc(c.name)}</div>
      <div class="wr">${r?pct(r.winRate)+' WR':esc(c.tags?.[0]||'')}</div></div>`; }).join('')}</div>`, soft);
}

/* ================= CAMPIONE ================= */
async function pageChampion(m, r, soft){
  const c = S.champList.find(x=>x.id===r[1]); if(!c){ location.hash='#/campioni'; return; }
  const key = +c.key, u = S.ui;
  if(u.chId!==c.id){ u.chId=c.id; u.chPos=''; u.editing=false; u.variant=0; u.abSel=0; u.runeSel=null; u.runeTree=false; u.ksSel=undefined; u.runeSet=0; }
  u.chMode = u.chMode||u.cmode||S.settings.defaultMode||'ranked'; u.chTier = u.chTier||u.ctier||'auto';
  if(soft && !$('#chbody')) soft = false;
  if(!soft) m.innerHTML = `<div class="page"><div class="champhero"><div class="bg" style="background-image:url('https://ddragon.leagueoflegends.com/cdn/img/champion/splash/${c.id}_0.jpg')"></div>
    <div class="in"><img class="ic xl" src="${cImg(key)}"><div class="sp"><div class="kicker" style="color:#c6cbe6">${esc(c.title)}</div><h1 style="font-size:34px">${esc(c.name)}</h1>
    <div class="htiles" id="chstats"></div></div>
    <div class="col" style="align-items:flex-end"><a class="btn ghost sm" href="#/campioni">← Tutti i campioni</a></div></div></div>
    <div class="filters mt"><select data-ui="chMode">${opt(MODES,u.chMode)}</select><select data-ui="chTier">${tierOptions(u.chTier)}</select><div class="chips" id="chpos"></div><span class="sp"></span>
      <span class="small muted" id="chsrc"></span></div>
    <div id="chbody"><div class="loading"><span class="spinner"></span>Calcolo la build…</div></div></div>`;
  let data, detail;
  try{
    [data, detail] = await Promise.all([api(`/api/champion?id=${key}&mode=${u.chMode}&tier=${u.chTier}&pos=${u.chPos}`), u.chDetail?.id===c.id ? u.chDetail.d : api(`/api/static/champion/${c.id}.json`).catch(()=>null)]);
  }catch(e){ if(!soft) $('#chbody').innerHTML = `<div class="card"><div class="empty">${esc(e.message)}</div></div>`; return; }
  if(route()[1]!==c.id || (soft && u.editing)) return;
  u.chData = data; u.chKey = key; u.chDetail = {id:c.id, d:detail};
  if(u.variant >= (data.build.variants||[]).length) u.variant = 0;
  const b = data.build;
  // le partite degli OTP arrivano in sottofondo la prima volta: ricarico una volta sola
  clearTimeout(u.otpT);
  if(b.otpPending && u.otpTry!==c.id+u.chPos){ u.otpTry=c.id+u.chPos; u.otpT=setTimeout(()=>{ if(route()[1]===c.id) pageChampion($('#main'), route(), true); }, 5000); }
  const tile = (k,v,cls='') => `<div class="htile"><div class="k">${k}</div><div class="v ${cls}">${v}</div></div>`;
  put($('#chstats'), b.games ? `${b.tierLabel?`<div class="htile tierT"><div class="k">Tier</div><div class="v">${tierBadge(b.tierLabel)}</div></div>`:''}${tile('Win rate',pct(b.winRate),b.winRate>=.5?'good':'bad')}${tile('Pick rate',pct(b.pickRate))}${tile('Ban rate',pct(b.banRate))}${tile('Partite',num(b.games))}${b.kda&&b.kda[1]?tile('KDA',`${b.kda[0].toFixed(1)} / ${b.kda[1].toFixed(1)} / ${b.kda[2].toFixed(1)}`):''}`
    : `<span class="badge">Nessuna statistica ancora per questo filtro</span>`, soft);
  put($('#chpos'), isPos(u.chMode) ? (b.positions||[]).filter(p=>p.position!=='ALL'&&p.share>=0.04).map(p=>`<span class="chip ${b.position===p.position?'on':''}" data-act="chpos" data-p="${p.position}">${POS[p.position]} <span class="tiny">${pct(p.share,0)}</span></span>`).join('') : '', soft);
  put($('#chsrc'), b.source?.startsWith('OP.GG') ? `Dati OP.GG${b.source==='OP.GG+'?' + partite rilevate (alternative)':''} · patch ${esc(b.patch)}` : b.source==='stats' ? `Patch ${esc(b.patch)}${b.widened?' · tutti i ranghi':''}` : '', soft);
  const prof = (S.profile?.season||[]).find(x=>x.champion===key) || S.profile?.champs?.find(x=>x.champion===key);
  put($('#chbody'), `<div class="card cbcard"><div class="ctitle"><h2>Build ${b.personal?'<span class="badge goldc">Personale</span>':''}</h2><span class="sp"></span>
        <button class="btn ghost sm" data-act="editbuild">${u.editing?'Chiudi editor':'✎ Build personale'}</button></div>
        <div id="buildbox">${buildHTML(b, data.personal, u.variant, 'variant')}</div></div>
    <div class="grid2 mt16">
      <div class="card"><h2>Matchup</h2>${matchList('Vince contro',b.best,'good')}${matchList('Soffre contro',b.worst,'bad')}
        ${!b.best?.length&&!b.worst?.length?'<div class="muted small">Servono più partite analizzate per i matchup.</div>':''}</div>
      <div class="card"><h2>Sinergie</h2>${matchList('Meglio in squadra con',b.synergy,'good')||'<div class="muted small">Servono più partite analizzate.</div>'}</div>
    </div>
    <div class="card mt16"><div class="ctitle"><h2>Build di OTP e pro</h2><span class="sp"></span><span class="tiny dim">i migliori giocatori di ${esc(c.name)} nella tua regione (classifica OP.GG) · clic su Importa per usare la loro build</span></div>
      ${otpRowsHTML(b.proGames||[])}</div>
    <div class="grid2 mt16" style="grid-template-columns:${prof?'2fr 1fr':'1fr'}">
      <div class="card"><h2>Abilità</h2>${detail? abilitiesHTML(detail.data[c.id]) : '<div class="muted">Non disponibili.</div>'}</div>
      ${prof?`<div class="card"><h2>Le tue partite <span class="tiny dim">Solo/Duo</span></h2><div class="stats"><div class="stat"><div class="k">Partite</div><div class="v">${prof.games}</div></div>
        <div class="stat"><div class="k">Win rate</div><div class="v ${prof.wins/prof.games>=.5?'good':'bad'}">${pct(prof.wins/prof.games,0)}</div></div><div class="stat"><div class="k">KDA</div><div class="v">${prof.kda.toFixed(2).replace('.',',')}</div></div></div></div>`:''}
    </div>
    <div class="row wrap mt16 extlinks"><span class="tiny dim">Confronta su</span>${[['u.gg',`https://u.gg/lol/champions/${c.id.toLowerCase()}/build`],['op.gg',`https://op.gg/lol/champions/${c.id.toLowerCase()}/build`],['lolalytics',`https://lolalytics.com/lol/${c.id.toLowerCase()}/build/`]].map(([n,l])=>`<a class="btn ghost sm" href="${l}" target="_blank">${n} ↗</a>`).join('')}</div>`, soft);
}
pageChampion.soft = true;

/* ---- pagina campione: vista build ---- */
function itCell(id, wr, cls=''){
  if(!id) return '';
  return `<div class="it ${cls}" title="${esc(S.items[id]?.name||'')}"><img src="${iImg(id)}">${wr!=null?`<span class="iwr ${wr>=.5?'good':'bad'}">${pct(wr,0)}</span>`:''}</div>`;
}
function runesCompact(r){
  if(!r || !r.perks?.length) return `<div class="muted small">Nessuna pagina disponibile. Con il client aperto vengono usate le rune consigliate da Riot.</div>`;
  const p = r.perks, nm = id => esc(S.perk[id]?.name || SHARDS[id]?.[0] || '');
  const ri = (id, c='') => `<img class="rc-r ${c}" src="${pImg(id)}" title="${nm(id)}" onerror="this.style.visibility='hidden'">`;
  return `<div class="rc"><div class="rc-main"><img class="rc-key" src="${pImg(p[0])}" title="${nm(p[0])}"><div class="sp"><div class="rc-kn">${nm(p[0])}</div>
      <div class="tiny dim"><img class="rc-ti" src="${stImg(r.primary)}">${esc(S.style[r.primary]?.name||'')} + <img class="rc-ti" src="${stImg(r.secondary)}">${esc(S.style[r.secondary]?.name||'')}</div></div></div>
    <div class="rc-rows"><div class="rc-row">${p.slice(1,4).map(x=>ri(x)).join('')}</div><div class="rc-row">${p.slice(4,6).map(x=>ri(x)).join('')}<span class="rc-gap"></span>${p.slice(6,9).map(x=>ri(x,'sh')).join('')}</div></div></div>`;
}
function optRow(icons, o, on, act, i){
  return `<div class="orow ${on?'on':''}" ${act?`data-act="${act}" data-i="${i}"`:''}><div class="oic">${icons}</div><span class="sp"></span>
    <div class="ost"><b>${pct(o.pick ?? o.share ?? 0,1)}</b><span class="tiny dim">pick</span></div>
    <div class="ost"><b class="${o.winRate>=.5?'good':'bad'}">${pct(o.winRate,1)}</b><span class="tiny dim">${num(o.games)} p.</span></div></div>`;
}
const SHARD_GRID = [[5008,5005,5007],[5008,5010,5001],[5011,5013,5001]];
function runeBoard(r){
  if(!r || !r.perks?.length) return `<div class="muted small">Nessuna pagina disponibile.</div>`;
  const p = r.perks, img = (x,on,cls='') => `<img class="${on?'on':''} ${cls}" src="https://ddragon.leagueoflegends.com/cdn/img/${x.icon}" title="${esc(x.name)}">`;
  const pr = S.style[r.primary], se = S.style[r.secondary];
  const prim = pr ? pr.slots.map((sl,i)=>`<div class="rb-row ${i===0?'ks':''}">${sl.runes.map(x=>img(x,p.slice(0,4).includes(x.id))).join('')}</div>`).join('') : '';
  const sec = se ? se.slots.slice(1).map(sl=>`<div class="rb-row">${sl.runes.map(x=>img(x,p.slice(4,6).includes(x.id))).join('')}</div>`).join('') : '';
  const sh = SHARD_GRID.map((row,i)=>`<div class="rb-row sh">${row.map(id=>`<img class="${p[6+i]===id?'on':''}" src="${pImg(id)}" title="${esc(SHARDS[id]?.[0]||'')}" onerror="this.style.visibility='hidden'">`).join('')}</div>`).join('');
  return `<div class="rboard"><div class="rb-col">${prim}</div><div class="rb-col">${sec}<div class="rb-sh">${sh}</div></div></div>`;
}
function champBuildView(b0, variant){
  const vs = (!b0.personal && b0.variants) || [];
  const v = vs[variant];
  const b = v ? {...b0, core:v.core, situational:v.situational} : b0;
  const wr = b0.itemWR || {};
  const iimg = (id, c='') => `<img class="${c}" src="${iImg(id)}" title="${esc(S.items[id]?.name||'')}">`;
  // ---- rune: keystone in alto, albero completo, set in basso (come onetricks.gg)
  const kss = (b0.runeKeystones || []).filter(o=>o.pick>=0.01);
  const pages = b0.runePages || [];
  const ksSel = S.ui.ksSel ?? (v?.keystone || kss[0]?.ids[0]);
  const ksPages = pages.filter(p=>p.perks[0]===ksSel).slice(0,4);
  const totKs = ksPages.reduce((a,p)=>a+p.games,0) || 1;
  const setSel = Math.min(S.ui.runeSet||0, Math.max(0,ksPages.length-1));
  const r = ksPages[setSel] || v?.runes || b0.runes;
  const runeCard = `<div class="cbsec ot"><div class="cbh"><span>Rune</span><button class="btn xs" data-act="imprunes">⇩ Importa rune</button></div>
    ${kss.length?`<div class="kschips">${kss.map(o=>`<button class="kschip ${o.ids[0]===ksSel?'on':''}" data-act="kssel" data-k="${o.ids[0]}"><img src="${pImg(o.ids[0])}"><b>${pct(o.pick,0)}</b></button>`).join('')}
      <span class="tiny dim" style="margin-left:auto">${r?.games?`${pct(r.winRate)} WR · ${num(r.games)} partite`:''}</span></div>`:''}
    ${runeBoard(r)}
    ${ksPages.length>1?`<div class="rsets">${ksPages.map((p,i)=>`<button class="rset ${i===setSel?'on':''}" data-act="runeset" data-i="${i}"><span>Set ${i+1}:</span><img src="${pImg(p.perks[0])}"><img class="st" src="${stImg(p.secondary)}"><i style="width:${Math.round(p.games/totKs*100)}%"></i></button>`).join('')}</div>`:''}</div>`;
  // ---- oggetti
  const tabs = vs.length>1 ? `<div class="isets">${vs.map((x,i)=>`<button class="iset ${i===variant?'on':''}" data-act="variant" data-i="${i}"><span class="tiny dim">${esc(x.label||'Build '+(i+1))}</span><div class="row gap4">${(x.boots?[x.boots]:[]).concat(x.path.slice(0,3)).map(id=>iimg(id)).join('')}</div><span class="tiny"><b class="${x.winRate>=.5?'good':'bad'}">${pct(x.winRate)}</b> <span class="dim">· ${num(x.games)}</span></span></button>`).join('')}</div>` : '';
  const core = (b.core||[]).filter(id=>!(S.items[id]?.tags||[]).includes('Boots')).slice(0,6);
  const after = (b.core||[]).concat(b.situational||[]).filter((id,i,a)=>id && a.indexOf(id)===i && !core.includes(id) && !(S.items[id]?.tags||[]).includes('Boots')).slice(0,8);
  const optRows = (list, icons) => `<div class="olist">${list.map((o,i)=>optRow(icons(o), o, i===0)).join('')}</div>`;
  const itemCard = `<div class="cbsec ot"><div class="cbh"><span>Oggetti</span><button class="btn xs" data-act="impitems">⇩ Importa oggetti</button></div>
    ${(b0.startSets||[]).length?`<div class="sublbl">Inizio</div>${optRows(b0.startSets.slice(0,2), o=>o.ids.map(id=>iimg(id)).join(''))}`:''}
    <div class="sublbl">Build principale</div>${tabs}
    <div class="ipath">${core.length?core.map((id,i)=>`${i?'<span class="parr">›</span>':''}${itCell(id,wr[id],'big')}`).join(''):'<span class="small dim">Ancora poche partite per questo filtro.</span>'}</div>
    ${(b0.bootsList||[]).length?`<div class="sublbl">Stivali</div><div class="pop">${b0.bootsList.map(o=>popCell(o)).join('')}</div>`:''}
    ${after.length?`<div class="sublbl">Poi, in base alla partita</div><div class="isit">${after.map(id=>itCell(id,wr[id])).join('')}</div>`:''}</div>`;
  // ---- incantesimi e abilità
  const skCard = `<div class="cbsec ot"><div class="cbh"><span>Incantesimi e abilità</span></div>
    ${(b0.spellSets||[]).length?optRows(b0.spellSets.slice(0,2), o=>o.ids.map(s=>`<img src="${sImg(s)}" title="${esc(S.spells[s]?.name||'')}">`).join('')):''}
    ${(b0.skillOrders||[]).length?`<div class="sublbl">Priorità</div>`+optRows(b0.skillOrders.slice(0,2), o=>`<div class="skprio">${o.key.split('>').map((k,j)=>`${j?'<span class="arr">›</span>':''}<span class="skey k${k}">${k}</span>`).join('')}</div>`):''}
    <div class="mt12">${skillGridHTML(b0.skillSeq)}</div></div>`;
  return `<div class="impbar"><button class="btn" data-act="import">⇩ Importa tutto nel client</button><span class="tiny dim">rune, incantesimi e oggetti della build e del set selezionati</span></div>
    <div class="otgrid"><div class="col">${runeCard}${skCard}</div><div class="col">${itemCard}</div></div>`;
}
function curRunes(){ const b0=S.ui.chData?.build; if(!b0) return null; const v=(b0.variants||[])[S.ui.variant||0]; const ks=S.ui.ksSel ?? (v?.keystone || b0.runeKeystones?.[0]?.ids[0]); const pg=(b0.runePages||[]).filter(p=>p.perks[0]===ks).slice(0,4); return pg[Math.min(S.ui.runeSet||0,Math.max(0,pg.length-1))] || v?.runes || b0.runes; }
function popCell(o){
  const id = o.ids[0];
  return `<div class="popc" title="${esc(S.items[id]?.name||'')}"><img src="${iImg(id)}"><b>${pct(o.pick,0)}</b><span class="${o.winRate>=.5?'good':'bad'}">${pct(o.winRate,0)}</span></div>`;
}
function matchList(t, list, cls){
  if(!list?.length) return '';
  return `<div class="cbh mt8"><span>${t}</span></div><div class="mlist">${list.slice(0,6).map(p=>`<div class="mrow" data-act="openchamp" data-id="${champ(p.champion).id}">${ci(p.champion,'s')}
    <span class="sp nm">${esc(champ(p.champion).name)}</span><span class="mbar"><i class="${cls}" style="width:${Math.round(Math.min(1,Math.max(0,(p.winRate-.35)/.3))*100)}%"></i></span>
    <b class="${p.winRate>=.5?'good':'bad'} small">${pct(p.winRate)}</b><span class="tiny dim gm">${p.games}</span></div>`).join('')}</div>`;
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
function variantsHTML(b, sel, act){
  const vs = b.variants||[];
  if(vs.length < 2 || b.personal) return '';
  return `<div class="blocklbl"><span>Build più giocate</span><span class="small dim">${vs.length} varianti</span></div><div class="variants">${vs.map((v,i)=>`<button class="var ${i===sel?'on':''}" data-act="${act}" data-i="${i}">
    <div class="vi">${v.boots?`<img src="${iImg(v.boots)}">`:''}${v.path.map(id=>`<img src="${iImg(id)}" title="${esc(S.items[id]?.name||'')}">`).join('')}</div>
    <div class="vl">${esc(v.label||('Build '+(i+1)))}</div><div class="vs2"><b class="${v.winRate>=.5?'good':'bad'}">${pct(v.winRate)} WR</b><span class="dim">${num(v.games)} partite · ${pct(v.share,0)}</span></div></button>`).join('')}</div>`;
}
function buildHTML(b, personal, variant=0, act='variant'){
  const ed = S.ui.editing && act==='variant';
  if(act==='variant' && !ed) return champBuildView(b, variant);
  const v = (b.variants||[])[variant];
  if(v && !b.personal) b = {...b, core:v.core, situational:v.situational};
  const src = ed ? (S.ui.edit ||= {starting:[...(personal?.starting||b.starting||[])], core:[...(personal?.core||b.core||[])], situational:[...(personal?.situational||b.situational||[])]}) : b;
  return `${ed?'':variantsHTML(arguments[0], variant, act)}${!ed && b.spells?.length?`<div class="blocklbl"><span>Incantesimi</span><span class="small muted">${b.spellsWR?pct(b.spellsWR)+' WR':''}</span></div><div class="row">${b.spells.map(s=>`<img class="ic" src="${sImg(s)}" title="${esc(S.spells[s]?.name||'')}">`).join('')}
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
  const rows = [[`${DD}${S.v}/img/passive/${d.passive.image.full}`,'P',d.passive.name,d.passive.description,'']]
    .concat(d.spells.map((s,i)=>[`${DD}${S.v}/img/spell/${s.image.full}`,'QWER'[i],s.name,s.description,s.cooldownBurn?`Ricarica ${s.cooldownBurn}s`:'']));
  const sel = Math.min(S.ui.abSel||0, rows.length-1), [img,k,n,desc,cd] = rows[sel];
  return `<div class="abtabs">${rows.map((r,i)=>`<button class="abt ${i===sel?'on':''}" data-act="abtab" data-i="${i}"><img src="${r[0]}"><span class="skey k${r[1]}">${r[1]}</span></button>`).join('')}</div>
    <div class="abdesc"><b>${esc(n)}</b>${cd?`<span class="tiny dim"> · ${cd}</span>`:''}<div class="d">${clean(desc)}</div></div>`;
}

/* ================= TIER LIST ================= */
async function pageTier(m, r, soft){
  const u = S.ui; u.tmode = u.tmode||S.settings.defaultMode||'ranked'; u.ttier=u.ttier||'auto'; u.tpos=u.tpos||'ALL'; u.tsort=u.tsort||'score';
  if(soft && !$('#tbody')) soft = false;
  if(!soft) m.innerHTML = `<div class="page"><div class="row mb"><h1>Tier list</h1><span class="sp"></span><span class="small muted" id="tinfo"></span></div>
    <div class="filters"><select data-ui="tmode">${opt(MODES,u.tmode)}</select><select data-ui="ttier">${tierOptions(u.ttier)}</select>
    <div class="chips">${isPos(u.tmode)?['ALL','TOP','JUNGLE','MIDDLE','BOTTOM','UTILITY'].map(p=>`<span class="chip ${u.tpos===p?'on':''}" data-act="tpos" data-p="${p}">${POS[p]}</span>`).join(''):''}</div></div>
    <div id="tbody"><div class="loading"><span class="spinner"></span></div></div></div>`;
  let d; try{ d = await api(`/api/tierlist?mode=${u.tmode}&tier=${u.ttier}&pos=${isPos(u.tmode)?u.tpos:''}`); }catch(e){ if(soft) return; d={rows:[]}; }
  if(route()[0]!=='tier') return;
  u.tdata = d; u.tierEmpty = !d.rows?.length;
  drawTier(soft);
}
pageTier.soft = true;
function drawTier(soft){
  const u = S.ui, d = u.tdata; const el = $('#tbody'); if(!el) return;
  const e = S.st?.engine||{};
  if(!d.rows?.length){
    put(el, `<div class="card"><div class="empty"><div class="big">📊</div><h2>Ancora nessun dato per questo filtro</h2>
      <p class="muted">La tier list è calcolata da BetterStomp analizzando partite pubbliche tramite la Riot API ufficiale.<br>
      ${!e.hasKey?'Inserisci la tua chiave Riot API nelle impostazioni per avviare il motore statistiche.':`Motore attivo: ${num(e.total)} partite analizzate finora${e.perHour?` (~${num(e.perHour)}/ora)`:''}. I dati crescono nel tempo e restano salvati sul PC.`}</p>
      ${!e.hasKey?'<a class="btn" href="#/impostazioni">Apri impostazioni</a>':''}</div></div>`, soft);
    return;
  }
  const tname = d.tier.includes('+')?TIER_IT[d.tier.replace('+','')]+'+':TIER_IT[d.tier]||'Tutti i ranghi';
  if(d.source==='OP.GG'){
    $('#tinfo').innerHTML = `Dati <b>OP.GG</b> · ${esc(tname)} · ${num(d.matches)} partite${d.loading?` · <span class="warn">${S.st?.opggSync?.running?`aggiornamento OP.GG in corso${S.st.opggSync.total?' ('+Math.round(S.st.opggSync.done/S.st.opggSync.total*100)+'%)':''}`:`carico le statistiche di questo elo ${d.done||0}/${d.total||'…'}`} · intanto vedi tutti i ranghi</span>`:''}`;
    clearTimeout(S.ui.tpoll);
    if(d.loading) S.ui.tpoll = setTimeout(()=>{ if(route()[0]==='tier') pageTier($('#main'), route(), true); }, 4000);
  } else
  $('#tinfo').innerHTML = `${num(d.eloMatches??d.matches)} partite ${esc(tname)} nella patch ${esc(String(d.patch).split(' ')[0])}${(d.eloMatches??d.matches)<3000?` · <b class="bad">ancora pochi dati: i valori si stabilizzano oltre le 3.000 partite</b>`:''}`;
  const rows = [...d.rows];
  const k = u.tsort, dir = u.tdir||-1;
  if(k!=='score') rows.sort((a,b)=> k==='name' ? champ(a.champion).name.localeCompare(champ(b.champion).name)*-dir : (a[k]-b[k])*dir);
  const th = (key,l) => `<th class="${u.tsort===key?'on':''}" data-act="tsort" data-k="${key}">${l}${u.tsort===key?(dir<0?' ↓':' ↑'):''}</th>`;
  put(el, `<table class="tl"><thead><tr><th>#</th>${th('name','Campione')}${isPos(u.tmode)?'<th>Ruolo</th>':''}${th('score','Tier')}${th('winRate','Win rate')}${th('pickRate','Pick rate')}${th('banRate','Ban rate')}${th('games','Partite')}</tr></thead><tbody>
    ${rows.map((r,i)=>`<tr data-act="openchamp" data-id="${champ(r.champion).id}" data-pos="${r.position}"><td class="dim b">${i+1}</td><td><div class="row">${ci(r.champion,'s')}<b>${esc(champ(r.champion).name)}</b></div></td>
    ${isPos(u.tmode)?`<td class="muted">${POS[r.position]||''}</td>`:''}<td>${tierBadge(r.tier)}</td><td class="b ${r.winRate>=.5?'good':'bad'}">${pct(r.winRate)}</td><td>${pct(r.pickRate)}</td><td>${pct(r.banRate)}</td><td class="muted">${num(r.games)}</td></tr>`).join('')}</tbody></table>`, soft);
}

/* ================= CHAMPION SELECT ================= */
function pageSelect(m, r, soft){
  const u = S.ui;
  if(soft && $('#selpage')){ updateSelect(true, true); return; }
  u.skind = u.skind||'matchup'; u.over = u.over||{}; u.sbuildKey=''; u.suggLoaded=false;
  u.stab = u.stab||'build'; u.vsort = u.vsort||'best'; u.selTier = u.selTier||'auto';
  m.innerHTML = `<div class="page" id="selpage">
    <div class="cshead"><h1>Champion Select</h1><span id="selphase" class="badge"></span><span class="sp"></span><div id="bans" class="row gap6 small"></div>
      <label class="autoimp" title="Rune, incantesimi e oggetti entrano nel client appena blocchi il campione">Import automatico ${sw('autoAll', S.settings.autoRunes&&S.settings.autoItems&&S.settings.autoSpells)}</label></div>
    <div class="cs3">
      <div class="card csteam" id="allies"></div>
      <div class="cscenter"><div class="cstool" id="cstool"></div><div id="csbody"></div></div>
      <div class="card csteam enemy" id="enemies"></div>
    </div>
  </div>`;
  if(!u.roleMap) api('/api/tierlist?mode=ranked&tier=ALL&pos=').then(d=>{ const best={}; for(const r of d.rows||[]){ if(r.position==='ALL') continue; if(!best[r.champion]||r.games>best[r.champion].g) best[r.champion]={p:r.position,g:r.games}; } u.roleMap=Object.fromEntries(Object.entries(best).map(([k,v])=>[k,v.p])); u.ssig=''; updateSelect(true); }).catch(()=>{});
  updateSelect(true);
}
pageSelect.soft = true;

function selState(){
  const c = S.st?.client, cs = c?.champSelect, u = S.ui;
  const mine = [], theirs = [];
  let myPos = '', myChamp = 0, meIdx = 0;
  if(cs){
    cs.myTeam.forEach((p,i)=>{ const ch = p.championId||p.championPickIntent; const me = p.cellId===cs.localPlayerCellId; if(me) meIdx=i;
      mine.push({champ:u.over['a'+i]||ch, pos:p.assignedPosition?.toUpperCase()||'', name:p.gameName||'', me}); });
    cs.theirTeam.forEach((p,i)=>theirs.push({champ:u.over['e'+i]||p.championId, pos:''}));
    myPos = cs.myPosition; myChamp = cs.myChampion;
  } else {
    for(let i=0;i<5;i++){ mine.push({champ:u.over['a'+i]||0,pos:'',me:i===0}); theirs.push({champ:u.over['e'+i]||0}); }
  }
  if(u.over['a'+meIdx]) myChamp = u.over['a'+meIdx];
  const posMap = {top:'TOP',jungle:'JUNGLE',middle:'MIDDLE',bottom:'BOTTOM',utility:'UTILITY'};
  return {cs, mine, theirs, meIdx, myPos: u.spos || posMap[(myPos||'').toLowerCase()] || (S.profile?.roles?.main) || 'MIDDLE', myChamp, mode: queueMode(c?.queueId)};
}
function queueMode(q){ return {420:'ranked',440:'flex',400:'normal',430:'normal',490:'normal',450:'aram',2400:'mayhem',1700:'arena',1710:'arena',900:'urf',1900:'urf'}[q] || S.ui.smode || 'ranked'; }

// compagni: rango, win rate ed etichette lette dal client (aggiornate ogni pochi secondi)
function refreshMates(){
  const u = S.ui, cs = S.st?.client?.champSelect;
  if(!cs || u.matesLoading || Date.now()-(u.matesAt||0) < 6000) return;
  u.matesLoading = true;
  api('/api/cs/mates').then(l=>{ u.mates = l||[]; u.matesAt = Date.now(); u.ssig=''; updateSelect(true, true); }).catch(()=>{}).finally(()=>u.matesLoading=false);
}
function teamDamage(list){
  let ad=0, ap=0;
  for(const c of list){ const i = champ(c)?.info; if(!i) continue; const a=i.attack||0, m=i.magic||0; if(a+m===0) continue; ad += a/(a+m); ap += m/(a+m); }
  return ad+ap ? ad/(ad+ap) : null;
}
function dmgBar(label, list){
  const r = teamDamage(list); if(r===null) return '';
  return `<div class="dmg"><span class="tiny dim">${label}</span><div class="dbar"><i class="ad" style="width:${Math.round(r*100)}%"></i><i class="ap" style="width:${100-Math.round(r*100)}%"></i></div><span class="tiny"><b class="adt">${Math.round(r*100)}% fisici</b> · <b class="apt">${100-Math.round(r*100)}% magici</b></span></div>`;
}
function updateSelect(force, soft){
  if(!$('#selpage')) return;
  const s = selState(), u = S.ui;
  refreshMates();
  const enemies = s.theirs.map(x=>x.champ).filter(Boolean);
  // avversario di corsia: il nemico che di solito gioca nel tuo ruolo
  if(!u.soppManual){ const g = enemies.find(e=>u.roleMap?.[e]===s.myPos); u.sopp = g||0; }
  const sig = JSON.stringify([s.mine,s.theirs,s.myPos,s.mode,u.skind,u.sopp,s.cs?.bans,s.myChamp,u.stab,u.selTier,u.matesAt]);
  const c = S.st?.client;
  $('#selphase').textContent = s.cs ? (s.cs.timer?.phase==='FINALIZATION'?'Finalizzazione':s.cs.timer?.phase==='BAN_PICK'?'Ban e scelte':'In corso')+(s.cs.timer?.adjustedTimeLeftInPhase>0?` · ${Math.ceil(s.cs.timer.adjustedTimeLeftInPhase/1000)}s`:'') : 'Modalità manuale';
  if(c?.imported && c.imported!==u.lastImp){ u.lastImp=c.imported; toast(c.imported); }
  if(!force && sig===u.ssig) return;
  u.ssig = sig;
  const mates = u.mates||[];
  const rankTxt2 = m => m?.tier ? `${TIER_IT[m.tier]||m.tier}${['MASTER','GRANDMASTER','CHALLENGER'].includes(m.tier)?'':' '+(m.division||'')}` : 'Senza rango';
  put($('#allies'), `<div class="csth ally">La tua squadra</div>` + s.mine.map((p,i)=>{
      const cell = s.cs?.myTeam?.[i]?.cellId, m = mates.find(x=>x.cellId===cell);
      const g = m ? m.wins+m.losses : 0, wr = g ? m.wins/g : 0;
      return `<div class="csp ${p.me?'me':''}" data-act="override" data-k="a${i}" title="Clicca per correggere il campione">
        <div class="cpic">${p.champ?`<img src="${cImg(p.champ)}">`:'<span>?</span>'}</div>
        <div class="sp" style="min-width:0"><div class="nm">${p.champ?esc(champ(p.champ).name):'<span class="dim">In scelta…</span>'} <span class="tiny dim">${POS[p.pos]||''}</span></div>
          <div class="tiny dim ell">${esc(p.me?'Tu':(m?.name||p.name||'Alleato'))}${m?.champGames?` · ${m.champWins}V ${m.champGames-m.champWins}S col campione`:''}</div>
          ${m?.tags?.length?`<div class="tags">${m.tags.map(t=>`<span class="tg ${/sconfitte|smurf|Mai/.test(t)?'bad':'good'}">${esc(t)}</span>`).join('')}</div>`:''}</div>
        <div class="rk"><b style="color:${TIER_COL[m?.tier]||'var(--muted)'}">${esc(rankTxt2(m))}</b>${g?`<div class="tiny"><b class="${wr>=.5?'good':'bad'}">${pct(wr,0)}</b> <span class="dim">${g}</span></div>`:''}</div></div>`; }).join('')
    + dmgBar('Danni', s.mine.map(x=>x.champ).filter(Boolean)), soft);
  const b = u.selData;
  const mu = id => [...(b?.best||[]),...(b?.worst||[])].find(x=>x.champion===id);
  put($('#enemies'), `<div class="csth enemy">Squadra nemica</div>` + s.theirs.map((p,i)=>{ const x = p.champ && mu(p.champ), opp = p.champ && p.champ===u.sopp;
      return `<div class="csp ${opp?'opp':''}" data-act="override" data-k="e${i}" title="Clicca per correggere il campione">
        <div class="cpic">${p.champ?`<img src="${cImg(p.champ)}">`:'<span>?</span>'}</div>
        <div class="sp" style="min-width:0"><div class="nm">${p.champ?esc(champ(p.champ).name):'<span class="dim">Non ancora scelto</span>'}</div>
          <div class="tiny dim">${p.champ?(POS[u.roleMap?.[p.champ]]||''):''}${opp?' · contro di te':''}</div></div>
        ${x?`<div class="rk"><b class="${x.winRate>=.5?'good':'bad'}">${pct(x.winRate,0)}</b><div class="tiny dim">per te</div></div>`:''}</div>`; }).join('')
    + dmgBar('Danni', enemies), soft);
  const bans = [...(s.cs?.bans?.myTeamBans||[]),...(s.cs?.bans?.theirTeamBans||[])].filter(Boolean);
  put($('#bans'), s.cs ? `<span class="dim">Ban</span>${bans.map(b=>`<img class="banic" src="${cImg(b)}" title="${esc(champ(b).name)}">`).join('')||'<span class="dim">nessuno</span>'}` : `<span class="dim">Fuori dalla champion select: clicca i ritratti per provare un matchup.</span>`, soft);
  put($('#cstool'), `<div class="seg"><button data-act="stab" data-k="build" class="${u.stab==='build'?'on':''}">Build</button><button data-act="stab" data-k="sugg" class="${u.stab==='sugg'?'on':''}">Cosa scegliere</button></div>
    ${!s.cs?`<select data-ui="smode">${opt(MODES,s.mode)}</select>`:''}
    ${isPos(s.mode)?`<select data-ui="spos">${opt(['TOP','JUNGLE','MIDDLE','BOTTOM','UTILITY'].map(p=>[p,POS[p]]), s.myPos)}</select>`:''}
    <span class="dim b">vs</span><select data-ui="sopp" data-num="1"><option value="0">Nessun avversario</option>${enemies.map(e=>`<option value="${e}" ${u.sopp===e?'selected':''}>${esc(champ(e).name)}</option>`).join('')}${u.sopp&&!enemies.includes(u.sopp)?`<option value="${u.sopp}" selected>${esc(champ(u.sopp).name)}</option>`:''}</select>
    <button class="btn ghost sm" data-act="pickopp" title="Scegli l'avversario fra tutti i campioni">…</button>
    <select data-ui="selTier" title="Elo delle statistiche">${tierOptions(u.selTier)}</select>`, soft);
  if(u.stab==='sugg' || (!s.myChamp && isPos(s.mode))){
    // nessun campione ancora: al posto dello spazio vuoto, i pick consigliati
    if(!$('#sugg3')) $('#csbody').innerHTML = sugg3Shell();
    loadSugg3(s);
    u.sbuildKey = '';
    return;
  }
  const bk = `${s.myChamp}|${s.myPos}|${s.mode}|${u.selTier}`;
  if(bk!==u.sbuildKey || !$('#buildbox')){ u.sbuildKey=bk; loadSelBuild(s, false); }
  else if(b) drawSelHeader(s);
}
function sugg3Shell(){
  return `<div class="sugg3" id="sugg3"><div class="card sgcol" id="sgm"></div><div class="card sgcol" id="sgc"></div><div class="card sgcol" id="sgt"></div></div>
    <div class="tiny dim mt12" style="text-align:center">Clicca un campione per vedere la sua build · dati OP.GG (matchup e sinergie su tutti i ranghi, tier list del tuo elo)</div>`;
}
async function loadSugg3(s){
  const u = S.ui;
  const allies = s.mine.filter(x=>!x.me && x.champ).map(x=>({champ:x.champ, pos:x.pos||u.roleMap?.[x.champ]||''}));
  const exclude = [...(s.cs?.bans?.myTeamBans||[]),...(s.cs?.bans?.theirTeamBans||[]),...s.mine.map(x=>x.champ),...s.theirs.map(x=>x.champ)].filter(Boolean);
  const req = {pos:s.myPos, tier:u.selTier, opponent:u.sopp||0, allies, exclude};
  const key = JSON.stringify(req);
  if(u.sg3?.key===key){ drawSugg3(s, u.sg3.data); return; }
  if(u.sg3load===key) return;
  u.sg3load = key;
  if(!u.sg3) ['#sgm','#sgc','#sgt'].forEach(id=>{ const e=$(id); if(e) e.innerHTML=`<div class="loading"><span class="spinner"></span></div>`; });
  let d = null; try{ d = await api('/api/cs/suggest', req); }catch(e){}
  if(u.sg3load!==key) return;
  u.sg3load = ''; u.sg3 = {key, data:d||{}};
  if($('#sugg3')) drawSugg3(selState(), u.sg3.data);
}
function sgRow(x, extra=''){
  return `<div class="sgr" data-act="suggpick" data-k="${x.champion}">${ci(x.champion)}
    <div class="sp" style="min-width:0"><div class="nm">${esc(champ(x.champion).name)} ${tierBadge(x.tier)}${extra}</div><div class="tiny dim ell">${esc(x.detail||'')}</div>
      ${x.with?.length?`<div class="sgw">${x.with.map(a=>ci(a,'xs')).join('')}</div>`:''}</div>
    <b class="wr ${x.winRate>=.5?'good':'bad'}">${pct(x.winRate,1)}</b></div>`;
}
function drawSugg3(s, d){
  const u = S.ui;
  const opp = u.sopp;
  put($('#sgm'), `<div class="sgh"><span class="sgi">⚔</span><div><b>Contro l'avversario</b><div class="tiny dim">${opp?`chi vince di più contro ${esc(champ(opp).name)} in ${POS[s.myPos]||''}`:'chi vince la corsia'}</div></div>${opp?ci(opp,'s'):''}</div>
    ${!opp?`<div class="empty small">Ancora nessun avversario di corsia.<br><span class="dim">Quando il nemico sceglie lo trovo da solo, oppure sceglilo dal menu "vs".</span></div>`
      : (d.matchup||[]).length ? d.matchup.map(x=>sgRow(x)).join('') : `<div class="empty small">Pochi dati su questo matchup.</div>`}`, true);
  // composizione: ritocco per bilanciare i danni della squadra
  const allies = s.mine.filter(x=>!x.me).map(x=>x.champ).filter(Boolean);
  const r = teamDamage(allies), need = r===null||allies.length<2 ? '' : r>0.62 ? 'ap' : r<0.38 ? 'ad' : '';
  const comp = (d.comp||[]).map(x=>{ const i = champ(x.champion)?.info||{}, a=i.attack||0, m=i.magic||0, ms = a+m ? m/(a+m) : .5;
    const fit = need==='ap' ? ms-.5 : need==='ad' ? .5-ms : 0;
    return {...x, adj: x.score + fit*0.03, fit}; }).sort((a,b)=>b.adj-a.adj).slice(0,8);
  const needTxt = need==='ap' ? 'alla squadra servono danni <b class="apt">magici</b>' : need==='ad' ? 'alla squadra servono danni <b class="adt">fisici</b>' : allies.length ? 'danni della squadra bilanciati' : 'quando gli alleati scelgono, guardo le sinergie';
  put($('#sgc'), `<div class="sgh"><span class="sgi">🤝</span><div><b>Per la composizione</b><div class="tiny dim">${needTxt}</div></div></div>
    ${comp.length ? comp.map(x=>sgRow(x, x.fit>0.15?` <span class="tg ${need==='ap'?'ap':'ad'}">${need==='ap'?'magici':'fisici'}</span>`:'')).join('') : `<div class="empty small">Nessun dato.</div>`}`, true);
  put($('#sgt'), `<div class="sgh"><span class="sgi">🔥</span><div><b>I più forti del ruolo</b><div class="tiny dim">tier list ${esc(POS[s.myPos]||'')} del tuo elo</div></div></div>
    ${(d.meta||[]).length ? d.meta.map(x=>sgRow(x)).join('') : `<div class="empty small">Tier list non ancora pronta.</div>`}`, true);
}
async function loadSuggestions(s, soft){
  const u = S.ui, el = $('#sugg'); if(!el) return;
  const allies = s.mine.filter(x=>!x.me).map(x=>x.champ).filter(Boolean), enemies = s.theirs.map(x=>x.champ).filter(Boolean);
  const bans = [...(s.cs?.bans?.myTeamBans||[]),...(s.cs?.bans?.theirTeamBans||[])].filter(Boolean);
  if(u.skind==='matchup' && !u.sopp){ put(el, `<div class="empty small">Scegli il tuo avversario di corsia nel menu "vs" per vedere i campioni migliori contro di lui.<br><span class="dim">In champion select i ruoli nemici non sono visibili: sceglilo dall'elenco o con "…".</span></div>`, soft); return; }
  if(!soft) el.innerHTML = `<div class="loading"><span class="spinner"></span></div>`;
  let list = [];
  try{ list = await api('/api/suggest', {mode:s.mode, tier:u.selTier, position:s.myPos, kind:u.skind, allies, enemies, opponent:u.sopp||0, exclude:bans}); }catch(e){}
  if(!$('#sugg')) return;
  if(!list?.length){ put(el, `<div class="empty small">${S.st?.engine?.total? 'Non ci sono ancora abbastanza partite per questo filtro.' : 'I suggerimenti si basano sul motore statistiche: aggiungi la chiave Riot API nelle impostazioni.'}</div>`, soft); return; }
  put(el, `<div class="sugg">${list.map(x=>`<div class="sg" data-act="suggpick" data-k="${x.champion}">${ci(x.champion)}<div class="sp"><b>${esc(champ(x.champion).name)}</b><div class="tiny muted">${esc(x.detail)}</div></div>
    <span class="sc ${x.score>=.5?'good':'bad'}">${pct(x.score,1)}</span></div>`).join('')}</div>`, soft);
}

/* ----- build in champion select (stile Blitz) ----- */
function parseRuneJS(k){
  if(!k) return null; const p = k.split(':'); if(p.length!==3) return null;
  const perks = p[2].split(',').map(Number); if(perks.length!==9) return null;
  return {primary:+p[0], secondary:+p[1], perks};
}
function sortedVariants(b){
  const vs = (b.variants||[]).map((v,i)=>({v,i})), base = b.winRate||0.5, k = S.ui.vsort;
  const score = v => (v.winRate*v.games + base*30)/(v.games+30);
  vs.sort((a,c)=> k==='common' ? c.v.games-a.v.games : k==='wr' ? c.v.winRate-a.v.winRate : score(c.v)-score(a.v));
  return vs;
}
function selView(b){
  const pk = S.ui.selPick || {t:'var', i:0};
  if(pk.t==='pro' && b.proGames?.[pk.i]){
    const g = b.proGames[pk.i];
    const boots = g.items.filter(id=>S.items[id]?.tags?.includes('Boots'));
    const core = [...boots, ...g.items.filter(id=>!boots.includes(id))];
    return {runes: parseRuneJS(g.runes), runeInfo: `Rune di ${g.name}`, spells: g.spells, spellsInfo: '', starting: g.start?.length?[...new Set(g.start)]:(b.starting||[]), core, situational: [], seq: g.seq, seqInfo: g.seq?`Ordine di ${g.name}`:'', wr: null, title: `Build di ${g.name}`};
  }
  const v = (b.variants||[])[pk.i] || null;
  return {runes: b.runes, runeInfo: b.runes?.games?`${pct(b.runes.winRate)} WR · ${num(b.runes.games)} partite`:(b.runes?.source||''), spells: b.spells,
    spellsInfo: b.spellsWR?`${pct(b.spellsWR)} WR`:'', starting: b.starting, core: v?v.core:b.core, situational: v?v.situational:b.situational,
    seq: b.skillSeq, seqInfo: b.skillSeqN?`${pct(b.skillSeqWR)} WR · ${num(b.skillSeqN)} partite`:'', title: v? v.label : 'Build'};
}
function otpRowsHTML(list){
  if(!list.length) return `<div class="small dim" style="padding:6px 2px">Nessuna partita recente trovata per questo campione.</div>`;
  return `<div class="otplist">${list.slice(0,12).map((g,i)=>{
    const rp = parseRuneJS(g.runes), ks = rp?.perks[0] || g.keystone;
    const who = g.pro ? '<span class="probadge">PRO</span>' : `<span class="tiny dim">${esc(TIER_IT[g.tier]||g.tier||'')}${g.lp?' '+g.lp+' LP':''}</span>`;
    return `<div class="otprow ${g.win?'w':'l'}">
      <div class="otpwho"><div class="nm">${esc(g.name)} ${who}</div><div class="tiny dim">${g.otpGames?`${num(g.otpGames)} partite · <b class="${g.otpWR>=.5?'good':'bad'}">${pct(g.otpWR,0)}</b> · `:''}${ago(g.when)}</div></div>
      <div class="otpres"><b class="${g.win?'good':'bad'}">${g.win?'Vittoria':'Sconfitta'}</b><div class="tiny dim">${g.k!=null&&(g.k||g.d||g.a)?`${g.k}/${g.d}/${g.a}`:''}${g.cs?` · ${g.cs} CS`:''}</div></div>
      <div class="otprune">${ks?`<img src="${pImg(ks)}" title="${esc(S.perk?.[ks]?.name||'')}">`:''}${(g.spells||[]).map(x=>`<img src="${sImg(x)}">`).join('')}</div>
      <div class="otpitems">${(g.items||[]).map(id=>`<img src="${iImg(id)}" title="${esc(S.items[id]?.name||'')}">`).join('')}</div>
      <button class="btn xs" data-act="otpimp" data-i="${i}">⇩ Importa</button></div>`; }).join('')}</div>`;
}
function proRowsHTML(list, clickable){
  if(!list?.length) return `<div class="small dim" style="padding:6px 2px">Nessuna partita ancora. Il motore raccoglie le partite di Master, Gran Maestro e Challenger; puoi aggiungere i tuoi pro preferiti in Impostazioni.</div>`;
  return list.map((g,i)=>{
    const rp = parseRuneJS(g.runes);
    const initials = (g.name||'?').slice(0,2).toUpperCase();
    return `<div class="csrow pro ${clickable&&S.ui.selPick?.t==='pro'&&S.ui.selPick.i===i?'on':''} ${g.win?'w':'l'}" ${clickable?`data-act="pickpro" data-i="${i}"`:''}>
      <span class="av2 ${g.pro?'gold':''}">${esc(initials)}</span>
      <div class="sp" style="min-width:0"><div class="nm">${esc(g.name)} ${g.pro?'<span class="probadge">PRO</span>':`<span class="tiny dim">${esc(TIER_IT[g.tier]||'')}</span>`}</div><div class="tiny dim">${ago(g.when)} · ${g.win?'<span class="good">V</span>':'<span class="bad">S</span>'}</div></div>
      <div class="ic3">${(rp?.perks[0]||g.keystone)?`<img src="${pImg(rp?.perks[0]||g.keystone)}">`:''}${(g.items||[]).filter(id=>!S.items[id]?.tags?.includes('Boots')).slice(0,2).map(id=>`<img src="${iImg(id)}">`).join('')}</div>
      ${g.opp?`<span class="tiny dim">vs</span><img class="ic xs" src="${cImg(g.opp)}">`:''}</div>`; }).join('');
}
function skillGridHTML(seq){
  if(!seq) return `<div class="small dim">L'ordine delle abilità compare quando ci sono abbastanza partite con la cronologia completa.</div>`;
  const n = seq.length, cols = Math.max(n, 15);
  const head = Array.from({length:cols},(_,i)=>`<span class="h">${i+1}</span>`).join('');
  const row = k => `<span class="k k${k}">${k}</span>`+Array.from({length:cols},(_,i)=>`<span class="c ${seq[i]===k?'on k'+k:''}">${seq[i]===k?k:''}</span>`).join('');
  return `<div class="sgrid" style="grid-template-columns:28px repeat(${cols},1fr)"><span></span>${head}${['Q','W','E','R'].map(row).join('')}</div>`;
}
async function loadSelBuild(s, soft){
  const el = $('#csbody'); if(!el) return;
  const u = S.ui;
  if(!s.myChamp){ put(el, `<div class="card"><div class="empty"><div class="big">🎯</div>Scegli (o passa sopra a) un campione: qui comparirà la build da importare con un clic.<br><span class="dim">Oppure apri "Cosa scegliere" per i campioni migliori contro il tuo avversario.</span></div></div>`, soft); return; }
  if(!soft) el.innerHTML = `<div class="loading"><span class="spinner"></span>Preparo la build…</div>`;
  let d; try{ d = await api(`/api/champion?id=${s.myChamp}&mode=${s.mode}&tier=${u.selTier}&pos=${isPos(s.mode)?s.myPos:''}`); }catch(e){ el.innerHTML = esc(e.message); return; }
  if(!$('#csbody')) return;
  const b = d.build; u.selBuild = {champ:s.myChamp, pos:b.position||s.myPos, mode:s.mode}; u.selData = b;
  // la vista build è la stessa della pagina campione (keystone, set, import separati)
  u.chData = d; u.chKey = s.myChamp; u.chMode = s.mode; u.variant = 0; u.ksSel = undefined; u.runeSet = 0; u.editing = false;
  el.innerHTML = `<div class="card cshero" id="cshero"></div><div class="card cbcard mt"><div id="buildbox">${buildHTML(b, d.personal, 0, 'variant')}</div></div>`;
  drawSelHeader(s);
  u.ssig=''; updateSelect(true, true);
}
function drawSelHeader(s){
  const u = S.ui, b = u.selData, el = $('#cshero'); if(!el || !b) return;
  const opp = u.sopp ? [...(b.best||[]),...(b.worst||[])].find(x=>x.champion===u.sopp) : null;
  const banned = new Set([...(s.cs?.bans?.myTeamBans||[]),...(s.cs?.bans?.theirTeamBans||[]),...s.theirs.map(x=>x.champ),...s.mine.map(x=>x.champ)]);
  const banSugg = (b.worst||[]).filter(x=>!banned.has(x.champion)).slice(0,5);
  put(el, `<div class="row" style="gap:16px;align-items:center">${ci(s.myChamp,'l')}
      <div class="sp"><h2 style="margin:0">${esc(champ(s.myChamp).name)} <span class="dim small">${POS[b.position]||''}</span></h2>
        <div class="small muted">${b.games?`${pct(b.winRate)} WR · ${num(b.games)} partite`:'Pochi dati per questo filtro'}${u.sopp?` · vs ${esc(champ(u.sopp).name)}: ${opp?`<b class="${opp.winRate>=.5?'good':'bad'}">${pct(opp.winRate)}</b> <span class="dim">(${opp.games})</span>`:'<span class="dim">matchup senza dati</span>'}`:''}</div></div>
      <button class="btn" data-act="import" ${S.st?.client?.connected?'':'disabled title="Apri il client di League"'}>⇩ Importa tutto</button></div>
    ${banSugg.length?`<div class="row gap6 mt12 small"><span class="dim">Ban consigliati</span>${banSugg.map(x=>`<span class="bansg" title="${esc(champ(x.champion).name)}: ${pct(x.winRate)} per te">${ci(x.champion,'s')}<b class="bad">${pct(x.winRate,0)}</b></span>`).join('')}</div>`:''}`, true);
}
function autoBox(bare){
  const s = S.settings;
  const inner = `<div class="setting"><div class="lbl"><b>Rune</b></div>${sw('autoRunes',s.autoRunes)}</div>
    <div class="setting"><div class="lbl"><b>Incantesimi</b><small>Flash su ${s.flashKey}</small></div>${sw('autoSpells',s.autoSpells)}</div>
    <div class="setting"><div class="lbl"><b>Oggetti (5 - 10 - 10)</b></div>${sw('autoItems',s.autoItems)}</div>`;
  return bare ? inner : `<div class="card flat mt"><div class="kicker">Import automatico quando blocchi il campione</div>${inner}</div>`;
}

/* ================= PATCH ================= */
async function pagePatch(m, r, soft, force){
  const u = S.ui; u.pf = u.pf||'all'; u.pq = u.pq||'';
  if(soft && !$('#phero')) soft = false;
  if(!soft) m.innerHTML = `<div class="page"><div id="phero"><div class="loading"><span class="spinner"></span>Carico le note della patch…</div></div><div id="pbody2"></div></div>`;
  let d; try{ d = await api('/api/patch'+(force?'?force=1':'')); }catch(e){ if(soft) return; d={error:e.message}; }
  if(route()[0]!=='patch') return;
  const rep = d.report;
  if(soft){ if(!rep || (u.patch && rep.fetched===u.patch.fetched && rep.patch===u.patch.patch)) return; soft = false; }
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
pagePatch.soft = true;
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

/* ================= OVERLAY (stile Blitz) ================= */
const TIMERS = [['dragonFirst','Primo drago'],['dragonRespawn','Respawn drago'],['elderRespawn','Respawn Drago Antico'],['grubsFirst','Larve del Vuoto'],['heraldFirst','Araldo'],['atakhanFirst','Atakhan (0 = disattivo)'],
  ['baronFirst','Primo Barone'],['baronRespawn','Respawn Barone'],['campFirst','Primo spawn campi'],['buffFirst','Primo spawn Blu/Rosso'],['campRespawn','Respawn campi piccoli'],['buffRespawn','Respawn Blu/Rosso'],
  ['baronBuff','Durata buff Barone'],['elderBuff','Durata buff Antico'],['relicFirst','ARAM: prime vite'],['relicRespawn','ARAM: respawn vite']];
const OVF = [
  {k:'bench', on:'cs.on', ic:'📊', n:'Benchmarking', d:'CS, oro, KP, KDA e livello confrontati con l\'elo che scegli'},
  {k:'notify', on:'notify.on', ic:'🔔', n:'Notifiche obiettivi', d:'Avviso prima che compaiano drago, barone, araldo, larve e Atakhan'},
  {k:'objectives', on:'objectives.on', ic:'🐉', n:'Timer obiettivi', d:'Riquadro con i tempi di drago, larve, araldo, Atakhan e barone'},
  {k:'minimap', on:'minimap', ic:'🗺️', n:'Timer sulla minimappa', d:'Campi della giungla, obiettivi e inibitori direttamente sulla minimappa'},
  {k:'camps', on:'camps.on', ic:'🌿', n:'Timer giungla (lista)', d:'Elenco dei campi con i respawn; in ARAM i timer delle vite'},
  {k:'buffs', on:'buffs.on', ic:'⏳', n:'Promemoria buff', d:'Scadenza del buff del Barone e del Drago Antico'},
  {k:'deaths', on:'deaths.on', ic:'💀', n:'Timer di respawn', d:'Quanto manca al respawn dei giocatori morti'},
  {k:'itemDiff', on:'itemDiff.on', ic:'💰', n:'Differenza valore oggetti', d:'Tenendo premuto TAB: la tua squadra e il tuo avversario diretto'},
  {k:'loading', on:'loading.on', ic:'👥', n:'Statistiche squadre', d:'Rango e win rate di tutti, in caricamento e con Ctrl+T'},
  {k:'timers', ic:'⏱️', n:'Durate dei timer', d:'I tempi di spawn usati da timer e notifiche'},
];
function pageOverlay(m){
  const o = S.settings.overlay;
  m.innerHTML = `<div class="page"><div class="row mb wrap"><h1>Overlay</h1><span class="sp"></span>
      <button class="btn ${o.editMode?'':'ghost'}" data-act="editmode">${o.editMode?'✓ Fine modifica':'✥ Sposta gli elementi sullo schermo'}</button>
      <label class="row b">${sw('overlay.enabled',o.enabled)} Overlay attivo</label></div>
    <div class="card flat mb small muted">In partita <b>tieni premuto TAB</b>: puoi trascinare ogni riquadro, bloccarlo con 🔒, aprire le sue impostazioni con ⚙ o chiuderlo con ✕, e sistemare il riquadro arancione della minimappa. Funziona con League in modalità <b>Senza bordi</b>.</div>
    <div class="grid2" style="grid-template-columns:minmax(0,1fr) minmax(0,1.1fr)">
      <div class="col gap16"><div class="ovlist">${OVF.map(f=>`<div class="ovrow"><span class="oi">${f.ic}</span><div class="on2"><b>${f.n}</b><small>${f.d}</small></div>
          ${['bench','notify','minimap','timers'].includes(f.k)?`<button class="gear" data-act="ovset" data-k="${f.k}" title="Impostazioni">⚙</button>`:''}${f.on?sw('overlay.'+f.on, getPath(o,f.on)):''}</div>`).join('')}</div>
        <div class="card"><div class="setting"><div class="lbl"><b>Dimensione dei riquadri</b></div><input type="range" min="0.7" max="1.6" step="0.05" value="${o.scale}" data-setnum="overlay.scale" style="width:170px"></div>
          <div class="setting"><div class="lbl"><b>Riporta tutto com'era</b><small>Posizioni e durate predefinite</small></div><button class="btn ghost sm" data-act="resetoverlay">Ripristina</button></div></div>
        <div class="card"><h2>Scorciatoie in partita</h2><div class="hk">
          <span><span class="kbd">TAB</span> (tenuto)</span><span>Sposta, blocca, imposta o chiudi i riquadri · differenza valore oggetti</span>
          <span><span class="kbd">Ctrl</span>+<span class="kbd">Shift</span>+<span class="kbd">E</span></span><span>Modalità modifica fissa (senza tenere TAB)</span>
          <span><span class="kbd">Ctrl</span>+<span class="kbd">Shift</span>+<span class="kbd">1…6</span></span><span>Campo del tuo lato ripulito: 1 Blu · 2 Gromp · 3 Lupi · 4 Pappagalli · 5 Rosso · 6 Sassi (in ARAM: vite 1-4)</span>
          <span><span class="kbd">Ctrl</span>+<span class="kbd">Alt</span>+<span class="kbd">1…6</span></span><span>Stesso campo, ma del lato nemico</span>
          <span><span class="kbd">Ctrl</span>+<span class="kbd">T</span></span><span>Statistiche delle squadre</span>
          <span><span class="kbd">Ctrl</span>+<span class="kbd">Shift</span>+<span class="kbd">C</span></span><span>Benchmark completo / riassunto</span>
          <span><span class="kbd">Ctrl</span>+<span class="kbd">Shift</span>+<span class="kbd">O</span></span><span>Nascondi/mostra tutto l'overlay</span></div></div>
      </div>
      <div><div class="card" style="position:sticky;top:0"><div class="ctitle"><h2>Anteprima</h2><span class="sp"></span><span class="small muted" id="ovstate"></span></div>
        <div class="screen" id="screen"></div><p class="small muted mt">✥ Trascina i riquadri con il mouse per posizionarli. Dati di esempio quando non sei in partita.</p></div></div>
    </div></div>`;
  drawOverlayPreview();
  clearInterval(S.ovTimer); S.ovTimer = setInterval(()=>{ if(route()[0]!=='overlay'){clearInterval(S.ovTimer);return;} drawOverlayPreview(); }, 1000);
  bindPreviewDrag();
  if(S.ui.openOv){ const k=S.ui.openOv; S.ui.openOv=null; actions.ovset({dataset:{k}}); }
}
function bzRow(label, ctrl){ return `<div class="bzrow"><span>${label}</span>${ctrl}</div>`; }
function ovSettingsHTML(k){
  const o = S.settings.overlay;
  const eloVal = o.csGoal==='off' ? 'off' : (o.csGoalTier||'MASTER');
  if(k==='bench') return bzRow('Visualizzazione', `<select data-set="overlay.compact" data-bool="1">${opt([['false','Completa'],['true','Riassunta']], String(!!o.compact))}</select>`)
    + bzRow('Stile classico a righe', sw('overlay.classic', !!o.classic)) + `<div class="tiny dim" style="margin-top:-6px">Spento = scheda con ritratto del campione, KDA, CS e timer di drago e barone</div>`
    + bzRow('Divisione di riferimento', `<select data-benchelo="1">${opt([['off','Nessuna'],['auto','Il mio rango']].concat(TIERS.map(t=>[t,TIER_IT[t]])), eloVal)}</select>`)
    + bzRow('Sempre visibile', sw('overlay.benchAlways', o.benchAlways)) + `<div class="tiny dim" style="margin-top:-6px">Se spento compare solo tenendo premuto TAB</div>`
    + bzRow('KDA', sw('overlay.benchKDA', o.benchKDA)) + bzRow('CS al minuto', sw('overlay.benchCSM', o.benchCSM)) + bzRow('Oro al minuto', sw('overlay.benchGPM', o.benchGPM))
    + bzRow('Partecipazione uccisioni', sw('overlay.showKP', o.showKP)) + bzRow('Livello campione', sw('overlay.benchLVL', o.benchLVL))
    + bzRow('Obiettivo CS/min fisso', `<input type="number" min="0" max="15" step="0.1" value="${o.csGoalValue||0}" data-setnum="overlay.csGoalValue">`) + `<div class="tiny dim" style="margin-top:-6px">0 = nessun obiettivo</div>`
    + bzRow('CS esatti letti dallo schermo', sw('overlay.ocr', o.ocr)) + `<div class="tiny dim" style="margin-top:-6px">L'API di Riot aggiorna i CS solo ogni 10: così il numero è sempre preciso</div>`;
  if(k==='notify') return bzRow('Dimensione', `<select data-set="overlay.notifySize">${opt([['small','Piccola'],['medium','Media'],['large','Grande']], o.notifySize||'medium')}</select>`)
    + bzRow('Durata', `<input type="number" min="3" max="60" step="1" value="${o.notifyDur||10}" data-setnum="overlay.notifyDur"> <span class="small">s</span>`)
    + `<div class="bzsec">Promemoria · secondi prima che l'obiettivo compaia</div><div id="remlist">${remListHTML()}</div>
       <button class="btn ghost sm" data-act="addrem">+ Aggiungi promemoria</button>`;
  if(k==='minimap') return bzRow('Grandezza dei numeri', `<input type="range" min="0.5" max="1.6" step="0.05" value="${o.pillScale}" data-setnum="overlay.pillScale" style="width:150px">`)
    + bzRow('Lato minimappa', `<input type="range" min="120" max="600" step="1" value="${o.minimapSize}" data-setnum="overlay.minimapSize" style="width:150px">`)
    + bzRow('Distanza dal bordo destro', `<input type="range" min="0" max="80" step="1" value="${o.minimapRight}" data-setnum="overlay.minimapRight" style="width:150px">`)
    + bzRow('Distanza dal bordo inferiore', `<input type="range" min="0" max="80" step="1" value="${o.minimapBottom}" data-setnum="overlay.minimapBottom" style="width:150px">`)
    + `<p class="tiny dim">Più comodo: in partita tieni premuto TAB e trascina/ridimensiona il riquadro arancione finché combacia con la minimappa.</p>`;
  if(k==='timers') return `<p class="tiny dim">In secondi. Riot cambia spesso questi tempi tra una stagione e l'altra: se non tornano, correggili qui.</p>`
    + TIMERS.map(([t,l])=>bzRow(l, `<input type="number" min="0" step="5" value="${o.timers[t]}" data-setnum="overlay.timers.${t}" style="min-width:90px;width:90px">`)).join('');
  return '';
}
function remListHTML(){ const l = S.settings.overlay.notifyBefore||[]; return l.map((v,i)=>`<div class="remrow"><input type="number" min="5" max="600" step="5" value="${v}" data-rem="${i}"><span class="small">s</span><button class="iconbtn" data-act="delrem" data-i="${i}">✕</button></div>`).join('') || '<div class="small dim mb">Nessun promemoria</div>'; }
async function drawOverlayPreview(){
  if(S.ovDrag) return;
  let md; try{ md = await api('/api/overlay/model?preview=1'); }catch(e){ return; }
  // posizioni spostate in gioco: allineo le impostazioni locali
  if(S.settings.overlay.editMode && !Object.keys(pending).length){ try{ const ns = await api('/api/settings'); S.settings.overlay = {...ns.overlay, editMode:S.settings.overlay.editMode}; }catch(e){} }
  const el = $('#screen'); if(!el || S.ovDrag) return;
  $('#ovstate').textContent = S.st?.inGame ? '● Partita in corso' : 'Dati di esempio';
  const W = el.clientWidth, H = el.clientHeight, o = S.settings.overlay, k = H/1080*(o.scale||1), U = H/1080;
  const px = v => (v*k).toFixed(2)+'px';
  let h = `<div class="mm" style="width:${o.minimapSize*U}px;height:${o.minimapSize*U}px;right:${o.minimapRight*U}px;bottom:${o.minimapBottom*U}px"></div>`;
  for(const w of md.widgets){
    if(w.pill){ const ps=o.pillScale||0.8; h += `<div class="mmtxt" style="left:${W - w.x*U}px;top:${H - w.y*U}px;color:${w.lines[0].color};font-size:${Math.max(4,12*ps*k)}px">${esc(w.lines[0].text)}</div>`; continue; }
    const left = w.x*W, top = w.y*H, center = w.anchor==='center';
    const pos = `left:${left}px;top:${top}px;${center?'transform:translateX(-50%);':''}`;
    if(w.card){ const c=w.card, ci=c.champ?`${DD}${S.v}/img/champion/${c.champ}.png`:'';
      const OBJB='https://raw.communitydragon.org/latest/game/assets/ux/minimap/icons/';
      const objIc = n=>{ n=n.toLowerCase(); const [k,c]= n.includes('antico')?['dragon_elder','#B8F0FF']:n.includes('drag')?['dragon','#FF8A3D']:n.includes('baron')||n.includes('barone')?['baron','#B48CFF']:n.includes('araldo')?['riftherald','#C79BFF']:n.includes('larve')?['grub','#A98BFF']:n.includes('atakhan')?['atakhan_v','#FF5C5C']:['','#38D6FF']; return [k?`<img src="${OBJB}${k}.png" style="width:80%;height:80%;object-fit:contain;border:0;border-radius:0">`:'', c]; };
      h += `<div class="ow2 bz ovcard" data-key="${w.key||''}" style="${pos};width:${px(200)};border-radius:${px(14)};padding:${px(14)};font-size:${px(12)}">
        ${c.kda?`<div style="margin-left:${px(14)}"><b style="font-size:${px(13)}">KDA</b><div style="font-size:${px(18)};font-weight:800">${esc(c.kda)}</div></div>
        <div style="margin:${px(8)} 0 0 ${px(14)}"><b style="font-size:${px(13)}">CS</b><div class="r1" style="justify-content:space-between"><b style="font-size:${px(18)};color:${c.csColor}">${esc(c.cs)}</b><span style="color:#8C93A3;font-size:${px(10.5)}">${esc(c.csSub)}</span></div></div>`:''}
        ${(c.rows||[]).map(l=>`<div class="ln" style="height:${px(20)};padding-left:${px(14)}"><span style="color:#8C93A3;font-weight:700">${esc(l.text)}</span><span><b style="color:${l.color}">${esc(l.right)}</b>${l.extra?`<span class="ex"> / ${esc(l.extra)}</span>`:''}</span></div>`).join('')}
        ${(c.objs||[]).length?(c.kda?`<div class="sepl" style="margin:${px(10)} 0"></div>`:'')+c.objs.map(o=>{const [s,col]=objIc(o.text); return `<div class="r1" style="gap:${px(12)};height:${px(44)}"><span class="oi" style="width:${px(34)};height:${px(34)};border-color:${col};color:${col};font-size:${px(16)}">${s}</span><div><b style="font-size:${px(12)}">${esc(o.text)}</b><div style="font-size:${px(15)};font-weight:800;color:${o.color}">${esc(o.right)}</div></div></div>`}).join(''):''}</div>`;
      continue; }
    if(w.teams){ const row=(p,col)=>`<div class="tr ${p.me?'me':''}" style="height:${px(52)};gap:${px(12)}"><img src="${p.champ?`${DD}${S.v}/img/champion/${p.champ}.png`:''}" style="width:${px(38)};height:${px(38)};border-color:${col}"><div class="sp"><b style="font-size:${px(13)}">${esc(p.champName||'')}</b><div style="color:#8C93A3;font-size:${px(11)}">${esc(p.name)}</div></div><div style="text-align:right"><b style="color:${p.rankColor};font-size:${px(12)}">${esc(p.rank)}</b><div style="color:${p.wrColor};font-size:${px(11)};font-weight:700">${esc(p.wr)}${p.games?' · '+esc(p.games):''}</div></div></div>`;
      h += `<div class="ow2 bz ovteams" data-key="${w.key||''}" style="left:${left}px;top:${top}px;transform:translateX(-50%);width:${px(780)};border-radius:${px(14)};padding:${px(10)} ${px(16)} ${px(16)};gap:${px(16)}">
        <div><div class="th" style="color:#38D6FF;font-size:${px(11)};height:${px(30)}">LA TUA SQUADRA</div>${w.teams.ally.map(p=>row(p,'#38D6FF')).join('')}</div>
        <div><div class="th" style="color:#FF5C7A;font-size:${px(11)};height:${px(30)}">SQUADRA NEMICA</div>${(w.teams.enemy||[]).map(p=>row(p,'#FF5C7A')).join('')}</div></div>`;
      continue; }
    if(w.segs?.length){
      h += `<div class="ow2 bz seg1" data-key="${w.key||''}" style="${pos};font-size:${px(11.5)};padding:0 ${px(8)};height:${px(24)};gap:${px(5)};border-radius:${px(6)}">${w.segs.map(sg=>`<span style="color:${sg.color};font-weight:${sg.bold?800:500}">${esc(sg.text)}</span>`).join('')}</div>`;
      continue;
    }
    h += `<div class="ow2 bz" data-key="${w.key||''}" style="${pos};width:${px(w.width)};border-radius:${px(8)};font-size:${px(11.5)}">
      ${w.title?`<div class="hd" style="height:${px(26)};padding:0 ${px(10)};font-size:${px(10.5)}"><span class="sp">${esc(w.title)}</span>${w.icons?`<span class="ics" style="gap:${px(4)}"><i class="${w.locked?'on':''}">${w.locked?'🔒':'🔓'}</i><i>⚙</i><i>✕</i></span>`:''}</div>`:''}
      <div style="padding:${px(8)} ${px(10)}">${(w.lines||[]).map(l=>`<div class="ln" style="height:${px(l.size>=16?26:18)}"><span style="color:${l.right?'#E6E8EE':l.color};font-weight:700">${esc(l.text)}</span><span><b style="color:${l.color}">${esc(l.right||'')}</b>${l.extra?`<span class="ex"> / ${esc(l.extra)}</span>`:''}</span></div>`).join('')}</div></div>`;
  }
  el.innerHTML = h;
}
// trascinamento degli elementi nell'anteprima
function bindPreviewDrag(){
  const el = $('#screen'); if(!el) return;
  el.addEventListener('mousedown', e=>{
    const w = e.target.closest('.ow2'); if(!w || !w.dataset.key) return;
    e.preventDefault();
    const key = w.dataset.key, r = el.getBoundingClientRect(), wr = w.getBoundingClientRect();
    const center = w.style.transform.includes('translateX');
    const offX = e.clientX - (center ? wr.left + wr.width/2 : wr.left), offY = e.clientY - wr.top;
    S.ovDrag = true; w.classList.add('dragging');
    const move = ev => {
      const x = Math.min(1, Math.max(0, (ev.clientX - offX - r.left)/r.width)), y = Math.min(1, Math.max(0, (ev.clientY - offY - r.top)/r.height));
      w.style.left = x*r.width+'px'; w.style.top = y*r.height+'px'; w._x = x; w._y = y;
    };
    const up = () => {
      document.removeEventListener('mousemove', move); document.removeEventListener('mouseup', up);
      w.classList.remove('dragging'); S.ovDrag = false;
      if(w._x!==undefined){ setSetting(`overlay.${key}.x`, +w._x.toFixed(4)); setSetting(`overlay.${key}.y`, +w._y.toFixed(4)); }
      setTimeout(drawOverlayPreview, 250);
    };
    document.addEventListener('mousemove', move); document.addEventListener('mouseup', up);
  });
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

/* ================= IMPOSTAZIONI ================= */
function pageSettings(m){
  const s = S.settings;
  m.innerHTML = `<div class="page"><h1 class="mb">Impostazioni</h1><div class="grid2">
    <div class="col gap16">
      <div class="card"><h2>Aspetto</h2>
        <div class="setting"><div class="lbl"><b>Tema</b></div><div class="seg"><button data-act="theme" data-k="dark" class="${s.theme!=='light'?'on':''}">Scuro</button><button data-act="theme" data-k="light" class="${s.theme==='light'?'on':''}">Chiaro</button></div></div>
        <div class="setting"><div class="lbl"><b>Versione</b><span class="small muted">Gli aggiornamenti arrivano da soli</span></div><span class="small">${esc(S.st?.update?.current||'')}</span></div>
        <div class="setting"><div class="lbl"><b>Animazioni</b></div>${sw('animations',s.animations!==false)}</div>
        <div class="setting"><div class="lbl"><b>Avvia con Windows</b><span class="small muted">Parte nascosta vicino all'orologio e si apre da sola in champion select</span></div>${sw('autostart',!!s.autostart)}</div>
        <div class="setting"><div class="lbl"><b>Notifiche di Windows</b><span class="small muted">Partita trovata, build importata, LP guadagnati o persi</span></div>${sw('notifications',s.notifications!==false)}</div>
        <div class="setting"><div class="lbl"><b>Fonte statistiche</b><span class="small muted">Tier list, build, rune e counter per il tuo elo</span></div><select data-set="opggOff" data-bool="1">${opt([['false','OP.GG (consigliato)'],['true','Solo il mio server']], String(!!s.opggOff))}</select></div>
        <div class="setting"><div class="lbl"><b>Barra di Windows</b><span class="small muted">Usa la cornice classica al posto di quella dell'app</span></div>${sw('nativeFrame',!!s.nativeFrame)}</div></div>
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
      <div class="card"><h2>Server statistiche</h2>
        <p class="small muted">Se hai caricato il motore sul tuo sito (cartella <b>bs-data</b>), raccoglie partite 24 ore su 24 anche a PC spento. BetterStomp scarica i dati ogni 5 minuti e li unisce ai suoi.</p>
        <div class="setting" style="flex-wrap:wrap"><div class="lbl"><b>Indirizzo</b></div><input type="text" data-set="remoteUrl" placeholder="https://www.aversano.design/bs-data/" value="${esc(s.remoteUrl||'')}" style="width:100%"></div>
        <div class="setting" style="flex-wrap:wrap"><div class="lbl"><b>Codice segreto</b><small>Lo stesso scritto in config.php sul server</small></div><input type="password" data-set="remoteToken" value="${esc(s.remoteToken||'')}" style="width:100%"></div>
        <div id="remotebox" class="small"></div></div>
      <div class="card"><div class="ctitle"><h2>Pro player da seguire</h2><span class="sp"></span><button class="btn ghost sm" data-act="addpro">+ Aggiungi</button></div>
        <p class="small muted">Le loro partite più recenti compaiono in champion select, sotto "Pro e Master+". Inserisci il Riot ID dell'account (Nome#TAG) e la regione: ne viene controllato uno ogni 3 minuti.</p>
        <div id="prolist"></div></div>
      <div class="card"><h2>Client di League</h2>
        <div class="setting" style="flex-wrap:wrap"><div class="lbl"><b>Cartella di installazione</b><small>Lascia vuoto per il rilevamento automatico</small></div>
          <input type="text" data-set="leaguePath" placeholder="C:\\Riot Games\\League of Legends" value="${esc(s.leaguePath||'')}" style="width:100%"></div></div>
      <div class="card flat"><p class="tiny dim">BetterStomp isn't endorsed by Riot Games and doesn't reflect the views or opinions of Riot Games or anyone officially involved in producing or managing Riot Games properties.</p></div>
    </div></div></div>`;
  updateEngineBox();
  drawProList();
}
function drawProList(){
  const el = $('#prolist'); if(!el) return;
  const l = S.settings.proPlayers||[];
  el.innerHTML = l.length ? l.map((p,i)=>`<div class="row" style="margin-bottom:8px"><input type="text" placeholder="Nome (es. Caps)" value="${esc(p.label)}" data-pro="${i}" data-f="label" style="width:120px">
    <input type="text" placeholder="Riot ID (Nome#TAG)" value="${esc(p.riotId)}" data-pro="${i}" data-f="riotId" style="flex:1"><select data-pro="${i}" data-f="region">${opt(REGIONS,p.region||S.settings.region)}</select>
    <button class="iconbtn" data-act="delpro" data-i="${i}" title="Rimuovi">✕</button></div>`).join('') : `<div class="small dim">Nessun pro inserito. Anche senza, vedi comunque le partite dei giocatori Master, Gran Maestro e Challenger.</div>`;
}
function updateEngineBox(){
  const rb = $('#remotebox'), e0 = S.st?.engine;
  if(rb && e0){ const h = !e0.hasRemote ? '<span class="dim">Non configurato</span>' : e0.remoteErr ? `<span class="bad">${esc(e0.remoteErr)}</span>`
      : `<span class="good">● Collegato</span> · <b>${num(e0.remote)}</b> partite sul server${e0.remoteAt?` · aggiornato ${ago(e0.remoteAt*1000)}`:''}`;
    if(rb._h!==h){ rb.innerHTML=h; rb._h=h; } }
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
    S.ui.openAn = S.ui.openAn || {};
    if(el.style.display!=='none'){ el.style.display='none'; delete S.ui.openAn[g.id]; return; }
    S.ui.openAn[g.id] = true;
    el.innerHTML = analysisHTML(g);
    el.style.display='block';
  },
  openchamp(t){ if(!t.dataset.id) return; if(t.dataset.pos && t.dataset.pos!=='ALL'){ S.ui.chPos=t.dataset.pos; S.ui.chId=t.dataset.id; }
    if(route()[0]==='tier'){ S.ui.chMode=S.ui.tmode; S.ui.chTier=S.ui.ttier; } if(route()[0]==='campioni'){ S.ui.chMode=S.ui.cmode; S.ui.chTier=S.ui.ctier; }
    location.hash='#/campione/'+t.dataset.id; },
  cpos(t){ S.ui.cpos=t.dataset.p; document.querySelectorAll('[data-act=cpos]').forEach(c=>c.classList.toggle('on',c===t)); drawChampGrid(); },
  chpos(t){ S.ui.chPos=t.dataset.p; render(); },
  tpos(t){ S.ui.tpos=t.dataset.p; render(); },
  tsort(t){ const k=t.dataset.k; if(S.ui.tsort===k) S.ui.tdir=-(S.ui.tdir||-1); else { S.ui.tsort=k; S.ui.tdir=k==='name'?1:-1; } drawTier(); },
  async import(){ const d=S.ui.chData; try{ const r=await api('/api/import',{champ:S.ui.chKey,pos:d.build.position,mode:S.ui.chMode,runes:false,spells:true,items:true,variant:S.ui.variant||0}); const rr=curRunes(); if(rr) await api('/api/import',{champ:S.ui.chKey,pos:d.build.position,runes:true,spells:false,items:false,custom:{runes:rr}}); toast(r.message||'Fatto'); }catch(e){ toast(e.message); } },
  async selimport(){ const b=S.ui.selBuild; if(!b) return; const pk=S.ui.selPick||{t:'var',i:0};
    const body={champ:b.champ,pos:b.pos,mode:b.mode,runes:true,spells:true,items:true,variant:pk.t==='var'?pk.i:0};
    if(pk.t==='pro'){ const v=selView(S.ui.selData); body.custom={runes:v.runes,spells:v.spells,starting:v.starting,core:v.core,situational:v.situational}; }
    try{ const r=await api('/api/import',body); toast(r.message||'Fatto'); }catch(e){ toast(e.message); } },
  stab(t){ S.ui.stab=t.dataset.k; S.ui.ssig=''; $('#csbody').innerHTML=''; updateSelect(true); },
  vsort(t){ S.ui.vsort=t.dataset.k; redrawSelBuild(); },
  pickvar(t){ S.ui.selPick={t:'var',i:+t.dataset.i}; redrawSelBuild(); },
  pickpro(t){ S.ui.selPick={t:'pro',i:+t.dataset.i}; redrawSelBuild(); },
  suggpick(t){ const s=selState(); S.ui.over['a'+s.meIdx]=+t.dataset.k; S.ui.stab='build'; S.ui.ssig=''; S.ui.sbuildKey=''; $('#csbody').innerHTML=''; updateSelect(true); },
  sidemore(t){ S.ui.sideMore=S.ui.sideMore||{}; S.ui.sideMore[t.dataset.k]=!S.ui.sideMore[t.dataset.k]; $('#side')._h=''; renderSide(); },
  addpro(){ const l=[...(S.settings.proPlayers||[]),{label:'',riotId:'',region:S.settings.region}]; setSetting('proPlayers',l); drawProList(); },
  delpro(t){ const l=[...(S.settings.proPlayers||[])]; l.splice(+t.dataset.i,1); setSetting('proPlayers',l); drawProList(); },
  cq2(t){ S.ui.cq2=t.dataset.k; $('#ptab').innerHTML=profileTab(S.ui.prof); },
  cr2(t){ S.ui.cr2=t.dataset.k; $('#ptab').innerHTML=profileTab(S.ui.prof); },
  benchtier(t){ S.ui.btier=t.dataset.k; S.ui.bench=null; render(true); },
  mq(t){ S.ui.mq=t.dataset.q; S.ui.mShow=20; render(true); },
  mmore(){ S.ui.mShow=(S.ui.mShow||20)+20; render(true); },
  kssel(t){ S.ui.ksSel=+t.dataset.k; S.ui.runeSet=0; refreshEditor(); },
  runeset(t){ S.ui.runeSet=+t.dataset.i; refreshEditor(); },
  async otpimp(t){ const d=S.ui.chData, g=(d.build.proGames||[])[+t.dataset.i]; if(!g) return;
    const boots=g.items.filter(id=>S.items[id]?.tags?.includes('Boots')), core=[...boots,...g.items.filter(id=>!boots.includes(id))];
    const rp=parseRuneJS(g.runes);
    try{ const r=await api('/api/import',{champ:S.ui.chKey,pos:d.build.position,runes:!!rp,spells:true,items:true,custom:{runes:rp,spells:g.spells,starting:d.build.starting||[],core,situational:[]}}); toast((r.message||'Fatto')+(rp?'':' (rune: solo keystone disponibile, pagina non importata)')); }catch(e){ toast(e.message); } },
  async imprunes(){ const rr=curRunes(); if(!rr) return toast('Nessuna pagina'); try{ const r=await api('/api/import',{champ:S.ui.chKey,pos:S.ui.chData.build.position,runes:true,spells:false,items:false,custom:{runes:rr}}); toast(r.message||'Rune importate'); }catch(e){ toast(e.message); } },
  async impitems(){ const d=S.ui.chData; try{ const r=await api('/api/import',{champ:S.ui.chKey,pos:d.build.position,mode:S.ui.chMode,runes:false,spells:false,items:true,variant:S.ui.variant||0}); toast(r.message||'Oggetti importati'); }catch(e){ toast(e.message); } },
  runesel(t){ const i=+t.dataset.i; S.ui.runeSel = S.ui.runeSel===i?null:i; refreshEditor(); },
  runetree(){ S.ui.runeTree=!S.ui.runeTree; refreshEditor(); },
  abtab(t){ S.ui.abSel=+t.dataset.i; const d=S.ui.chDetail?.d; if(d) t.closest('.card').innerHTML='<h2>Abilità</h2>'+abilitiesHTML(d.data[S.ui.chDetail.id]); },
  variant(t){ S.ui.runeSel=null; S.ui.ksSel=undefined; S.ui.runeSet=0; S.ui.variant=+t.dataset.i; $('#buildbox').innerHTML=buildHTML(S.ui.chData.build, S.ui.chData.personal, S.ui.variant, 'variant'); },
  editbuild(){ S.ui.editing=!S.ui.editing; S.ui.edit=null; $('#buildbox').innerHTML=buildHTML(S.ui.chData.build, S.ui.chData.hasPersonal?S.ui.chData.personal:null, S.ui.variant, 'variant');
    document.querySelector('[data-act=editbuild]').textContent = S.ui.editing?'Chiudi editor':'✎ Build personale'; },
  pickitem(t,e){ if(e.target.closest('.x')) return; itemPicker(id=>{ const b=t.dataset.b, i=+t.dataset.i; const arr=S.ui.edit[b]; arr[i]=id; S.ui.edit[b]=arr.map(x=>x||0); refreshEditor(); }); },
  clearitem(t){ const b=t.dataset.b, i=+t.dataset.i; S.ui.edit[b].splice(i,1); refreshEditor(); },
  async savebuild(){ const e=S.ui.edit; const cur=S.ui.chData.hasPersonal?S.ui.chData.personal:{};
    await api('/api/personal?id='+S.ui.chKey, {...cur, starting:e.starting.filter(Boolean), core:e.core.filter(Boolean), situational:e.situational.filter(Boolean)});
    toast('Build personale salvata'); S.ui.editing=false; render(); },
  async resetbuild(){ await api('/api/personal?id='+S.ui.chKey, {starting:[],core:[],situational:[]}); toast('Build personale eliminata'); S.ui.editing=false; render(); },
  async saverunes(){ const d=S.ui.chData; const cur=d.hasPersonal?d.personal:{}; const pg=d.build.runePages||[]; const rr=(S.ui.runeSel!=null&&pg[S.ui.runeSel])||d.build.variants?.[S.ui.variant]?.runes||d.build.runes; await api('/api/personal?id='+S.ui.chKey, {...cur, runes:rr, spells:d.build.spells}); toast('Rune e incantesimi salvati come personali'); },
  skind(t){ S.ui.skind=t.dataset.k; document.querySelectorAll('[data-act=skind]').forEach(b=>b.classList.toggle('on',b===t)); S.ui.ssig=''; updateSelect(true); },
  override(t){ champPicker(k=>{ S.ui.over[t.dataset.k]=k; S.ui.ssig=''; S.ui.sbuildKey=''; updateSelect(true); }); },
  pickopp(){ champPicker(k=>{ S.ui.sopp=k; S.ui.soppManual=true; S.ui.ssig=''; updateSelect(true); }); },
  pf(t){ S.ui.pf=t.dataset.k; document.querySelectorAll('[data-act=pf]').forEach(c=>c.classList.toggle('on',c===t)); drawPatch(); },
  patchrefresh(){ pagePatch($('#main'), route(), false, true); },
  async resetoverlay(){ S.settings = await api('/api/settings/reset-overlay'); render(); toast('Overlay ripristinato'); },
  ovset(t){ const f = OVF.find(x=>x.k===t.dataset.k); if(!f) return;
    const d = modal(`${f.ic} ${f.n}`, `<div>${ovSettingsHTML(f.k)}</div>`); d.querySelector('.box').classList.add('bzbox'); },
  addrem(){ const l=[...(S.settings.overlay.notifyBefore||[]), 30]; setSetting('overlay.notifyBefore', l); $('#remlist').innerHTML=remListHTML(); },
  delrem(t){ const l=[...(S.settings.overlay.notifyBefore||[])]; l.splice(+t.dataset.i,1); setSetting('overlay.notifyBefore', l); $('#remlist').innerHTML=remListHTML(); },
  editmode(t){ const on=!S.settings.overlay.editMode; setSetting('overlay.editMode', on); t.classList.toggle('ghost',!on); t.textContent = on?'✓ Fine modifica':'✥ Sposta gli elementi sullo schermo';
    toast(on?'Modalità modifica attiva: trascina gli elementi sullo schermo, poi premi "Fine modifica"':'Posizioni salvate'); },
  copyobs(){ navigator.clipboard.writeText($('#obsurl').value).then(()=>toast('Indirizzo copiato')); },
  theme(t){ setSetting('theme', t.dataset.k); document.querySelectorAll('[data-act=theme]').forEach(b=>b.classList.toggle('on',b===t)); },
  flash(t){ setSetting('flashKey', t.dataset.k); document.querySelectorAll('[data-act=flash]').forEach(b=>b.classList.toggle('on',b===t)); },
  showkey(){ const i=$('#apikey'); i.type = i.type==='password'?'text':'password'; },
  savekey(){ setSetting('riotApiKey', $('#apikey').value.trim()); toast('Chiave salvata: il motore statistiche parte in pochi secondi'); },
};
function refreshEditor(){ $('#buildbox').innerHTML = buildHTML(S.ui.chData.build, S.ui.chData.personal, S.ui.variant, 'variant'); }

document.addEventListener('click', e=>{
  const t = e.target.closest('[data-act]'); if(!t) return;
  const f = actions[t.dataset.act]; if(f){ e.preventDefault(); f(t, e); }
});
document.addEventListener('change', e=>{
  const t = e.target;
  if(t.dataset.rem!==undefined){ const l=[...(S.settings.overlay.notifyBefore||[])]; l[+t.dataset.rem]=Math.max(1, parseFloat(t.value)||60); setSetting('overlay.notifyBefore', l); return; }
  if(t.dataset.benchelo){ if(t.value==='off') setSetting('overlay.csGoal','off'); else { setSetting('overlay.csGoal','elo'); setSetting('overlay.csGoalTier', t.value); } return; }
  if(t.dataset.bool && t.dataset.set){ setSetting(t.dataset.set, t.value==='true'); return; }
  if(t.dataset.pro!==undefined){ const l=[...(S.settings.proPlayers||[])]; const i=+t.dataset.pro; l[i]={...l[i],[t.dataset.f]:t.value.trim()}; setSetting('proPlayers',l); return; }
  if(t.dataset.set){
    if(t.dataset.set==='_why'){ S.ui.why=t.checked; drawPatch(); return; }
    if(t.dataset.set==='autoAll'){ ['autoRunes','autoSpells','autoItems'].forEach(k=>setSetting(k, t.checked)); toast(t.checked?'Import automatico attivo':'Import automatico spento'); return; }
    setSetting(t.dataset.set, t.type==='checkbox' ? t.checked : t.value);
    if(t.dataset.set==='stream.goalTier' || t.dataset.set==='overlay.csGoal') render();
    if(t.type==='checkbox') document.querySelectorAll(`[data-set="${t.dataset.set}"]`).forEach(x=>x.checked=t.checked);
  }
  if(t.dataset.ui){ S.ui[t.dataset.ui] = t.dataset.num ? +t.value : t.value; if(t.dataset.ui==='sopp'){ S.ui.soppManual=+t.value>0; } if(t.dataset.ui==='chMode') S.ui.chPos=''; if(['sopp','spos','smode','selTier'].includes(t.dataset.ui)){ S.ui.ssig=''; if(t.dataset.ui!=='sopp') S.ui.sbuildKey=''; updateSelect(true); } else render(); }
});
document.addEventListener('input', e=>{
  const t = e.target;
  if(t.dataset.setnum){ setSetting(t.dataset.setnum, parseFloat(t.value)); if(route()[0]==='overlay'){ const sm=t.closest('.setting')?.querySelector('small'); if(sm) sm.textContent=t.value+'px'; drawOverlayPreview(); } }
});
$('#themebtn').addEventListener('click', ()=>{ setSetting('theme', S.settings.theme==='light'?'dark':'light'); if(route()[0]==='impostazioni') render(); });

boot();
