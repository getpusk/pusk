import S from './state.js';
import {get,set,remove,getJSON,setJSON} from './storage.js';
import {$,esc,escJs,nameColor,fmtTime,md,toast,t,api,showLoading} from './util.js';
import {connectWS,registerPush,disconnectWS} from './ws.js';

// ── Messages ──
export function addMsg(m){const el=$('msgs');if(el.querySelector('.m-empty'))el.innerHTML='';
  if(m.date){const d=new Date(m.date);const ds=d.toDateString();if(ds!==S.lastMsgDate){S.lastMsgDate=ds;const now=new Date();let label;if(ds===now.toDateString())label=S.lang==='ru'?'Сегодня':'Today';else{const y=new Date(now);y.setDate(y.getDate()-1);if(ds===y.toDateString())label=S.lang==='ru'?'Вчера':'Yesterday';else label=d.toLocaleDateString(S.lang==='ru'?'ru':'en',{day:'numeric',month:'long'})}const sep=document.createElement('div');sep.className='day-sep';sep.innerHTML=`<span>${label}</span>`;el.appendChild(sep)}}
  const who=m.sender==='bot'?(m.bot_name||$('hdr-title').textContent.replace('# ','')||'Bot'):(m.sender_name||get('uname')||'You');const tm=fmtTime(m.date);
  const isBot=m.sender==='bot';const isMine=m.sender==='user'&&m.sender_name===get('uname');
  const txt=m.text||'';let alertCls='';if(txt.includes('**ALERT**')||txt.includes('status: firing'))alertCls=' m-alert';if(txt.includes('**Resolved**')||txt.includes('status: resolved'))alertCls=' m-resolved';if(txt.includes('**ACK**')||txt.includes('**Muted'))alertCls=' m-acked';

  const wrap=document.createElement('div');
  wrap.className='m'+(isBot?' m-bot':'')+alertCls;
  wrap.id='m-'+m.message_id;
  wrap.dataset.sender=m.sender||'';
  wrap.dataset.sname=m.sender_name||'';
  wrap.dataset.mine=isMine?'1':'0';

  const ava=document.createElement('div');
  ava.className='m-ava';
  ava.style.background=nameColor(who);
  ava.textContent=who[0].toUpperCase();
  wrap.appendChild(ava);

  if(m.reply_to){const orig=document.getElementById('m-'+m.reply_to);const qtext=orig?orig.querySelector('.m-text')?.textContent?.substring(0,40):(S.lang==='ru'?'Удалённое сообщение':'Deleted message');const qname=orig?orig.querySelector('.m-name')?.textContent:'';const quote=document.createElement('div');quote.className='m-quote';quote.innerHTML='<b>'+esc(qname)+'</b> '+esc(qtext);wrap.appendChild(quote)}

  const head=document.createElement('div');
  head.className='m-head';
  const nameSpan=document.createElement('span');
  nameSpan.className='m-name';
  nameSpan.textContent=who;
  head.appendChild(nameSpan);
  const timeSpan=document.createElement('span');
  timeSpan.className='m-time';
  timeSpan.textContent=tm;
  head.appendChild(timeSpan);
  if(m.edited_at){const ed=document.createElement('span');ed.className='m-edited';ed.textContent=S.lang==='ru'?'(ред.)':'(edited)';head.appendChild(ed)}
  wrap.appendChild(head);

  if(alertCls===' m-alert'&&m.date){const elapsed=document.createElement('span');elapsed.className='m-elapsed';elapsed.dataset.ts=new Date(m.date).getTime();wrap.appendChild(elapsed)}

  if(m.file_id){
    if(m.file_type==='photo'){const img=document.createElement('img');img.src='/file/'+m.file_id+'?token='+S.token;img.className='m-photo';img.dataset.fileUrl='/file/'+m.file_id+'?token='+S.token;wrap.appendChild(img)}
    else if(m.file_type==='voice'){const audio=document.createElement('audio');audio.controls=true;audio.src='/file/'+m.file_id+'?token='+S.token;audio.className='m-audio';wrap.appendChild(audio)}
    else if(m.file_type==='video'){const video=document.createElement('video');video.controls=true;video.src='/file/'+m.file_id+'?token='+S.token;video.className='m-video';wrap.appendChild(video)}
    else{const a=document.createElement('a');a.href='/file/'+m.file_id+'?token='+S.token;a.target='_blank';a.className='m-file-link';a.textContent=m.text||'File';wrap.appendChild(a)}
  }

  const textDiv=document.createElement('div');
  textDiv.className='m-text';
  textDiv.innerHTML=md(m.text||'');
  wrap.appendChild(textDiv);

  if(m.reply_markup){try{const kb=typeof m.reply_markup==='string'?JSON.parse(m.reply_markup):m.reply_markup;if(kb.inline_keyboard){const kbDiv=document.createElement('div');kbDiv.className='m-kb';kb.inline_keyboard.forEach(row=>{const rowDiv=document.createElement('div');rowDiv.className='m-kb-row';row.forEach(btn=>{const b=document.createElement('button');b.className='m-kb-btn';b.dataset.cb=btn.callback_data;b.dataset.mid=m.message_id;b.textContent=btn.text;rowDiv.appendChild(b)});kbDiv.appendChild(rowDiv)});wrap.appendChild(kbDiv)}}catch{}}

  if(S.curChan){const replyBtn=document.createElement('button');replyBtn.className='m-del';replyBtn.dataset.action='reply';replyBtn.dataset.mid=m.message_id;replyBtn.dataset.who=who;replyBtn.dataset.preview=(m.text||'').substring(0,40);replyBtn.textContent='\u21a9';wrap.appendChild(replyBtn)}
  if(S.curChat){const delBtn=document.createElement('button');delBtn.className='m-del';delBtn.dataset.action='delete';delBtn.dataset.mid=m.message_id;delBtn.textContent='x';wrap.appendChild(delBtn)}

  el.appendChild(wrap);if(alertCls===' m-alert')updateElapsed()}

