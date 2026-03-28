// js/storage.js — centralized localStorage access
const K = {
  token: 'pusk_token',
  uid: 'pusk_uid',
  uname: 'pusk_uname',
  role: 'pusk_role',
  org: 'pusk_org',
  orgs: 'pusk_orgs',
  lang: 'pusk_lang',
  view: 'pusk_view',
  installDismissed: 'pusk_install_dismissed',
};

export function get(key) { return localStorage.getItem(K[key] || key); }
export function set(key, val) { localStorage.setItem(K[key] || key, val); }
export function remove(key) { localStorage.removeItem(K[key] || key); }
export function getJSON(key) { try { return JSON.parse(localStorage.getItem(K[key] || key) || 'null'); } catch { return null; } }
export function setJSON(key, val) { localStorage.setItem(K[key] || key, JSON.stringify(val)); }


// Migrate old localStorage keys (pre-prefix) to pusk_* keys
(function migrateKeys() {
  const migrations = [
    ['token', 'pusk_token'],
    ['uid', 'pusk_uid'],
    ['uname', 'pusk_uname'],
    ['role', 'pusk_role'],
    ['org', 'pusk_org'],
    ['orgs', 'pusk_orgs'],
    ['lang', 'pusk_lang'],
    ['view', 'pusk_view'],
  ];
  for (const [old, nw] of migrations) {
    const v = localStorage.getItem(old);
    if (v && !localStorage.getItem(nw)) {
      localStorage.setItem(nw, v);
    }
  }
})();
