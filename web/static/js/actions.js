import S from './state.js';
import {$,esc,escJs,md,t,te,toast,api,confirmDialog} from './util.js';
import {addMsg,scrollDown,showList,renderPinBar} from './views.js';

// ── Send message ──
$('msg-send').onclick=sendMsg;
$('msg-in').onkeydown=e=>{if(e.key==='Enter'&&!e.shiftKey){e.preventDefault();sendMsg()}};

// ── File upload ──
$('file-input').onchange=async function(){if(!this.files.length)return;const file=this.files[0];if(!S.curChan){toast(S.lang==="ru"?"Только в каналах":"Only in channels");this.value="";return}const fd=new FormData();fd.append("file",file);fd.append("caption",file.name);toast(S.lang==="ru"?"Загрузка...":"Uploading...");const opts={method:"POST",headers:{},body:fd};if(S.token)opts.headers.Authorization=S.token;try{const r=await fetch(`/api/channels/${S.curChan}/upload`,opts);const msg=await r.json();if(msg&&msg.message_id){toast(S.lang==="ru"?"Отправлено":"Sent");addMsg(msg);scrollDown()}}catch(e){toast("Error: "+e.message)}this.value=""};

// ── @mention & slash autocomplete ──
const _slashCmds=['/start','/help','/status'];
$('msg-in').addEventListener('input',function(){const v=this.value;const cursor=this.selectionStart;const before=v.substring(0,cursor);
  if(S.curChat){const slashMatch=before.match(/^\/(\w*)$/);if(slashMatch){const q=slashMatch[1].toLowerCase();const matches=_slashCmds.filter(c=>c.substring(1).startsWith(q));if(matches.length>0){const ml=$('mention-list');ml.innerHTML=matches.map(c=>`<div class="ac-item" onmousedown="window.insertSlash('${c}')">${c}</div>`).join('');ml.style.display='block';return}}}
  if(S.curChan){const atMatch=before.match(/@(\w*)$/);if(atMatch){const query=atMatch[1].toLowerCase();const me=localStorage.getItem('pusk_uname')||'';const matches=S.mentionUsers.filter(u=>u.username.toLowerCase().startsWith(query)&&u.username!==me);if(matches.length>0){const ml=$('mention-list');ml.innerHTML=matches.slice(0,8).map(u=>`<div class="ac-item-user" onmousedown="window.insertMention('${escJs(u.username)}')">${esc(u.username)}</div>`).join('');ml.style.display='block';return}}}$('mention-list').style.display='none';
  if(S.curChan&&S.ws&&S.ws.readyState===WebSocket.OPEN){
    if(!S.typingTimer){S.ws.send(JSON.stringify({type:'typing',channel_id:S.curChan}))}
    clearTimeout(S.typingTimer);
    S.typingTimer=setTimeout(()=>{S.typingTimer=null},2000);
  }
});
function insertSlash(cmd){$('msg-in').value=cmd;$('mention-list').style.display='none';$('msg-in').focus()}
function insertMention(username){const inp=$('msg-in');const v=inp.value;const cursor=inp.selectionStart;const before=v.substring(0,cursor);const after=v.substring(cursor);const atPos=before.lastIndexOf('@');inp.value=before.substring(0,atPos)+'@'+username+' '+after;inp.selectionStart=inp.selectionEnd=atPos+username.length+2;$('mention-list').style.display='none';inp.focus()}

// ── Reply ──
function startReply(mid,name,text){if(!S.curChan)return;S.replyToId=mid;S.replyToText=text;$('reply-text').innerHTML='<b>'+esc(name)+'</b>: '+esc(text);$('reply-bar').style.display='flex';$('msg-in').focus()}
$('reply-cancel').onclick=()=>{S.replyToId=0;$('reply-bar').style.display='none'};