export function updateElapsed(){document.querySelectorAll('.m-elapsed').forEach(el=>{const ts=parseInt(el.dataset.ts);if(!ts)return;const sec=Math.floor((Date.now()-ts)/1000);let txt,cls;if(sec<60){txt=sec+(S.lang==='ru'?' сек':' sec');cls='el-ok'}else if(sec<3600){const m=Math.floor(sec/60);txt=m+(S.lang==='ru'?' мин':' min');cls=m<5?'el-ok':m<15?'el-warn':'el-crit'}else{const h=Math.floor(sec/3600);const m=Math.floor((sec%3600)/60);txt=h+(S.lang==='ru'?'ч ':' h ')+m+(S.lang==='ru'?' мин':' min');cls='el-crit'}el.textContent=txt;el.className='m-elapsed '+cls})}
S.elapsedTimer=setInterval(updateElapsed,30000);

export function scrollDown(){const el=$('msgs');setTimeout(()=>el.scrollTop=el.scrollHeight,50)}

export function renderPinBar(mid,text,who){
  const bar=$('pinned-bar');bar.innerHTML='';
  const pinDiv=document.createElement('div');pinDiv.className='m-pinned';
  const pinText=document.createElement('span');pinText.className='pin-text';pinText.dataset.mid=mid;pinText.innerHTML='<b>\ud83d\udccc</b> '+md(text.substring(0,80));
  const pinWho=document.createElement('span');pinWho.className='pin-who';pinWho.textContent=who||'';
  const pinClose=document.createElement('button');pinClose.className='pin-close';pinClose.dataset.action='unpin';pinClose.textContent='\u2715';
  pinDiv.appendChild(pinText);pinDiv.appendChild(pinWho);pinDiv.appendChild(pinClose);
  bar.appendChild(pinDiv);
}

// ── Event delegation on #msgs container ──
$('msgs').addEventListener('click',e=>{
  // Inline keyboard button
  const kbBtn=e.target.closest('.m-kb-btn');
  if(kbBtn){e.stopPropagation();_onCb(kbBtn);return}
  // Photo click → open in new tab
  const photo=e.target.closest('.m-photo');
  if(photo){window.open(photo.dataset.fileUrl,'_blank');return}
  // Reply/delete buttons
  const actionBtn=e.target.closest('.m-del');
  if(actionBtn){e.stopPropagation();const action=actionBtn.dataset.action;if(action==='reply'){_startReply(+actionBtn.dataset.mid,actionBtn.dataset.who,actionBtn.dataset.preview)}else if(action==='delete'){_onDel(+actionBtn.dataset.mid)}return}
  // Slash command
  const cmd=e.target.closest('.md-cmd');
  if(cmd){$('msg-in').value=cmd.dataset.cmd;$('msg-in').focus();return}
});

