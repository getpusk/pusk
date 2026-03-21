const { defineConfig } = require('@playwright/test');
module.exports = defineConfig({
  testDir: '.',
  testMatch: 'screencast.spec.js',
  timeout: 60000,
  use: {
    baseURL: 'https://getpusk.ru',
    viewport: { width: 1280, height: 720 },
    video: 'on',
    actionTimeout: 10000,
  },
  reporter: [['list']],
});
