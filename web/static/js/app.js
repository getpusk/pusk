import S from './state.js';
import {get,set,remove,getJSON} from './storage.js';
import {$,t,api,setLang,initEye,toast} from './util.js';
import {showApp,logout} from './views.js';
import {initLandingChat,hideLanding} from './landing.js';
// Side-effect imports: register event handlers
import './ws.js';
import './actions.js';
import './settings.js';

// ── PWA Install Prompt ──
window.addEventListener('beforeinstallprompt',e=>{
  e.preventDefault();S.deferredPrompt=e;
  if(!get('installDismissed')){$('install-banner').style.display='block'}
});
$('install-btn').onclick=async()=>{
  if(S.deferredPrompt){S.deferredPrompt.prompt();const r=await S.deferredPrompt.userChoice;if(r.outcome==='accepted')console.log('[pwa] installed');S.deferredPrompt=null}
  $('install-banner').style.display='none';
};
$('install-dismiss').onclick=()=>{$('install-banner').style.display='none';set('installDismissed','1')};

// ── Eye toggle (all DOM ready) ──
initEye('eye-btn','a-pin');
initEye('org-eye-btn','org-pin');

// ── Init ──
try{
setLang();
(async()=>{const h=await fetch('/api/health').then(r=>r.json()).catch(()=>({}));const ver=h.version||'';$('a-ver').textContent=ver;if($('land-ver'))$('land-ver').textContent=ver})();

const _p=new URLSearchParams(location.search);
// Handle cross-org push URL (from SW openWindow or location.reload)
const _pushOrg=_p.get('org');const _pushCh=_p.get('channel');const _pushChat=_p.get('chat');
const _curOrg=get('org')||'default';
if(_pushOrg&&(_pushCh||_pushChat)&&_pushOrg!==_curOrg){
  const _orgs=getJSON('orgs')||{};const _o=_orgs[_pushOrg];
  if(_o&&_o.token){
    S.token=_o.token;set('token',_o.token);set('org',_pushOrg);
    if(_o.user)set('uname',_o.user);
    if(_o.display_name)set('display_name',_o.display_name);
    if(_o.role)set('role',_o.role);
    sessionStorage.setItem('pushNav',JSON.stringify({channel:_pushCh,chat:_pushChat}));
    history.replaceState(null,'',location.pathname);
  }
}
S.invite=_p.get('invite');
if(S.invite){
  hideLanding();$('auth').style.display='flex';
  const _invOrg=_p.get('org');if(_invOrg)$('a-org').value=_invOrg;
  $('btn-demo').style.display='none';
  $('btn-reg').className='abtn abtn-p';$('btn-reg').textContent=t('register_btn');
  $('btn-login').style.display='none';
  $('auth-sub').textContent=t('invite_hint');$('a-user').setAttribute('autocomplete','off');$('auth-sub').style.color='var(--accent)';$('auth-sub').style.fontSize='15px';
  const _invCode=S.invite;const _invOrg2=_p.get('org')||'';
  $('a-user').addEventListener('blur',async()=>{const u=$('a-user').value.trim();if(!u||!_invCode||!_invOrg2)return;try{const r=await fetch('/api/invite/check-user?code='+_invCode+'&org='+_invOrg2+'&username='+encodeURIComponent(u));const d=await r.json();if(d.exists){$('btn-login').style.display='';$('btn-login').textContent=S.lang==='ru'?'Войти':'Login';$('btn-reg').style.display='none';$('auth-sub').textContent=S.lang==='ru'?'Аккаунт найден — введите пароль':'Account found — enter password'}else{$('btn-reg').style.display='';$('btn-reg').textContent=t('register_btn');$('btn-login').style.display='none';$('auth-sub').textContent=t('invite_hint')}}catch{}});
} else if(_p.get('demo')==='1'&&!S.token){
  hideLanding();$('auth').style.display='flex';$('btn-demo').click();
} else if(S.token){
  // Validate token — if guest in default org, check if user has real orgs
  const savedOrg=get('org')||'default';
  const savedUser=get('uname')||'';
  if(savedUser==='guest'&&savedOrg==='default'){
    // Guest session from demo — don't auto-login, show landing
    remove('token');S.token=null;
    initLandingChat();
  } else {
    hideLanding();api('GET','/api/bots').then(r=>{if(r&&!r.error)showApp();else{const o=get('org');const u=get('uname');logout();if(o&&o!=='default'){$('a-org').value=o;if(u)$('a-user').value=u;$('a-err').textContent=S.lang==='ru'?'Сессия истекла — войдите снова':'Session expired — login again';$('a-err').style.color='var(--accent)'}}}).catch(()=>{const o=get('org');const u=get('uname');logout();if(o&&o!=='default'){$('a-org').value=o;if(u)$('a-user').value=u;$('a-err').textContent=S.lang==='ru'?'Сессия истекла — войдите снова':'Session expired — login again';$('a-err').style.color='var(--accent)'}});
  }
} else {
  initLandingChat();
}

}catch(e){console.error("[pusk] init error:",e)}

