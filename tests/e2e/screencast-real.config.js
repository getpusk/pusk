const { defineConfig } = require('@playwright/test');
module.exports = defineConfig({
  testDir: '.',
  testMatch: 'screencast-real.spec.js',
  timeout: 90000,
  use: {
    baseURL: 'http://localhost:18888',
    viewport: { width: 1280, height: 720 },
    video: 'on',
    actionTimeout: 10000,
  },
  reporter: [['list']],
});
