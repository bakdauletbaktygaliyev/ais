import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { ApiService } from '../../core/services/api.service';
import { WebSocketService } from '../../core/services/websocket.service';
import { GraphStateService } from '../../core/services/graph-state.service';

@Component({
    selector: 'app-home',
    standalone: true,
    imports: [CommonModule, ReactiveFormsModule],
    template: `
    <div class="home-container">
      <header class="hero">
        <div class="hero-icon">
          <svg viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
            <circle cx="24" cy="24" r="20" stroke="#6366f1" stroke-width="2"/>
            <circle cx="12" cy="24" r="4" fill="#6366f1"/>
            <circle cx="36" cy="12" r="4" fill="#6366f1"/>
            <circle cx="36" cy="36" r="4" fill="#6366f1"/>
            <line x1="16" y1="24" x2="32" y2="14" stroke="#6366f1" stroke-width="1.5"/>
            <line x1="16" y1="24" x2="32" y2="34" stroke="#6366f1" stroke-width="1.5"/>
          </svg>
        </div>
        <h1 class="hero-title">Architecture Insight System</h1>
        <p class="hero-subtitle">
          Understand any codebase in minutes. Visualise dependency graphs,
          detect cycles, and chat with your code using AI.
        </p>
      </header>

      <section class="submit-section">
        <form [formGroup]="form" (ngSubmit)="onSubmit()" class="submit-form">
          <div class="input-group">
            <svg class="input-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"/>
            </svg>
            <input
              formControlName="url"
              type="text"
              class="repo-input"
              placeholder="https://github.com/owner/repository"
              [class.error]="form.get('url')?.invalid && form.get('url')?.touched"
            />
            <button
              type="submit"
              class="analyze-btn"
              [disabled]="form.invalid || isSubmitting"
            >
              <span *ngIf="!isSubmitting">Analyze</span>
              <span *ngIf="isSubmitting" class="btn-loading">
                <span class="spinner"></span>
                Starting...
              </span>
            </button>
          </div>
          <div class="input-error" *ngIf="form.get('url')?.invalid && form.get('url')?.touched">
            Please enter a valid GitHub repository URL (e.g. https://github.com/owner/repo)
          </div>
          <div class="submit-error" *ngIf="submitError">{{ submitError }}</div>
        </form>

        <div class="examples">
          <span class="examples-label">Try:</span>
          <button *ngFor="let ex of examples" class="example-btn" (click)="useExample(ex.url)">
            {{ ex.label }}
          </button>
        </div>
      </section>

      <section class="features-grid">
        <div class="feature-card" *ngFor="let f of features">
          <div class="feature-icon" [innerHTML]="f.icon"></div>
          <h3>{{ f.title }}</h3>
          <p>{{ f.description }}</p>
        </div>
      </section>
    </div>
  `,
    styles: [`
    .home-container {
      min-height: 100vh;
      background: #0f0f13;
      color: #e2e8f0;
      display: flex;
      flex-direction: column;
      align-items: center;
      padding: 0 1.5rem 4rem;
    }
    .hero {
      text-align: center;
      padding: 5rem 1rem 3rem;
      max-width: 700px;
    }
    .hero-icon {
      width: 64px;
      height: 64px;
      margin: 0 auto 1.5rem;
    }
    .hero-title {
      font-size: 2.8rem;
      font-weight: 700;
      background: linear-gradient(135deg, #6366f1, #a78bfa);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
      margin-bottom: 1rem;
    }
    .hero-subtitle {
      font-size: 1.15rem;
      color: #94a3b8;
      line-height: 1.7;
    }
    .submit-section {
      width: 100%;
      max-width: 680px;
    }
    .submit-form { margin-bottom: 1rem; }
    .input-group {
      display: flex;
      align-items: center;
      background: #1e1e2e;
      border: 1px solid #334155;
      border-radius: 12px;
      padding: 0.25rem 0.25rem 0.25rem 1rem;
      transition: border-color 0.2s;
    }
    .input-group:focus-within { border-color: #6366f1; }
    .input-icon {
      width: 20px;
      height: 20px;
      color: #64748b;
      flex-shrink: 0;
      margin-right: 0.75rem;
    }
    .repo-input {
      flex: 1;
      background: transparent;
      border: none;
      outline: none;
      color: #e2e8f0;
      font-size: 1rem;
      padding: 0.75rem 0;
    }
    .repo-input::placeholder { color: #475569; }
    .repo-input.error { color: #f87171; }
    .analyze-btn {
      background: linear-gradient(135deg, #6366f1, #7c3aed);
      color: white;
      border: none;
      border-radius: 10px;
      padding: 0.75rem 1.75rem;
      font-size: 0.95rem;
      font-weight: 600;
      cursor: pointer;
      transition: opacity 0.2s;
      white-space: nowrap;
    }
    .analyze-btn:disabled { opacity: 0.5; cursor: not-allowed; }
    .analyze-btn:not(:disabled):hover { opacity: 0.9; }
    .btn-loading { display: flex; align-items: center; gap: 0.5rem; }
    .spinner {
      width: 14px; height: 14px;
      border: 2px solid rgba(255,255,255,0.3);
      border-top-color: white;
      border-radius: 50%;
      animation: spin 0.7s linear infinite;
    }
    @keyframes spin { to { transform: rotate(360deg); } }
    .input-error, .submit-error {
      font-size: 0.85rem;
      color: #f87171;
      margin-top: 0.5rem;
      padding-left: 0.5rem;
    }
    .examples {
      display: flex;
      align-items: center;
      gap: 0.5rem;
      flex-wrap: wrap;
      margin-top: 0.75rem;
    }
    .examples-label { font-size: 0.85rem; color: #64748b; }
    .example-btn {
      background: #1e1e2e;
      border: 1px solid #334155;
      color: #94a3b8;
      border-radius: 6px;
      padding: 0.3rem 0.75rem;
      font-size: 0.8rem;
      cursor: pointer;
      transition: all 0.15s;
    }
    .example-btn:hover { border-color: #6366f1; color: #c4b5fd; }
    .features-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
      gap: 1.25rem;
      width: 100%;
      max-width: 900px;
      margin-top: 4rem;
    }
    .feature-card {
      background: #1e1e2e;
      border: 1px solid #1e293b;
      border-radius: 12px;
      padding: 1.5rem;
    }
    .feature-icon { font-size: 1.75rem; margin-bottom: 0.75rem; }
    .feature-card h3 { font-size: 1rem; font-weight: 600; margin-bottom: 0.4rem; }
    .feature-card p { font-size: 0.875rem; color: #64748b; line-height: 1.5; }
  `]
})
export class HomeComponent {
    private fb = inject(FormBuilder);
    private router = inject(Router);
    private api = inject(ApiService);
    private ws = inject(WebSocketService);
    private graphState = inject(GraphStateService);

