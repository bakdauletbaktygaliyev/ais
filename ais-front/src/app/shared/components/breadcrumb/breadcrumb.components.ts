import { Component, Input, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';
import { BreadcrumbItem } from '../../../core/models';

@Component({
    selector: 'app-breadcrumb',
    standalone: true,
    imports: [CommonModule],
    template: `
    <nav class="breadcrumb" aria-label="breadcrumb">
      <button class="crumb home-crumb" (click)="navigate.emit(-1)" title="Root">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M3 9l9-7 9 7v11a2 2 0 01-2 2H5a2 2 0 01-2-2z"/>
          <polyline points="9 22 9 12 15 12 15 22"/>
        </svg>
      </button>
      <ng-container *ngFor="let crumb of items; let i = index; let last = last">
        <span class="crumb-sep">›</span>
        <button
          class="crumb"
          [class.active]="last"
          [disabled]="last"
          (click)="!last && navigate.emit(i)"
        >{{ crumb.name }}</button>
      </ng-container>
    </nav>
  `,
    styles: [`
    .breadcrumb { display:flex; align-items:center; gap:0.25rem; flex-wrap:wrap; }
    .crumb { background:none; border:none; color:#94a3b8; font-size:0.8rem; cursor:pointer; padding:0.2rem 0.4rem; border-radius:4px; transition:color 0.15s, background 0.15s; white-space:nowrap; }
    .crumb:hover:not(:disabled) { color:#c4b5fd; background:#1e293b; }
    .crumb.active { color:#e2e8f0; font-weight:600; cursor:default; }
    .crumb:disabled { cursor:default; }
    .home-crumb svg { width:14px; height:14px; display:block; }
    .crumb-sep { color:#334155; font-size:0.75rem; user-select:none; }
  `]
})
export class BreadcrumbComponent {
    @Input() items: BreadcrumbItem[] = [];
    /** Emits the index to navigate to, or -1 for root */
    @Output() navigate = new EventEmitter<number>();
}