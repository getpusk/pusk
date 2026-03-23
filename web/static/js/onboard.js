import S from './state.js';
import {$,t,api,toast} from './util.js';
import {get,set} from './storage.js';

let step = 0;
let botToken = '';

export async function startOnboarding() {
  // Only show for admin, only once per org
  if (get('role') !== 'admin') return;
  const orgSlug = get('org') || 'default';
  if (get('onboarded_' + orgSlug)) return;

  // Check if org already has alerts channel
  const chs = await api('GET', '/api/channels');
  if (chs && chs.some(c => c.name === 'alerts')) {
    set('onboarded_' + orgSlug, '1');
    return;
  }

  // Get bot token for webhook URL
  const bots = await api('GET', '/api/bots');
  if (bots && bots.length) botToken = bots[0].token;

  step = 0;
  showStep();
  $('onboard-bg').classList.add('open');
}

function showStep() {
  const body = $('onboard-body');
  const next = $('onboard-next');
  const host = location.origin;

  if (step === 0) {
    $('onboard-title').textContent = S.lang === 'ru' ? '\u0428\u0430\u0433 1: \u041a\u0430\u043d\u0430\u043b \u0430\u043b\u0435\u0440\u0442\u043e\u0432' : 'Step 1: Alerts channel';
    body.innerHTML = '<p style="color:var(--text2);margin-bottom:12px">' +
      (S.lang === 'ru' ? '\u0423 \u0432\u0430\u0441 \u0435\u0441\u0442\u044c #general. \u0421\u043e\u0437\u0434\u0430\u0439\u0442\u0435 #alerts \u0434\u043b\u044f \u043c\u043e\u043d\u0438\u0442\u043e\u0440\u0438\u043d\u0433\u0430:' : 'Your org has #general. Create #alerts for monitoring:') + '</p>';
    next.textContent = S.lang === 'ru' ? '\u0421\u043e\u0437\u0434\u0430\u0442\u044c #alerts' : 'Create #alerts';
    next.onclick = async () => {
      next.textContent = '...';
      const r = await api('POST', '/admin/channel', {name: 'alerts', description: 'Monitoring alerts'});
      if (r && r.ok) {
        toast(S.lang === 'ru' ? '#alerts \u0441\u043e\u0437\u0434\u0430\u043d!' : '#alerts created!');
        step = 1;
        showStep();
      } else {
        toast(r?.error || 'Error');
        next.textContent = S.lang === 'ru' ? '\u0421\u043e\u0437\u0434\u0430\u0442\u044c #alerts' : 'Create #alerts';
      }
    };
  } else if (step === 1) {
    const webhookUrl = host + '/hook/' + botToken + '?format=alertmanager';
    $('onboard-title').textContent = S.lang === 'ru' ? '\u0428\u0430\u0433 2: Webhook URL' : 'Step 2: Webhook URL';
    body.innerHTML = '<p style="color:var(--text2);margin-bottom:8px">' +
      (S.lang === 'ru' ? '\u0423\u043a\u0430\u0436\u0438\u0442\u0435 \u044d\u0442\u043e\u0442 URL \u0432 Alertmanager/Grafana/Zabbix:' : 'Point your monitoring at this URL:') +
      '</p><div class="land-code" style="font-size:12px;margin-bottom:0;cursor:pointer;word-break:break-all" id="onboard-url">' + webhookUrl + '</div>';
    next.textContent = S.lang === 'ru' ? '\u0421\u043a\u043e\u043f\u0438\u0440\u043e\u0432\u0430\u0442\u044c' : 'Copy URL';
    next.onclick = () => {
      navigator.clipboard.writeText(webhookUrl);
      toast(S.lang === 'ru' ? '\u0421\u043a\u043e\u043f\u0438\u0440\u043e\u0432\u0430\u043d\u043e!' : 'Copied!');
      step = 2;
      showStep();
    };
  } else if (step === 2) {
    const curlCmd = "curl -X POST '" + location.origin + '/hook/' + botToken + "?format=alertmanager' \\\n" +
      "  -H 'Content-Type: application/json' \\\n" +
      '  -d \'{"status":"firing","alerts":[{"status":"firing","labels":{"alertname":"TestAlert","severity":"warning"},"annotations":{"summary":"Onboarding test alert"}}]}\'';
    $('onboard-title').textContent = S.lang === 'ru' ? '\u0428\u0430\u0433 3: \u0422\u0435\u0441\u0442' : 'Step 3: Test';
    body.innerHTML = '<p style="color:var(--text2);margin-bottom:8px">' +
      (S.lang === 'ru' ? '\u041e\u0442\u043f\u0440\u0430\u0432\u044c\u0442\u0435 \u0442\u0435\u0441\u0442\u043e\u0432\u044b\u0439 \u0430\u043b\u0435\u0440\u0442:' : 'Send a test alert:') +
      '</p><div class="land-code" style="font-size:11px;margin-bottom:0;white-space:pre-wrap">' + curlCmd + '</div>';
    next.textContent = S.lang === 'ru' ? '\u041e\u0442\u043f\u0440\u0430\u0432\u0438\u0442\u044c \u0442\u0435\u0441\u0442' : 'Send test';
    next.onclick = async () => {
      next.textContent = '...';
      try {
        const r = await fetch(location.origin + '/hook/' + botToken + '?format=alertmanager', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({status: 'firing', alerts: [{status: 'firing', labels: {alertname: 'TestAlert', severity: 'warning'}, annotations: {summary: 'Onboarding test'}}]})
        });
        if (r.ok) {
          toast(S.lang === 'ru' ? '\u0410\u043b\u0435\u0440\u0442 \u043e\u0442\u043f\u0440\u0430\u0432\u043b\u0435\u043d! \u041f\u0440\u043e\u0432\u0435\u0440\u044c\u0442\u0435 #alerts' : 'Alert sent! Check #alerts');
          step = 3;
          showStep();
        } else {
          next.textContent = S.lang === 'ru' ? '\u041e\u0442\u043f\u0440\u0430\u0432\u0438\u0442\u044c \u0442\u0435\u0441\u0442' : 'Send test';
          toast('Error: ' + r.status);
        }
      } catch (e) {
        next.textContent = S.lang === 'ru' ? '\u041e\u0442\u043f\u0440\u0430\u0432\u0438\u0442\u044c \u0442\u0435\u0441\u0442' : 'Send test';
        toast(e.message);
      }
    };
  } else {
    $('onboard-title').textContent = S.lang === 'ru' ? '\u0413\u043e\u0442\u043e\u0432\u043e!' : 'Done!';
    body.innerHTML = '<p style="color:var(--text2)">' +
      (S.lang === 'ru' ? '\u041a\u0430\u043d\u0430\u043b #alerts \u0441\u043e\u0437\u0434\u0430\u043d, webhook \u043d\u0430\u0441\u0442\u0440\u043e\u0435\u043d. \u0422\u0435\u043f\u0435\u0440\u044c \u043f\u043e\u0434\u043a\u043b\u044e\u0447\u0438\u0442\u0435 \u0432\u0430\u0448 Alertmanager/Grafana/Zabbix.' : 'Channel #alerts created, webhook configured. Now connect your monitoring stack.') + '</p>';
    next.textContent = S.lang === 'ru' ? '\u041d\u0430\u0447\u0430\u0442\u044c' : 'Start';
    const orgSlug = get('org') || 'default';
    set('onboarded_' + orgSlug, '1');
    next.onclick = () => {
      $('onboard-bg').classList.remove('open');
      // Refresh the main list to show the new channel
      import('./views.js').then(v => v.showList());
    };
  }
}

// Skip button
document.addEventListener('DOMContentLoaded', () => {
  const skipBtn = $('onboard-skip');
  if (skipBtn) {
    skipBtn.onclick = () => {
      const orgSlug = get('org') || 'default';
      set('onboarded_' + orgSlug, '1');
      $('onboard-bg').classList.remove('open');
    };
  }
});
