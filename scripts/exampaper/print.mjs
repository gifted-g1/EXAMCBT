// Renders docs/exam-paper/computing-comprehensive-exam.html to PDF with the
// Chromium that the web E2E tests already use.
//
//   go run ./scripts/exampaper && node scripts/exampaper/print.mjs
import { createRequire } from 'node:module'
import { fileURLToPath, pathToFileURL } from 'node:url'
import path from 'node:path'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const { chromium } = createRequire(path.join(root, 'web/package.json'))('@playwright/test')
const src = path.join(root, 'docs/exam-paper/computing-comprehensive-exam.html')
const out = path.join(root, 'docs/exam-paper/computing-comprehensive-exam.pdf')

const browser = await chromium.launch()
const page = await browser.newPage()
await page.goto(pathToFileURL(src).href, { waitUntil: 'load' })
await page.pdf({
  path: out,
  format: 'A4',
  printBackground: true,
  preferCSSPageSize: true,
  displayHeaderFooter: true,
  headerTemplate: '<span></span>',
  footerTemplate: `<div style="width:100%;font-family:Georgia,serif;font-size:8pt;padding:0 18mm;display:flex;justify-content:space-between;color:#000">
    <span>Faculty of Computing Comprehensive Examination</span>
    <span>Page <span class="pageNumber"></span> of <span class="totalPages"></span></span></div>`,
})
await browser.close()
console.log('wrote', path.relative(root, out))
