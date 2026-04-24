// Shared mutable state — imported by all modules
import {get} from './storage.js';
const S = {
  token: get('token'),
  curChat: null,
  curBotId: null,
  curChan: null,
  ws: null,
  replyToId: 0,
  replyToText: '',
  channels: [],
  mentionUsers: [],
  lang: get('lang') || (navigator.language && navigator.language.startsWith('ru') ? 'ru' : 'en'),
  lastMsgDate: '',
  editMsgId: 0,
  editChanId: 0,
  typingTimer: null,
  deferredPrompt: null,
  invite: null,
  inviteUrl: '',
  landToken: null,
  landChat: null,
  audioCtx: null,
  loading: false,
  isDemo: false,
  wsReconnectTimer: null,
  elapsedTimer: null,
};
export default S;
