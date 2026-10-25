export interface ModelInfo {
  name: string
  priceIn: string
  priceOut: string
  status: 'operational' | 'high-load'
}

export const models: ModelInfo[] = [
  { name: 'gpt-5.2', priceIn: '$1.25', priceOut: '$10', status: 'operational' },
  { name: 'claude-opus-4.5', priceIn: '$5', priceOut: '$25', status: 'operational' },
  { name: 'claude-sonnet-4.5', priceIn: '$3', priceOut: '$15', status: 'operational' },
  { name: 'gemini-3-pro', priceIn: '$1.25', priceOut: '$10', status: 'operational' },
  { name: 'deepseek-v3.2', priceIn: '$0.27', priceOut: '$1.1', status: 'high-load' },
  { name: 'qwen3-max', priceIn: '$0.86', priceOut: '$3.44', status: 'operational' },
  { name: 'kimi-k2.5', priceIn: '$0.6', priceOut: '$2.5', status: 'operational' },
  { name: 'grok-4', priceIn: '$3', priceOut: '$15', status: 'operational' },
  { name: 'glm-5', priceIn: '$0.6', priceOut: '$2.2', status: 'operational' },
  { name: 'doubao-pro-1.6', priceIn: '$0.11', priceOut: '$0.55', status: 'operational' },
]
