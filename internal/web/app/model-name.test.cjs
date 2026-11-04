const test = require('node:test');
const assert = require('node:assert/strict');
const name = require('./model-name.js');

const cases = {
  'deepseek-v4.1-flash': 'DeepSeek V4.1 Flash',
  'cloud/deepseek-v4.1-flash': 'DeepSeek V4.1 Flash',
  'zen/deepseek-v4.1-flash': 'DeepSeek V4.1 Flash',
  'claude-sonnet-5-5': 'Claude Sonnet 5.5',
  'claude-opus-5-5': 'Claude Opus 5.5',
  'claude-3-5-sonnet-20241022': 'Claude 3.5 Sonnet',
  'gpt-5.1-codex': 'GPT-5.1 Codex',
  'gpt-4o-mini': 'GPT-4o Mini',
  'llama-3.3-70b-instruct': 'Llama 3.3 70B Instruct',
  'llama3.2:3b': 'Llama 3.2 3B',
  'qwen3-coder-30b-a3b': 'Qwen3 Coder 30B A3B',
  'gemini-2.5-pro': 'Gemini 2.5 Pro',
  'mistral-large-latest': 'Mistral Large',
  'o3-mini': 'o3 Mini',
  'kimi-k2-thinking': 'Kimi K2 Thinking',
  'accounts/fireworks/models/glm-4.6': 'GLM 4.6',
};
for (const [id, want] of Object.entries(cases)) {
  test(id, () => assert.equal(name(id), want));
}
test('buit i nul no peten', () => { assert.equal(name(''), ''); assert.equal(name(null), ''); });
