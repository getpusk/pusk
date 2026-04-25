const { defineConfig } = require('@playwright/test');
module.exports = defineConfig({
  testDir: '.',
  testIgnore: ['**/screencast*.spec.js'],
  timeout: 30000,
  retries: 0,
  use: {
    baseURL: process.env.PUSK_URL || process.env.BASE_URL || 'http://localhost:8443',
    viewport: { width: 1200, height: 720 },
    actionTimeout: 10000,
  },
  reporter: [['list']],
});
