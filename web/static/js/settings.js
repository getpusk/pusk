import S from './state.js';
import {get,set,getJSON,setJSON} from './storage.js';
import {$,esc,escJs,t,api,toast,setLang,confirmDialog} from './util.js';
import {showApp,showList,logout} from './views.js';
import {registerPush,disconnectWS} from './ws.js';

// ── Settings panel ──
$('hdr-ava').onclick=$('hdr-name').onclick=()=>{const v=$('settings').style.display==='block';$('settings').style.display=v?'none':'flex';$('settings-bg').style.display=v?'none':'block';if(!v){history.pushState(null,'',location.href);renderOrgSwitch();renderSettings();renderUsers()}};

async function renderSettings(){const u=get('uname')||'?';const o=get('org')||'default';$('s-profile').textContent=u+' @ '+o;const h=await api('GET','/api/health');$('s-about').innerHTML='Pusk '+(h.version||'?')+' <a href="https://github.com/getpusk/pusk" target="_blank" class="s-about-link">GitHub</a> <a href="https://github.com/getpusk/pusk#readme" target="_blank" class="s-about-link">Docs</a>';if((get('org')||'default')==='default'){$('s-push-btn').textContent=S.lang==='ru'?'Создайте организацию':'Create org';$('s-push-btn').style.opacity='0.5';
  if(!$('s-demo-banner')){const b=document.createElement('div');b.id='s-demo-banner';b.style.cssText='background:var(--bg2);border:1px solid var(--accent);border-radius:8px;padding:12px;margin:8px 0;text-align:center';b.innerHTML=(S.lang==='ru'?'<div style="font-size:13px;color:var(--text2);margin-bottom:8px">Демо-режим. Push, смена пароля и приглашения доступны только в организации.</div>':'<div style="font-size:13px;color:var(--text2);margin-bottom:8px">Demo mode. Push, password change and invites are only available in organizations.</div>');const rb=document.createElement('button');rb.className='s-btn s-full-btn';rb.style.cssText='background:var(--accent);color:#fff;margin-top:4px';rb.textContent=S.lang==='ru'?'Зарегистрироваться и создать организацию':'Register & create organization';rb.onclick=()=>{$('settings').style.display='none';$('settings-bg').style.display='none';logout()};b.appendChild(rb);$('s-org-switch').before(b)}
}else{(async()=>{let pushLabel;if(Notification.permission==='denied'){pushLabel=S.lang==='ru'?'Заблокировано ✕':'Blocked ✕'}else if(Notification.permission==='granted'&&'serviceWorker' in navigator){try{const r=await navigator.serviceWorker.ready;const s=await r.pushManager.getSubscription();pushLabel=s?(S.lang==='ru'?'Push: Вкл ✓':'Push: On ✓'):(S.lang==='ru'?'Push: Выкл':'Push: Off')}catch{pushLabel=S.lang==='ru'?'Push: Вкл ✓':'Push: On ✓'}}else{pushLabel=S.lang==='ru'?'Подключить':'Connect'}$('s-push-btn').textContent=pushLabel})();$('s-push-btn').style.opacity='1';const db=$('s-demo-banner');if(db)db.remove()}
  const ib=$('s-install');const isStandalone=window.matchMedia('(display-mode: standalone)').matches||window.navigator.standalone;if(!isStandalone){ib.style.display='block';if(S.deferredPrompt){ib.textContent=S.lang==='ru'?'\u0423\u0441\u0442\u0430\u043d\u043e\u0432\u0438\u0442\u044c \u043f\u0440\u0438\u043b\u043e\u0436\u0435\u043d\u0438\u0435':'Install app';ib.onclick=async()=>{if(S.deferredPrompt){S.deferredPrompt.prompt();const r=await S.deferredPrompt.userChoice;if(r.outcome==='accepted'){ib.style.display='none';toast(S.lang==='ru'?'\u0423\u0441\u0442\u0430\u043d\u043e\u0432\u043b\u0435\u043d\u043e!':'Installed!')}S.deferredPrompt=null}}}else{ib.style.display='none'}}else{ib.style.display='none'}

  // Hide invite for non-admin
  if(get("role")!=="admin"||(get("org")||"default")==="default"){$("s-invite").style.display="none"}else{$("s-invite").style.display=""}
  // Password change (small link, not full button)
  if(!$("s-change-pwd")&&(get("org")||"default")!=="default"){const p=document.createElement("button");p.id="s-change-pwd";p.className="s-btn";p.style.cssText="width:100%;font-size:12px;padding:6px;margin-top:4px;color:var(--text2)";p.textContent=S.lang==="ru"?"Сменить пароль":"Change password";p.onclick=async()=>{const o=prompt(S.lang==="ru"?"Текущий пароль:":"Current password:");if(!o)return;const n=prompt(S.lang==="ru"?"Новый пароль (мин. 6):":"New password (min 6):");if(!n||n.length<6){toast(S.lang==="ru"?"Минимум 6 символов":"Min 6 chars");return}const r=await api("POST","/api/change-password",{old_pin:o,new_pin:n});if(r.ok)toast(S.lang==="ru"?"Пароль изменён":"Password changed");else toast(r.error||"Error")};$("s-out").before(p)}
}

