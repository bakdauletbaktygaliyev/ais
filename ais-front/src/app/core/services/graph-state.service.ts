import { Injectable, signal, computed } from '@angular/core';
import { BehaviorSubject } from 'rxjs';
import {
    GraphNode, GraphView, NodeDetail,
    BreadcrumbItem, Repo, GraphMetrics
} from '../models';

@Injectable({ providedIn: 'root' })
export class GraphStateService {
    // Current repository
    private _repo = new BehaviorSubject<Repo | null>(null);
    readonly repo$ = this._repo.asObservable();

    // Current graph view
    private _currentView = new BehaviorSubject<GraphView | null>(null);
    readonly currentView$ = this._currentView.asObservable();

    // Selected node (for detail panel)
    private _selectedNode = new BehaviorSubject<GraphNode | null>(null);
    readonly selectedNode$ = this._selectedNode.asObservable();

    // Node detail (loaded on selection)
    private _nodeDetail = new BehaviorSubject<NodeDetail | null>(null);
    readonly nodeDetail$ = this._nodeDetail.asObservable();

    // Breadcrumb navigation trail
    private _breadcrumbs = new BehaviorSubject<BreadcrumbItem[]>([]);
    readonly breadcrumbs$ = this._breadcrumbs.asObservable();

    // Graph metrics
    private _metrics = new BehaviorSubject<GraphMetrics | null>(null);
    readonly metrics$ = this._metrics.asObservable();

    // Loading states
    private _graphLoading = new BehaviorSubject<boolean>(false);
    readonly graphLoading$ = this._graphLoading.asObservable();

    private _detailLoading = new BehaviorSubject<boolean>(false);
    readonly detailLoading$ = this._detailLoading.asObservable();

    // ---------------------------------------------------------------------------
    // Mutations
    // ---------------------------------------------------------------------------

    setRepo(repo: Repo): void {
        this._repo.next(repo);
    }

    setCurrentView(view: GraphView): void {
        this._currentView.next(view);
    }

    setSelectedNode(node: GraphNode | null): void {
        this._selectedNode.next(node);
    }

    setNodeDetail(detail: NodeDetail | null): void {
        this._nodeDetail.next(detail);
    }

    setMetrics(metrics: GraphMetrics): void {
        this._metrics.next(metrics);
    }

    setGraphLoading(loading: boolean): void {
        this._graphLoading.next(loading);
    }

    setDetailLoading(loading: boolean): void {
        this._detailLoading.next(loading);
    }

    // ---------------------------------------------------------------------------
    // Breadcrumb navigation
    // ---------------------------------------------------------------------------

    pushBreadcrumb(item: BreadcrumbItem): void {
        const current = this._breadcrumbs.value;
        // Avoid duplicate
        if (current.length > 0 && current[current.length - 1].nodeId === item.nodeId) {
            return;
        }
        this._breadcrumbs.next([...current, item]);
    }

    popToBreadcrumb(nodeId: string): BreadcrumbItem[] {
        const current = this._breadcrumbs.value;
        const idx = current.findIndex((b) => b.nodeId === nodeId);
        if (idx === -1) return current;
        const trimmed = current.slice(0, idx + 1);
        this._breadcrumbs.next(trimmed);
        return trimmed;
    }

    resetBreadcrumbs(root?: BreadcrumbItem): void {
        this._breadcrumbs.next(root ? [root] : []);
    }

    // ---------------------------------------------------------------------------
    // Helpers
    // ---------------------------------------------------------------------------

    getCurrentRepo(): Repo | null {
        return this._repo.value;
    }

    getCurrentView(): GraphView | null {
        return this._currentView.value;
    }

    getSelectedNode(): GraphNode | null {
        return this._selectedNode.value;
    }

    getBreadcrumbs(): BreadcrumbItem[] {
        return this._breadcrumbs.value;
    }

    reset(): void {
        this._repo.next(null);
        this._currentView.next(null);
        this._selectedNode.next(null);
        this._nodeDetail.next(null);
        this._breadcrumbs.next([]);
        this._metrics.next(null);
    }
}