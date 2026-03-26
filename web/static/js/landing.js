import S from './state.js';
import {get} from './storage.js';
import {$,esc,md,nameColor,fmtTime,t,te,api} from './util.js';
import {auth,showApp} from './views.js';

// ── Landing live chat ──
async function landApi(method,path,body){
  const o={method,headers:{'Content-Type':'application/json'}};
  if(S.landToken)o.headers.Authorization=S.landToken;
  if(body)o.body=JSON.stringify(body);
  try{const r=await fetch(path,o);return await r.json()}catch{return{}}
}
function landAddMsg(who,text,tm,markup){
  const el=$('land-msgs');const col=nameColor(who);

  const wrap=document.createElement('div');wrap.className='m';

  const ava=document.createElement('div');ava.className='m-ava';ava.style.background=col;ava.textContent=who[0].toUpperCase();wrap.appendChild(ava);

  const head=document.createElement('div');head.className='m-head';
  const nameSpan=document.createElement('span');nameSpan.className='m-name';nameSpan.textContent=who;head.appendChild(nameSpan);
  const timeSpan=document.createElement('span');timeSpan.className='m-time';timeSpan.textContent=tm||'';head.appendChild(timeSpan);
  wrap.appendChild(head);

  const textDiv=document.createElement('div');textDiv.className='m-text';textDiv.innerHTML=md(text||'');wrap.appendChild(textDiv);

  if(markup){try{const m=typeof markup==='string'?JSON.parse(markup):markup;if(m.inline_keyboard){const kbDiv=document.createElement('div');kbDiv.className='m-kb';m.inline_keyboard.forEach(row=>{const rowDiv=document.createElement('div');rowDiv.className='m-kb-row';row.forEach(btn=>{const b=document.createElement('button');b.className='m-kb-btn';b.dataset.cb=btn.callback_data;b.textContent=btn.text;rowDiv.appendChild(b)});kbDiv.appendChild(rowDiv)});wrap.appendChild(kbDiv)}}catch{}}

  el.appendChild(wrap);
  el.scrollTop=el.scrollHeight;
}

// ── Event delegation on #land-msgs ──
$('land-msgs').addEventListener('click',e=>{
  const kbBtn=e.target.closest('.m-kb-btn');
  if(kbBtn){landCb(kbBtn.dataset.cb);return}
  const cmd=e.target.closest('.md-cmd');
  if(cmd){$('land-input').value=cmd.dataset.cmd;$('land-input').focus();return}
});

export async function initLandingChat(){
  let r=await landApi('POST','/api/auth',{username:'guest',pin:'guest'});
  if(!r.token)r=await landApi('POST','/api/register',{username:'guest',pin:'guest',display_name:'Guest'});
  if(!r.token)return;
  S.landToken=r.token;
  const chat=await landApi('POST','/api/bots/1/start');
  if(!chat.id)return;S.landChat=chat.id;
  const msgs=await landApi('GET',`/api/chats/${chat.id}/messages`);
  const fb=document.getElementById("land-fallback");if(fb)fb.remove();
  if(msgs&&msgs.length)msgs.reverse().forEach(m=>{
    const who=m.sender==='bot'?'DemoBot':'Guest';
    landAddMsg(who,m.text,fmtTime(m.date),m.reply_markup);
  });
}
$('land-send').onclick=landSend;
$('land-input').onkeydown=e=>{if(e.key==='Enter')landSend()};
async function landSend(){const inp=$('land-input');const txt=inp.value.trim();if(!txt||!S.landChat)return;inp.value='';landAddMsg('Guest',txt,'');await landApi('POST',`/api/chats/${S.landChat}/send`,{text:txt})}
async function landCb(data){if(!S.landChat)return;await landApi('POST',`/api/chats/${S.landChat}/callback`,{data,message_id:0});setTimeout(async()=>{const msgs=await landApi('GET',`/api/chats/${S.landChat}/messages?limit=1`);if(msgs&&msgs.length){const m=msgs[0];if(m.sender==='bot')landAddMsg('DemoBot',m.text,fmtTime(m.date),m.reply_markup)}},1500)}
export function hideLanding(){$('landing').style.display='none'}
$('land-login').onclick=()=>{hideLanding();$('auth').style.display='flex';const savedOrg=get('org');if(savedOrg)$('a-org').value=savedOrg}
$('land-demo').onclick=async()=>{let r=await api('POST','/api/auth',{username:'guest',pin:'guest'});if(!r.token)r=await api('POST','/api/register',{username:'guest',pin:'guest',display_name:'Guest'});if(!r.token)return;S.token=r.token;S.isDemo=true;hideLanding();showApp()};

// ── Auth buttons ──
$('btn-login').onclick=async()=>{const u=$('a-user').value.trim(),p=$('a-pin').value.trim(),o=$('a-org').value.trim()||undefined;if(!u||!p){$('a-err').textContent=t('err_empty');return}$('btn-login').textContent='...';$('btn-login').disabled=true;const r=await api('POST','/api/auth',{username:u,pin:p,org:o});$('btn-login').textContent=t('login');$('btn-login').disabled=false;if(r.error||!r.token){$('a-err').textContent=(r.error&&r.error.includes('specify org'))?te(r.error):t('err_wrong');return}auth(r)};
$('btn-reg').onclick=async()=>{const u=$('a-user').value.trim(),p=$('a-pin').value.trim(),o=$('a-org').value.trim()||undefined;if(!u||!p){$('a-err').textContent=t('err_empty');return}let r;if(S.invite){r=await api('POST','/api/invite/accept'+(o?'?org='+o:''),{code:S.invite,username:u,pin:p,display_name:u})}else{r=await api('POST','/api/register',{username:u,pin:p,display_name:u,org:o})}if(r.error){$('a-err').textContent=r.error.includes('UNIQUE')?u+' '+t('err_taken'):te(r.error);return}auth(r)};
$('btn-demo').onclick=async()=>{let r=await api('POST','/api/auth',{username:'guest',pin:'guest'});if(!r.token)r=await api('POST','/api/register',{username:'guest',pin:'guest',display_name:'Guest'});if(!r.token){$('a-err').textContent=t('err_demo');return}S.token=r.token;S.isDemo=true;hideLanding();$('auth').style.display='none';showApp()};

// ── Org creation ──
$('land-create-org').onclick=()=>{$('org-modal-bg').classList.add('open');$('org-slug').focus()};
$('org-cancel').onclick=()=>$('org-modal-bg').classList.remove('open');
$('org-modal-bg').onclick=e=>{if(e.target===$('org-modal-bg'))$('org-modal-bg').classList.remove('open')};
$('org-ok').onclick=async()=>{
  const slug=$('org-slug').value.trim().toLowerCase().replace(/[^a-z0-9-]/g,'');
  const name=$('org-name').value.trim()||slug;const user=$('org-user').value.trim();const pin=$('org-pin').value.trim();const msg=$('org-msg');
  if(!slug||!user||!pin){msg.textContent=t('fill_all');msg.style.color='#e05d44';return}msg.textContent='';
  const r=await api('POST','/api/org/register',{slug,name,username:user,pin});
  if(r.ok&&r.token){msg.textContent=name+' \u0441\u043e\u0437\u0434\u0430\u043d\u0430!';msg.style.color='#3db887';setTimeout(()=>{$('org-modal-bg').classList.remove('open');hideLanding();auth(r)},800)}
  else{msg.textContent=te(r.error||'Error');msg.style.color='#e05d44'}
};

// ── Window binding for views.js logout() ──
window.initLandingChat=initLandingChat;