async function togglePush(){if((get('org')||'default')==='default'){toast(S.lang==='ru'?'Push доступен только в организации. Создайте организацию в настройках.':'Push available only in organizations. Create one in settings.');return}
  if(Notification.permission==='denied'){
    $('s-push-btn').textContent=S.lang==='ru'?'Заблокировано ✕':'Blocked ✕';
    toast(S.lang==='ru'?'Push заблокирован в браузере. Разрешите в настройках сайта.':'Push blocked by browser. Allow in site settings.');
    return;
  }
  // Check if already subscribed
  if('serviceWorker' in navigator&&'PushManager' in window){
    try{
      const reg=await navigator.serviceWorker.ready;
      const sub=await reg.pushManager.getSubscription();
      if(sub){
        // Currently subscribed -> unsubscribe
        const ep=sub.endpoint;
        await sub.unsubscribe();
        await api('DELETE','/api/push/subscribe',{endpoint:ep});
        $('s-push-btn').textContent=S.lang==='ru'?'Push: Выкл':'Push: Off';
        toast(S.lang==='ru'?'Отписан от push уведомлений':'Unsubscribed from push');
        return;
      }
    }catch(e){console.warn('push unsub check error',e)}
  }
  // Not subscribed -> subscribe
  if(Notification.permission==='granted'){
    registerPush();
    toast(S.lang==='ru'?'Подписан на push уведомления':'Subscribed to push');
    $('s-push-btn').textContent=S.lang==='ru'?'Push: Вкл ✓':'Push: On ✓';
  }else{Notification.requestPermission().then(p=>{if(p==='granted'){registerPush();$('s-push-btn').textContent=S.lang==='ru'?'Push: Вкл ✓':'Push: On ✓'}else{$('s-push-btn').textContent=S.lang==='ru'?'Заблокировано ✕':'Blocked ✕'}})}}

function testPush(){if((get('org')||'default')==='default'){toast(S.lang==='ru'?'Push доступен только в организации. Создайте организацию в настройках.':'Push available only in organizations. Create one in settings.');return}api('POST','/api/push/test').then(r=>{if(r.ok){toast(S.lang==='ru'?'Push отправлен! Если не получили — проверьте: 1) Разрешения Chrome 2) Оптимизация батареи 3) Установите как приложение':'Push sent! Check: 1) Chrome permissions 2) Battery optimization 3) Install as app')}else{toast(r.error||(S.lang==='ru'?'Нет подписки на push. Включите Push в настройках.':'No push subscription. Enable Push first.'))}})}

function renderOrgSwitch(){const el=$('s-org-switch');const orgs=getJSON('orgs')||{};const cur=get('org')||'default';const keys=Object.keys(orgs);
  el.innerHTML='';
  if(keys.length>1){
    const label=document.createElement('div');label.className='s-label';label.textContent=t('orgs_title');el.appendChild(label);
    keys.forEach(k=>{const o=orgs[k];const btn=document.createElement('button');btn.className='s-btn s-org-btn'+(k===cur?' active':'');btn.dataset.org=k;
      const b=document.createElement('b');b.textContent=k;btn.appendChild(b);
      const span=document.createElement('span');span.className='s-org-user';span.textContent=' ('+(o.user||'?')+')';btn.appendChild(span);
      if(k===cur)btn.appendChild(document.createTextNode(' \u2713'));
      el.appendChild(btn);
    });
  }
  if(true){
  const addBtn=document.createElement('button');addBtn.className='s-btn s-full-btn';addBtn.textContent=S.lang==='ru'?'+ \u041d\u043e\u0432\u0430\u044f \u043e\u0440\u0433\u0430\u043d\u0438\u0437\u0430\u0446\u0438\u044f':'+ New organization';
  addBtn.onclick=()=>{$('settings').style.display='none';$('settings-bg').style.display='none';$('org-modal-bg').classList.add('open');history.pushState(null,'',location.href);$('org-slug').focus()};
  el.appendChild(addBtn);
  }
}

