import { Component, OnInit, OnDestroy, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { Subject, interval } from 'rxjs';
import { takeUntil, switchMap, filter } from 'rxjs/operators';

import { ApiService } from '../../core/services/api.service';
import { WebSocketService } from '../../core/services/websocket.service';
import { GraphStateService } from '../../core/services/graph-state.service';
import { ProgressEvent, PipelineStep, Repo } from '../../core/models';

interface StepDisplay {
    key: PipelineStep;
    label: string;
    progressRange: [number, number];
}

@Component({
    selector: 'app-analysis',
    standalone: true,
    imports: [CommonModule],
    template: `
    <div class="analysis-container">
      <div class="analysis-card">
        <div class="analysis-header">
          <h2>Analyzing Repository</h2>
          <p class="repo-name" *ngIf="repo">
            <svg viewBox="0 0 16 16" fill="currentColor" class="github-icon">
              <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0 0 16 8c0-4.42-3.58-8-8-8z"/>
            </svg>
            {{ repo.owner }}/{{ repo.name }}
          </p>
        </div>

        <!-- Overall progress bar -->
        <div class="progress-section">
          <div class="progress-bar-outer">
            <div
              class="progress-bar-inner"
              [style.width.%]="currentProgress"
              [class.complete]="currentProgress >= 100"
            ></div>
          </div>
          <div class="progress-info">
            <span class="progress-pct">{{ currentProgress }}%</span>
            <span class="progress-msg">{{ currentMessage }}</span>
          </div>
        </div>

        <!-- Step indicators -->
        <div class="steps-list">
          <div
            *ngFor="let step of steps"
            class="step-item"
            [class.complete]="isStepComplete(step.key)"
            [class.active]="isStepActive(step.key)"
            [class.error]="hasError && isStepActive(step.key)"
          >
            <div class="step-icon">
              <svg *ngIf="isStepComplete(step.key)" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                <polyline points="20 6 9 17 4 12"/>
              </svg>
              <span *ngIf="isStepActive(step.key) && !hasError" class="step-spinner"></span>
              <svg *ngIf="hasError && isStepActive(step.key)" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                <circle cx="12" cy="12" r="10"/>
                <line x1="12" y1="8" x2="12" y2="12"/>
                <line x1="12" y1="16" x2="12.01" y2="16"/>
              </svg>
              <span *ngIf="!isStepComplete(step.key) && !isStepActive(step.key) && !hasError" class="step-dot"></span>
            </div>
            <span class="step-label">{{ step.label }}</span>
          </div>
        </div>

        <!-- Error state -->
        <div class="error-block" *ngIf="hasError">
          <p class="error-title">Analysis Failed</p>
          <p class="error-msg">{{ errorMessage }}</p>
          <button class="retry-btn" (click)="goHome()">Try Another Repository</button>
        </div>
      </div>
    </div>
  `,
    styles: [`
    .analysis-container {
      min-height: 100vh;
      background: #0f0f13;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 2rem;
    }
    .analysis-card {
      background: #1e1e2e;
      border: 1px solid #1e293b;
      border-radius: 16px;
      padding: 2.5rem;
      width: 100%;
      max-width: 560px;
    }
    .analysis-header { margin-bottom: 2rem; }
    .analysis-header h2 {
      font-size: 1.5rem;
      font-weight: 700;
      color: #e2e8f0;
      margin-bottom: 0.5rem;
    }
    .repo-name {
      display: flex;
      align-items: center;
      gap: 0.5rem;
      color: #94a3b8;
      font-size: 0.95rem;
    }
    .github-icon { width: 16px; height: 16px; }
    .progress-section { margin-bottom: 2rem; }
    .progress-bar-outer {
      height: 8px;
      background: #0f172a;
      border-radius: 4px;
      overflow: hidden;
      margin-bottom: 0.75rem;
    }
    .progress-bar-inner {
      height: 100%;
      background: linear-gradient(90deg, #6366f1, #a78bfa);
      border-radius: 4px;
      transition: width 0.4s ease;
    }
    .progress-bar-inner.complete { background: #22c55e; }
    .progress-info { display: flex; align-items: center; gap: 1rem; }
    .progress-pct { font-size: 1.25rem; font-weight: 700; color: #a78bfa; min-width: 3.5rem; }
    .progress-msg { font-size: 0.875rem; color: #64748b; }
    .steps-list { display: flex; flex-direction: column; gap: 0.75rem; }
    .step-item { display: flex; align-items: center; gap: 0.75rem; }
    .step-icon {
      width: 24px; height: 24px;
      display: flex; align-items: center; justify-content: center;
      flex-shrink: 0;
    }
    .step-icon svg { width: 18px; height: 18px; }
    .step-item.complete .step-icon { color: #22c55e; }
    .step-item.active .step-icon { color: #6366f1; }
    .step-item.error .step-icon { color: #ef4444; }
    .step-dot {
      width: 8px; height: 8px;
      background: #334155;
      border-radius: 50%;
    }
    .step-spinner {
      width: 16px; height: 16px;
      border: 2px solid rgba(99,102,241,0.3);
      border-top-color: #6366f1;
      border-radius: 50%;
      animation: spin 0.7s linear infinite;
      display: block;
    }
    @keyframes spin { to { transform: rotate(360deg); } }
    .step-label {
      font-size: 0.9rem;
      color: #94a3b8;
    }
    .step-item.complete .step-label { color: #e2e8f0; }
    .step-item.active .step-label { color: #c4b5fd; font-weight: 500; }
    .error-block {
      margin-top: 1.5rem;
      background: rgba(239,68,68,0.1);
      border: 1px solid rgba(239,68,68,0.3);
      border-radius: 8px;
      padding: 1.25rem;
    }
    .error-title { font-weight: 600; color: #f87171; margin-bottom: 0.4rem; }
    .error-msg { font-size: 0.875rem; color: #94a3b8; margin-bottom: 1rem; }
    .retry-btn {
      background: #1e293b;
      border: 1px solid #334155;
      color: #e2e8f0;
      border-radius: 8px;
      padding: 0.5rem 1.25rem;
      cursor: pointer;
      font-size: 0.875rem;
    }
    .retry-btn:hover { background: #334155; }
  `]
})
export class AnalysisComponent implements OnInit, OnDestroy {
    private route = inject(ActivatedRoute);
    private router = inject(Router);
    private api = inject(ApiService);
    private ws = inject(WebSocketService);
    private graphState = inject(GraphStateService);
    private destroy$ = new Subject<void>();

    repoId = '';
    repo: Repo | null = null;
    currentProgress = 0;
    currentMessage = 'Initializing...';
    currentStep: PipelineStep = 'validate';
    hasError = false;
    errorMessage = '';

    steps: StepDisplay[] = [
        { key: 'validate', label: 'Validating repository', progressRange: [0, 5] },
        { key: 'clone', label: 'Cloning repository', progressRange: [5, 20] },
        { key: 'detect', label: 'Detecting language', progressRange: [20, 30] },
        { key: 'walk_fs', label: 'Traversing file system', progressRange: [30, 50] },
        { key: 'parse_ast', label: 'Parsing source files', progressRange: [50, 70] },
        { key: 'build_graph', label: 'Building dependency graph', progressRange: [70, 85] },
        { key: 'index_ai', label: 'Indexing for AI search', progressRange: [85, 95] },
        { key: 'done', label: 'Analysis complete', progressRange: [95, 100] },
    ];

    ngOnInit(): void {
        this.repoId = this.route.snapshot.paramMap.get('id')!;

        // Connect WebSocket and subscribe to progress
        this.ws.connect();
        this.ws.subscribeToRepo(this.repoId);

        // Listen for progress events
        this.ws.messagesOfType('progress').pipe(
            takeUntil(this.destroy$),
            filter((msg) => msg.repoId === this.repoId),
        ).subscribe((msg) => {
            const evt = msg as unknown as ProgressEvent;
            this.currentProgress = evt.progress;
            this.currentMessage = evt.message;
            this.currentStep = evt.step;

            if (evt.step === 'done') {
                this.currentProgress = 100;
                setTimeout(() => this.navigateToGraph(), 600);
            }
        });

        // Listen for errors
        this.ws.messagesOfType('error').pipe(
            takeUntil(this.destroy$),
        ).subscribe((msg) => {
            this.hasError = true;
            this.errorMessage = msg.error || 'An unknown error occurred';
        });

        // Poll repo status as a fallback
        interval(3000).pipe(
            takeUntil(this.destroy$),
            switchMap(() => this.api.getRepo(this.repoId)),
        ).subscribe({
            next: (repo) => {
                this.repo = repo;
                this.graphState.setRepo(repo);

                if (repo.status === 'ready') {
                    this.navigateToGraph();
                } else if (repo.status === 'error') {
                    this.hasError = true;
                    this.errorMessage = repo.errorMessage || 'Analysis failed';
                }
            },
            error: () => { }, // ignore polling errors
        });
    }

    isStepComplete(step: PipelineStep): boolean {
        const stepOrder: PipelineStep[] = [
            'validate', 'clone', 'detect', 'walk_fs',
            'parse_ast', 'build_graph', 'index_ai', 'done'
        ];
        const currentIdx = stepOrder.indexOf(this.currentStep);
        const stepIdx = stepOrder.indexOf(step);
        return currentIdx > stepIdx || this.currentStep === 'done';
    }

    isStepActive(step: PipelineStep): boolean {
        return this.currentStep === step;
    }

    navigateToGraph(): void {
        this.router.navigate(['/graph', this.repoId]);
    }

    goHome(): void {
        this.router.navigate(['/']);
    }

    ngOnDestroy(): void {
        this.destroy$.next();
        this.destroy$.complete();
    }
}