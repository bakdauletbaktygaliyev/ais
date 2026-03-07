import {
    Component, OnInit, OnDestroy, AfterViewInit,
    ViewChild, ElementRef, inject, ChangeDetectorRef
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { Subject } from 'rxjs';
import { takeUntil } from 'rxjs/operators';
import cytoscape, { Core, NodeSingular, EventObject } from 'cytoscape';

import { ApiService } from '../../core/services/api.service';
import { GraphStateService } from '../../core/services/graph-state.service';
import {
    GraphNode, GraphView, NodeType, BreadcrumbItem
} from '../../core/models';
import { buildCytoscapeElements, getCytoscapeStyle, layoutConfig } from './cytoscape-config';
import { ChatComponent } from '../chat/chat.component';

@Component({
    selector: 'app-graph',
    standalone: true,
    imports: [CommonModule, ChatComponent],
    template: `
    <div class="graph-layout">
      <!-- Sidebar: node detail / metrics -->
      <aside class="sidebar" [class.open]="sidebarOpen">
        <div class="sidebar-header">
          <h3>{{ selectedNode?.name || 'Graph Overview' }}</h3>
          <button class="sidebar-close" (click)="sidebarOpen = false">✕</button>
        </div>

        <div class="sidebar-content" *ngIf="selectedNode">
          <div class="node-badge" [attr.data-type]="selectedNode.type">
            {{ selectedNode.type }}
          </div>
          <p class="node-path">{{ selectedNode.path }}</p>

          <div class="metrics-grid">
            <div class="metric-card">
              <span class="metric-val">{{ selectedNode.fanIn }}</span>
              <span class="metric-lbl">Fan-In</span>
            </div>
            <div class="metric-card">
              <span class="metric-val">{{ selectedNode.fanOut }}</span>
              <span class="metric-lbl">Fan-Out</span>
            </div>
          </div>

          <div class="cycle-warning" *ngIf="selectedNode.hasCycle">
            ⚠️ Part of a circular dependency
          </div>

          <div class="sidebar-actions">
            <button
              class="action-btn primary"
              *ngIf="selectedNode.hasChildren"
              (click)="drillDown(selectedNode)"
            >
              Drill Down →
            </button>
            <button
              class="action-btn"
              *ngIf="selectedNode.type === 'File'"
              (click)="viewSource(selectedNode)"
            >
              View Source
            </button>
            <button
              class="action-btn"
              (click)="openDetail(selectedNode)"
            >
              Full Detail
            </button>
          </div>
        </div>

        <div class="sidebar-content overview" *ngIf="!selectedNode && graphMetrics">
          <div class="overview-stats">
            <div class="stat"><span>{{ graphMetrics.fileCount }}</span>Files</div>
            <div class="stat"><span>{{ graphMetrics.dirCount }}</span>Dirs</div>
            <div class="stat"><span>{{ graphMetrics.functionCount }}</span>Functions</div>
            <div class="stat" [class.has-cycles]="graphMetrics.cycleCount > 0">
              <span>{{ graphMetrics.cycleCount }}</span>Cycles
            </div>
          </div>
        </div>
      </aside>

      <!-- Main graph area -->
      <main class="graph-main">
        <!-- Breadcrumb -->
        <nav class="breadcrumb-nav">
          <button class="breadcrumb-home" (click)="navigateToRoot()">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>
            </svg>
          </button>
          <ng-container *ngFor="let crumb of breadcrumbs; let last = last">
            <span class="breadcrumb-sep">/</span>
            <button
              class="breadcrumb-item"
              [class.active]="last"
              (click)="!last && navigateToBreadcrumb(crumb)"
            >
              {{ crumb.name }}
            </button>
          </ng-container>
        </nav>

        <!-- Cytoscape canvas -->
        <div #cytoscapeContainer class="cytoscape-container"></div>

        <!-- Loading overlay -->
        <div class="graph-loading" *ngIf="loading">
          <div class="loading-spinner"></div>
          <p>Loading graph...</p>
        </div>

        <!-- Toolbar -->
        <div class="graph-toolbar">
          <button class="toolbar-btn" (click)="fitGraph()" title="Fit to view">⊞</button>
          <button class="toolbar-btn" (click)="zoomIn()" title="Zoom in">+</button>
          <button class="toolbar-btn" (click)="zoomOut()" title="Zoom out">−</button>
          <button class="toolbar-btn" (click)="toggleChat()" title="AI Chat">🤖</button>
        </div>
      </main>

      <!-- Chat panel -->
      <div class="chat-panel" [class.open]="chatOpen">
        <app-chat
          *ngIf="chatOpen && repoId"
          [repoId]="repoId"
          [nodeId]="selectedNode?.id"
          (close)="chatOpen = false"
        ></app-chat>
      </div>
    </div>
  `,
    styles: [`
    .graph-layout {
      display: flex;
      height: 100vh;
      background: #0f0f13;
      color: #e2e8f0;
      overflow: hidden;
      position: relative;
    }
    .sidebar {
      width: 0; overflow: hidden;
      background: #1e1e2e;
      border-right: 1px solid #1e293b;
      transition: width 0.25s ease;
      flex-shrink: 0;
    }
    .sidebar.open { width: 280px; }
    .sidebar-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 1rem 1.25rem;
      border-bottom: 1px solid #1e293b;
    }
    .sidebar-header h3 {
      font-size: 0.95rem;
      font-weight: 600;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .sidebar-close {
      background: none; border: none; color: #64748b;
      cursor: pointer; font-size: 1rem; padding: 0.25rem;
    }
    .sidebar-content { padding: 1.25rem; }
    .node-badge {
      display: inline-block;
      font-size: 0.7rem;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      padding: 0.2rem 0.5rem;
      border-radius: 4px;
      margin-bottom: 0.75rem;
      background: #334155;
      color: #94a3b8;
    }
    [data-type="File"] { background: rgba(99,102,241,0.2); color: #a78bfa; }
    [data-type="Dir"] { background: rgba(245,158,11,0.2); color: #fbbf24; }
    [data-type="Function"] { background: rgba(34,197,94,0.2); color: #4ade80; }
    [data-type="Class"] { background: rgba(239,68,68,0.2); color: #f87171; }
    .node-path {
      font-size: 0.78rem;
      color: #64748b;
      word-break: break-all;
      margin-bottom: 1rem;
      font-family: 'JetBrains Mono', monospace;
    }
    .metrics-grid {
      display: grid; grid-template-columns: 1fr 1fr;
      gap: 0.75rem; margin-bottom: 1rem;
    }
    .metric-card {
      background: #0f172a;
      border-radius: 8px;
      padding: 0.75rem;
      text-align: center;
    }
    .metric-val { display: block; font-size: 1.5rem; font-weight: 700; color: #a78bfa; }
    .metric-lbl { font-size: 0.7rem; color: #64748b; text-transform: uppercase; }
    .cycle-warning {
      background: rgba(239,68,68,0.1);
      border: 1px solid rgba(239,68,68,0.3);
      border-radius: 8px;
      padding: 0.6rem 0.75rem;
      font-size: 0.8rem;
      color: #f87171;
      margin-bottom: 1rem;
    }
    .sidebar-actions { display: flex; flex-direction: column; gap: 0.5rem; }
    .action-btn {
      width: 100%;
      padding: 0.6rem 1rem;
      border-radius: 8px;
      border: 1px solid #334155;
      background: transparent;
      color: #e2e8f0;
      cursor: pointer;
      font-size: 0.875rem;
      transition: all 0.15s;
    }
    .action-btn:hover { background: #334155; }
    .action-btn.primary {
      background: #6366f1;
      border-color: #6366f1;
      color: white;
    }
    .action-btn.primary:hover { background: #4f46e5; }
    .overview-stats {
      display: grid; grid-template-columns: 1fr 1fr;
      gap: 0.75rem;
    }
    .stat {
      background: #0f172a;
      border-radius: 8px;
      padding: 0.75rem;
      text-align: center;
    }
    .stat span { display: block; font-size: 1.5rem; font-weight: 700; color: #a78bfa; }
    .stat { font-size: 0.7rem; color: #64748b; }
    .stat.has-cycles span { color: #f87171; }
    .graph-main {
      flex: 1;
      position: relative;
      overflow: hidden;
    }
    .breadcrumb-nav {
      position: absolute;
      top: 1rem; left: 1rem;
      z-index: 10;
      display: flex;
      align-items: center;
      gap: 0.25rem;
      background: rgba(30,30,46,0.9);
      backdrop-filter: blur(8px);
      border: 1px solid #1e293b;
      border-radius: 8px;
      padding: 0.4rem 0.75rem;
      font-size: 0.8rem;
    }
    .breadcrumb-home {
      background: none; border: none;
      color: #64748b; cursor: pointer;
      display: flex; align-items: center;
      padding: 0.1rem;
    }
    .breadcrumb-home svg { width: 14px; height: 14px; }
    .breadcrumb-home:hover { color: #a78bfa; }
    .breadcrumb-sep { color: #334155; }
    .breadcrumb-item {
      background: none; border: none;
      color: #94a3b8; cursor: pointer;
      padding: 0;
    }
    .breadcrumb-item:hover { color: #a78bfa; }
    .breadcrumb-item.active { color: #e2e8f0; cursor: default; }
    .cytoscape-container {
      width: 100%;
      height: 100%;
    }
    .graph-loading {
      position: absolute;
      inset: 0;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      background: rgba(15,15,19,0.7);
      z-index: 20;
    }
    .loading-spinner {
      width: 40px; height: 40px;
      border: 3px solid rgba(99,102,241,0.3);
      border-top-color: #6366f1;
      border-radius: 50%;
      animation: spin 0.7s linear infinite;
      margin-bottom: 0.75rem;
    }
    @keyframes spin { to { transform: rotate(360deg); } }
    .graph-toolbar {
      position: absolute;
      bottom: 1.5rem; right: 1.5rem;
      display: flex;
      flex-direction: column;
      gap: 0.5rem;
      z-index: 10;
    }
    .toolbar-btn {
      width: 40px; height: 40px;
      background: rgba(30,30,46,0.9);
      backdrop-filter: blur(8px);
      border: 1px solid #1e293b;
      border-radius: 8px;
      color: #94a3b8;
      cursor: pointer;
      font-size: 1rem;
      display: flex; align-items: center; justify-content: center;
      transition: all 0.15s;
    }
    .toolbar-btn:hover { background: #1e293b; color: #e2e8f0; }
    .chat-panel {
      width: 0; overflow: hidden;
      background: #1e1e2e;
      border-left: 1px solid #1e293b;
      transition: width 0.25s ease;
      flex-shrink: 0;
    }
    .chat-panel.open { width: 380px; }
  `]
})
export class GraphComponent implements OnInit, AfterViewInit, OnDestroy {
    @ViewChild('cytoscapeContainer') containerRef!: ElementRef<HTMLDivElement>;

    private route = inject(ActivatedRoute);
    private router = inject(Router);
    private api = inject(ApiService);
    private graphState = inject(GraphStateService);
    private cdr = inject(ChangeDetectorRef);
    private destroy$ = new Subject<void>();

    repoId = '';
    cy: Core | null = null;
    loading = false;
    sidebarOpen = false;
    chatOpen = false;

    selectedNode: GraphNode | null = null;
    breadcrumbs: BreadcrumbItem[] = [];
    graphMetrics: any = null;

    ngOnInit(): void {
        this.repoId = this.route.snapshot.paramMap.get('id')!;

        this.graphState.selectedNode$.pipe(takeUntil(this.destroy$)).subscribe((node) => {
            this.selectedNode = node;
            if (node) this.sidebarOpen = true;
            this.cdr.markForCheck();
        });

        this.graphState.breadcrumbs$.pipe(takeUntil(this.destroy$)).subscribe((crumbs) => {
            this.breadcrumbs = crumbs;
            this.cdr.markForCheck();
        });

        // Load metrics
        this.api.getMetrics(this.repoId).subscribe((m) => {
            this.graphMetrics = m;
            this.graphState.setMetrics(m);
        });
    }

    ngAfterViewInit(): void {
        this.initCytoscape();
        this.loadRootGraph();
    }

    private initCytoscape(): void {
        this.cy = cytoscape({
            container: this.containerRef.nativeElement,
            elements: [],
            style: getCytoscapeStyle(),
            layout: { name: 'preset' },
            minZoom: 0.1,
            maxZoom: 3,
            wheelSensitivity: 0.3,
        });

        // Double-click = drill down
        this.cy.on('dblclick', 'node', (evt: EventObject) => {
            const node = evt.target as NodeSingular;
            const data = node.data() as GraphNode;
            if (data.hasChildren) {
                this.drillDown(data);
            } else if (data.type === 'File') {
                this.viewSource(data);
            }
        });

        // Single click = select
        this.cy.on('click', 'node', (evt: EventObject) => {
            const data = (evt.target as NodeSingular).data() as GraphNode;
            this.graphState.setSelectedNode(data);
            this.highlightSelection(data.id);
        });

        // Click on background = deselect
        this.cy.on('click', (evt: EventObject) => {
            if (evt.target === this.cy) {
                this.graphState.setSelectedNode(null);
                this.cy?.elements().removeClass('dimmed selected');
            }
        });
    }

    loadRootGraph(): void {
        this.loading = true;
        this.api.getRootGraph(this.repoId).pipe(
            takeUntil(this.destroy$)
        ).subscribe({
            next: (view) => {
                this.renderView(view);
                this.graphState.setCurrentView(view);
                this.graphState.resetBreadcrumbs();
                this.loading = false;
                this.cdr.markForCheck();
            },
            error: () => {
                this.loading = false;
                this.cdr.markForCheck();
            }
        });
    }

    drillDown(node: GraphNode): void {
        this.loading = true;

        // Update breadcrumbs
        const crumb: BreadcrumbItem = {
            nodeId: node.id,
            name: node.name,
            type: node.type,
            path: node.path,
        };
        this.graphState.pushBreadcrumb(crumb);

        this.api.getNodeChildren(this.repoId, node.id).pipe(
            takeUntil(this.destroy$)
        ).subscribe({
            next: (view) => {
                this.renderView(view);
                this.graphState.setCurrentView(view);
                this.loading = false;
                this.cdr.markForCheck();
            },
            error: () => {
                this.loading = false;
                this.cdr.markForCheck();
            }
        });
    }

    navigateToRoot(): void {
        this.graphState.resetBreadcrumbs();
        this.loadRootGraph();
    }

    navigateToBreadcrumb(crumb: BreadcrumbItem): void {
        this.graphState.popToBreadcrumb(crumb.nodeId);
        if (crumb.nodeId === '') {
            this.loadRootGraph();
        } else {
            this.loading = true;
            this.api.getNodeChildren(this.repoId, crumb.nodeId).pipe(
                takeUntil(this.destroy$)
            ).subscribe({
                next: (view) => {
                    this.renderView(view);
                    this.loading = false;
                    this.cdr.markForCheck();
                },
                error: () => { this.loading = false; }
            });
        }
    }

    private renderView(view: GraphView): void {
        if (!this.cy) return;
        const elements = buildCytoscapeElements(view);
        this.cy.elements().remove();
        this.cy.add(elements);
        this.cy.layout(layoutConfig).run();
        this.cy.fit(undefined, 40);
    }

    private highlightSelection(nodeId: string): void {
        if (!this.cy) return;
        this.cy.elements().addClass('dimmed');
        const node = this.cy.getElementById(nodeId);
        node.removeClass('dimmed').addClass('selected');
        node.neighborhood().removeClass('dimmed');
    }

    viewSource(node: GraphNode): void {
        this.router.navigate(['/code', this.repoId, node.id]);
    }

    openDetail(node: GraphNode): void {
        this.router.navigate(['/detail', this.repoId, node.id]);
    }

    fitGraph(): void { this.cy?.fit(undefined, 40); }
    zoomIn(): void { this.cy?.zoom(this.cy.zoom() * 1.3); }
    zoomOut(): void { this.cy?.zoom(this.cy.zoom() * 0.77); }
    toggleChat(): void { this.chatOpen = !this.chatOpen; }

    ngOnDestroy(): void {
        this.destroy$.next();
        this.destroy$.complete();
        this.cy?.destroy();
    }
}