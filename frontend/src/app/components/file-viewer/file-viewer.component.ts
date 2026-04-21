import {
  Component, Input, Output, EventEmitter,
  OnChanges, OnDestroy, AfterViewInit,
  ElementRef, ViewChild, NgZone, SimpleChanges
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { GraphData, GraphNode } from '../../models/project.model';
import { ApiService } from '../../services/api.service';
import { langColor } from '../graph/graph.utils';

interface DepNode { id: string; name: string; path: string; }

const MONACO_LANG: Record<string, string> = {
  go: 'go', python: 'python', typescript: 'typescript',
  javascript: 'javascript', java: 'java', rust: 'rust',
  cpp: 'cpp', c: 'c', csharp: 'csharp', ruby: 'ruby',
  php: 'php', swift: 'swift', kotlin: 'kotlin', vue: 'html',
  html: 'html', css: 'css', scss: 'scss', yaml: 'yaml',
  json: 'json', markdown: 'markdown', shell: 'shell',
  dockerfile: 'dockerfile',
};

@Component({
  selector: 'app-file-viewer',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './file-viewer.component.html',
  styleUrls: ['./file-viewer.component.css']
})
export class FileViewerComponent implements OnChanges, AfterViewInit, OnDestroy {
  @Input() node: GraphNode | null = null;
  @Input() projectId = '';
  @Input() graph: GraphData = { nodes: [], edges: [] };
  @Output() close = new EventEmitter<void>();

  @ViewChild('editorContainer') editorContainer!: ElementRef<HTMLDivElement>;

  imports: DepNode[] = [];
  importedBy: DepNode[] = [];
  loading = false;
  error = '';

  private editor: any = null;
  private monacoReady = false;
  private pendingContent: { code: string; lang: string } | null = null;

  constructor(private zone: NgZone, private api: ApiService) {}

  ngAfterViewInit() {
    this.loadMonaco().then(() => {
      this.monacoReady = true;
      this.initEditor();
      if (this.pendingContent) {
        this.setEditorContent(this.pendingContent.code, this.pendingContent.lang);
        this.pendingContent = null;
      }
    });
  }

  ngOnChanges(changes: SimpleChanges) {
    if (changes['node'] && this.node) {
      this.computeDeps();
      this.fetchContent();
    }
    if (changes['graph']) {
      this.computeDeps();
    }
  }

  ngOnDestroy() {
    this.editor?.dispose();
  }

  private computeDeps() {
    if (!this.node) return;
    const map = new Map(this.graph.nodes.map(n => [n.id, n]));
    const id = this.node.id;

    this.imports = this.graph.edges
      .filter(e => e.source === id)
      .map(e => map.get(e.target as string))
      .filter((n): n is GraphNode => !!n)
      .map(n => ({ id: n.id, name: n.name, path: n.path }));

    this.importedBy = this.graph.edges
      .filter(e => e.target === id)
      .map(e => map.get(e.source as string))
      .filter((n): n is GraphNode => !!n)
      .map(n => ({ id: n.id, name: n.name, path: n.path }));
  }

  private fetchContent() {
    if (!this.node || !this.projectId) return;
    this.loading = true;
    this.error = '';

    this.api.getFileContent(this.projectId, this.node.path).subscribe({
      next: (res) => {
        this.loading = false;
        const lang = MONACO_LANG[this.node!.language] ?? 'plaintext';
        if (this.monacoReady && this.editor) {
          this.setEditorContent(res.content, lang);
        } else {
          this.pendingContent = { code: res.content, lang };
        }
      },
      error: (err) => {
        this.loading = false;
        this.error = err.error?.error ?? 'Could not load file content';
      }
    });
  }

  private initEditor() {
    const container = this.editorContainer?.nativeElement;
    if (!container || !(window as any).monaco) return;

    this.zone.runOutsideAngular(() => {
      this.editor = (window as any).monaco.editor.create(container, {
        value: '',
        language: 'plaintext',
        theme: 'vs-dark',
        readOnly: true,
        fontSize: 13,
        fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
        lineNumbers: 'on',
        minimap: { enabled: false },
        scrollBeyondLastLine: false,
        wordWrap: 'off',
        renderLineHighlight: 'line',
        automaticLayout: true,
        scrollbar: { verticalScrollbarSize: 6, horizontalScrollbarSize: 6 },
      });
    });
  }

  private setEditorContent(code: string, lang: string) {
    if (!this.editor || !(window as any).monaco) return;
    this.zone.runOutsideAngular(() => {
      const m = (window as any).monaco;
      const old = this.editor.getModel();
      const model = m.editor.createModel(code, lang);
      this.editor.setModel(model);
      old?.dispose();
    });
  }

  private loadMonaco(): Promise<void> {
    return new Promise(resolve => {
      if ((window as any).monaco) { resolve(); return; }
      const win = window as any;
      const onLoaded = () => {
        win.require.config({ paths: { vs: '/assets/vs' } });
        win.require(['vs/editor/editor.main'], () => resolve());
      };
      if (win.require?.config) { onLoaded(); return; }
      const script = document.createElement('script');
      script.src = '/assets/vs/loader.js';
      script.onload = onLoaded;
      document.head.appendChild(script);
    });
  }

  get nodeColor(): string {
    return this.node ? langColor(this.node.language, this.node.type) : '#6e7681';
  }
}
