import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { ChatResponse, GraphData, Project } from '../models/project.model';

export type { Project, GraphNode, GraphEdge, GraphData, ChatResponse } from '../models/project.model';

@Injectable({ providedIn: 'root' })
export class ApiService {
  private readonly api = '/api';
  private readonly ai = '/ai';

  constructor(private http: HttpClient) {}

  analyze(url: string): Observable<{ id: string; status: string; name: string }> {
    return this.http.post<{ id: string; status: string; name: string }>(
      `${this.api}/analyze`, { url }
    );
  }

  getProjects(): Observable<Project[]> {
    return this.http.get<Project[]>(`${this.api}/projects`);
  }

  getProject(id: string): Observable<Project> {
    return this.http.get<Project>(`${this.api}/projects/${id}`);
  }

  getGraph(id: string, path?: string): Observable<GraphData> {
    const url = path
      ? `${this.api}/projects/${id}/graph?path=${encodeURIComponent(path)}`
      : `${this.api}/projects/${id}/graph`;
    return this.http.get<GraphData>(url);
  }

  deleteProject(id: string): Observable<void> {
    return this.http.delete<void>(`${this.api}/projects/${id}`);
  }

  getFileContent(projectId: string, path: string): Observable<{ path: string; content: string }> {
    return this.http.get<{ path: string; content: string }>(
      `${this.api}/projects/${projectId}/file?path=${encodeURIComponent(path)}`
    );
  }

  chat(projectId: string, question: string, currentPath: string): Observable<ChatResponse> {
    return this.http.post<ChatResponse>(`${this.ai}/chat`, {
      project_id: projectId,
      question,
      current_path: currentPath,
    });
  }
}
