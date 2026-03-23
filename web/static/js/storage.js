// localStorage key centralization — future use
// Import and use: import { storage } from './storage.js';
// storage.get('token') instead of localStorage.getItem('pusk_token')

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

export const storage = {
  get(key) { return localStorage.getItem(K[key] || key); },
  set(key, val) { localStorage.setItem(K[key] || key, val); },
  remove(key) { localStorage.removeItem(K[key] || key); },
  getJSON(key) { try { return JSON.parse(localStorage.getItem(K[key] || key) || 'null'); } catch { return null; } },
  setJSON(key, val) { localStorage.setItem(K[key] || key, JSON.stringify(val)); },
};