// ── Event delegation on #s-org-switch ──
$('s-org-switch').addEventListener('click',e=>{
  const btn=e.target.closest('[data-org]');
  if(btn)switchOrg(btn.dataset.org);
});

async function renderUsers(){const el=$('s-users');const isAdmin=get('role')==='admin';const users=await api('GET','/api/users');if(!users||!users.length){el.innerHTML='';return}const me=get('uname');
  el.innerHTML='';
  const label=document.createElement('div');label.className='s-label';label.textContent=(S.lang==='ru'?'\u041f\u043e\u043b\u044c\u0437\u043e\u0432\u0430\u0442\u0435\u043b\u0438':'Users')+' ('+users.length+'):';el.appendChild(label);
  users.forEach(u=>{
    const row=document.createElement('div');row.className='user-row';
    const nameSpan=document.createElement('span');nameSpan.className=u.role==='admin'?'user-admin':'user-member';nameSpan.textContent=u.username;row.appendChild(nameSpan);
    const roleSpan=document.createElement('span');roleSpan.className='user-role';roleSpan.textContent=u.role;row.appendChild(roleSpan);
    if(isAdmin&&u.username!==me&&u.id!==1){
      const actions=document.createElement('span');actions.className='user-actions';
      const roleBtn=document.createElement('button');roleBtn.className='s-btn s-btn-sm';roleBtn.dataset.action='set-role';roleBtn.dataset.uid=u.id;roleBtn.dataset.newRole=u.role==='admin'?'member':'admin';roleBtn.textContent=u.role==='admin'?'\u2192member':'\u2192admin';actions.appendChild(roleBtn);
      const delBtn=document.createElement('button');delBtn.className='s-btn s-btn-sm danger';delBtn.dataset.action='del-user';delBtn.dataset.uid=u.id;delBtn.dataset.username=u.username;delBtn.textContent='x';actions.appendChild(delBtn);
      row.appendChild(actions);
    }
    el.appendChild(row);
  });
}

// ── Event delegation on #s-users ──
$('s-users').addEventListener('click',e=>{
  const roleBtn=e.target.closest('[data-action="set-role"]');
  if(roleBtn){setRole(+roleBtn.dataset.uid,roleBtn.dataset.newRole);return}
  const delBtn=e.target.closest('[data-action="del-user"]');
  if(delBtn){delUser(+delBtn.dataset.uid,delBtn.dataset.username);return}
});

function setRole(uid,role){api('POST',`/api/users/${uid}/role`,{role}).then(()=>renderUsers())}
async function delUser(uid,name){if(!await confirmDialog((S.lang==='ru'?'\u0423\u0434\u0430\u043b\u0438\u0442\u044c \u043f\u043e\u043b\u044c\u0437\u043e\u0432\u0430\u0442\u0435\u043b\u044f ':'Delete user ')+name+'?'))return;await api('DELETE',`/api/users/${uid}`);renderUsers()}
function switchOrg(slug){const orgs=getJSON('orgs')||{};const o=orgs[slug];if(!o)return;S.token=o.token;set('token',o.token);set('uname',o.user);set('org',slug);if(o.role)set('role',o.role);$('settings').style.display='none';$('settings-bg').style.display='none';disconnectWS();showApp()}

$('settings-bg').onclick=()=>{$('settings').style.display='none';$('settings-bg').style.display='none'};
$('s-lang-btn').onclick=()=>{S.lang=S.lang==='ru'?'en':'ru';set('lang',S.lang);setLang();renderSettings();if($('app').style.display==='flex')showList()};
$('s-invite').onclick=async()=>{if(S.inviteUrl){navigator.clipboard.writeText(S.inviteUrl);$('s-invite').textContent=t('invited');setTimeout(()=>{$('s-invite').textContent=t('invite')},2000);return}const r=await api('POST','/api/invite');if(r.code){const o=get('org')||'default';S.inviteUrl=location.origin+r.url+'?org='+o;$('s-invite-result').textContent=S.inviteUrl;navigator.clipboard.writeText(S.inviteUrl);$('s-invite').textContent=t('invited');setTimeout(()=>{$('s-invite').textContent=t('invite')},2000)}};
$('s-out').onclick=()=>{$('settings').style.display='none';$('settings-bg').style.display='none';logout()};

// ── Push/test buttons: use onclick on existing DOM elements ──
$('s-push-btn').onclick=togglePush;
$('s-test-push').onclick=testPush;
