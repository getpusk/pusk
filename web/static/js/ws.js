import S from './state.js';
import {get} from './storage.js';
import {$,md,toast} from './util.js';
import {addMsg,scrollDown} from './views.js';

// ── Audio ──
export function beep(){try{if(!S.audioCtx)S.audioCtx=new(window.AudioContext||window.webkitAudioContext)();const o=S.audioCtx.createOscillator(),g=S.audioCtx.createGain();o.connect(g);g.connect(S.audioCtx.destination);o.frequency.value=800;g.gain.value=0.1;o.start();g.gain.exponentialRampToValueAtTime(0.001,S.audioCtx.currentTime+0.15);o.stop(S.audioCtx.currentTime+0.15)}catch{}}

// ── WebSocket ──
export function connectWS(){
  if(!S.token)return;
  clearTimeout(S.wsReconnectTimer);
  const p=location.protocol==='https:'?'wss:':'ws:';
  S.ws=new WebSocket(`${p}//${location.host}/api/ws?token=${S.token}`);
  S.ws.onopen=()=>{$('hdr-dot').style.color='#3db887';const ob=$('offline-bar');if(ob)ob.classList.remove('show')};
  S.ws.onclose=()=>{$('hdr-dot').style.color='#e05d44';const ob=$('offline-bar');if(ob){ob.textContent=S.lang==='ru'?'Переподключение...':'Reconnecting...';ob.classList.add('show')}S.wsReconnectTimer=setTimeout(connectWS,3000)};
  S.ws.onmessage=e=>{const ev=JSON.parse(e.data);const d=ev.payload;
if(ev.type==='new_message'&&ev.chat_id===S.curChat){addMsg(d.message);scrollDown()}
if(ev.type==='channel_message'){const myName=get('uname');const msg=d.message||d;const senderName=msg.sender_name||d.sender_name||'';if(ev.chat_id===S.curChan){if(senderName===myName){const els=$('msgs').querySelectorAll('.m[data-mine="1"]');for(let i=0;i<els.length;i++){const fid=parseInt(els[i].id.replace('m-',''));if(fid>1e12){els[i].id='m-'+msg.message_id;break}}}else{if(!msg.sender)msg.sender='bot';beep();addMsg(msg);scrollDown()}}else if(senderName!==myName){beep();const badge=document.querySelector(`.ch-badge-${ev.chat_id}`);if(badge){badge.style.display='inline-block';const n=parseInt(badge.textContent||'0')+1;badge.textContent=n}}}
if(ev.type==='edit_message'){const old=document.getElementById('m-'+d.message_id);if(old)old.remove();addMsg(d);scrollDown()}
if(ev.type==='channel_message_edit'){const old=document.getElementById('m-'+d.message_id);if(old){const txt=old.querySelector('.m-text');if(txt)txt.innerHTML=md(d.text||'');const head=old.querySelector('.m-head');if(head&&!head.querySelector('.m-edited')){const s=document.createElement('span');s.className='m-edited';s.textContent=S.lang==='ru'?'(ред.)':'(edited)';head.appendChild(s)}}}
if(ev.type==='channel_message_delete'){const mid=d.message_id;const el=document.getElementById('m-'+mid);if(el)el.remove();document.querySelectorAll('.m[data-reply="'+mid+'"]').forEach(r=>{const q=r.querySelector('.m-quote');if(q)q.innerHTML='<b></b> '+(S.lang==='ru'?'Удалённое сообщение':'Deleted message')})}
if(ev.type==='typing'&&ev.chat_id===S.curChan){const td=ev.payload;$('typing-bar').textContent=td.username+(S.lang==='ru'?' печатает...' :' is typing...');$('typing-bar').style.display='block';clearTimeout(window._typingHide);window._typingHide=setTimeout(()=>{$('typing-bar').style.display='none'},3000)}
if(ev.type==='callback_answer'){if(d.show_alert)alert(d.text);else toast(d.text)}
if(ev.type==='mention'){beep();toast('@'+get('uname')+' in #'+(d.channel||''))}
if(ev.type==='user_status'){const ud=ev.payload;if(ud&&ud.username){document.querySelectorAll('.m-ava[data-uname="'+ud.username+'"]').forEach(el=>{el.classList.remove('online','away');if(ud.status==='online')el.classList.add('online');else if(ud.status==='away')el.classList.add('away')});import('./util.js').then(u=>{const api=u.api;api('GET','/api/online').then(r=>{if(!r)return;const us=r.users||[];const on=us.filter(x=>x.status==='online').map(x=>x.username);const aw=us.filter(x=>x.status==='away').map(x=>x.username);let s='';if(on.length)s=on.join(', ');if(aw.length)s+=(s?' \xb7 ':'')+(S.lang==='ru'?'\u043e\u0442\u043e\u0448\u043b\u0438: ':'away: ')+aw.join(', ');$('hdr-online').textContent=s})})}}}}

