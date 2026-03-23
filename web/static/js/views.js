import S from './state.js';
import {get,set,remove,getJSON,setJSON} from './storage.js';
import {$,esc,escJs,nameColor,fmtTime,md,toast,t,api,showLoading} from './util.js';
import {connectWS,registerPush} from './ws.js';

// ── Messages ──
export function addMsg(m){const el=$('msgs');if(el.querySelector('.m-empty'))el.innerHTML='';
  if(m.date){const d=new Date(m.date);const ds=d.toDateString();if(ds!==S.lastMsgDate){S.lastMsgDate=ds;const now=new Date();let label;if(ds===now.toDateString())label=S.lang==='ru'?'Сегодня':'Today';else{const y=new Date(now);y.setDate(y.getDate()-1);if(ds===y.toDateString())label=S.lang==='ru'?'Вчера':'Yesterday';else label=d.toLocaleDateString(S.lang==='ru'?'ru':'en',{day:'numeric',month:'long'})}el.innerHTML+=`<div class="day-sep"><span>${label}</span></div>`}}
  const who=m.sender==='bot'?(m.bot_name||$('hdr-title').textContent.replace('# ','')||'Bot'):(m.sender_name||get('uname')||'You');const tm=fmtTime(m.date);
  const isBot=m.sender==='bot';const isMine=m.sender==='user'&&m.sender_name===get('uname');
  const txt=m.text||'';let alertCls='';if(txt.includes('**ALERT**')||txt.includes('status: firing'))alertCls=' m-alert';if(txt.includes('**Resolved**')||txt.includes('status: resolved'))alertCls=' m-resolved';if(txt.includes('**ACK**')||txt.includes('**Muted'))alertCls=' m-acked';
  let h=`<div class="m${isBot?' m-bot':''}${alertCls}" id="m-${m.message_id}" data-sender="${esc(m.sender)}" data-sname="${esc(m.sender_name||'')}" data-mine="${isMine?1:0}"><div class="m-ava" style="background:${nameColor(who)}">${who[0].toUpperCase()}</div>`;
  if(m.reply_to){const orig=document.getElementById('m-'+m.reply_to);const qtext=orig?orig.querySelector('.m-text')?.textContent?.substring(0,40):(S.lang==='ru'?'Удалённое сообщение':'Deleted message');const qname=orig?orig.querySelector('.m-name')?.textContent:'';h+=`<div class="m-quote"><b>${esc(qname)}</b> ${esc(qtext)}</div>`}
  const editedTag=m.edited_at?`<span class="m-edited">(${S.lang==='ru'?'ред.':'edited'})</span>`:'';
  h+=`<div class="m-head"><span class="m-name">${esc(who)}</span><span class="m-time">${tm}</span>${editedTag}</div>`;
  if(alertCls===' m-alert'&&m.date){h+=`<span class="m-elapsed" data-ts="${new Date(m.date).getTime()}"></span>`}
  if(m.file_id){if(m.file_type==='photo')h+=`<img src="/file/${m.file_id}?token=${S.token}" class="m-photo" onclick="window.open('/file/${m.file_id}?token=${S.token}','_blank')">`;else if(m.file_type==='voice')h+=`<audio controls src="/file/${m.file_id}?token=${S.token}" class="m-audio"></audio>`;else if(m.file_type==='video')h+=`<video controls src="/file/${m.file_id}?token=${S.token}" class="m-video"></video>`;else h+=`<a href="/file/${m.file_id}?token=${S.token}" target="_blank" class="m-file-link">${esc(m.text||'File')}</a>`}
  h+=`<div class="m-text">${md(m.text||'')}</div>`;
  if(m.reply_markup){try{const kb=typeof m.reply_markup==='string'?JSON.parse(m.reply_markup):m.reply_markup;if(kb.inline_keyboard){h+='<div class="m-kb">';kb.inline_keyboard.forEach(row=>{h+='<div class="m-kb-row">';row.forEach(btn=>{h+=`<button class="m-kb-btn" data-cb="${btn.callback_data}" data-mid="${m.message_id}" onclick="window.onCb(this)">${esc(btn.text)}</button>`});h+='</div>'});h+='</div>'}}catch{}}
  if(S.curChan)h+=`<button class="m-del" onclick="event.stopPropagation();window.startReply(${m.message_id},'${escJs(who)}','${escJs((m.text||'').substring(0,40))}')">↩</button>`;
  if(S.curChat)h+=`<button class="m-del" onclick="window.onDel(${m.message_id})">x</button>`;h+='</div>';el.innerHTML+=h;if(alertCls===' m-alert')updateElapsed()}

