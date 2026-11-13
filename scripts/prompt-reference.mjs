// Runs the repository's TypeScript prompt builder with its real template tags.
// No model or network calls are made. Requires common-tags and iso-639-1 from
// the repository's dependencies, or --dependencies <directory> for a scratch install.
import { readFile, writeFile, mkdir } from 'node:fs/promises'
import { createRequire, stripTypeScriptTypes } from 'node:module'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import vm from 'node:vm'

const root = fileURLToPath(new URL('../../', import.meta.url))
const dependencyIndex = process.argv.indexOf('--dependencies')
const dependencies = dependencyIndex < 0 ? root : path.resolve(process.argv[dependencyIndex + 1])
const require = createRequire(path.join(dependencies, 'package.json'))
const tags = require('common-tags')
const ISO6391 = require('iso-639-1')
let captured
let wordMode
const sandbox = vm.createContext({
    ...tags, ISO6391, Intl, console: { debug() {} },
    uuidv4: () => 'reference',
    ensurePerfTrace() {}, markPerf() {}, setPerfMeta() {},
    getSettings: async () => ({ provider: 'OpenAI' }),
    getEngine: () => ({ sendMessage: async (request) => {
        captured = { rolePrompt: request.rolePrompt, commandPrompt: request.commandPrompt }
        await request.onMessage({ content: '', role: '' })
    } }),
})
function evaluate(source) {
    const code = stripTypeScriptTypes(source.replace(/^import[\s\S]*?from ['"][^'"]+['"]\r?\n/gm, ''), { mode: 'strip' })
    vm.runInContext(code.replace(/\bexport /g, ''), sandbox)
}
const languageSource = await readFile(path.join(root, 'src/common/lang/data.ts'), 'utf8')
const languageIndex = await readFile(path.join(root, 'src/common/lang/index.ts'), 'utf8')
const translateSource = await readFile(path.join(root, 'src/common/translate.ts'), 'utf8')
evaluate(languageSource + '\nglobalThis.languages = LANG_CONFIGS')
evaluate(`const langMap = new Map(Object.entries(LANG_CONFIGS).map(([code, config]) => [code, config.name]));\n` +
    languageIndex.match(/export function getLangName[\s\S]*?\n}/)[0] + '\n' +
    languageIndex.match(/export function getLangConfig[\s\S]*?\n}/)[0])
evaluate(await readFile(path.join(root, 'src/common/traditional-or-simplified.ts'), 'utf8'))
evaluate(languageIndex.match(/export async function localDetectLang[\s\S]*?\n}/)[0] + '\nglobalThis.detectSource = localDetectLang')
evaluate(translateSource + '\nglobalThis.buildReference = translate; globalThis.segmentReference = isAWord')

