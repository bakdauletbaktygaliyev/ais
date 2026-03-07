import { Injectable, inject } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from '@env/environment';
import {
    Repo, GraphView, NodeDetail, GraphMetrics,
    CycleResult, ShortestPathResult, SearchResult, GraphNode
} from '../models';

@Injectable({ providedIn: 'root' })
export class ApiService {
    private readonly http = inject(HttpClient);
    private readonly baseUrl = environment.apiUrl;

    // ---------------------------------------------------------------------------
    // Repository
    // ---------------------------------------------------------------------------

    submitRepo(url: string): Observable<{ id: string; url: string; status: string }> {
        return this.http.post<{ id: string; url: string; status: string }>(
            `${this.baseUrl}/repos`,
            { url }
        );
    }

    getRepo(repoId: string): Observable<Repo> {
        return this.http.get<Repo>(`${this.baseUrl}/repos/${repoId}`);
    }

    // ---------------------------------------------------------------------------
    // Graph navigation
    // ---------------------------------------------------------------------------

    getRootGraph(repoId: string): Observable<GraphView> {
        return this.http.get<GraphView>(`${this.baseUrl}/repos/${repoId}/graph`);
    }

    getNodeChildren(repoId: string, nodeId: string): Observable<GraphView> {
        return this.http.get<GraphView>(`${this.baseUrl}/repos/${repoId}/nodes/${nodeId}`);
    }

    getNodeDetail(repoId: string, nodeId: string): Observable<NodeDetail> {
        return this.http.get<NodeDetail>(`${this.baseUrl}/repos/${repoId}/nodes/${nodeId}/detail`);
    }

    getFileSource(repoId: string, fileId: string): Observable<{ source: string; fileId: string }> {
        return this.http.get<{ source: string; fileId: string }>(
            `${this.baseUrl}/repos/${repoId}/files/${fileId}`
        );
    }

    // ---------------------------------------------------------------------------
    // Analysis queries
    // ---------------------------------------------------------------------------

    getMetrics(repoId: string): Observable<GraphMetrics> {
        return this.http.get<GraphMetrics>(`${this.baseUrl}/repos/${repoId}/metrics`);
    }

    getCycles(repoId: string): Observable<{ cycles: CycleResult[]; count: number }> {
        return this.http.get<{ cycles: CycleResult[]; count: number }>(
            `${this.baseUrl}/repos/${repoId}/cycles`
        );
    }

    getShortestPath(repoId: string, fromId: string, toId: string): Observable<ShortestPathResult> {
        const params = new HttpParams().set('from', fromId).set('to', toId);
        return this.http.get<ShortestPathResult>(`${this.baseUrl}/repos/${repoId}/path`, { params });
    }

    getTopFanIn(repoId: string, limit = 10): Observable<{ nodes: GraphNode[] }> {
        const params = new HttpParams().set('limit', limit.toString());
        return this.http.get<{ nodes: GraphNode[] }>(
            `${this.baseUrl}/repos/${repoId}/top-fan-in`, { params }
        );
    }

    getTopFanOut(repoId: string, limit = 10): Observable<{ nodes: GraphNode[] }> {
        const params = new HttpParams().set('limit', limit.toString());
        return this.http.get<{ nodes: GraphNode[] }>(
            `${this.baseUrl}/repos/${repoId}/top-fan-out`, { params }
        );
    }

    searchSimilar(repoId: string, query: string, topK = 5): Observable<{ results: SearchResult[]; count: number }> {
        const params = new HttpParams().set('q', query).set('topK', topK.toString());
        return this.http.get<{ results: SearchResult[]; count: number }>(
            `${this.baseUrl}/repos/${repoId}/search`, { params }
        );
    }
}
