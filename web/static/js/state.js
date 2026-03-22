// Shared mutable state — imported by all modules
const S = {
  token: localStorage.getItem('pusk_token'),
  curChat: null,
  curChan: null,
  ws: null,
  replyToId: 0,
  replyToText: '',
  channels: [],
  mentionUsers: [],
  lang: localStorage.getItem('pusk_lang') || 'ru',
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
};
export default S;