// ── Event delegation on #pinned-bar ──
$('pinned-bar').addEventListener('click',e=>{
  const pinText=e.target.closest('.pin-text');
  if(pinText){const mid=pinText.dataset.mid;const el=document.getElementById('m-'+mid);if(el)el.scrollIntoView({behavior:'smooth'});return}
  const unpinBtn=e.target.closest('[data-action="unpin"]');
  if(unpinBtn){_unpinMsg();return}
});

// Internal refs — set by actions.js via setMsgHandlers()
let _onCb=()=>{},_startReply=()=>{},_onDel=()=>{},_unpinMsg=()=>{};
export function setMsgHandlers(handlers){
  _onCb=handlers.onCb;_startReply=handlers.startReply;_onDel=handlers.onDel;_unpinMsg=handlers.unpinMsg;
}

// ── Auth ──
export function auth(r){S.token=r.token;set('token',S.token);set('uid',r.user_id);set('uname',r.username||'');if(r.role)set('role',r.role);if(r.org){set('org',r.org);const orgs=getJSON('orgs')||{};orgs[r.org]={token:r.token,user:r.username,name:r.org,role:r.role||'member'};setJSON('orgs',orgs)}showApp()}

export function logout(){S.token=null;S.curChat=null;S.curChan=null;remove('token');remove('uid');remove('uname');remove('view');remove('role');remove('org');disconnectWS();$('landing').style.display='flex';$('auth').style.display='none';$('app').style.display='none';$('fab').style.display='none';$('settings').style.display='none';$('settings-bg').style.display='none';window.initLandingChat()}

// ── App views ──
export async function showApp(){$('auth').style.display='none';$('app').style.display='flex';const u=get('uname')||'?';$('hdr-ava').textContent=u[0].toUpperCase();$('hdr-ava').style.background=nameColor(u);$('hdr-name').textContent=u;connectWS();registerPush();await showList();const v=get('view');if(v){try{const o=JSON.parse(v);if(o.t==='chat')openChat(o.id,o.n);else if(o.t==='ch')openChan(o.id,o.n)}catch{}}import('./onboard.js').then(m=>m.startOnboarding())}

export async function showList(){S.curChat=null;S.curChan=null;S.replyToId=0;$('reply-bar').style.display='none';$('pinned-bar').innerHTML='';$('typing-bar').style.display='none';remove('view');$('main-list').style.display='block';$('chat-view').style.display='none';$('input-bar').style.display='none';$('hdr-back').style.display='none';$('hdr-title').textContent=t('main');$('hdr-online').textContent='';const isGuest=get('uname')==='guest';$('fab').style.display=isGuest?'none':'flex';
  const el=$('main-list');
  showLoading(el);
  const[bots,chs,health]=await Promise.all([api('GET','/api/bots'),api('GET','/api/channels'),api('GET','/api/health')]);
  el.innerHTML='';S.channels=chs||[];
  if(chs&&chs.length){
    const secTitle=document.createElement('div');secTitle.className='sec-title';secTitle.textContent=t('ch');el.appendChild(secTitle);
    for(const c of chs){
      const row=document.createElement('div');row.className='ch-row';row.dataset.chanId=c.id;row.dataset.chanName=c.name;
      row.setAttribute("tabindex","0");
      row.addEventListener("keydown",e=>{if(e.key==="Enter"||e.key===" "){e.preventDefault();row.click()}});

      const hash=document.createElement('span');hash.className='ch-hash';hash.textContent='#';row.appendChild(hash);

      const body=document.createElement('div');body.className='ch-body';
      const chName=document.createElement('div');chName.className='ch-name';chName.textContent=c.name;
      if(c.unread>0){const badge=document.createElement('span');badge.className='ch-unread ch-badge ch-badge-'+c.id;badge.textContent=c.unread;chName.appendChild(badge)}else{const badge=document.createElement('span');badge.className='ch-unread ch-badge ch-badge-'+c.id;badge.style.display='none';chName.appendChild(badge)}
      body.appendChild(chName);
      const desc=document.createElement('div');desc.className='ch-desc';desc.textContent=c.description||'';body.appendChild(desc);
      row.appendChild(body);

      const subBtn=document.createElement('button');subBtn.className=c.subscribed?'sub-btn on':'sub-btn';subBtn.dataset.chanId=c.id;subBtn.textContent=c.subscribed?t('sub_on'):t('sub_off');row.appendChild(subBtn);

      el.appendChild(row);
    }
  }
  const isAdmin=get("role")==="admin";
  if(bots&&bots.length){
    const secTitle=document.createElement('div');secTitle.className='sec-title';secTitle.textContent=t('bots');el.appendChild(secTitle);
    for(const b of bots){
      const row=document.createElement('div');row.className='bot-row';row.dataset.botId=b.id;row.dataset.botName=b.name;
      row.setAttribute("tabindex","0");
      row.addEventListener("keydown",e=>{if(e.key==="Enter"||e.key===" "){e.preventDefault();row.click()}});

      const ava=document.createElement('div');ava.className='bot-ava';ava.style.background=nameColor(b.name);ava.textContent=b.name[0].toUpperCase();row.appendChild(ava);

      const info=document.createElement('div');
      const bName=document.createElement('div');bName.className='bot-name';bName.textContent=b.name;info.appendChild(bName);

      if(isAdmin){const tokDiv=document.createElement('div');tokDiv.className='bot-token';tokDiv.dataset.token=b.token;tokDiv.textContent=t('show_tok');info.appendChild(tokDiv)}

      row.appendChild(info);
      el.appendChild(row);
    }
  }
  if((!bots||!bots.length)&&(!chs||!chs.length)){const empty=document.createElement('div');empty.className='m-empty';empty.textContent=t('no_items');el.appendChild(empty)}
  const v=health||{};const foot=document.createElement('div');foot.className='list-foot';foot.textContent=t('foot')(bots?bots.length:0,chs?chs.length:0,v.online||0);el.appendChild(foot);
  $('main-list').focus()}