    isSubmitting = false;
    submitError = '';

    form = this.fb.group({
        url: ['', [
            Validators.required,
            Validators.pattern(/^https?:\/\/github\.com\/[^/]+\/[^/]+/)
        ]]
    });

    examples = [
        { label: 'next.js', url: 'https://github.com/vercel/next.js' },
        { label: 'gin', url: 'https://github.com/gin-gonic/gin' },
        { label: 'rxjs', url: 'https://github.com/ReactiveX/rxjs' },
    ];

    features = [
        {
            icon: '🔍',
            title: 'Instant Graph Visualization',
            description: 'Every directory becomes a node. Drill down from root to individual files with double-click navigation.',
        },
        {
            icon: '🔄',
            title: 'Cycle Detection',
            description: 'Automatically detect circular imports highlighted with a red dashed ring, pinpointing architectural debt.',
        },
        {
            icon: '🤖',
            title: 'AI Chat with RAG',
            description: 'Ask questions in natural language. Claude answers with full codebase context via vector search.',
        },
        {
            icon: '📊',
            title: 'Dependency Metrics',
            description: 'Fan-in and fan-out metrics reveal your most critical files and potential coupling hotspots.',
        },
    ];

    useExample(url: string): void {
        this.form.patchValue({ url });
    }

    onSubmit(): void {
        if (this.form.invalid || this.isSubmitting) return;

        this.isSubmitting = true;
        this.submitError = '';
        const url = this.form.value.url!.trim();

        this.api.submitRepo(url).subscribe({
            next: (result) => {
                this.isSubmitting = false;
                this.graphState.reset();
                this.router.navigate(['/analysis', result.id]);
            },
            error: (err) => {
                this.isSubmitting = false;
                this.submitError = err.message || 'Failed to start analysis. Please try again.';
            }
        });
    }
}