export function disconnectWS(){
  clearTimeout(S.wsReconnectTimer);
  S.wsReconnectTimer=null;
  if(S.ws){S.ws.onclose=null;S.ws.close();S.ws=null}
}

// ── Away status ──
document.addEventListener('visibilitychange',()=>{
  if(S.ws&&S.ws.readyState===WebSocket.OPEN){
    S.ws.send(JSON.stringify({type:'status',status:document.hidden?'away':'online'}));
  }
});

// ── Push ──
export async function registerPush(){
  if(!('serviceWorker' in navigator)||!('PushManager' in window)||!S.token)return;
  if((localStorage.getItem('pusk_org')||'default')==='default')return;
  try{
    const perm=await Notification.requestPermission();
    if(perm!=='granted')return;
    const reg=await navigator.serviceWorker.register('/sw.js');
    await navigator.serviceWorker.ready;
    const r=await fetch('/api/push/vapid');const{key}=await r.json();if(!key)return;
    const appKey=Uint8Array.from(atob(key.replace(/-/g,'+').replace(/_/g,'/')+'='.repeat((4-key.length%4)%4)),c=>c.charCodeAt(0));
    let sub=await reg.pushManager.getSubscription();
    if(sub){try{const old=await fetch('/api/push/subscribe',{method:'POST',headers:{'Content-Type':'application/json',Authorization:S.token},body:JSON.stringify(sub.toJSON())});if(!old.ok){await sub.unsubscribe();sub=null}}catch{await sub.unsubscribe();sub=null}}
    if(!sub)sub=await reg.pushManager.subscribe({userVisibleOnly:true,applicationServerKey:appKey});
    await fetch('/api/push/subscribe',{method:'POST',headers:{'Content-Type':'application/json',Authorization:S.token},body:JSON.stringify(sub.toJSON())});
    // Detect browser and push provider mismatch
    const ep=sub.endpoint||'';const ua=navigator.userAgent;
    const bName=(()=>{if(ua.includes('YaBrowser'))return'Яндекс Браузер';if(ua.includes('Edg/'))return'Edge';if(ua.includes('OPR/')||ua.includes('Opera'))return'Opera';if(ua.includes('Vivaldi'))return'Vivaldi';if(ua.includes('Brave'))return'Brave';if(ua.includes('Firefox'))return'Firefox';if(ua.includes('Safari')&&!ua.includes('Chrome'))return'Safari';if(ua.includes('Chrome'))return'Chrome';return''})();
    const epName=ep.includes('mozilla')?'Firefox':ep.includes('fcm.googleapis')?'Chrome/Chromium':ep.includes('windows.com')?'Edge':ep.includes('apple.com')?'Safari':'';
    const isFF=bName==='Firefox';const isMoz=ep.includes('mozilla');const isChromium=!isFF&&!ep.includes('apple')&&!ep.includes('windows');
    if((isFF&&!isMoz)||(!isFF&&isMoz)){const{toast:t2}=await import('./util.js');const ru=localStorage.getItem('pusk_lang')!=='en';t2(ru?'Push подписка от '+epName+'. Вы в '+bName+'. Нажмите Push Вкл для подписки '+bName+'.':'Push subscription from '+epName+'. You are in '+bName+'. Click Push On to subscribe.')}
  }catch(e){}
}
