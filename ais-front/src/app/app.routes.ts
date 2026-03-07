import { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./features/home/home.component').then((m) => m.HomeComponent),
  },
  {
    path: 'analysis/:id',
    loadComponent: () =>
      import('./features/analysis/analysis.component').then((m) => m.AnalysisComponent),
  },
  {
    path: 'graph/:id',
    loadComponent: () =>
      import('./features/graph/graph.component').then((m) => m.GraphComponent),
  },
  {
    path: 'detail/:repoId/:nodeId',
    loadComponent: () =>
      import('./features/node-detail/node-detail.component').then((m) => m.NodeDetailComponent),
  },
  {
    path: 'code/:repoId/:fileId',
    loadComponent: () =>
      import('./features/code-viewer/code-viewer.component').then((m) => m.CodeViewerComponent),
  },
  { path: '**', redirectTo: '' },
];