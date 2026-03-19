const { defineConfig } = require('@playwright/test');
module.exports = defineConfig({
  testDir: '.',
  timeout: 30000,
  retries: 0,
  use: {
    baseURL: process.env.PUSK_URL || 'https://getpusk.ru',
    viewport: { width: 1200, height: 720 },
    actionTimeout: 10000,
  },
  reporter: [['list']],
});
