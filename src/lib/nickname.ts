/**
 * 随机昵称形容词池
 */
const ADJECTIVES = [
  '热爱', '坚持', '专注', '阳光', '温柔', '快乐', '宁静', '勇敢',
  '智慧', '勤奋', '好奇', '开朗', '认真', '从容', '独立', '真诚'
]

/**
 * 随机昵称名词池
 */
const NOUNS = [
  '读者', '书友', '学者', '行者', '探索者', '思考者', '记录者',
  '观察者', '追梦人', '读书郎', '阅己者', '知新者'
]

/**
 * 生成随机昵称
 * 格式：形容词 + 名词 + 4位随机数字，如「热爱读者3827」
 * @returns 随机生成的昵称字符串
 */
export function generateNickname(): string {
  const adj = ADJECTIVES[Math.floor(Math.random() * ADJECTIVES.length)]
  const noun = NOUNS[Math.floor(Math.random() * NOUNS.length)]
  const suffix = Math.floor(1000 + Math.random() * 9000)
  return `${adj}${noun}${suffix}`
}