// ── Swipe to reply (mobile) ──
let _swipeX=0,_swipeEl=null;
document.addEventListener('touchstart',e=>{const m=e.target.closest('.m');if(m&&S.curChan){_swipeX=e.touches[0].clientX;_swipeEl=m}},{passive:true});
document.addEventListener('touchend',e=>{if(!_swipeEl)return;const dx=e.changedTouches[0].clientX-_swipeX;if(dx>80){const name=_swipeEl.querySelector('.m-name')?.textContent||'';const text=_swipeEl.querySelector('.m-text')?.textContent?.substring(0,40)||'';const mid=parseInt(_swipeEl.id.replace('m-',''));if(mid)startReply(mid,name,text)}_swipeEl.style.transform='';_swipeEl=null},{passive:true});
document.addEventListener('touchmove',e=>{if(!_swipeEl)return;const dx=e.touches[0].clientX-_swipeX;if(dx>10)_swipeEl.style.transform=`translateX(${Math.min(dx,60)}px)`},{passive:true});

// ── Edit mode ──
$('edit-cancel').onclick=cancelEdit;
function cancelEdit(){S.editMsgId=0;S.editChanId=0;$('edit-bar').style.display='none';$('msg-in').value='';$('msg-send').textContent='\u2192'}
function startEdit(mid,chanId,text){S.editMsgId=mid;S.editChanId=chanId;S.replyToId=0;$('reply-bar').style.display='none';$('edit-bar').style.display='flex';$('edit-text').innerHTML='<b>'+(S.lang==='ru'?'Редактирование':'Editing')+'</b>';$('msg-in').value=text;$('msg-in').focus();$('msg-send').textContent='\u2713'}

async function sendMsg(){const inp=$('msg-in');const txt=inp.value.trim();if(!txt)return;inp.value='';
  if(S.editMsgId&&S.editChanId){const mid=S.editMsgId;const cid=S.editChanId;cancelEdit();await api('PUT',`/api/channels/${cid}/messages/${mid}`,{text:txt});const el=document.getElementById('m-'+mid);if(el){const t=el.querySelector('.m-text');if(t)t.innerHTML=md(txt);const head=el.querySelector('.m-head');if(head&&!head.querySelector('.m-edited')){const s=document.createElement('span');s.className='m-edited';s.textContent=S.lang==='ru'?'(ред.)':'(edited)';head.appendChild(s)}}return}
  const uname=localStorage.getItem('pusk_uname')||'You';const rid=S.replyToId;S.replyToId=0;$('reply-bar').style.display='none';if(S.curChan){addMsg({sender:'user',sender_name:uname,text:txt,reply_to:rid,date:new Date().toISOString(),message_id:Date.now()});scrollDown();await api('POST',`/api/channels/${S.curChan}/send`,{text:txt,reply_to:rid})}else if(S.curChat){addMsg({sender:'user',text:txt,date:new Date().toISOString(),message_id:Date.now()});scrollDown();await api('POST',`/api/chats/${S.curChat}/send`,{text:txt})}}

// ── Context menu ──
let _ctxMsgEl=null,_ctxSavedText='';
function showCtxMenu(x,y,msgEl){
  const mid=parseInt(msgEl.id.replace('m-',''));
  const isMine=msgEl.dataset.mine==='1';
  const isAdmin=localStorage.getItem('pusk_role')==='admin'||localStorage.getItem('pusk_uid')==='1';
  const sender=msgEl.dataset.sender;
  const text=msgEl.querySelector('.m-text')?.textContent||'';
  const name=msgEl.querySelector('.m-name')?.textContent||'';
  _ctxMsgEl=msgEl;_ctxSavedText=text;
  const menu=$('ctx-menu');let items='';
  if(S.curChan){
    items+=`<div class="ctx-item" onclick="window.hideCtx();window.startReply(${mid},'${escJs(name)}','${escJs(text.substring(0,40))}')">${S.lang==='ru'?'Ответить':'Reply'}</div>`;
    if(isMine&&sender==='user')items+=`<div class="ctx-item" onclick="window.ctxEdit(${mid},${S.curChan})">${S.lang==='ru'?'Редактировать':'Edit'}</div>`;
    if(isAdmin)items+=`<div class="ctx-item" onclick="window.hideCtx();window.pinMsg(${mid})">${S.lang==='ru'?'Закрепить':'Pin'}</div>`;
    if(isMine||isAdmin)items+=`<div class="ctx-item danger" onclick="window.hideCtx();window.delChanMsg(${mid})">${S.lang==='ru'?'Удалить':'Delete'}</div>`;
  }
  if(S.curChat){
    if(isMine||sender==='user')items+=`<div class="ctx-item danger" onclick="window.hideCtx();window.onDel(${mid})">${S.lang==='ru'?'Удалить':'Delete'}</div>`;
  }
  items+=`<div class="ctx-item" onclick="window.ctxCopy()">${S.lang==='ru'?'Копировать':'Copy'}</div>`;
  menu.innerHTML=items;menu.style.display='block';
  const mw=menu.offsetWidth,mh=menu.offsetHeight;
  menu.style.left=Math.min(x,window.innerWidth-mw-8)+'px';
  menu.style.top=Math.min(y,window.innerHeight-mh-8)+'px';
}
function ctxEdit(mid,chanId){const t=_ctxSavedText;hideCtx();startEdit(mid,chanId,t)}
function ctxCopy(){const t=_ctxSavedText;hideCtx();navigator.clipboard.writeText(t)}
function hideCtx(){$('ctx-menu').style.display='none';_ctxMsgEl=null;_ctxSavedText=''}
document.addEventListener('click',e=>{if(!e.target.closest('#ctx-menu'))hideCtx()});