// ── Event delegation on #main-list ──
$('main-list').addEventListener('click',e=>{
  // Sub button
  const subBtn=e.target.closest('.sub-btn');
  if(subBtn){e.stopPropagation();toggleSub(e,+subBtn.dataset.chanId);return}
  // Bot token toggle
  const tokDiv=e.target.closest('.bot-token');
  if(tokDiv){e.stopPropagation();if(tokDiv.dataset.show){navigator.clipboard.writeText(tokDiv.dataset.token);tokDiv.textContent=t('copied');setTimeout(()=>{tokDiv.textContent='token: '+tokDiv.dataset.token},2000)}else{tokDiv.dataset.show='1';tokDiv.textContent='token: '+tokDiv.dataset.token+' ('+t('tap_copy')+')'}return}
  // Channel row
  const chRow=e.target.closest('.ch-row');
  if(chRow){openChan(+chRow.dataset.chanId,chRow.dataset.chanName);return}
  // Bot row
  const botRow=e.target.closest('.bot-row');
  if(botRow){openChat(+botRow.dataset.botId,botRow.dataset.botName);return}
});

export async function toggleSub(ev,chId){ev.stopPropagation();const btn=ev.target.closest('.sub-btn');const on=btn.classList.contains('on');btn.style.opacity='0.5';await api('POST',`/api/channels/${chId}/${on?'unsubscribe':'subscribe'}`);btn.classList.toggle('on');btn.textContent=on?t('sub_off'):t('sub_on');btn.style.opacity='1'}

export async function openChat(botId,name){S.lastMsgDate='';setJSON('view',{t:'chat',id:botId,n:name});history.pushState(null,'',location.href);$('fab').style.display='none';$('pinned-bar').innerHTML='';S.replyToId=0;$('reply-bar').style.display='none';$('typing-bar').style.display='none';$('main-list').style.display='none';$('chat-view').style.display='flex';$('input-bar').style.display='flex';$('hdr-back').style.display='inline';$('hdr-title').textContent=name;$('msg-in').placeholder=t('msg');api('GET','/api/online').then(r=>{if(!r)return;let s='';if(r.online>0)s=r.online+' online';const away=(r.total_connected||0)-r.online;if(away>0)s+=(s?', ':'')+away+' away';$('hdr-online').textContent=s});const chat=await api('POST',`/api/bots/${botId}/start`);S.curChat=chat.id;const msgs=await api('GET',`/api/chats/${chat.id}/messages`);$('msgs').innerHTML='';if(msgs&&msgs.length)msgs.reverse().forEach(m=>addMsg(m));else $('msgs').innerHTML=`<div class="m-empty">${t('no_msg')}</div>`;scrollDown();$('msg-in').focus()}

