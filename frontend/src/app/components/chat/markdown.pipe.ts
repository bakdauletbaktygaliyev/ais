import { Pipe, PipeTransform } from '@angular/core';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';

@Pipe({ name: 'markdown', standalone: true })
export class MarkdownPipe implements PipeTransform {
  constructor(private sanitizer: DomSanitizer) {}

  transform(raw: string): SafeHtml {
    if (!raw) return this.sanitizer.bypassSecurityTrustHtml('');
    return this.sanitizer.bypassSecurityTrustHtml(this.parse(raw));
  }

  private escape(text: string): string {
    return text
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }

  private inline(text: string): string {
    return text
      .replace(/`([^`]+)`/g, '<code class="md-code">$1</code>')
      .replace(/\*\*([^*\n]+)\*\*/g, '<strong>$1</strong>')
      .replace(/\*([^*\n]+)\*/g, '<em>$1</em>');
  }

  private parse(raw: string): string {
    const blocks: string[] = [];
    const MARK = '\x02BLOCK\x02';

    // Extract code blocks first
    let text = raw.replace(/```(\w*)\n?([\s\S]*?)```/g, (_, lang, code) => {
      const langLabel = lang ? `<span class="md-lang">${lang}</span>` : '';
      const escaped = this.escape(code.replace(/\n$/, ''));
      blocks.push(`<div class="md-pre-wrap">${langLabel}<pre class="md-pre"><code>${escaped}</code></pre></div>`);
      return `${MARK}${blocks.length - 1}\x03`;
    });

    const lines = text.split('\n');
    const out: string[] = [];
    let listType: 'ul' | 'ol' | null = null;

    const closeList = () => {
      if (listType) { out.push(`</${listType}>`); listType = null; }
    };

    for (const line of lines) {
      const blockMatch = line.match(/^\x02BLOCK\x02(\d+)\x03$/);
      if (blockMatch) {
        closeList();
        out.push(blocks[+blockMatch[1]]);
        continue;
      }

      const h3 = line.match(/^### (.+)/);
      const h2 = line.match(/^## (.+)/);
      const h1 = line.match(/^# (.+)/);
      if (h3) { closeList(); out.push(`<h5 class="md-h">${this.inline(this.escape(h3[1]))}</h5>`); continue; }
      if (h2) { closeList(); out.push(`<h4 class="md-h">${this.inline(this.escape(h2[1]))}</h4>`); continue; }
      if (h1) { closeList(); out.push(`<h3 class="md-h">${this.inline(this.escape(h1[1]))}</h3>`); continue; }

      const ulMatch = line.match(/^[-*] (.+)/);
      if (ulMatch) {
        if (listType !== 'ul') { closeList(); out.push('<ul class="md-ul">'); listType = 'ul'; }
        out.push(`<li>${this.inline(this.escape(ulMatch[1]))}</li>`);
        continue;
      }

      const olMatch = line.match(/^\d+\. (.+)/);
      if (olMatch) {
        if (listType !== 'ol') { closeList(); out.push('<ol class="md-ol">'); listType = 'ol'; }
        out.push(`<li>${this.inline(this.escape(olMatch[1]))}</li>`);
        continue;
      }

      if (!line.trim()) { closeList(); continue; }

      closeList();
      out.push(`<p class="md-p">${this.inline(this.escape(line))}</p>`);
    }

    closeList();
    return out.join('');
  }
}
