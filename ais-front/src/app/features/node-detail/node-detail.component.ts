import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { ApiService } from '../../core/services/api.service';
import { NodeDetail, GraphNode } from '../../core/models';

@Component({
    selector: 'app-node-detail',
    standalone: true,
    imports: [CommonModule, RouterModule],
    template: `
    <div class="detail-page">
      <header class="detail-header">
        <button class="back-btn" (click)="goBack()">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M19 12H5M12 5l-7 7 7 7"/>
          </svg>
          Back
        </button>
        <div class="header-info" *ngIf="detail">
          <span class="node-type-badge" [attr.data-type]="detail.node.type">{{ detail.node.type }}</span>
          <h1 class="node-name">{{ detail.node.name }}</h1>
          <p class="node-path">{{ detail.node.path }}</p>
        </div>
      </header>

      <div class="detail-loading" *ngIf="loading">
        <div class="spinner"></div>
      </div>

      <div class="detail-content" *ngIf="detail && !loading">
        <!-- Metrics row -->
        <section class="metrics-row">
          <div class="metric-tile">
            <span class="metric-num">{{ detail.metrics.fanIn }}</span>
            <span class="metric-name">Fan-In</span>
            <span class="metric-hint">files importing this</span>
          </div>
          <div class="metric-tile">
            <span class="metric-num">{{ detail.metrics.fanOut }}</span>
            <span class="metric-name">Fan-Out</span>
            <span class="metric-hint">files this imports</span>
          </div>
          <div class="metric-tile" [class.danger]="detail.metrics.coupling > 0.7">
            <span class="metric-num">{{ (detail.metrics.coupling * 100).toFixed(0) }}%</span>
            <span class="metric-name">Coupling</span>
            <span class="metric-hint">fan-in / (fan-in + fan-out)</span>
          </div>
          <div class="metric-tile" [class.danger]="detail.metrics.isInCycle">
            <span class="metric-num">{{ detail.metrics.isInCycle ? '⚠️ Yes' : '✅ No' }}</span>
            <span class="metric-name">In Cycle</span>
            <span class="metric-hint">circular dependency</span>
          </div>
          <div class="metric-tile" *ngIf="detail.node.type === 'File'">
            <span class="metric-num">{{ detail.functions.length }}</span>
            <span class="metric-name">Functions</span>
            <span class="metric-hint">defined in file</span>
          </div>
          <div class="metric-tile" *ngIf="detail.node.type === 'File'">
            <span class="metric-num">{{ detail.classes.length }}</span>
            <span class="metric-name">Classes</span>
            <span class="metric-hint">defined in file</span>
          </div>
        </section>

        <!-- Cycle warning -->
        <section class="cycle-warning-banner" *ngIf="detail.metrics.isInCycle">
          <span>⚠️ This file participates in a circular dependency.</span>
          <span *ngIf="detail.metrics.cycleMembers?.length">
            Cycle: {{ detail.metrics.cycleMembers!.join(' → ') }}
          </span>
        </section>

        <div class="two-col">
          <!-- Imports -->
          <section class="detail-section" *ngIf="detail.imports.length">
            <h2 class="section-title">
              <span class="section-icon">→</span> Imports ({{ detail.imports.length }})
            </h2>
            <ul class="node-list">
              <li *ngFor="let n of detail.imports" class="node-list-item" (click)="navigateTo(n)">
                <span class="node-pill" [attr.data-type]="n.type">{{ n.type }}</span>
                <span class="node-item-name">{{ n.name }}</span>
                <span class="node-item-path">{{ n.path }}</span>
                <span class="fan-badge" title="fan-in">↙{{ n.fanIn }}</span>
              </li>
            </ul>
          </section>

          <!-- Imported by -->
          <section class="detail-section" *ngIf="detail.importedBy.length">
            <h2 class="section-title">
              <span class="section-icon">←</span> Imported By ({{ detail.importedBy.length }})
            </h2>
            <ul class="node-list">
              <li *ngFor="let n of detail.importedBy" class="node-list-item" (click)="navigateTo(n)">
                <span class="node-pill" [attr.data-type]="n.type">{{ n.type }}</span>
                <span class="node-item-name">{{ n.name }}</span>
                <span class="node-item-path">{{ n.path }}</span>
                <span class="fan-badge" title="fan-out">↗{{ n.fanOut }}</span>
              </li>
            </ul>
          </section>

          <!-- Functions -->
          <section class="detail-section" *ngIf="detail.functions.length">
            <h2 class="section-title">
              <span class="section-icon">ƒ</span> Functions ({{ detail.functions.length }})
            </h2>
            <ul class="node-list">
              <li *ngFor="let n of detail.functions" class="node-list-item" (click)="navigateTo(n)">
                <span class="node-pill" data-type="Function">fn</span>
                <span class="node-item-name">{{ n.name }}</span>
                <span class="node-item-path" *ngIf="n.startLine">L{{ n.startLine }}–{{ n.endLine }}</span>
              </li>
            </ul>
          </section>

          <!-- Classes -->
          <section class="detail-section" *ngIf="detail.classes.length">
            <h2 class="section-title">
              <span class="section-icon">◻</span> Classes ({{ detail.classes.length }})
            </h2>
            <ul class="node-list">
              <li *ngFor="let n of detail.classes" class="node-list-item" (click)="navigateTo(n)">
                <span class="node-pill" data-type="Class">cls</span>
                <span class="node-item-name">{{ n.name }}</span>
                <span class="node-item-path" *ngIf="n.startLine">L{{ n.startLine }}–{{ n.endLine }}</span>
              </li>
            </ul>
          </section>

          <!-- Callers -->
          <section class="detail-section" *ngIf="detail.callers.length">
            <h2 class="section-title">
              <span class="section-icon">↑</span> Callers ({{ detail.callers.length }})
            </h2>
            <ul class="node-list">
              <li *ngFor="let n of detail.callers" class="node-list-item" (click)="navigateTo(n)">
                <span class="node-pill" [attr.data-type]="n.type">{{ n.type }}</span>
                <span class="node-item-name">{{ n.name }}</span>
              </li>
            </ul>
          </section>

          <!-- Callees -->
          <section class="detail-section" *ngIf="detail.callees.length">
            <h2 class="section-title">
              <span class="section-icon">↓</span> Calls ({{ detail.callees.length }})
            </h2>
            <ul class="node-list">
              <li *ngFor="let n of detail.callees" class="node-list-item" (click)="navigateTo(n)">
                <span class="node-pill" [attr.data-type]="n.type">{{ n.type }}</span>
                <span class="node-item-name">{{ n.name }}</span>
              </li>
            </ul>
          </section>
        </div>

        <!-- View source button for file nodes -->
        <div class="source-action" *ngIf="detail.node.type === 'File'">
          <button class="view-source-btn" (click)="viewSource()">
            View Source Code →
          </button>
        </div>
      </div>
    </div>
  `,
    styles: [`
    .detail-page { min-height:100vh; background:#0f0f13; color:#e2e8f0; }
    .detail-header { display:flex; align-items:flex-start; gap:1.5rem; padding:1.5rem 2rem; border-bottom:1px solid #1e293b; background:#1e1e2e; }
    .back-btn { display:flex; align-items:center; gap:0.4rem; background:none; border:1px solid #334155; color:#94a3b8; border-radius:8px; padding:0.5rem 0.85rem; cursor:pointer; font-size:0.875rem; white-space:nowrap; transition:all 0.15s; }
    .back-btn svg { width:16px; height:16px; }
    .back-btn:hover { border-color:#6366f1; color:#a78bfa; }
    .header-info { flex:1; }
    .node-type-badge { display:inline-block; font-size:0.7rem; font-weight:700; text-transform:uppercase; letter-spacing:0.05em; padding:0.2rem 0.5rem; border-radius:4px; margin-bottom:0.5rem; background:#334155; color:#94a3b8; }
    [data-type="File"] { background:rgba(99,102,241,0.2); color:#a78bfa; }
    [data-type="Dir"] { background:rgba(245,158,11,0.2); color:#fbbf24; }
    [data-type="Function"] { background:rgba(34,197,94,0.2); color:#4ade80; }
    [data-type="Class"] { background:rgba(236,72,153,0.2); color:#f472b6; }
    .node-name { font-size:1.5rem; font-weight:700; margin-bottom:0.25rem; }
    .node-path { font-size:0.8rem; color:#64748b; font-family:monospace; }
    .detail-loading { display:flex; justify-content:center; padding:4rem; }
    .spinner { width:40px; height:40px; border:3px solid rgba(99,102,241,0.3); border-top-color:#6366f1; border-radius:50%; animation:spin 0.7s linear infinite; }
    @keyframes spin { to{transform:rotate(360deg)} }
    .detail-content { padding:2rem; max-width:1200px; margin:0 auto; }
    .metrics-row { display:flex; flex-wrap:wrap; gap:1rem; margin-bottom:1.5rem; }
    .metric-tile { background:#1e1e2e; border:1px solid #1e293b; border-radius:12px; padding:1.25rem 1.5rem; min-width:130px; flex:1; }
    .metric-tile.danger { border-color:rgba(239,68,68,0.4); background:rgba(239,68,68,0.05); }
    .metric-num { display:block; font-size:1.75rem; font-weight:700; color:#a78bfa; }
    .metric-tile.danger .metric-num { color:#f87171; }
    .metric-name { display:block; font-size:0.8rem; font-weight:600; color:#e2e8f0; margin-top:0.25rem; }
    .metric-hint { display:block; font-size:0.7rem; color:#64748b; margin-top:0.15rem; }
    .cycle-warning-banner { background:rgba(239,68,68,0.1); border:1px solid rgba(239,68,68,0.3); border-radius:8px; padding:0.75rem 1rem; font-size:0.875rem; color:#f87171; margin-bottom:1.5rem; display:flex; flex-direction:column; gap:0.25rem; }
    .two-col { display:grid; grid-template-columns:repeat(auto-fill, minmax(380px, 1fr)); gap:1.5rem; }
    .detail-section { background:#1e1e2e; border:1px solid #1e293b; border-radius:12px; padding:1.25rem; }
    .section-title { font-size:0.9rem; font-weight:600; color:#94a3b8; margin-bottom:0.75rem; display:flex; align-items:center; gap:0.5rem; }
    .section-icon { color:#6366f1; font-weight:700; font-size:1rem; }
    .node-list { list-style:none; padding:0; margin:0; display:flex; flex-direction:column; gap:0.4rem; }
    .node-list-item { display:flex; align-items:center; gap:0.5rem; padding:0.5rem 0.6rem; border-radius:6px; cursor:pointer; transition:background 0.15s; font-size:0.83rem; }
    .node-list-item:hover { background:#0f172a; }
    .node-pill { font-size:0.65rem; font-weight:700; padding:0.15rem 0.4rem; border-radius:4px; text-transform:uppercase; flex-shrink:0; background:#334155; color:#94a3b8; }
    [data-type="File"].node-pill { background:rgba(99,102,241,0.2); color:#a78bfa; }
    [data-type="Function"].node-pill { background:rgba(34,197,94,0.2); color:#4ade80; }
    [data-type="Class"].node-pill { background:rgba(236,72,153,0.2); color:#f472b6; }
    .node-item-name { font-weight:500; color:#e2e8f0; flex:1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
    .node-item-path { font-size:0.72rem; color:#475569; font-family:monospace; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; max-width:160px; }
    .fan-badge { font-size:0.7rem; color:#64748b; flex-shrink:0; }
    .source-action { margin-top:2rem; display:flex; justify-content:center; }
    .view-source-btn { background:linear-gradient(135deg, #6366f1, #7c3aed); color:white; border:none; border-radius:10px; padding:0.75rem 2rem; font-size:0.95rem; font-weight:600; cursor:pointer; transition:opacity 0.2s; }
    .view-source-btn:hover { opacity:0.9; }
  `]
})
export class NodeDetailComponent implements OnInit {
    private route = inject(ActivatedRoute);
    private router = inject(Router);
    private api = inject(ApiService);

    repoId = '';
    nodeId = '';
    detail: NodeDetail | null = null;
    loading = false;

    ngOnInit(): void {
        this.repoId = this.route.snapshot.paramMap.get('repoId')!;
        this.nodeId = this.route.snapshot.paramMap.get('nodeId')!;
        this.loadDetail();
    }

    loadDetail(): void {
        this.loading = true;
        this.api.getNodeDetail(this.repoId, this.nodeId).subscribe({
            next: (d) => { this.detail = d; this.loading = false; },
            error: () => { this.loading = false; },
        });
    }

    navigateTo(node: GraphNode): void {
        this.router.navigate(['/detail', this.repoId, node.id]);
    }

    viewSource(): void {
        this.router.navigate(['/code', this.repoId, this.nodeId]);
    }

    goBack(): void {
        this.router.navigate(['/graph', this.repoId]);
    }
}