export function updateElapsed(){document.querySelectorAll('.m-elapsed').forEach(el=>{const ts=parseInt(el.dataset.ts);if(!ts)return;const sec=Math.floor((Date.now()-ts)/1000);let txt,cls;if(sec<60){txt=sec+(S.lang==='ru'?' сек':' sec');cls='el-ok'}else if(sec<3600){const m=Math.floor(sec/60);txt=m+(S.lang==='ru'?' мин':' min');cls=m<5?'el-ok':m<15?'el-warn':'el-crit'}else{const h=Math.floor(sec/3600);const m=Math.floor((sec%3600)/60);txt=h+(S.lang==='ru'?'ч ':' h ')+m+(S.lang==='ru'?' мин':' min');cls='el-crit'}el.textContent=txt;el.className='m-elapsed '+cls})}
S.elapsedTimer=setInterval(updateElapsed,30000);

export function scrollDown(){const el=$('msgs');setTimeout(()=>el.scrollTop=el.scrollHeight,50)}

export function renderPinBar(mid,text,who){$('pinned-bar').innerHTML=`<div class="m-pinned"><span class="pin-text" onclick="document.getElementById('m-${mid}')?.scrollIntoView({behavior:'smooth'})"><b>📌</b> ${md(text.substring(0,80))}</span><span class="pin-who">${esc(who||'')}</span><button class="pin-close" onclick="window.unpinMsg()">✕</button></div>`}

// ── Auth ──
export function auth(r){S.token=r.token;set('token',S.token);set('uid',r.user_id);set('uname',r.username||'');if(r.role)set('role',r.role);if(r.org){set('org',r.org);const orgs=getJSON('orgs')||{};orgs[r.org]={token:r.token,user:r.username,name:r.org,role:r.role||'member'};setJSON('orgs',orgs)}showApp()}

export function logout(){S.token=null;S.curChat=null;S.curChan=null;remove('token');remove('uid');remove('uname');remove('view');if(window.disconnectWS)window.disconnectWS();$('landing').style.display='flex';$('auth').style.display='none';$('app').style.display='none';$('fab').style.display='none';$('settings').style.display='none';$('settings-bg').style.display='none';window.initLandingChat()}

// ── App views ──
export async function showApp(){$('auth').style.display='none';$('app').style.display='flex';const u=get('uname')||'?';$('hdr-ava').textContent=u[0].toUpperCase();$('hdr-ava').style.background=nameColor(u);$('hdr-name').textContent=u;connectWS();registerPush();await showList();const v=get('view');if(v){try{const o=JSON.parse(v);if(o.t==='chat')openChat(o.id,o.n);else if(o.t==='ch')openChan(o.id,o.n)}catch{}}}

