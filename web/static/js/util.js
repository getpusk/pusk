import S from './state.js';

// ── DOM helpers ──
export function $(id){return document.getElementById(id)}
export function escJs(s){return(s||'').replace(/\\/g,'\\\\').replace(/'/g,"\\'").replace(/"/g,'\\"').replace(/</g,'\\x3c').replace(/>/g,'\\x3e').replace(/\n/g,'\\n')}
export function esc(s){if(!s)return'';const d=document.createElement('div');d.textContent=s;return d.innerHTML}
export function nameColor(n){let h=0;for(let i=0;i<n.length;i++)h=n.charCodeAt(i)+((h<<5)-h);return['#5865F2','#57F287','#FEE75C','#EB459E','#ED4245','#3BA55D','#FAA61A','#4082bc'][Math.abs(h)%8]}
export function fmtTime(d){if(!d)return'';const dt=new Date(d),now=new Date(),h=dt.getHours().toString().padStart(2,'0'),m=dt.getMinutes().toString().padStart(2,'0'),s=dt.getSeconds().toString().padStart(2,'0');const tm=h+':'+m+':'+s;if(dt.toDateString()===now.toDateString())return tm;const y=new Date(now);y.setDate(y.getDate()-1);if(dt.toDateString()===y.toDateString())return t('yest')+' '+tm;return dt.toLocaleDateString('ru',{day:'numeric',month:'short'})+' '+tm}
export function md(s){if(!s)return'';let h=s;const bl=[];h=h.replace(/```([\s\S]*?)```/g,(_,c)=>{bl.push('<pre class="md-pre">'+esc(c)+'</pre>');return'\x00'+bl.length+'\x00'});h=h.replace(/`([^`]+)`/g,(_,c)=>{bl.push('<code class="md-code">'+esc(c)+'</code>');return'\x00'+bl.length+'\x00'});h=esc(h);h=h.replace(/\*\*(.+?)\*\*/g,'<b>$1</b>');h=h.replace(/\*(.+?)\*/g,'<b>$1</b>');h=h.replace(/\[([^\]]+)\]\((https?:\/\/[^)]+)\)/g,'<a href="$2" target="_blank" rel="noopener" class="md-link">$1</a>');h=h.replace(/(^|[^"\/])(https?:\/\/[^\s<]+)/g,'$1<a href="$2" target="_blank" rel="noopener" class="md-link">$2</a>');h=h.replace(/(^|\s)(\/\w+)/gm,'$1<span class="md-cmd" onclick="window.$p(\'msg-in\').value=\'$2\';window.$p(\'msg-in\').focus()">$2</span>');h=h.replace(/@(\w+)/g,'<span class="md-mention">@$1</span>');h=h.replace(/\n/g,'<br>');h=h.replace(/\x00(\d+)\x00/g,(_,i)=>bl[i-1]);return h}
export function toast(msg){const el=$('toast');el.textContent=msg;el.style.display='block';setTimeout(()=>el.style.display='none',2000)}

// ── Loading skeleton ──
export function showLoading(el) {
  el.innerHTML = '<div class="loading-skeleton"><div class="skel-row"></div><div class="skel-row"></div><div class="skel-row"></div></div>';
}

// ── Eye toggle ──
const _eyeOn='<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>';
const _eyeOff='<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></svg>';
export function initEye(btnId,inputId){const b=document.getElementById(btnId);if(!b)return;b.innerHTML=_eyeOff;b.onclick=function(){const i=document.getElementById(inputId);const show=i.type==='password';i.type=show?'text':'password';this.innerHTML=show?_eyeOn:_eyeOff}}

// ── i18n ──
export const L={
  ru:{sub:'Бот-платформа',login:'Войти',reg:'Регистрация',demo:'Демо',user:'Логин',pin:'Пароль',
    err_empty:'Введите логин и пароль',err_wrong:'Неверный логин или пароль',err_taken:'занят',
    err_demo:'Демо недоступно',main:'Главная',ch:'Каналы',bots:'Боты',sub_on:'Подписан ✓',sub_off:'Подписаться',
    no_msg:'Пока нет сообщений',no_items:'Пока пусто',msg:'Сообщение...',lang:'Язык',out:'Выход',
    new_ch:'Новый канал',new_bot:'Новый бот',cancel:'Отмена',create:'Создать',name_req:'Введите имя',
    ch_lbl:'Канал',bot_lbl:'Бот',yest:'Вчера',invite:'Пригласить коллегу',invited:'Скопировано!',
    show_tok:'Показать token',tap_copy:'тап=копировать',copied:'Скопировано!',
    orgs_title:'Организации:',fill_all:'Заполните все поля',register_btn:'Зарегистрироваться',
    invite_hint:'Создайте логин и пароль для входа',push_on:'Вкл',push_off:'Выкл',push_blocked:'Заблокировано',push_reload:'Вкл (перезагрузите для откл.)',
    foot:(b,c,o)=>`Pusk · ${b} бот. · ${c} кан. · ${o} онлайн`},
  en:{sub:'Bot platform',login:'Login',reg:'Register',demo:'Demo',user:'Username',pin:'Password',
    err_empty:'Enter username and password',err_wrong:'Wrong username or password',err_taken:'taken',
    err_demo:'Demo unavailable',main:'Home',ch:'Channels',bots:'Bots',sub_on:'Joined ✓',sub_off:'Subscribe',
    no_msg:'No messages yet',no_items:'Nothing here yet',msg:'Message...',lang:'Language',out:'Logout',
    new_ch:'New Channel',new_bot:'New Bot',cancel:'Cancel',create:'Create',name_req:'Enter name',
    ch_lbl:'Channel',bot_lbl:'Bot',yest:'Yesterday',invite:'Invite colleague',invited:'Copied!',
    show_tok:'Show token',tap_copy:'tap=copy',copied:'Copied!',
    orgs_title:'Organizations:',fill_all:'Fill all fields',register_btn:'Register',
    invite_hint:'Create username and password',push_on:'ON',push_off:'OFF',push_blocked:'Blocked',push_reload:'ON (reload to disable)',
    foot:(b,c,o)=>`Pusk · ${b} bots · ${c} ch. · ${o} online`}
};
export function t(k){return L[S.lang][k]||L.en[k]||k}
export const ERR={
  'forbidden':'Доступ запрещён / Access denied',
  'unauthorized':'Не авторизован / Unauthorized',
  'not found':'Не найдено / Not found',
  'internal error':'Ошибка сервера / Server error',
  'org not found':'Организация не найдена / Org not found',
  'invalid credentials':'Неверный логин или пароль / Wrong credentials',
  'invalid credentials — specify org / укажите организацию':'Укажите ID организации / Specify org ID',
  'not subscribed':'Не подписаны на канал / Not subscribed',
  'text required':'Введите текст / Text required',
  'admin only':'Только для админа / Admin only',
  'can only edit own messages':'Можно редактировать только свои / Can only edit own',
  'Channel already exists':'Канал уже существует / Channel already exists',
  'invite already used':'Приглашение уже использовано / Invite already used',
  'invite expired':'Приглашение истекло / Invite expired',
  'invite not found':'Приглашение не найдено / Invite not found',
};
export function te(s){return ERR[s]||s}
export function setLang(){
  $('auth-sub').textContent=t('sub');$('a-user').placeholder=t('user');$('a-pin').placeholder=t('pin');
  $('btn-login').textContent=t('login');$('btn-reg').textContent=t('reg');$('btn-demo').textContent=t('demo');
  $('msg-in').placeholder=t('msg');$('s-lang-lbl').textContent=t('lang');$('s-lang-btn').textContent=S.lang==='ru'?'EN':'RU';
  $('s-out').textContent=t('out');
  const sel=$('m-type');if(sel){sel.options[0].text=t('ch_lbl');sel.options[1].text=t('bot_lbl')}
  $('m-cancel').textContent=t('cancel');$('m-ok').textContent=t('create');
  if($('s-invite'))$('s-invite').textContent=t('invite');
}


// ── Confirm dialog ──
export function confirmDialog(msg) {
  return new Promise(resolve => {
    $('confirm-title').textContent = msg;
    $('confirm-yes').textContent = S.lang === 'ru' ? 'Да' : 'Yes';
    $('confirm-no').textContent = S.lang === 'ru' ? 'Нет' : 'No';
    $('confirm-bg').classList.add('open');
    $('confirm-yes').onclick = () => { $('confirm-bg').classList.remove('open'); resolve(true); };
    $('confirm-no').onclick = () => { $('confirm-bg').classList.remove('open'); resolve(false); };
    $('confirm-bg').onclick = (e) => { if(e.target===$('confirm-bg')){ $('confirm-bg').classList.remove('open'); resolve(false); }};
  });
}

// ── API ──
export async function api(method,path,body){const o={method,headers:{'Content-Type':'application/json'}};if(S.token)o.headers.Authorization=S.token;if(body)o.body=JSON.stringify(body);try{const r=await fetch(path,o);const txt=await r.text();if(!r.ok)console.warn('[pusk]',method,path,r.status);try{return JSON.parse(txt)}catch{return{error:txt}}}catch(e){return{error:e.message}}}

// ── Window bindings for dynamic HTML onclick handlers ──
window.$p=$;
window.t=t;
