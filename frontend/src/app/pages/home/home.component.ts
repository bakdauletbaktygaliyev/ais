import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { ApiService, Project } from '../../services/api.service';

@Component({
  selector: 'app-home',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './home.component.html',
  styleUrls: ['./home.component.css']
})
export class HomeComponent implements OnInit {
  repoUrl = '';
  loading = false;
  error = '';
  projects: Project[] = [];

  constructor(private api: ApiService, private router: Router) {}

  ngOnInit() {
    this.loadProjects();
  }

  loadProjects() {
    this.api.getProjects().subscribe({
      next: (p) => this.projects = p,
      error: () => {}
    });
  }

  analyze() {
    if (!this.repoUrl.trim()) return;
    this.loading = true;
    this.error = '';
    this.api.analyze(this.repoUrl.trim()).subscribe({
      next: (res) => {
        this.loading = false;
        this.router.navigate(['/project', res.id]);
      },
      error: (err) => {
        this.loading = false;
        this.error = err.error?.error || 'Failed to start analysis';
      }
    });
  }

  openProject(id: string) {
    this.router.navigate(['/project', id]);
  }

  deleteProject(id: string, event: Event) {
    event.stopPropagation();
    this.api.deleteProject(id).subscribe({
      next: () => this.loadProjects(),
      error: () => {}
    });
  }

  statusLabel(status: string): string {
    const map: Record<string, string> = {
      pending: 'Queued',
      analyzing: 'Analyzing…',
      done: 'Ready',
      error: 'Error'
    };
    return map[status] || status;
  }
}
