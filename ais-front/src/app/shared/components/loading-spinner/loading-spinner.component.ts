import { Component, Input } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
    selector: 'app-loading-spinner',
    standalone: true,
    imports: [CommonModule],
    template: `
    <div class="spinner-wrap" [class.fullpage]="fullpage">
      <div class="spinner" [style.width.px]="size" [style.height.px]="size"></div>
      <p class="spinner-label" *ngIf="label">{{ label }}</p>
    </div>
  `,
    styles: [`
    .spinner-wrap { display:flex; flex-direction:column; align-items:center; justify-content:center; gap:0.75rem; }
    .spinner-wrap.fullpage { min-height:60vh; }
    .spinner {
      border:3px solid rgba(99,102,241,0.25);
      border-top-color:#6366f1;
      border-radius:50%;
      animation:spin 0.7s linear infinite;
    }
    @keyframes spin { to { transform:rotate(360deg); } }
    .spinner-label { font-size:0.85rem; color:#64748b; }
  `]
})
export class LoadingSpinnerComponent {
    @Input() size = 36;
    @Input() label = '';
    @Input() fullpage = false;
}