export async function showList(){S.curChat=null;S.curChan=null;S.replyToId=0;$('reply-bar').style.display='none';$('pinned-bar').innerHTML='';$('typing-bar').style.display='none';remove('view');$('main-list').style.display='block';$('chat-view').style.display='none';$('input-bar').style.display='none';$('hdr-back').style.display='none';$('hdr-title').textContent=t('main');$('hdr-online').textContent='';const isGuest=get('uname')==='guest';$('fab').style.display=isGuest?'none':'flex';
  showLoading(el);
  const[bots,chs,health]=await Promise.all([api('GET','/api/bots'),api('GET','/api/channels'),api('GET','/api/health')]);
  const el=$('main-list');el.innerHTML='';S.channels=chs||[];
  if(chs&&chs.length){el.innerHTML+=`<div class="sec-title">${t('ch')}</div>`;for(const c of chs){const cls=c.subscribed?'sub-btn on':'sub-btn';const txt=c.subscribed?t('sub_on'):t('sub_off');el.innerHTML+=`<div class="ch-row"><span class="ch-hash" onclick="window.openChan(${c.id},'${escJs(c.name)}')">#</span><div class="ch-body" onclick="window.openChan(${c.id},'${escJs(c.name)}')"><div class="ch-name">${esc(c.name)}${c.unread>0?`<span class="ch-unread ch-badge ch-badge-${c.id}">${c.unread}</span>`:`<span class="ch-unread ch-badge ch-badge-${c.id}" style="display:none"></span>`}</div><div class="ch-desc">${esc(c.description||'')}</div></div><button class="${cls}" onclick="window.toggleSub(event,${c.id})">${txt}</button></div>`}}
  const isAdmin=get("role")==="admin"||get("uid")==="1";
  if(bots&&bots.length){el.innerHTML+=`<div class="sec-title">${t('bots')}</div>`;for(const b of bots){let bh=`<div class="bot-row" onclick="window.openChat(${b.id},'${escJs(b.name)}')"><div class="bot-ava" style="background:${nameColor(b.name)}">${b.name[0].toUpperCase()}</div><div><div class="bot-name">${esc(b.name)}</div>`;if(isAdmin)bh+=`<div class="bot-token" onclick="event.stopPropagation();if(this.dataset.show){navigator.clipboard.writeText('${escJs(b.token)}');this.textContent=window.t('copied');setTimeout(()=>{this.textContent='token: ${escJs(b.token)}'},2000)}else{this.dataset.show='1';this.textContent='token: ${escJs(b.token)} ('+window.t('tap_copy')+')'}">${t('show_tok')}</div>`;bh+=`</div></div>`;el.innerHTML+=bh}}
  if((!bots||!bots.length)&&(!chs||!chs.length))el.innerHTML=`<div class="m-empty">${t('no_items')}</div>`;
  const v=health||{};el.innerHTML+=`<div class="list-foot">${t('foot')(bots?bots.length:0,chs?chs.length:0,v.online||0)}</div>`;$('main-list').focus()}

export async function toggleSub(ev,chId){ev.stopPropagation();const btn=ev.target;const on=btn.classList.contains('on');btn.style.opacity='0.5';await api('POST',`/api/channels/${chId}/${on?'unsubscribe':'subscribe'}`);btn.classList.toggle('on');btn.textContent=on?t('sub_off'):t('sub_on');btn.style.opacity='1'}

export async function openChat(botId,name){S.lastMsgDate='';setJSON('view',{t:'chat',id:botId,n:name});history.pushState(null,'',location.href);$('fab').style.display='none';$('pinned-bar').innerHTML='';S.replyToId=0;$('reply-bar').style.display='none';$('typing-bar').style.display='none';$('main-list').style.display='none';$('chat-view').style.display='flex';$('input-bar').style.display='flex';$('hdr-back').style.display='inline';$('hdr-title').textContent=name;$('msg-in').placeholder=t('msg');api('GET','/api/online').then(r=>{if(!r)return;let s='';if(r.online>0)s=r.online+' online';const away=(r.total_connected||0)-r.online;if(away>0)s+=(s?', ':'')+away+' away';$('hdr-online').textContent=s});const chat=await api('POST',`/api/bots/${botId}/start`);S.curChat=chat.id;const msgs=await api('GET',`/api/chats/${chat.id}/messages`);$('msgs').innerHTML='';if(msgs&&msgs.length)msgs.reverse().forEach(m=>addMsg(m));else $('msgs').innerHTML=`<div class="m-empty">${t('no_msg')}</div>`;scrollDown();$('msg-in').focus()}

