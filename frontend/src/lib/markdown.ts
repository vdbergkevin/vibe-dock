import { marked, Renderer, type Tokens } from "marked";

function escapeHtml(value: string) {
  return value.replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "&#039;");
}

function externalUrl(value: string) {
  try {
    const url = new URL(value);
    return url.protocol === "https:" || url.protocol === "http:" ? url.toString() : "";
  } catch {
    return "";
  }
}

const renderer = new Renderer();

renderer.html = ({ text }: Tokens.HTML | Tokens.Tag) => escapeHtml(text);

renderer.code = ({ text, lang }: Tokens.Code) => {
  const language = lang?.trim().split(/\s+/)[0] || "Code";
  return `<pre><div class="code-label">${escapeHtml(language)}</div><code>${escapeHtml(text)}</code></pre>`;
};

renderer.link = function ({ href, title, tokens }: Tokens.Link) {
  const label = this.parser.parseInline(tokens);
  const safeHref = externalUrl(href);
  if (!safeHref) return label;
  const safeTitle = title ? ` title="${escapeHtml(title)}"` : "";
  return `<a href="${escapeHtml(safeHref)}" target="_blank" rel="noopener noreferrer"${safeTitle}>${label}</a>`;
};

renderer.image = ({ href, title, text }: Tokens.Image) => {
  const safeHref = externalUrl(href);
  const label = escapeHtml(text || "Open image");
  if (!safeHref) return label;
  const safeTitle = title ? ` title="${escapeHtml(title)}"` : "";
  return `<a class="markdown-image-link" href="${escapeHtml(safeHref)}" target="_blank" rel="noopener noreferrer"${safeTitle}>${label}</a>`;
};

export function markdown(value: string) {
  return marked.parse(value || "", {
    async: false,
    breaks: true,
    gfm: true,
    renderer
  }) as string;
}
