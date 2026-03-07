import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { ApiService } from '../../core/services/api.service';

@Component({
    selector: 'app-code-viewer',
    standalone: true,
    imports: [CommonModule],
    template: `
    <div class="code-viewer-page">
      <header class="viewer-header">
        <button class="back-btn" (click)="goBack()">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M19 12H5M12 5l-7 7 7 7"/>
          </svg>
          Back
        </button>
        <div class="file-info">
          <span class="file-lang-badge">{{ detectedLang }}</span>
          <span class="file-path">{{ filePath }}</span>
        </div>
        <div class="viewer-actions">
          <button class="toolbar-btn" (click)="copySource()" title="Copy source">
            {{ copied ? '✓ Copied' : 'Copy' }}
          </button>
          <button class="toolbar-btn" (click)="openDetail()">Detail →</button>
        </div>
      </header>

      <div class="viewer-loading" *ngIf="loading">
        <div class="spinner"></div>
        <p>Loading source...</p>
      </div>

      <div class="viewer-error" *ngIf="error">
        <p>{{ error }}</p>
        <button (click)="goBack()">Go Back</button>
      </div>

      <!-- Syntax-highlighted code display -->
      <div class="code-container" *ngIf="!loading && !error && source">
        <div class="line-numbers" aria-hidden="true">
          <span *ngFor="let line of lineNumbers">{{ line }}</span>
        </div>
        <pre class="code-pre"><code [innerHTML]="highlightedSource"></code></pre>
      </div>
    </div>
  `,
    styles: [`
    .code-viewer-page { display:flex; flex-direction:column; height:100vh; background:#0f0f13; color:#e2e8f0; }
    .viewer-header { display:flex; align-items:center; gap:1rem; padding:0.75rem 1.5rem; border-bottom:1px solid #1e293b; background:#1e1e2e; flex-shrink:0; }
    .back-btn { display:flex; align-items:center; gap:0.4rem; background:none; border:1px solid #334155; color:#94a3b8; border-radius:8px; padding:0.45rem 0.75rem; cursor:pointer; font-size:0.8rem; transition:all 0.15s; }
    .back-btn svg { width:14px; height:14px; }
    .back-btn:hover { border-color:#6366f1; color:#a78bfa; }
    .file-info { display:flex; align-items:center; gap:0.75rem; flex:1; overflow:hidden; }
    .file-lang-badge { background:#334155; color:#94a3b8; font-size:0.7rem; font-weight:700; text-transform:uppercase; padding:0.2rem 0.5rem; border-radius:4px; flex-shrink:0; }
    .file-path { font-family:monospace; font-size:0.85rem; color:#a78bfa; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
    .viewer-actions { display:flex; gap:0.5rem; flex-shrink:0; }
    .toolbar-btn { background:#1e293b; border:1px solid #334155; color:#94a3b8; border-radius:6px; padding:0.35rem 0.75rem; font-size:0.8rem; cursor:pointer; transition:all 0.15s; }
    .toolbar-btn:hover { border-color:#6366f1; color:#a78bfa; }
    .viewer-loading { display:flex; flex-direction:column; align-items:center; justify-content:center; flex:1; gap:1rem; color:#64748b; }
    .spinner { width:36px; height:36px; border:3px solid rgba(99,102,241,0.3); border-top-color:#6366f1; border-radius:50%; animation:spin 0.7s linear infinite; }
    @keyframes spin { to{transform:rotate(360deg)} }
    .viewer-error { display:flex; flex-direction:column; align-items:center; justify-content:center; flex:1; gap:1rem; color:#f87171; }
    .viewer-error button { background:#1e293b; border:1px solid #334155; color:#e2e8f0; border-radius:8px; padding:0.5rem 1.25rem; cursor:pointer; }
    .code-container { display:flex; flex:1; overflow:auto; font-family:'JetBrains Mono','Fira Mono',monospace; font-size:0.85rem; line-height:1.7; }
    .line-numbers { padding:1rem 0.75rem 1rem 1.25rem; background:#161620; color:#334155; text-align:right; user-select:none; flex-shrink:0; border-right:1px solid #1e293b; display:flex; flex-direction:column; }
    .line-numbers span { display:block; }
    .code-pre { margin:0; padding:1rem 1.5rem; overflow:visible; flex:1; white-space:pre; }
    code { color:#e2e8f0; }
    /* Basic syntax token colours */
    :host ::ng-deep .kw { color:#c792ea; }
    :host ::ng-deep .str { color:#c3e88d; }
    :host ::ng-deep .num { color:#f78c6c; }
    :host ::ng-deep .cmt { color:#546e7a; font-style:italic; }
    :host ::ng-deep .fn { color:#82aaff; }
    :host ::ng-deep .tp { color:#ffcb6b; }
  `]
})
export class CodeViewerComponent implements OnInit {
    private route = inject(ActivatedRoute);
    private router = inject(Router);
    private api = inject(ApiService);

