import S from './state.js';
import {get,set,getJSON,setJSON} from './storage.js';
import {$,esc,escJs,t,api,toast,setLang,confirmDialog} from './util.js';
import {showApp,showList,logout} from './views.js';
import {registerPush,disconnectWS} from './ws.js';

// ── Settings panel ──
$('hdr-ava').onclick=$('hdr-name').onclick=()=>{const v=$('settings').style.display==='block';$('settings').style.display=v?'none':'block';$('settings-bg').style.display=v?'none':'block';if(!v){history.pushState(null,'',location.href);renderOrgSwitch();renderSettings();renderUsers()}};

async function renderSettings(){const u=get('uname')||'?';const o=get('org')||'default';$('s-profile').textContent=u+' @ '+o;const h=await api('GET','/api/health');$('s-about').innerHTML='Pusk '+(h.version||'?')+' <a href="https://github.com/getpusk/pusk" target="_blank" class="s-about-link">GitHub</a> <a href="https://github.com/getpusk/pusk#readme" target="_blank" class="s-about-link">Docs</a>';$('s-push-btn').textContent=Notification.permission==='granted'?t('push_on'):t('push_off');
  const ib=$('s-install');const isStandalone=window.matchMedia('(display-mode: standalone)').matches||window.navigator.standalone;if(!isStandalone){ib.style.display='block';if(S.deferredPrompt){ib.textContent=S.lang==='ru'?'\u0423\u0441\u0442\u0430\u043d\u043e\u0432\u0438\u0442\u044c \u043f\u0440\u0438\u043b\u043e\u0436\u0435\u043d\u0438\u0435':'Install app';ib.onclick=async()=>{if(S.deferredPrompt){S.deferredPrompt.prompt();const r=await S.deferredPrompt.userChoice;if(r.outcome==='accepted'){ib.style.display='none';toast(S.lang==='ru'?'\u0423\u0441\u0442\u0430\u043d\u043e\u0432\u043b\u0435\u043d\u043e!':'Installed!')}S.deferredPrompt=null}}}else{ib.textContent=S.lang==='ru'?'\u0423\u0441\u0442\u0430\u043d\u043e\u0432\u0438\u0442\u044c: \u041c\u0435\u043d\u044e \u0431\u0440\u0430\u0443\u0437\u0435\u0440\u0430 \u2192 \u041d\u0430 \u0433\u043b\u0430\u0432\u043d\u044b\u0439 \u044d\u043a\u0440\u0430\u043d':'Install: Browser menu \u2192 Add to home screen';ib.onclick=null;ib.style.opacity='0.7'}}else{ib.style.display='none'}

  // Hide invite for non-admin
  if(get("role")!=="admin"){$("s-invite").style.display="none"}else{$("s-invite").style.display=""}
  // Password change (small link, not full button)
  if(!$("s-change-pwd")){const p=document.createElement("button");p.id="s-change-pwd";p.className="s-btn";p.style.cssText="width:100%;font-size:12px;padding:6px;margin-top:4px;color:var(--text2)";p.textContent=S.lang==="ru"?"Сменить пароль":"Change password";p.onclick=async()=>{const o=prompt(S.lang==="ru"?"Текущий пароль:":"Current password:");if(!o)return;const n=prompt(S.lang==="ru"?"Новый пароль (мин. 6):":"New password (min 6):");if(!n||n.length<6){toast(S.lang==="ru"?"Минимум 6 символов":"Min 6 chars");return}const r=await api("POST","/api/change-password",{old_pin:o,new_pin:n});if(r.ok)toast(S.lang==="ru"?"Пароль изменён":"Password changed");else toast(r.error||"Error")};$("s-out").before(p)}
}

function togglePush(){if(Notification.permission==='granted'){$('s-push-btn').textContent=t('push_reload')}else{Notification.requestPermission().then(p=>{if(p==='granted'){registerPush();$('s-push-btn').textContent=t('push_on')}else{$('s-push-btn').textContent=t('push_blocked')}})}}

function testPush(){api('POST','/api/push/test').then(r=>{if(r.ok){toast(S.lang==='ru'?'Push \u043e\u0442\u043f\u0440\u0430\u0432\u043b\u0435\u043d! \u0415\u0441\u043b\u0438 \u043d\u0435 \u043f\u043e\u043b\u0443\u0447\u0438\u043b\u0438 \u2014 \u043f\u0440\u043e\u0432\u0435\u0440\u044c\u0442\u0435: 1) \u0420\u0430\u0437\u0440\u0435\u0448\u0435\u043d\u0438\u044f Chrome 2) \u041e\u043f\u0442\u0438\u043c\u0438\u0437\u0430\u0446\u0438\u044f \u0431\u0430\u0442\u0430\u0440\u0435\u0438 3) \u0423\u0441\u0442\u0430\u043d\u043e\u0432\u0438\u0442\u0435 \u043a\u0430\u043a \u043f\u0440\u0438\u043b\u043e\u0436\u0435\u043d\u0438\u0435':'Push sent! Check: 1) Chrome permissions 2) Battery optimization 3) Install as app')}else{toast(S.lang==='ru'?'\u041d\u0435\u0442 \u043f\u043e\u0434\u043f\u0438\u0441\u043a\u0438 \u043d\u0430 push. \u0412\u043a\u043b\u044e\u0447\u0438\u0442\u0435 Push \u0432 \u043d\u0430\u0441\u0442\u0440\u043e\u0439\u043a\u0430\u0445.':'No push subscription. Enable Push first.')}})}

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
  if(get("uname")!=="guest"){
  const addBtn=document.createElement('button');addBtn.className='s-btn s-full-btn';addBtn.textContent=S.lang==='ru'?'+ \u041d\u043e\u0432\u0430\u044f \u043e\u0440\u0433\u0430\u043d\u0438\u0437\u0430\u0446\u0438\u044f':'+ New organization';
  addBtn.onclick=()=>{$('settings').style.display='none';$('settings-bg').style.display='none';$('org-modal-bg').classList.add('open');history.pushState(null,'',location.href);$('org-slug').focus()};
  el.appendChild(addBtn);
  }
    }
}

// ── Event delegation on #s-org-switch ──
$('s-org-switch').addEventListener('click',e=>{
  const btn=e.target.closest('[data-org]');
  if(btn)switchOrg(btn.dataset.org);
});

async function renderUsers(){const el=$('s-users');const isAdmin=get('role')==='admin'||get('uid')==='1';const users=await api('GET','/api/users');if(!users||!users.length){el.innerHTML='';return}const me=get('uname');
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
$('s-lang-btn').onclick=()=>{S.lang=S.lang==='ru'?'en':'ru';set('lang',S.lang);setLang();if($('app').style.display==='flex')showList()};
$('s-invite').onclick=async()=>{if(S.inviteUrl){navigator.clipboard.writeText(S.inviteUrl);$('s-invite').textContent=t('invited');setTimeout(()=>{$('s-invite').textContent=t('invite')},2000);return}const r=await api('POST','/api/invite');if(r.code){const o=get('org')||'default';S.inviteUrl=location.origin+r.url+'?org='+o;$('s-invite-result').textContent=S.inviteUrl;navigator.clipboard.writeText(S.inviteUrl);$('s-invite').textContent=t('invited');setTimeout(()=>{$('s-invite').textContent=t('invite')},2000)}};
$('s-out').onclick=()=>{$('settings').style.display='none';$('settings-bg').style.display='none';logout()};

// ── Push/test buttons: use onclick on existing DOM elements ──
$('s-push-btn').onclick=togglePush;
$('s-test-push').onclick=testPush;