export async function openChan(chId,name){S.lastMsgDate='';setJSON('view',{t:'ch',id:chId,n:name});history.pushState(null,'',location.href);$('fab').style.display='none';S.curChan=chId;S.curChat=null;S.replyToId=0;$('reply-bar').style.display='none';$('typing-bar').style.display='none';$('main-list').style.display='none';$('chat-view').style.display='flex';$('input-bar').style.display='flex';$('hdr-back').style.display='inline';$('hdr-title').textContent='# '+name;$('msg-in').placeholder='\u041d\u0430\u043f\u0438\u0441\u0430\u0442\u044c \u0432 #'+name+'...';api('GET','/api/online').then(r=>{if(!r)return;let s='';if(r.online>0)s=r.online+' online';const away=(r.total_connected||0)-r.online;if(away>0)s+=(s?', ':'')+away+' away';$('hdr-online').textContent=s});api('GET','/api/users').then(r=>{if(r&&Array.isArray(r))S.mentionUsers=r});const msgs=await api('GET',`/api/channels/${chId}/messages`);$('msgs').innerHTML='';const chInfo=S.channels.find(c=>c.id===chId);const pinnedId=chInfo?chInfo.pinned_message_id:0;$('pinned-bar').innerHTML='';if(msgs&&msgs.length){msgs.reverse().forEach(m=>{if(!m.sender)m.sender='bot';addMsg(m)});if(pinnedId>0){const pinMsg=msgs.find(m=>m.message_id===pinnedId);if(pinMsg){renderPinBar(pinnedId,pinMsg.text||'')}}}else $('msgs').innerHTML=`<div class="m-empty">${t('no_msg')}</div>`;
  // Load more button for channels with 50+ messages
  if(msgs&&msgs.length>=50){const loadMore=document.createElement('div');loadMore.className='m-empty';loadMore.style.cursor='pointer';loadMore.textContent=S.lang==='ru'?'\u2191 \u0417\u0430\u0433\u0440\u0443\u0437\u0438\u0442\u044c \u0435\u0449\u0451':'\u2191 Load more';loadMore.onclick=async()=>{loadMore.textContent='...';const oldMsgs=await api('GET',`/api/channels/${chId}/messages?limit=200`);if(oldMsgs&&oldMsgs.length>msgs.length){S.lastMsgDate='';$('msgs').innerHTML='';oldMsgs.reverse().forEach(m=>{if(!m.sender)m.sender='bot';addMsg(m)})}else{loadMore.textContent=S.lang==='ru'?'\u041d\u0435\u0442 \u0431\u043e\u043b\u0435\u0435 \u0441\u0442\u0430\u0440\u044b\u0445':'No older messages'}};$('msgs').insertBefore(loadMore,$('msgs').firstChild)}
  // Alert filter bar
  const alertMsgs=$('msgs').querySelectorAll('.m-alert,.m-resolved,.m-acked');
  if(alertMsgs.length>0){const filterBar=document.createElement('div');filterBar.className='alert-filter';
    const btnAll=document.createElement('button');btnAll.className='af-btn af-active';btnAll.dataset.filter='all';btnAll.textContent='All';
    const btnFiring=document.createElement('button');btnFiring.className='af-btn';btnFiring.dataset.filter='firing';btnFiring.textContent=S.lang==='ru'?'\u0410\u043a\u0442\u0438\u0432\u043d\u044b\u0435':'Firing';
    const btnResolved=document.createElement('button');btnResolved.className='af-btn';btnResolved.dataset.filter='resolved';btnResolved.textContent=S.lang==='ru'?'\u0420\u0435\u0448\u0451\u043d\u043d\u044b\u0435':'Resolved';
    filterBar.appendChild(btnAll);filterBar.appendChild(btnFiring);filterBar.appendChild(btnResolved);
    $('msgs').insertBefore(filterBar,$('msgs').firstChild)}
  scrollDown();$('msg-in').focus();api('GET',`/api/channels/${chId}/readers`).then(readers=>{if(!readers||!readers.length)return;const myName=get('uname');const lastMsgId=msgs&&msgs.length?msgs[msgs.length-1].message_id:0;const whoRead=readers.filter(r=>r.last_read_id>=lastMsgId&&r.username!==myName);if(whoRead.length>0){const names=whoRead.map(r=>r.username).join(', ');const readDiv=document.createElement('div');readDiv.className='read-receipt';readDiv.textContent=(S.lang==='ru'?'\u041f\u0440\u043e\u0447\u0438\u0442\u0430\u043d\u043e: ':'Read by: ')+names;$('msgs').appendChild(readDiv)}})}

// ── Navigation ──
$('hdr-back').onclick=showList;
