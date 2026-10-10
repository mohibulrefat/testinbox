import { renderToStaticMarkup } from 'react-dom/server'
import { expect, test } from 'vitest'
import App from './App'

test('identifies TestInbox and the current foundation scope', () => {
 const html = renderToStaticMarkup(<App />)
 expect(html).toContain('<h1>TestInbox</h1>')
 expect(html).toContain('Message capture and storage are not available yet.')
 expect(html).toContain('<main>')
})