// Long press (mobile)
let _longTimer=null,_longX=0,_longY=0;
document.addEventListener('touchstart',e=>{
  if(e.target.closest('.m-kb-btn'))return;const m=e.target.closest('.m');if(!m)return;
  _longX=e.touches[0].clientX;_longY=e.touches[0].clientY;
  _longTimer=setTimeout(()=>{_longTimer=null;_swipeEl=null;showCtxMenu(_longX,_longY,m)},600);
},{passive:true});
document.addEventListener('touchend',()=>{if(_longTimer){clearTimeout(_longTimer);_longTimer=null}},{passive:true});
document.addEventListener('touchmove',e=>{if(_longTimer){const dx=e.touches[0].clientX-_longX,dy=e.touches[0].clientY-_longY;if(dx*dx+dy*dy>100){clearTimeout(_longTimer);_longTimer=null}}},{passive:true});
document.addEventListener('contextmenu',e=>{const m=e.target.closest('.m');if(!m||(!S.curChan&&!S.curChat))return;e.preventDefault();showCtxMenu(e.clientX,e.clientY,m)});

// ── Pin / Delete ──
async function delChanMsg(mid){const ci=S.channels.find(c=>c.id===S.curChan);const isPinned=ci&&ci.pinned_message_id===mid;const msg=isPinned?(S.lang==='ru'?'Это закреплённое сообщение. Удалить и открепить?':'This is a pinned message. Delete and unpin?'):(S.lang==='ru'?'Удалить сообщение?':'Delete message?');if(!await confirmDialog(msg))return;await api('DELETE',`/api/channels/messages/${mid}`);const el=document.getElementById('m-'+mid);if(el)el.remove();if(isPinned){$('pinned-bar').innerHTML='';ci.pinned_message_id=0}}
async function pinMsg(mid){if(!S.curChan)return;const r=await api('POST',`/api/channels/${S.curChan}/pin`,{message_id:mid});if(r&&r.ok){const u=localStorage.getItem('pusk_uname')||'?';toast(S.lang==='ru'?'Закреплено':'Pinned');const msgEl=document.getElementById('m-'+mid);const text=msgEl?msgEl.querySelector('.m-text')?.textContent||'':'';renderPinBar(mid,text,u);const ci=S.channels.find(c=>c.id===S.curChan);if(ci)ci.pinned_message_id=mid}}
async function unpinMsg(){if(!S.curChan)return;const r=await api('POST',`/api/channels/${S.curChan}/pin`,{message_id:0});if(r&&r.ok){toast(S.lang==='ru'?'Откреплено':'Unpinned');$('pinned-bar').innerHTML='';const ci=S.channels.find(c=>c.id===S.curChan);if(ci)ci.pinned_message_id=0}}