export async function openChan(chId,name){S.lastMsgDate='';setJSON('view',{t:'ch',id:chId,n:name});history.pushState(null,'',location.href);$('fab').style.display='none';S.curChan=chId;S.curChat=null;S.replyToId=0;$('reply-bar').style.display='none';$('typing-bar').style.display='none';$('main-list').style.display='none';$('chat-view').style.display='flex';$('input-bar').style.display='flex';$('hdr-back').style.display='inline';$('hdr-title').textContent='# '+name;$('msg-in').placeholder='Написать в #'+name+'...';api('GET','/api/online').then(r=>{if(!r)return;let s='';if(r.online>0)s=r.online+' online';const away=(r.total_connected||0)-r.online;if(away>0)s+=(s?', ':'')+away+' away';$('hdr-online').textContent=s});api('GET','/api/users').then(r=>{if(r&&Array.isArray(r))S.mentionUsers=r});const msgs=await api('GET',`/api/channels/${chId}/messages`);$('msgs').innerHTML='';const chInfo=S.channels.find(c=>c.id===chId);const pinnedId=chInfo?chInfo.pinned_message_id:0;$('pinned-bar').innerHTML='';if(msgs&&msgs.length){msgs.reverse().forEach(m=>{if(!m.sender)m.sender='bot';addMsg(m)});if(pinnedId>0){const pinMsg=msgs.find(m=>m.message_id===pinnedId);if(pinMsg){renderPinBar(pinnedId,pinMsg.text||'')}}}else $('msgs').innerHTML=`<div class="m-empty">${t('no_msg')}</div>`;
  // Load more button for channels with 50+ messages
  if(msgs&&msgs.length>=50){const loadMore=document.createElement('div');loadMore.className='m-empty';loadMore.style.cursor='pointer';loadMore.textContent=S.lang==='ru'?'↑ Загрузить ещё':'↑ Load more';loadMore.onclick=async()=>{loadMore.textContent='...';const oldMsgs=await api('GET',`/api/channels/${chId}/messages?limit=200`);if(oldMsgs&&oldMsgs.length>msgs.length){S.lastMsgDate='';$('msgs').innerHTML='';oldMsgs.reverse().forEach(m=>{if(!m.sender)m.sender='bot';addMsg(m)})}else{loadMore.textContent=S.lang==='ru'?'Нет более старых':'No older messages'}};$('msgs').insertBefore(loadMore,$('msgs').firstChild)}
  // Alert filter bar
  const alertMsgs=$('msgs').querySelectorAll('.m-alert,.m-resolved,.m-acked');
  if(alertMsgs.length>0){const filterBar=document.createElement('div');filterBar.className='alert-filter';filterBar.innerHTML=`<button class="af-btn af-active" onclick="window.filterAlerts('all')">All</button><button class="af-btn" onclick="window.filterAlerts('firing')">${S.lang==='ru'?'Активные':'Firing'}</button><button class="af-btn" onclick="window.filterAlerts('resolved')">${S.lang==='ru'?'Решённые':'Resolved'}</button>`;$('msgs').insertBefore(filterBar,$('msgs').firstChild)}
  scrollDown();$('msg-in').focus();api('GET',`/api/channels/${chId}/readers`).then(readers=>{if(!readers||!readers.length)return;const myName=get('uname');const lastMsgId=msgs&&msgs.length?msgs[msgs.length-1].message_id:0;const whoRead=readers.filter(r=>r.last_read_id>=lastMsgId&&r.username!==myName);if(whoRead.length>0){const names=whoRead.map(r=>r.username).join(', ');const readDiv=document.createElement('div');readDiv.className='read-receipt';readDiv.textContent=(S.lang==='ru'?'Прочитано: ':'Read by: ')+names;$('msgs').appendChild(readDiv)}})}

// ── Navigation ──
$('hdr-back').onclick=showList;

// ── Window bindings ──
window.openChan=openChan;
window.openChat=openChat;
window.toggleSub=toggleSub;