const metadata = Object.entries(sandbox.languages).map(([code, config]) => ({
    code, name: config.name, nameEn: config.nameEn,
    promptName: ISO6391.getName(code) || config.name,
    phoneticNotation: config.phoneticNotation === false ? '' : config.phoneticNotation || 'transcription',
    rolePrompt: config.rolePrompt || 'You are a professional translator.',
}))
await writeFile(path.join(root, 'native/internal/core/languages.json'), JSON.stringify(metadata, null, 2) + '\n')
const variants = vm.runInContext('({simplified:S.join(""), traditional:T.join("")})', sandbox)
variants.vietnamese = JSON.parse('"' + languageIndex.match(/const VietnameseCharsRegEx\s*=\s*\/\[([^\]]+)\]\//)[1] + '"')
await writeFile(path.join(root, 'native/internal/core/chinese-variants.json'), JSON.stringify(variants) + '\n')

const inputs = [
    { name: 'Chinese word', text: '你好', source: 'zh-Hans', target: 'en' },
    { name: 'Chinese punctuation', text: '你好。', source: 'zh-Hans', target: 'en' },
    { name: 'Chinese ASCII punctuation', text: '你好.', source: 'zh-Hans', target: 'en' },
    { name: 'Chinese sentence', text: '你好，这是一个完整的句子。', source: 'zh-Hans', target: 'en' },
    { name: 'Chinese phrase', text: '中华人民共和国', source: 'zh-Hans', target: 'en' },
    { name: 'English word to Chinese', text: 'hello', source: 'en', target: 'zh-Hans' },
    { name: 'English word to traditional Chinese', text: 'hello', source: 'en', target: 'zh-Hant' },
    { name: 'English word same language', text: 'hello', source: 'en', target: 'en' },
    { name: 'Short target Chinese', text: 'hi!', source: 'en', target: 'zh-Hans' },
    { name: 'Word trims whitespace', text: '  hello  ', source: 'en', target: 'zh-Hans' },
    { name: 'Sentence preserves leading whitespace', text: '  Hello, world!  \n', source: 'en', target: 'fr' },
    { name: 'Hyphenated word', text: 'well-known', source: 'en', target: 'zh-Hans' },
    { name: 'Contraction', text: "don't", source: 'en', target: 'zh-Hans' },
    { name: 'Punctuation segment', text: '.', source: 'en', target: 'zh-Hans' },
    { name: 'Number segment', text: '123', source: 'en', target: 'en' },
    { name: 'Japanese word', text: 'こんにちは', source: 'ja', target: 'en' },
    { name: 'Vietnamese no phonetic notation', text: 'hello', source: 'en', target: 'vi' },
    { name: 'Korean banmal', text: 'Hello, how are you?', source: 'en', target: 'ko-banmal' },
    { name: 'Writing bypasses word mode', text: 'hello', source: 'en', target: 'zh-Hans', writing: true },
    { name: 'Selected word context', text: 'The bank is closed.', source: 'en', target: 'zh-Hans', selectedWord: 'bank' },
    { name: 'Polishing', text: 'This are a test.', source: 'en', target: 'zh-Hans', mode: 'polishing' },
    { name: 'Summarize', text: 'One paragraph. Another paragraph.', source: 'en', target: 'zh-Hans', mode: 'summarize' },
    { name: 'Analyze', text: 'This is a test.', source: 'en', target: 'zh-Hans', mode: 'analyze' },
    { name: 'Code', text: 'const x = 1\n', source: 'en', target: 'zh-Hans', mode: 'explain-code' },
    { name: 'Story', text: 'apple, river, cloud', mode: 'big-bang', articlePrompt: 'a science fiction story' },
    { name: 'Custom', text: 'Hello', source: 'en', target: 'zh-Hans', mode: '', rolePrompt: 'Translate from ${sourceLang} to ${targetLang}', commandPrompt: 'Be concise.' },
    { name: 'Custom embeds text', text: 'Hello', source: 'en', target: 'zh-Hans', mode: '', rolePrompt: 'Explain ${text}', commandPrompt: 'Use ${targetLang}', format: 'markdown' },
    { name: 'Custom text in command', text: 'Hello', source: 'en', target: 'zh-Hans', mode: '', rolePrompt: '', commandPrompt: 'Explain ${text}', format: 'text' },
    { name: 'Custom repeated substitution', text: 'Hello', source: 'en', target: 'zh-Hans', mode: '', rolePrompt: '${targetLang} ${targetLang} ${text} ${text}', commandPrompt: '' },
    { name: 'Auto Chinese', text: '你好', source: 'auto', target: 'en' },
    { name: 'Auto English', text: 'Hello, world!', source: 'auto', target: 'zh-Hans' },
    { name: 'Auto traditional Chinese', text: '這是一個測試。', source: 'auto', target: 'en' },
    { name: 'Auto Japanese', text: '日本語のテストです。', source: 'auto', target: 'zh-Hans' },
    { name: 'Auto Vietnamese', text: 'Đây là một bài kiểm tra tiếng Việt.', source: 'auto', target: 'en' },
    { name: 'Auto Korean', text: '안녕하세요', source: 'auto', target: 'en' },
    { name: 'Auto Russian', text: 'Привет, мир!', source: 'auto', target: 'en' },
    { name: 'Auto Spanish', text: 'El niño está aquí.', source: 'auto', target: 'en' },
    { name: 'Auto French', text: 'Où est la bibliothèque?', source: 'auto', target: 'en' },
    { name: 'Auto German', text: 'Schöne Grüße!', source: 'auto', target: 'en' },
    { name: 'Auto Thai', text: 'สวัสดีครับ', source: 'auto', target: 'en' },
]
for (const language of metadata) {
    inputs.push({ name: `Language ${language.code}`, text: 'A full sentence for translation.', source: 'en', target: language.code })
}
const fixtures = []
for (const input of inputs) {
    const source = input.source === 'auto' ? await sandbox.detectSource(input.text) : input.source
    const mode = input.mode ?? 'translate'
    captured = undefined
    wordMode = false
    await sandbox.buildReference({
        mode, text: input.text, detectFrom: source, detectTo: input.target,
        writing: input.writing, selectedWord: input.selectedWord, articlePrompt: input.articlePrompt,
        action: { mode: mode || undefined, rolePrompt: input.rolePrompt, commandPrompt: input.commandPrompt, outputRenderingFormat: input.format },
        signal: new AbortController().signal,
        onMessage: async (message) => { wordMode = message.isWordMode }, onError() {}, onFinish() {},
    })
    const combined = captured.rolePrompt ? captured.rolePrompt + '\n\n' + captured.commandPrompt : captured.commandPrompt
    fixtures.push({ name: input.name, query: {
        Text: input.text, Source: input.source === 'auto' ? '自动检测' : input.source, Target: input.target || '',
        Writing: input.writing || false, SelectedWord: input.selectedWord || '', ArticlePrompt: input.articlePrompt || '',
        Action: { mode, prompt: input.rolePrompt || '', commandPrompt: input.commandPrompt || '', format: input.format || '' },
    }, expected: { ...captured, wordMode }, messages: [{ role: 'user', content: combined }] })
}
await mkdir(path.join(root, 'native/internal/core/testdata'), { recursive: true })
await writeFile(path.join(root, 'native/internal/core/testdata/prompts.json'), JSON.stringify(fixtures, null, 2) + '\n')
const segments = []
for (const [locale, texts] of Object.entries({
    'zh-Hans': ['你好', '你好。', '你好.', '中华人民共和国', '这是一个句子', '世界', '你好 世界'],
    'en': ['hello', 'hello.', "don't", 'well-known', 'Hello world', '.', '123', '😀', ''],
    'ja': ['こんにちは', '日本語', '日本語のテスト'], 'th': ['สวัสดี', 'สวัสดีครับ'],
})) for (const text of texts) segments.push({ locale, text, expected: sandbox.segmentReference(locale, text) })
await writeFile(path.join(root, 'native/internal/core/testdata/word-segments.json'), JSON.stringify(segments, null, 2) + '\n')
const openAISource = await readFile(path.join(root, 'src/common/engines/abstract-openai.ts'), 'utf8')
const bodyMethod = openAISource.match(/async getBaseRequestBody\([\s\S]*?\n    }/)[0]
evaluate(`class ReferenceOpenAI { ${bodyMethod} }\nglobalThis.referenceParameters = model => new ReferenceOpenAI().getBaseRequestBody(model)`)
const modelParameters = []
for (const model of ['gpt-3.5-turbo', 'gpt-4o-mini', 'GPT-4.1', 'o1', 'o3-mini', 'gpt-5', 'gpt-5-mini', 'gpt-5.0', 'gpt-5-pro', 'gpt-5.0-mini-pro', 'gpt-5-chat-latest', 'gpt-5-codex', 'gpt-5.1', 'gpt-5.5', 'antigravity/gemini-3.5-flash-lite', 'deepseek-chat']) {
    modelParameters.push({ model, expected: await sandbox.referenceParameters(model) })
}
await writeFile(path.join(root, 'native/internal/core/testdata/model-parameters.json'), JSON.stringify(modelParameters, null, 2) + '\n')
const geminiSource = await readFile(path.join(root, 'src/common/engines/gemini.ts'), 'utf8')
evaluate(geminiSource.match(/export function resolveGeminiThinkingConfig\([\s\S]*?\n}/)[0] + '\nglobalThis.referenceGeminiThinking = resolveGeminiThinkingConfig')
const geminiThinking = []
for (const model of ['gemini-2.5-flash', 'gemini-2.5-pro', 'gemini-3-flash', 'gemini-3.6-flash', 'gemini-3.8-flash', 'gemini-4-argon', 'gemini-robotics-er-2', 'gemma-4', 'gemini-flash-latest', 'gateway-alias']) for (const enabled of [false, true]) {
    geminiThinking.push({ model, enabled, expected: sandbox.referenceGeminiThinking(model, enabled) ?? null })
}
await writeFile(path.join(root, 'native/internal/core/testdata/gemini-thinking.json'), JSON.stringify(geminiThinking, null, 2) + '\n')
console.log(`Generated ${fixtures.length} prompt fixtures, ${segments.length} segmentation fixtures, ${metadata.length} languages, ${modelParameters.length} model-parameter fixtures and ${geminiThinking.length} thinking fixtures from the original source.`)
