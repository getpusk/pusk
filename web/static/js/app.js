import S from './state.js';
import {$,t,api,setLang,initEye} from './util.js';
import {showApp,logout} from './views.js';
import {initLandingChat,hideLanding} from './landing.js';
// Side-effect imports: register event handlers
import './ws.js';
import './actions.js';
import './settings.js';

// ── PWA Install Prompt ──
window.addEventListener('beforeinstallprompt',e=>{
  e.preventDefault();S.deferredPrompt=e;
  if(!localStorage.getItem('pusk_install_dismissed')){$('install-banner').style.display='block'}
});
$('install-btn').onclick=async()=>{
  if(S.deferredPrompt){S.deferredPrompt.prompt();const r=await S.deferredPrompt.userChoice;if(r.outcome==='accepted')console.log('[pwa] installed');S.deferredPrompt=null}
  $('install-banner').style.display='none';
};
$('install-dismiss').onclick=()=>{$('install-banner').style.display='none';localStorage.setItem('pusk_install_dismissed','1')};

// ── Eye toggle (all DOM ready) ──
initEye('eye-btn','a-pin');
initEye('org-eye-btn','org-pin');

// ── Init ──
setLang();
(async()=>{const h=await fetch('/api/health').then(r=>r.json()).catch(()=>({}));const ver=h.version||'';$('a-ver').textContent=ver;if($('land-ver'))$('land-ver').textContent=ver})();

const _p=new URLSearchParams(location.search);
S.invite=_p.get('invite');
if(S.invite&&!S.token){
  hideLanding();$('auth').style.display='flex';
  const _invOrg=_p.get('org');if(_invOrg)$('a-org').value=_invOrg;
  $('btn-login').style.display='none';$('btn-demo').style.display='none';
  $('btn-reg').className='abtn abtn-p';$('btn-reg').textContent=t('register_btn');
  $('a-err').style.color='var(--accent)';$('a-err').textContent=t('invite_hint');
} else if(_p.get('demo')==='1'&&!S.token){
  hideLanding();$('auth').style.display='flex';$('btn-demo').click();
} else if(S.token){
  hideLanding();api('GET','/api/bots').then(r=>{if(r&&!r.error)showApp();else logout()}).catch(logout);
} else {
  initLandingChat();
}

// ── Offline indicator ──
const offBar = $('offline-bar');
if (offBar) {
  window.addEventListener('online', () => offBar.classList.remove('show'));
  window.addEventListener('offline', () => offBar.classList.add('show'));
  if (!navigator.onLine) offBar.classList.add('show');
}

// ── SW update notification ──
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js').then(reg => {
    reg.addEventListener('updatefound', () => {
      const newSW = reg.installing;
      newSW.addEventListener('statechange', () => {
        if (newSW.state === 'activated' && navigator.serviceWorker.controller) {
          // New version available
          const bar = $('update-bar');
          if (bar) bar.classList.add('show');
        }
      });
    });
  });
}
