// ── PWA Install Prompt ──
let deferredPrompt=null;
window.addEventListener('beforeinstallprompt',e=>{
  e.preventDefault();deferredPrompt=e;
  if(!localStorage.getItem('pusk_install_dismissed')){
    $('install-banner').style.display='block';
  }
});
$('install-btn').onclick=async()=>{
  if(deferredPrompt){
    deferredPrompt.prompt();
    const r=await deferredPrompt.userChoice;
    if(r.outcome==='accepted')console.log('[pwa] installed');
    deferredPrompt=null;
  }
  $('install-banner').style.display='none';
};
$('install-dismiss').onclick=()=>{
  $('install-banner').style.display='none';
  localStorage.setItem('pusk_install_dismissed','1');
};

// ── Eye toggle (now after all DOM) ──
initEye('eye-btn','a-pin');
initEye('org-eye-btn','org-pin');

// ── Init ──
setLang();
(async()=>{
  const h=await fetch('/api/health').then(r=>r.json()).catch(()=>({}));
  const ver=h.version||'';
  $('a-ver').textContent=ver;
  if($('land-ver'))$('land-ver').textContent=ver;
})();

const _p=new URLSearchParams(location.search);
const _invite=_p.get('invite');
if(_invite&&!token){
  hideLanding();$('auth').style.display='flex';
  const _invOrg=_p.get('org');if(_invOrg)$('a-org').value=_invOrg;
  $('btn-login').style.display='none';$('btn-demo').style.display='none';
  $('btn-reg').className='abtn abtn-p';$('btn-reg').textContent=t('register_btn');
  $('a-err').style.color='var(--accent)';$('a-err').textContent=t('invite_hint');
} else if(_p.get('demo')==='1'&&!token){
  hideLanding();$('auth').style.display='flex';$('btn-demo').click();
} else if(token){
  hideLanding();api('GET','/api/bots').then(r=>{if(r&&!r.error)showApp();else logout()}).catch(logout);
} else {
  initLandingChat();
}
