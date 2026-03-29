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
// Show available orgs when username entered on login form
$('a-user').addEventListener('blur',async function(){const u=this.value.trim();if(!u||$('a-org').value)return;if(new URLSearchParams(location.search).get('invite'))return;try{const r=await fetch('/api/my-orgs?username='+encodeURIComponent(u));const orgs=await r.json();if(orgs&&orgs.length===1){$('a-org').value=orgs[0].slug}else if(orgs&&orgs.length>1){$('a-org').placeholder=orgs.map(o=>o.slug).join(', ')}}catch{}});
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
  if(r.ok&&r.token){msg.textContent=name+(S.lang==='ru'?' создана!':' created!');msg.style.color='#3db887';setTimeout(()=>{$('org-modal-bg').classList.remove('open');hideLanding();auth(r)},800)}
  else{msg.textContent=te(r.error||'Error');msg.style.color='#e05d44'}
};

// ── Landing i18n ──
function translateLanding(){
  const ru=S.lang==='ru';
  const tx=(sel,ruT,enT)=>{const el=document.querySelector(sel);if(el)el.textContent=ru?ruT:enT};
  const th=(sel,ruH,enH)=>{const el=document.querySelector(sel);if(el)el.innerHTML=ru?ruH:enH};

  tx('.land-tag','Self-hosted платформа для алертов, совместимая с Telegram Bot API','Self-hosted alert platform, Telegram Bot API compatible');
  th('.land-desc','Self-hosted Telegram Bot API. Мониторинг, алерты, командный чат без внешних зависимостей. Работает, когда всё остальное заблокировано.','Self-hosted Telegram Bot API. Monitoring, alerts, team chat with no dependencies. Works when everything else is blocked.');

  // Stats
  const stats=document.querySelectorAll('.land-stat span');
  if(stats[2])stats[2].textContent=ru?'Bot API методов':'Bot API methods';
  if(stats[3])stats[3].textContent=ru?'старт':'startup';

  // Features list
  const feats=document.querySelectorAll('.land-feat li');
  const featRu=['Telegram Bot API — миграция в одну строку','Webhook Relay — localhost боты без ngrok','Каналы, inline-кнопки, Web Push','PWA клиент из коробки','Один бинарник, SQLite, zero config'];
  const featEn=['Telegram Bot API — one-line migration','Webhook Relay — localhost bots without ngrok','Channels, inline buttons, Web Push','PWA client out of the box','Single binary, SQLite, zero config'];
  feats.forEach((li,i)=>{if(i<5)li.textContent=ru?featRu[i]:featEn[i]});

  // FAQ
  tx('.land-faq h3','Частые вопросы','FAQ');
  const summaries=document.querySelectorAll('.land-faq summary');
  const sumRu=['Что это и зачем?','Нужно ставить приложение?','Как приходят уведомления на телефон?','Зачем это если есть Telegram?','Это ещё один мессенджер?'];
  const sumEn=['What is this and why?','Do I need to install an app?','How do phone notifications work?','Why this if Telegram exists?','Is this another messenger?'];
  summaries.forEach((s,i)=>{if(i<5)s.textContent=ru?sumRu[i]:sumEn[i]});

  const answers=document.querySelectorAll('.land-faq details p');
  const ansRu=[
    'Pusk — платформа алертов для ops-команд. Webhook из мониторинга (Grafana, Zabbix, Alertmanager), ACK одной кнопкой, push-уведомления. Плюс командный чат. Данные на вашем сервере.',
    'Нет. Открываете ссылку в браузере — и работаете. Можно добавить на главный экран как иконку, но это необязательно. Работает в Chrome, Firefox, Edge на телефоне и ПК.',
    'Через Web Push — стандарт браузера (как у Slack, Discord, Notion). Приходят даже когда браузер закрыт. На Android работает отлично, на iOS с Safari 16.4+.',
    'Pusk не заменяет Telegram для болтовни. Это инструмент для ops-команды: алерты с кнопкой ACK, данные на своём сервере, совместимость с Telegram-ботами — переехать можно за 1 строку кода.',
    'Нет, это замена Telegram для рабочих задач. Один канал для алертов, один для команды. Без стикеров, гифок и спама — только работа.'
  ];
  const ansEn=[
    'Pusk is an alert platform for ops teams. Webhooks from monitoring (Grafana, Zabbix, Alertmanager), one-click ACK, push notifications. Plus team chat. Data on your server.',
    'No. Open the link in your browser and start working. You can add it to your home screen as an icon, but it is optional. Works in Chrome, Firefox, Edge on phone and PC.',
    'Via Web Push — a browser standard (like Slack, Discord, Notion). Arrives even when browser is closed. Works great on Android, on iOS with Safari 16.4+.',
    'Pusk doesn\'t replace Telegram for casual chat. It\'s a tool for ops teams: alerts with ACK button, data on your server, Telegram bot compatibility — migrate in one line of code.',
    'No, it\'s a Telegram replacement for work. One channel for alerts, one for the team. No stickers, gifs, or spam — just work.'
  ];
  answers.forEach((p,i)=>{if(i<5)p.textContent=ru?ansRu[i]:ansEn[i]});

  // Buttons
  tx('#land-demo','Попробовать демо','Try demo');
  tx('#land-create-org','Создать организацию','Create organization');
  tx('#land-login','Войти','Login');

  // Links
  const links=document.querySelectorAll('.land-links a');
  if(links[0])links[0].textContent=ru?'Документация':'Documentation';
  if(links[1])links[1].textContent=ru?'Миграция':'Migration';

  // Demo loading
  const fb=document.getElementById('land-fallback');
  if(fb)fb.textContent=ru?'Загрузка демо...':'Loading demo...';

  // Org creation modal
  const orgTitle=document.querySelector('#org-modal-bg h3');
  if(orgTitle)orgTitle.textContent=ru?'Создать организацию':'Create organization';
  const orgInputs=['org-slug','org-name','org-user','org-pin'];
  const orgRu=['ID организации (латиница, напр: acme)','Название (напр: Acme Corp)','Логин администратора','Пароль'];
  const orgEn=['Organization ID (latin, e.g., acme)','Name (e.g., Acme Corp)','Admin username','Password'];
  orgInputs.forEach((id,i)=>{const el=document.getElementById(id);if(el)el.placeholder=ru?orgRu[i]:orgEn[i]});
  tx('#org-cancel','Отмена','Cancel');
  tx('#org-ok','Создать','Create');

  // Install banner
  const installText=document.querySelector('#install-banner span');
  if(installText)installText.textContent=ru?'Установите Pusk для push-уведомлений':'Install Pusk for push notifications';
  tx('#install-btn','Установить','Install');
  tx('#install-dismiss','Не сейчас','Not now');

  // Migration code block labels
  const codeEl=document.querySelector('.land-code');
  if(codeEl){
    codeEl.innerHTML=codeEl.innerHTML.replace(ru?'Was (Telegram)':'Было (Telegram)',ru?'Было (Telegram)':'Was (Telegram)');
    codeEl.innerHTML=codeEl.innerHTML.replace(ru?'Now (Pusk)':'Стало (Pusk)',ru?'Стало (Pusk)':'Now (Pusk)');
  }

  // "Migrate in one line" hint
  const migrateHints=document.querySelectorAll('#landing .land-left > p[style]');
  if(migrateHints[0])migrateHints[0].textContent=ru?'Мигрируйте за одну строку:':'Migrate in one line:';
  if(migrateHints[1])migrateHints[1].textContent=ru?'Запуск за 30 секунд:':'Start in 30 seconds:';

  // Demo chat header
  tx('.land-chat-hdr',null,null);// keep bot name as-is

  // Landing chat input
  const landInput=document.getElementById('land-input');
  if(landInput)landInput.placeholder=ru?'Напишите боту...':'Write to bot...';

  // Auth org placeholder
  const aOrg=document.getElementById('a-org');
  if(aOrg)aOrg.placeholder=ru?'ID организации (напр: acme)':'Organization ID (e.g., acme)';
}
window.translateLanding=translateLanding;

// ── Window binding for views.js logout() ──
window.initLandingChat=initLandingChat;