// ── Push notification navigation (from SW IDB or postMessage) ──
function _handlePushTarget(url) {
  const p = new URLSearchParams(url.replace(/^.*\?/, ''));
  const ch = p.get('channel');
  const chat = p.get('chat');
  const pushOrg = p.get('org');
  const curOrg = get('org') || 'default';
  if (pushOrg && pushOrg !== curOrg) {
    const orgs = getJSON('orgs') || {};
    const o = orgs[pushOrg];
    if (o && o.token) {
      sessionStorage.setItem('pushNav', JSON.stringify({channel: ch, chat: chat}));
      set('token', o.token);
      set('org', pushOrg);
      if (o.user) set('uname', o.user);
      if (o.display_name) set('display_name', o.display_name);
      if (o.role) set('role', o.role);
      location.reload();
      return true;
    }
  } else if (ch) {
    import('./views.js').then(v => v.openChan(+ch, ''));
    return true;
  } else if (chat) {
    import('./views.js').then(v => v.openChat(+chat, ''));
    return true;
  }
  return false;
}
// IDB helpers (same DB as SW)
function _idbGet(key){return new Promise((resolve)=>{try{const req=indexedDB.open('pusk-sw',1);req.onupgradeneeded=()=>req.result.createObjectStore('kv');req.onsuccess=()=>{const db=req.result;const tx=db.transaction('kv','readonly');const g=tx.objectStore('kv').get(key);g.onsuccess=()=>{db.close();resolve(g.result)};g.onerror=()=>{db.close();resolve(null)}};req.onerror=()=>resolve(null)}catch(e){resolve(null)}})}
function _idbDel(key){try{const req=indexedDB.open('pusk-sw',1);req.onsuccess=()=>{const db=req.result;const tx=db.transaction('kv','readwrite');tx.objectStore('kv').delete(key);tx.oncomplete=()=>db.close()}}catch(e){}}
// Check IDB for pending push target (from SW notificationclick)
function _checkPushIDB(){_idbGet('pushTarget').then(target=>{if(target){_idbDel('pushTarget');_handlePushTarget(target)}})}
_checkPushIDB();
// Also check when PWA comes to foreground (user clicked notification while app was in background)
document.addEventListener('visibilitychange',()=>{if(!document.hidden)setTimeout(_checkPushIDB,100)});
if ('serviceWorker' in navigator && navigator.serviceWorker) {
  try { navigator.serviceWorker.addEventListener('message', e => {
    // SW sends push-click when notification was tapped
    if (e.data && e.data.type === 'push-click') {
      _idbGet('pushTarget').then(target=>{if(target){_idbDel('pushTarget');_handlePushTarget(target)}});
      return;
    }
    if (e.data && e.data.type === 'push-navigate') {
      _handlePushTarget(e.data.url);
      return;
    }
  });
  } catch(e) {}
}
// ── Offline indicator ──
const offBar = $('offline-bar');
if (offBar) {
  window.addEventListener('online', () => offBar.classList.remove('show'));
  window.addEventListener('offline', () => offBar.classList.add('show'));
  if (!navigator.onLine) offBar.classList.add('show');
}

// ── Update bar handlers ──
const updReload = $('update-reload');
if (updReload) updReload.onclick = () => location.reload();
const updDismiss = $('update-dismiss');
if (updDismiss) updDismiss.onclick = () => $('update-bar').classList.remove('show');

// ── SW update notification ──
if ('serviceWorker' in navigator && navigator.serviceWorker) {
  try { navigator.serviceWorker.register('/sw.js').then(reg => {
    // Check for updates when tab becomes visible (covers Android PWA)
    // Mobile keyboard: scroll messages when virtual keyboard opens
  if (window.visualViewport) {
    window.visualViewport.addEventListener('resize', () => {
      const msgs = $('msgs');
      if (msgs && msgs.style && getComputedStyle(msgs).display !== 'none') {
        setTimeout(() => msgs.scrollTop = msgs.scrollHeight, 100);
      }
    });
  }

  document.addEventListener('visibilitychange', () => {
      if (!document.hidden) reg.update().catch(() => {});
    });
    reg.addEventListener('updatefound', () => {
      const newSW = reg.installing;
      newSW.addEventListener('statechange', () => {
        if (newSW.state === 'activated' && navigator.serviceWorker.controller) {
          const bar = $('update-bar');
          if (bar) bar.classList.add('show');
        }
      });
    });
  }).catch(() => {});
  } catch(e) {}
}

// ── Mobile back button ──
window.addEventListener('popstate', () => {
  // Close overlays first
  for(const id of ['ctx-menu']){const el=$(id);if(el&&el.style.display==='block'){el.style.display='none';history.pushState(null,'',location.href);return}}
  for(const id of ['onboard-bg','confirm-bg','modal-bg','org-modal-bg']){const el=$(id);if(el&&el.classList.contains('open')){el.classList.remove('open');history.pushState(null,'',location.href);return}}
  const stg=$('settings');if(stg&&stg.style.display==='block'){stg.style.display='none';$('settings-bg').style.display='none';history.pushState(null,'',location.href);return}
  // Navigate back from chat/channel
  if (S.curChat || S.curChan) {
    import('./views.js').then(v => v.showList());
    return;
  }
  // On main list: double-back to exit
  if (!S._backExit) {
    S._backExit = true;
    history.pushState(null, '', location.href);
    toast(S.lang === 'ru' ? 'Нажмите ещё раз для выхода' : 'Press back again to exit');
    setTimeout(() => { S._backExit = false; }, 2000);
  }
});

// ── Escape key closes overlays ──
document.addEventListener('keydown', e => {
  if (e.key === 'Escape') {
    const ctx = $('ctx-menu');
    if (ctx && ctx.style.display === 'block') { ctx.style.display = 'none'; return; }
    const onboard = $('onboard-bg');
    if (onboard && onboard.classList.contains('open')) { onboard.classList.remove('open'); return; }
    const confirm = $('confirm-bg');
    if (confirm && confirm.classList.contains('open')) { confirm.classList.remove('open'); return; }
    const modal = $('modal-bg');
    if (modal && modal.classList.contains('open')) { modal.classList.remove('open'); return; }
    const orgModal = $('org-modal-bg');
    if (orgModal && orgModal.classList.contains('open')) { orgModal.classList.remove('open'); return; }
    const settings = $('settings');
    if (settings && settings.style.display === 'block') { settings.style.display = 'none'; $('settings-bg').style.display = 'none'; return; }
  }
});