// ── Callbacks & Delete ──
async function onCb(el){if(S.curChan){await api('POST',`/api/channels/${S.curChan}/ack`,{action:el.dataset.cb,message_id:parseInt(el.dataset.mid)});const u=localStorage.getItem('pusk_uname')||'?';const tm=new Date().toLocaleTimeString('ru',{hour:'2-digit',minute:'2-digit'});el.closest('.m-kb').innerHTML=`<span class="ack-result">${el.textContent} @${u} ${tm}</span>`;toast(el.textContent+' ✓');const msg=el.closest('.m');if(msg){msg.classList.remove('m-alert');msg.classList.add('m-acked')}}else if(S.curChat){el.disabled=true;el.style.opacity='0.5';await api('POST',`/api/chats/${S.curChat}/callback`,{data:el.dataset.cb,message_id:parseInt(el.dataset.mid)})}}
async function onDel(mid){if(!await confirmDialog(S.lang==='ru'?'Удалить сообщение?':'Delete message?'))return;await api('DELETE',`/api/messages/${mid}`);const el=document.getElementById('m-'+mid);if(el)el.remove()}

// ── FAB / Create modal ──
$('fab').onclick=()=>{$('modal-bg').classList.add('open');$('m-name').value='';$('m-desc').value='';$('m-msg').textContent='';updModal();$('m-name').focus()};
$('m-cancel').onclick=()=>$('modal-bg').classList.remove('open');
$('modal-bg').onclick=e=>{if(e.target===$('modal-bg'))$('modal-bg').classList.remove('open')};
$('m-type').onchange=updModal;
function updModal(){const ch=$('m-type').value==='channel';$('m-title').textContent=ch?t('new_ch'):t('new_bot');$('m-tok-row').style.display=ch?'none':'block';$('m-name').placeholder=ch?'alerts':'MonitorBot';$('m-desc').placeholder=ch?'Description':'Webhook URL'}
$('m-ok').onclick=async()=>{const name=$('m-name').value.trim();if(!name){$('m-msg').textContent=t('name_req');$('m-msg').style.color='#e05d44';return}$('m-msg').textContent='';const type=$('m-type').value;if(type==='channel'){const r=await api('POST','/admin/channel',{name,description:$('m-desc').value.trim()});if(r&&r.ok){$('m-msg').textContent='# '+name+' ✓';$('m-msg').style.color='#3db887';setTimeout(()=>{$('modal-bg').classList.remove('open');showList()},800)}else{$('m-msg').textContent=te(r.error||r.description||'Error');$('m-msg').style.color='#e05d44'}}else{const tok=$('m-tok').value.trim()||name.toLowerCase().replace(/[^a-z0-9]/g,'-')+'-'+Math.random().toString(36).substr(2,6);const r=await api('POST','/admin/bots',{token:tok,name});if(r&&r.id){$('m-msg').textContent=name+' ✓ token: '+tok;$('m-msg').style.color='#3db887';setTimeout(()=>{$('modal-bg').classList.remove('open');showList()},1200)}else{$('m-msg').textContent=te(r.error||'Error');$('m-msg').style.color='#e05d44'}}};

// ── Alert filter ──
function filterAlerts(type) {
  document.querySelectorAll('.m').forEach(m => {
    if (type === 'all') { m.style.display = ''; return; }
    const isAlert = m.classList.contains('m-alert');
    const isResolved = m.classList.contains('m-resolved') || m.classList.contains('m-acked');
    if (type === 'firing') m.style.display = isAlert ? '' : 'none';
    if (type === 'resolved') m.style.display = isResolved ? '' : 'none';
  });
  document.querySelectorAll('.af-btn').forEach(b => b.classList.remove('af-active'));
  event.target.classList.add('af-active');
}
window.filterAlerts = filterAlerts;

// ── Window bindings for HTML onclick ──
window.onCb=onCb;
window.onDel=onDel;
window.startReply=startReply;
window.insertSlash=insertSlash;
window.insertMention=insertMention;
window.hideCtx=hideCtx;
window.ctxEdit=ctxEdit;
window.ctxCopy=ctxCopy;
window.delChanMsg=delChanMsg;
window.pinMsg=pinMsg;
window.unpinMsg=unpinMsg;
