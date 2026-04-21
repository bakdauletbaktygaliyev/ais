import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { Subscription, interval } from 'rxjs';
import { switchMap, takeWhile } from 'rxjs/operators';
import { ApiService, Project, GraphData, GraphNode } from '../../services/api.service';
import { GraphComponent } from '../../components/graph/graph.component';
import { ChatComponent } from '../../components/chat/chat.component';
import { FileViewerComponent } from '../../components/file-viewer/file-viewer.component';

@Component({
  selector: 'app-project',
  standalone: true,
  imports: [CommonModule, GraphComponent, ChatComponent, FileViewerComponent],
  templateUrl: './project.component.html',
  styleUrls: ['./project.component.css']
})
export class ProjectComponent implements OnInit, OnDestroy {
  project: Project | null = null;
  graph: GraphData = { nodes: [], edges: [] };
  currentPath = '';
  breadcrumbs: string[] = [];
  chatOpen = false;
  loading = true;
  selectedFile: GraphNode | null = null;
  showLeaveConfirm = false;

  private sub: Subscription | null = null;

  constructor(
    private route: ActivatedRoute,
    private router: Router,
    private api: ApiService
  ) {}

  ngOnInit() {
    const id = this.route.snapshot.paramMap.get('id')!;
    this.sub = interval(2000).pipe(
      switchMap(() => this.api.getProject(id)),
      takeWhile(p => p.status === 'pending' || p.status === 'analyzing', true)
    ).subscribe({
      next: (p) => {
        this.project = p;
        if (p.status === 'done') { this.loading = false; this.loadGraph(); }
        else if (p.status === 'error') { this.loading = false; }
      },
      error: () => { this.loading = false; }
    });

    this.api.getProject(id).subscribe({
      next: (p) => {
        this.project = p;
        if (p.status === 'done') { this.loading = false; this.loadGraph(); }
        else if (p.status === 'error') { this.loading = false; }
      }
    });
  }

  ngOnDestroy() { this.sub?.unsubscribe(); }

  loadGraph() {
    if (!this.project) return;
    this.api.getGraph(this.project.id, this.currentPath || undefined).subscribe({
      next: (g) => this.graph = g,
      error: () => {}
    });
  }

  onBackClick() {
    const isAnalyzing = this.project?.status === 'pending' || this.project?.status === 'analyzing';
    if (isAnalyzing) {
      this.showLeaveConfirm = true;
    } else {
      this.router.navigate(['/']);
    }
  }

  confirmLeave() {
    this.showLeaveConfirm = false;
    this.router.navigate(['/']);
  }

  goUp() {
    if (this.breadcrumbs.length === 0) return;
    this.navigateTo(this.breadcrumbs.length - 2);
  }

  onNodeDrillDown(node: GraphNode) {
    if (node.type !== 'directory') return;
    this.currentPath = node.path;
    this.breadcrumbs = node.path.split('/').filter(Boolean);
    this.selectedFile = null;
    this.loadGraph();
  }

  onFileSelect(node: GraphNode) {
    this.selectedFile = node;
  }

  closeFileViewer() {
    this.selectedFile = null;
  }

  navigateTo(index: number) {
    if (index < 0) { this.currentPath = ''; this.breadcrumbs = []; }
    else {
      this.breadcrumbs = this.breadcrumbs.slice(0, index + 1);
      this.currentPath = this.breadcrumbs.join('/');
    }
    this.selectedFile = null;
    this.loadGraph();
  }

  get statusMessage(): string {
    if (!this.project) return 'Loading…';
    switch (this.project.status) {
      case 'pending':   return 'Queued for analysis…';
      case 'analyzing': return 'Cloning and analyzing repository…';
      case 'error':     return `Error: ${this.project.error}`;
      default:          return '';
    }
  }
}
