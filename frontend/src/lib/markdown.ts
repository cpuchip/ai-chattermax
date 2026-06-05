import MarkdownIt from 'markdown-it'

// Chat message renderer. html:false escapes any raw HTML in messages (XSS-safe
// with v-html); linkify auto-links bare URLs; breaks turns newlines into <br>.
const md = new MarkdownIt({ html: false, linkify: true, breaks: true })

// Link rendering: a churchofjesuschrist.org link gets a marker class so the chat
// intercepts the click and opens it in the in-app ScripturePanel (iframe popup);
// every other link opens in a new tab.
const renderToken = (tokens: any, idx: number, options: any, _env: any, self: any) =>
  self.renderToken(tokens, idx, options)
const defaultLinkOpen = md.renderer.rules.link_open || renderToken

md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  const href = tokens[idx].attrGet('href') || ''
  if (/(^|\/\/)([\w.-]*\.)?churchofjesuschrist\.org\//i.test(href)) {
    tokens[idx].attrSet('class', 'cjc-link')
  } else {
    tokens[idx].attrSet('target', '_blank')
    tokens[idx].attrSet('rel', 'noopener noreferrer')
  }
  return defaultLinkOpen(tokens, idx, options, env, self)
}

/** Render a chat message body (markdown) to sanitized HTML. */
export function renderMarkdown(src: string): string {
  return md.render(src || '')
}
