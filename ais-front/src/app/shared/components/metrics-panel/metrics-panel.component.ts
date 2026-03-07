import { Component, Input } from '@angular/core';
import { CommonModule } from '@angular/common';
import { GraphMetrics } from '../../../core/models';

@Component({
    selector: 'app-metrics-panel',
    standalone: true,
    imports: [CommonModule],
    template: `
    <div class="metrics-panel" *ngIf="metrics">
      <h3 class="panel-title">Repository Metrics</h3>
      <div class="metrics-grid">
        <div class="metric">
          <span class="metric-val">{{ metrics.fileCount }}</span>
          <span class="metric-key">Files</span>
        </div>
        <div class="metric">
          <span class="metric-val">{{ metrics.dirCount }}</span>
          <span class="metric-key">Directories</span>
        </div>
        <div class="metric">
          <span class="metric-val">{{ metrics.functionCount }}</span>
          <span class="metric-key">Functions</span>
        </div>
        <div class="metric">
          <span class="metric-val">{{ metrics.classCount }}</span>
          <span class="metric-key">Classes</span>
        </div>
        <div class="metric" [class.danger]="metrics.cycleCount > 0">
          <span class="metric-val">{{ metrics.cycleCount }}</span>
          <span class="metric-key">Cycles</span>
        </div>
        <div class="metric">
          <span class="metric-val">{{ metrics.nodeCount }}</span>
          <span class="metric-key">Graph Nodes</span>
        </div>
        <div class="metric">
          <span class="metric-val">{{ metrics.maxFanIn }}</span>
          <span class="metric-key">Max Fan-In</span>
        </div>
        <div class="metric">
          <span class="metric-val">{{ metrics.maxFanOut }}</span>
          <span class="metric-key">Max Fan-Out</span>
        </div>
        <div class="metric">
          <span class="metric-val">{{ metrics.avgFanIn | number:'1.1-1' }}</span>
          <span class="metric-key">Avg Fan-In</span>
        </div>
        <div class="metric">
          <span class="metric-val">{{ metrics.avgFanOut | number:'1.1-1' }}</span>
          <span class="metric-key">Avg Fan-Out</span>
        </div>
      </div>
    </div>
    <div class="no-metrics" *ngIf="!metrics">
      <p>No metrics available.</p>
    </div>
  `,
    styles: [`
    .metrics-panel { }
    .panel-title { font-size:0.8rem; font-weight:700; text-transform:uppercase; letter-spacing:0.06em; color:#64748b; margin-bottom:0.75rem; }
    .metrics-grid { display:grid; grid-template-columns:repeat(2,1fr); gap:0.6rem; }
    .metric { background:#0f172a; border:1px solid #1e293b; border-radius:8px; padding:0.6rem 0.75rem; }
    .metric.danger { border-color:rgba(239,68,68,0.4); }
    .metric-val { display:block; font-size:1.25rem; font-weight:700; color:#a78bfa; line-height:1; }
    .metric.danger .metric-val { color:#f87171; }
    .metric-key { display:block; font-size:0.7rem; color:#64748b; margin-top:0.2rem; }
    .no-metrics { color:#64748b; font-size:0.85rem; padding:1rem 0; text-align:center; }
  `]
})
export class MetricsPanelComponent {
    @Input() metrics: GraphMetrics | null = null;
}