import S from './state.js';
import {$,esc,escJs,t,api,toast,setLang} from './util.js';
import {showApp,showList,logout} from './views.js';
import {registerPush} from './ws.js';

// ── Settings panel ──
$('hdr-ava').onclick=$('hdr-name').onclick=()=>{const v=$('settings').style.display==='block';$('settings').style.display=v?'none':'block';$('settings-bg').style.display=v?'none':'block';if(!v){renderOrgSwitch();renderSettings();renderUsers()}};

async function renderSettings(){const u=localStorage.getItem('pusk_uname')||'?';const o=localStorage.getItem('pusk_org')||'default';$('s-profile').textContent=u+' @ '+o;const h=await api('GET','/api/health');$('s-about').innerHTML='Pusk '+(h.version||'?')+' <a href="https://github.com/getpusk/pusk" target="_blank" style="color:var(--accent);text-decoration:none;margin-left:8px">GitHub</a> <a href="https://github.com/getpusk/pusk#readme" target="_blank" style="color:var(--accent);text-decoration:none;margin-left:8px">Docs</a>';$('s-push-btn').textContent=Notification.permission==='granted'?t('push_on'):t('push_off');
  const ib=$('s-install');const isStandalone=window.matchMedia('(display-mode: standalone)').matches||window.navigator.standalone;if(!isStandalone){ib.style.display='block';if(S.deferredPrompt){ib.textContent=S.lang==='ru'?'Установить приложение':'Install app';ib.onclick=async()=>{if(S.deferredPrompt){S.deferredPrompt.prompt();const r=await S.deferredPrompt.userChoice;if(r.outcome==='accepted'){ib.style.display='none';toast(S.lang==='ru'?'Установлено!':'Installed!')}S.deferredPrompt=null}}}else{ib.textContent=S.lang==='ru'?'Установить: Меню браузера → На главный экран':'Install: Browser menu → Add to home screen';ib.onclick=null;ib.style.opacity='0.7'}}else{ib.style.display='none'}}

function togglePush(){if(Notification.permission==='granted'){$('s-push-btn').textContent=t('push_reload')}else{Notification.requestPermission().then(p=>{if(p==='granted'){registerPush();$('s-push-btn').textContent=t('push_on')}else{$('s-push-btn').textContent=t('push_blocked')}})}}

function testPush(){api('POST','/api/push/test').then(r=>{if(r.ok){toast(S.lang==='ru'?'Push отправлен! Если не получили — проверьте: 1) Разрешения Chrome 2) Оптимизация батареи 3) Установите как приложение':'Push sent! Check: 1) Chrome permissions 2) Battery optimization 3) Install as app')}else{toast(S.lang==='ru'?'Нет подписки на push. Включите Push в настройках.':'No push subscription. Enable Push first.')}})}

function renderOrgSwitch(){const el=$('s-org-switch');const orgs=JSON.parse(localStorage.getItem('pusk_orgs')||'{}');const cur=localStorage.getItem('pusk_org')||'default';const keys=Object.keys(orgs);if(keys.length<=1){el.innerHTML='';return}el.innerHTML='<div style="font-size:12px;color:var(--text2);margin-bottom:4px">'+t('orgs_title')+'</div>'+keys.map(k=>{const o=orgs[k];return`<button class="s-btn" style="width:100%;margin-bottom:4px;padding:8px;text-align:left;${k===cur?'background:var(--primary);color:#fff':''}" onclick="window.switchOrg('${k}')"><b>${k}</b> <span style="opacity:0.7;font-size:12px">(${o.user||'?'})</span>${k===cur?' ✓':''}</button>`}).join('')}

async function renderUsers(){const el=$('s-users');const isAdmin=localStorage.getItem('pusk_role')==='admin'||localStorage.getItem('pusk_uid')==='1';const users=await api('GET','/api/users');if(!users||!users.length){el.innerHTML='';return}const me=localStorage.getItem('pusk_uname');el.innerHTML='<div style="font-size:12px;color:var(--text2);margin-bottom:4px">'+(S.lang==='ru'?'Пользователи':'Users')+' ('+users.length+'):</div>'+users.map(u=>{let actions='';if(isAdmin&&u.username!==me&&u.id!==1){actions=`<span style="margin-left:auto;display:flex;gap:4px"><button class="s-btn" style="font-size:11px;padding:2px 8px" onclick="window.setRole(${u.id},'${u.role==='admin'?'member':'admin'}')">${u.role==='admin'?'→member':'→admin'}</button><button class="s-btn" style="font-size:11px;padding:2px 8px;color:#e05d44;border-color:#e05d44" onclick="window.delUser(${u.id},'${escJs(u.username)}')">x</button></span>`}return`<div style="display:flex;align-items:center;padding:4px 0;font-size:13px"><span style="color:${u.role==='admin'?'var(--accent)':'var(--text)'}">${esc(u.username)}</span><span style="color:var(--text2);font-size:11px;margin-left:6px">${u.role}</span>${actions}</div>`}).join('')}

function setRole(uid,role){api('POST',`/api/users/${uid}/role`,{role}).then(()=>renderUsers())}
function delUser(uid,name){if(!confirm((S.lang==='ru'?'Удалить пользователя ':'Delete user ')+name+'?'))return;api('DELETE',`/api/users/${uid}`).then(()=>renderUsers())}
function switchOrg(slug){const orgs=JSON.parse(localStorage.getItem('pusk_orgs')||'{}');const o=orgs[slug];if(!o)return;S.token=o.token;localStorage.setItem('pusk_token',o.token);localStorage.setItem('pusk_uname',o.user);localStorage.setItem('pusk_org',slug);if(o.role)localStorage.setItem('pusk_role',o.role);$('settings').style.display='none';$('settings-bg').style.display='none';if(S.ws)S.ws.close();showApp()}

$('settings-bg').onclick=()=>{$('settings').style.display='none';$('settings-bg').style.display='none'};
$('s-lang-btn').onclick=()=>{S.lang=S.lang==='ru'?'en':'ru';localStorage.setItem('pusk_lang',S.lang);setLang();if($('app').style.display==='flex')showList()};
$('s-invite').onclick=async()=>{if(S.inviteUrl){navigator.clipboard.writeText(S.inviteUrl);$('s-invite').textContent=t('invited');setTimeout(()=>{$('s-invite').textContent=t('invite')},2000);return}const r=await api('POST','/api/invite');if(r.code){const o=localStorage.getItem('pusk_org')||'default';S.inviteUrl=location.origin+r.url+'?org='+o;$('s-invite-result').textContent=S.inviteUrl;navigator.clipboard.writeText(S.inviteUrl);$('s-invite').textContent=t('invited');setTimeout(()=>{$('s-invite').textContent=t('invite')},2000)}};
$('s-out').onclick=()=>{$('settings').style.display='none';$('settings-bg').style.display='none';logout()};

// ── Window bindings ──
window.togglePush=togglePush;
window.testPush=testPush;
window.switchOrg=switchOrg;
window.setRole=setRole;
window.delUser=delUser;
