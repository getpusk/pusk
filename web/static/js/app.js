import S from './state.js';
import {get,set} from './storage.js';
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
S.invite=_p.get('invite');
if(S.invite&&!S.token){
  hideLanding();$('auth').style.display='flex';
  const _invOrg=_p.get('org');if(_invOrg)$('a-org').value=_invOrg;
  $('btn-login').style.display='none';$('btn-demo').style.display='none';
  $('btn-reg').className='abtn abtn-p';$('btn-reg').textContent=t('register_btn');
  $('a-err').style.color='var(--accent)';$('a-err').textContent=t('invite_hint');$('a-err').style.fontSize='15px';$('a-err').style.marginBottom='12px';
} else if(_p.get('demo')==='1'&&!S.token){
  hideLanding();$('auth').style.display='flex';$('btn-demo').click();
} else if(S.token){
  // Validate token — if guest in default org, check if user has real orgs
  const savedOrg=localStorage.getItem('org')||'default';
  const savedUser=localStorage.getItem('uname')||'';
  if(savedUser==='guest'&&savedOrg==='default'){
    // Guest session from demo — don't auto-login, show landing
    localStorage.removeItem('token');S.token=null;
    initLandingChat();
  } else {
    hideLanding();api('GET','/api/bots').then(r=>{if(r&&!r.error)showApp();else{const o=localStorage.getItem('org');const u=localStorage.getItem('uname');logout();if(o&&o!=='default'){$('a-org').value=o;if(u)$('a-user').value=u;$('a-err').textContent=S.lang==='ru'?'Сессия истекла — войдите снова':'Session expired — login again';$('a-err').style.color='var(--accent)'}}}).catch(()=>{const o=localStorage.getItem('org');const u=localStorage.getItem('uname');logout();if(o&&o!=='default'){$('a-org').value=o;if(u)$('a-user').value=u;$('a-err').textContent=S.lang==='ru'?'Сессия истекла — войдите снова':'Session expired — login again';$('a-err').style.color='var(--accent)'}});
  }
} else {
  initLandingChat();
}

}catch(e){console.error("[pusk] init error:",e)}

// ── Push notification navigation (from SW postMessage) ──
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.addEventListener('message', e => {
    if (e.data && e.data.type === 'push-navigate') {
      const p = new URLSearchParams(e.data.url.replace(/^.*\?/, ''));
      const ch = p.get('channel');
      const chat = p.get('chat');
      const pushOrg = p.get('org');
      // If push is from a different org, ignore (user must switch manually)
      const curOrg = localStorage.getItem('org') || 'default';
      if (pushOrg && pushOrg !== curOrg) {
        import('./util.js').then(u => u.toast(u.S.lang==='ru' ? 'Переключитесь в орг '+pushOrg : 'Switch to org '+pushOrg));
        return;
      }
      if (ch) import('./views.js').then(v => v.openChan(+ch, ''));
      else if (chat) import('./views.js').then(v => v.openChat(+chat, ''));
    }
  });
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
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js').then(reg => {
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
  });
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
