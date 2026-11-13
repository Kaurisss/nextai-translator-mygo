import { readFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const read = async name => JSON.parse(await readFile(path.join(root, 'src/common/i18n/locales', name, 'translation.json'), 'utf8'))
const aliases = JSON.parse(await readFile(new URL('./ui-labels.json', import.meta.url), 'utf8'))
const source = await read('zh-Hans')
const keys = Object.fromEntries(Object.entries(source).map(([key, value]) => [value, key]))
const result = {}
for (const language of ['en', 'zh-Hant', 'ja', 'ko', 'th', 'tr', 'pt-BR']) {
    const translated = await read(language)
    result[language] = {}
    for (const [label, key] of Object.entries({ ...keys, ...aliases })) {
        result[language][label] = translated[key] || key
    }
}
await writeFile(path.join(root, 'native/internal/app/ui-locales.json'), JSON.stringify(result, null, 2) + '\n')
console.log(`Generated ${Object.keys(result).length} UI catalogues from original translations and native labels.`)
