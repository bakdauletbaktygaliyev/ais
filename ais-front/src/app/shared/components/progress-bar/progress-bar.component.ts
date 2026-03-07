import { Component, Input } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
    selector: 'app-progress-bar',
    standalone: true,
    imports: [CommonModule],
    template: `
    <div class="progress-outer" [attr.aria-valuenow]="value" role="progressbar">
      <div
        class="progress-inner"
        [style.width.%]="value"
        [class.complete]="value >= 100"
      ></div>
    </div>
    <div class="progress-label" *ngIf="showLabel">
      <span>{{ label }}</span>
      <span>{{ value }}%</span>
    </div>
  `,
    styles: [`
    .progress-outer { height:6px; background:#1e293b; border-radius:3px; overflow:hidden; }
    .progress-inner { height:100%; background:linear-gradient(90deg,#6366f1,#a78bfa); border-radius:3px; transition:width 0.4s ease; }
    .progress-inner.complete { background:linear-gradient(90deg,#22c55e,#4ade80); }
    .progress-label { display:flex; justify-content:space-between; margin-top:0.4rem; font-size:0.75rem; color:#64748b; }
  `]
})
export class ProgressBarComponent {
    @Input() value = 0;        // 0–100
    @Input() label = '';
    @Input() showLabel = true;
}