    repoId = '';
    fileId = '';
    filePath = '';
    source = '';
    highlightedSource = '';
    detectedLang = '';
    loading = false;
    error = '';
    copied = false;
    lineNumbers: number[] = [];

    ngOnInit(): void {
        this.repoId = this.route.snapshot.paramMap.get('repoId')!;
        this.fileId = this.route.snapshot.paramMap.get('fileId')!;
        this.loadSource();
    }

    loadSource(): void {
        this.loading = true;
        this.api.getFileSource(this.repoId, this.fileId).subscribe({
            next: (result) => {
                this.source = result.source;
                this.filePath = result.fileId;
                this.detectedLang = this.detectLang(this.filePath);
                this.lineNumbers = Array.from({ length: this.source.split('\n').length }, (_, i) => i + 1);
                this.highlightedSource = this.basicHighlight(this.source, this.detectedLang);
                this.loading = false;
            },
            error: (err) => {
                this.error = err.message || 'Failed to load source code.';
                this.loading = false;
            },
        });
    }

    detectLang(path: string): string {
        if (path.endsWith('.ts') || path.endsWith('.tsx')) return 'typescript';
        if (path.endsWith('.js') || path.endsWith('.jsx')) return 'javascript';
        if (path.endsWith('.go')) return 'go';
        return 'text';
    }

    /** Minimal regex-based syntax highlighting — real apps use Monaco or Prism */
    basicHighlight(src: string, lang: string): string {
        // Escape HTML first
        let s = src
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;');

        if (lang === 'go') {
            s = s.replace(/\b(package|import|func|type|struct|interface|var|const|return|if|else|for|range|switch|case|default|go|defer|chan|map|make|new|nil|true|false|error)\b/g,
                '<span class="kw">$1</span>');
        } else {
            s = s.replace(/\b(import|export|from|const|let|var|function|class|interface|type|return|if|else|for|while|switch|case|default|new|this|super|extends|implements|async|await|true|false|null|undefined|void|package|func|struct|defer|chan|map|make)\b/g,
                '<span class="kw">$1</span>');
        }

        // Strings
        s = s.replace(/(["'`])((?:\\.|(?!\1)[^\\])*)\1/g, '<span class="str">$1$2$1</span>');
        // Comments
        s = s.replace(/(\/\/[^\n]*)/g, '<span class="cmt">$1</span>');
        s = s.replace(/(\/\*[\s\S]*?\*\/)/g, '<span class="cmt">$1</span>');
        // Numbers
        s = s.replace(/\b(\d+\.?\d*)\b/g, '<span class="num">$1</span>');

        return s;
    }

    copySource(): void {
        navigator.clipboard.writeText(this.source).then(() => {
            this.copied = true;
            setTimeout(() => (this.copied = false), 2000);
        });
    }

    openDetail(): void {
        this.router.navigate(['/detail', this.repoId, this.fileId]);
    }

    goBack(): void {
        history.back();
    }
}