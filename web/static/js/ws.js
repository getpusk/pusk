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
  S.ws.onopen=()=>$('hdr-dot').style.color='#3db887';
  S.ws.onclose=()=>{$('hdr-dot').style.color='#e05d44';S.wsReconnectTimer=setTimeout(connectWS,3000)};
  S.ws.onmessage=e=>{const ev=JSON.parse(e.data);const d=ev.payload;
if(ev.type==='new_message'&&ev.chat_id===S.curChat){addMsg(d.message);scrollDown()}
if(ev.type==='channel_message'){const myName=get('uname');const msg=d.message||d;const senderName=msg.sender_name||d.sender_name||'';if(ev.chat_id===S.curChan){if(senderName===myName){const els=$('msgs').querySelectorAll('.m[data-mine="1"]');for(let i=0;i<els.length;i++){const fid=parseInt(els[i].id.replace('m-',''));if(fid>1e12){els[i].id='m-'+msg.message_id;break}}}else{if(!msg.sender)msg.sender='bot';beep();addMsg(msg);scrollDown()}}else if(senderName!==myName){beep();const badge=document.querySelector(`.ch-badge-${ev.chat_id}`);if(badge){badge.style.display='inline-block';const n=parseInt(badge.textContent||'0')+1;badge.textContent=n}}}
if(ev.type==='edit_message'){const old=document.getElementById('m-'+d.message_id);if(old)old.remove();addMsg(d);scrollDown()}
if(ev.type==='channel_message_edit'){const old=document.getElementById('m-'+d.message_id);if(old){const txt=old.querySelector('.m-text');if(txt)txt.innerHTML=md(d.text||'');const head=old.querySelector('.m-head');if(head&&!head.querySelector('.m-edited')){const s=document.createElement('span');s.className='m-edited';s.textContent=S.lang==='ru'?'(ред.)':'(edited)';head.appendChild(s)}}}
if(ev.type==='channel_message_delete'){const el=document.getElementById('m-'+d.message_id);if(el)el.remove()}
if(ev.type==='typing'&&ev.chat_id===S.curChan){const td=ev.payload;$('typing-bar').textContent=td.username+(S.lang==='ru'?' печатает...' :' is typing...');$('typing-bar').style.display='block';clearTimeout(window._typingHide);window._typingHide=setTimeout(()=>{$('typing-bar').style.display='none'},3000)}
if(ev.type==='callback_answer'){if(d.show_alert)alert(d.text);else toast(d.text)}
if(ev.type==='mention'){beep();toast('@'+get('uname')+' in #'+(d.channel||''))}}}

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
    // Warn if subscription provider doesn't match browser
    const ep=sub.endpoint||'';const isFF=navigator.userAgent.includes('Firefox');const isMoz=ep.includes('mozilla');
    if(isFF&&!isMoz){const{toast:t2}=await import('./util.js');t2(localStorage.getItem('pusk_lang')==='en'?'Push registered via Chrome. Open in Firefox to get Firefox push.':'Push подписка от Chrome. Откройте в Firefox и нажмите Push Вкл для подписки Firefox.')}
  }catch(e){}
}
