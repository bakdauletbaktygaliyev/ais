import {
  Component, Input, Output, EventEmitter, OnChanges, SimpleChanges,
  ElementRef, ViewChild, AfterViewInit, NgZone, OnDestroy, ChangeDetectorRef
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { GraphData, GraphNode, GraphEdge } from '../../models/project.model';
import { langColor } from './graph.utils';
import * as d3 from 'd3';

interface LangStat {
  name: string;
  files: number;
  lines: number;
  color: string;
  pct: number;
}

interface ProjectStats {
  totalFiles: number;
  totalDirs: number;
  totalLines: number;
  totalDeps: number;
  cycleCount: number;
  languages: LangStat[];
  topFiles: { name: string; path: string; lines: number }[];
  deadCount: number;
}

const CARD_W = 168;
const CARD_H = 58;
const CARD_H_DIR = 68;
const CARD_RX = 10;
const ACCENT_W = 3;
const PAD_LEFT = 36;

function cardH(d: { type: string }): number {
  return d.type === 'directory' ? CARD_H_DIR : CARD_H;
}

// Returns the point on the rect centered at (tx,ty) that faces (sx,sy)
function rectEdgePoint(
  sx: number, sy: number,
  tx: number, ty: number,
  tw: number, th: number
): [number, number] {
  const ddx = sx - tx;
  const ddy = sy - ty;
  if (!ddx && !ddy) return [tx, ty];
  const hw = tw / 2;
  const hh = th / 2;
  const scaleX = ddx !== 0 ? hw / Math.abs(ddx) : Infinity;
  const scaleY = ddy !== 0 ? hh / Math.abs(ddy) : Infinity;
  const scale = Math.min(scaleX, scaleY);
  return [tx + ddx * scale, ty + ddy * scale];
}

@Component({
  selector: 'app-graph',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './graph.component.html',
  styleUrls: ['./graph.component.css']
})
export class GraphComponent implements OnChanges, AfterViewInit, OnDestroy {
  @Input() graph: GraphData = { nodes: [], edges: [] };
  @Input() currentPath = '';
  @Input() projectName = '';
  @Input() deadCodeIds = new Set<string>();
  @Output() drillDown = new EventEmitter<GraphNode>();
  @Output() fileSelect = new EventEmitter<GraphNode>();

  @ViewChild('svgContainer') svgContainer!: ElementRef<HTMLDivElement>;

  tooltip = { visible: false, x: 0, y: 0, node: null as GraphNode | null };
  showStats = false;
  showDeadCode = false;
  stats: ProjectStats | null = null;
  private cycleEdgeKeys = new Set<string>();
  private simulation: d3.Simulation<any, any> | null = null;
  private initialized = false;
  private resizeObserver: ResizeObserver | null = null;
  private nodeGSel: any = null;
  private connectedIdsSaved = new Set<string>();

  constructor(private zone: NgZone, private cdr: ChangeDetectorRef) {}

  get deadCodeCount(): number {
    return this.deadCodeIds.size;
  }

  get visibleDeadCodeNodes(): GraphNode[] {
    return this.graph.nodes.filter(n => n.type === 'file' && this.deadCodeIds.has(n.id));
  }

  ngAfterViewInit() {
    this.initialized = true;
    this.resizeObserver = new ResizeObserver(() => {
      this.zone.run(() => this.renderGraph());
    });
    this.resizeObserver.observe(this.svgContainer.nativeElement);
    this.renderGraph();
  }

  ngOnChanges(changes: SimpleChanges) {
    if (changes['graph']) this.computeStats();
    if (this.initialized && (changes['graph'] || changes['currentPath'])) {
      this.renderGraph();
    } else if (this.initialized && changes['deadCodeIds'] && this.showDeadCode) {
      this.applyDeadCodeStyles();
    }
  }

  ngOnDestroy() {
    this.simulation?.stop();
    this.resizeObserver?.disconnect();
  }

  private detectCycles(): void {
    this.cycleEdgeKeys.clear();
    const adj = new Map<string, string[]>();
    for (const e of this.graph.edges) {
      const src = e.source as string;
      const tgt = e.target as string;
      if (!adj.has(src)) adj.set(src, []);
      adj.get(src)!.push(tgt);
    }

    const visited = new Set<string>();
    const visiting = new Set<string>();

    const dfs = (node: string): void => {
      visiting.add(node);
      for (const neighbor of adj.get(node) ?? []) {
        if (visiting.has(neighbor)) {
          this.cycleEdgeKeys.add(`${node}->${neighbor}`);
        } else if (!visited.has(neighbor)) {
          dfs(neighbor);
        }
      }
      visiting.delete(node);
      visited.add(node);
    };

    const allNodes = new Set(
      this.graph.edges.flatMap(e => [e.source as string, e.target as string])
    );
    for (const node of allNodes) {
      if (!visited.has(node)) dfs(node);
    }
  }

  private computeStats() {
    this.detectCycles();
    const files = this.graph.nodes.filter(n => n.type === 'file');
    const dirs = this.graph.nodes.filter(n => n.type === 'directory');
    const totalLines = files.reduce((s, n) => s + (n.lines || 0), 0);

    const langMap = new Map<string, { files: number; lines: number }>();
    for (const n of files) {
      const lang = n.language || 'text';
      const prev = langMap.get(lang) ?? { files: 0, lines: 0 };
      langMap.set(lang, { files: prev.files + 1, lines: prev.lines + (n.lines || 0) });
    }
    const total = files.length || 1;
    const languages: LangStat[] = Array.from(langMap.entries())
      .map(([name, { files: f, lines }]) => ({
        name, files: f, lines,
        color: langColor(name, 'file'),
        pct: Math.round((f / total) * 100),
      }))
      .sort((a, b) => b.files - a.files)
      .slice(0, 7);

    const topFiles = [...files]
      .sort((a, b) => (b.lines || 0) - (a.lines || 0))
      .slice(0, 5)
      .map(n => ({ name: n.name, path: n.path, lines: n.lines || 0 }));

    this.stats = {
      totalFiles: files.length,
      totalDirs: dirs.length,
      totalLines,
      totalDeps: this.graph.edges.length,
      cycleCount: this.cycleEdgeKeys.size,
      languages,
      topFiles,
      deadCount: this.deadCodeCount,
    };
  }

  exportPng() {
    const container = this.svgContainer?.nativeElement;
    if (!container) return;
    const svgEl = container.querySelector('svg');
    if (!svgEl) return;

    const w = svgEl.clientWidth;
    const h = svgEl.clientHeight;
    const scale = 2;

    const serializer = new XMLSerializer();
    let svgStr = serializer.serializeToString(svgEl);
    if (!svgStr.includes('xmlns=')) {
      svgStr = svgStr.replace('<svg', '<svg xmlns="http://www.w3.org/2000/svg"');
    }

    const blob = new Blob([svgStr], { type: 'image/svg+xml;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const img = new Image();
    img.onload = () => {
      const canvas = document.createElement('canvas');
      canvas.width = w * scale;
      canvas.height = h * scale;
      const ctx = canvas.getContext('2d')!;
      ctx.scale(scale, scale);
      ctx.fillStyle = '#0d1117';
      ctx.fillRect(0, 0, w, h);
      ctx.drawImage(img, 0, 0, w, h);
      URL.revokeObjectURL(url);
      const a = document.createElement('a');
      a.download = `${this.projectName || 'graph'}.png`;
      a.href = canvas.toDataURL('image/png');
      a.click();
    };
    img.onerror = () => URL.revokeObjectURL(url);
    img.src = url;
  }

  toggleDeadCode() {
    this.showDeadCode = !this.showDeadCode;
    if (this.showDeadCode) this.showStats = false;
    this.applyDeadCodeStyles();
  }

  private applyDeadCodeStyles() {
    if (!this.nodeGSel) return;
    if (this.showDeadCode) {
      this.nodeGSel.select('rect.card-bg')
        .attr('stroke', (d: any) => this.deadCodeIds.has(d.id) ? '#d29922' : langColor(d.language, d.type))
        .attr('stroke-width', (d: any) => this.deadCodeIds.has(d.id) ? 2 : 1)
        .attr('stroke-opacity', (d: any) => this.deadCodeIds.has(d.id) ? 0.9 : 0.08)
        .attr('stroke-dasharray', (d: any) => this.deadCodeIds.has(d.id) ? '5 3' : 'none');
      this.nodeGSel.select('rect.card-accent')
        .attr('fill', (d: any) => this.deadCodeIds.has(d.id) ? '#d29922' : langColor(d.language, d.type))
        .attr('fill-opacity', (d: any) => this.deadCodeIds.has(d.id) ? 0.9 : 0.12);
      this.nodeGSel.select('text.card-name')
        .attr('fill', (d: any) => this.deadCodeIds.has(d.id) ? '#e3b341' : '#3d444d');
      this.nodeGSel.select('text.card-sub')
        .attr('fill', (d: any) => this.deadCodeIds.has(d.id) ? '#9e7200' : '#21262d');
      this.nodeGSel.select('circle')
        .attr('fill', (d: any) => this.deadCodeIds.has(d.id) ? '#d29922' : langColor(d.language, d.type));
    } else {
      const isConn = (d: any) => this.connectedIdsSaved.has(d.id);
      this.nodeGSel.select('rect.card-bg')
        .attr('stroke', (d: any) => langColor(d.language, d.type))
        .attr('stroke-width', 1)
        .attr('stroke-opacity', (d: any) => isConn(d) ? 0.55 : 0.2)
        .attr('stroke-dasharray', 'none');
      this.nodeGSel.select('rect.card-accent')
        .attr('fill', (d: any) => langColor(d.language, d.type))
        .attr('fill-opacity', (d: any) => isConn(d) ? 0.85 : 0.35);
      this.nodeGSel.select('text.card-name')
        .attr('fill', (d: any) => isConn(d) ? '#e6edf3' : '#8b949e');
      this.nodeGSel.select('text.card-sub')
        .attr('fill', '#6e7681');
      this.nodeGSel.select('circle')
        .attr('fill', (d: any) => langColor(d.language, d.type));
    }
  }

  private renderGraph() {
    const container = this.svgContainer?.nativeElement;
    if (!container) return;

    this.simulation?.stop();
    d3.select(container).selectAll('*').remove();

    const { width, height } = container.getBoundingClientRect();
    if (width === 0 || height === 0) return;

    const nodes: (GraphNode & d3.SimulationNodeDatum)[] = this.graph.nodes.map(n => ({ ...n }));

    if (nodes.length === 0) {
      this.renderEmpty(container, width, height);
      return;
    }

    // Build valid edges (only between nodes that exist)
    const nodeById = new Map(nodes.map(n => [n.id, n]));
    const edges: (GraphEdge & d3.SimulationLinkDatum<any> & { _key: string })[] = this.graph.edges
      .filter(e => nodeById.has(e.source as string) && nodeById.has(e.target as string))
      .map(e => ({ ...e, _key: `${e.source as string}->${e.target as string}` }));

    const isCycleEdge = (e: any): boolean => {
      const key = e._key ?? (() => {
        const s = typeof e.source === 'object' ? (e.source as any).id : e.source;
        const t = typeof e.target === 'object' ? (e.target as any).id : e.target;
        return `${s}->${t}`;
      })();
      return this.cycleEdgeKeys.has(key);
    };

    // Track which nodes have connections
    const connectedIds = new Set<string>();
    edges.forEach(e => {
      connectedIds.add(e.source as string);
      connectedIds.add(e.target as string);
    });
    this.connectedIdsSaved = connectedIds;

    const svg = d3.select(container).append('svg')
      .attr('width', width)
      .attr('height', height)
      .style('background', 'var(--bg)');

    // Defs
    const defs = svg.append('defs');

    // Drop shadow for cards
    const shadow = defs.append('filter')
      .attr('id', 'card-shadow')
      .attr('x', '-20%').attr('y', '-20%')
      .attr('width', '140%').attr('height', '140%');
    shadow.append('feDropShadow')
      .attr('dx', 0).attr('dy', 3)
      .attr('stdDeviation', 5)
      .attr('flood-color', 'rgba(0,0,0,0.55)');

    // Arrows
    defs.append('marker')
      .attr('id', 'arrow')
      .attr('viewBox', '0 -4 10 8')
      .attr('refX', 10).attr('refY', 0)
      .attr('markerWidth', 7).attr('markerHeight', 7)
      .attr('orient', 'auto')
      .append('path').attr('d', 'M0,-4L10,0L0,4')
      .attr('fill', '#58a6ff').attr('opacity', 0.65);

    defs.append('marker')
      .attr('id', 'arrow-out')
      .attr('viewBox', '0 -4 10 8')
      .attr('refX', 10).attr('refY', 0)
      .attr('markerWidth', 7).attr('markerHeight', 7)
      .attr('orient', 'auto')
      .append('path').attr('d', 'M0,-4L10,0L0,4')
      .attr('fill', '#f0883e');

    defs.append('marker')
      .attr('id', 'arrow-in')
      .attr('viewBox', '0 -4 10 8')
      .attr('refX', 10).attr('refY', 0)
      .attr('markerWidth', 7).attr('markerHeight', 7)
      .attr('orient', 'auto')
      .append('path').attr('d', 'M0,-4L10,0L0,4')
      .attr('fill', '#3fb950');

    defs.append('marker')
      .attr('id', 'arrow-cycle')
      .attr('viewBox', '0 -4 10 8')
      .attr('refX', 10).attr('refY', 0)
      .attr('markerWidth', 7).attr('markerHeight', 7)
      .attr('orient', 'auto')
      .append('path').attr('d', 'M0,-4L10,0L0,4')
      .attr('fill', '#f85149');

    const g = svg.append('g');

    const zoom = d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.05, 6])
      .on('zoom', event => g.attr('transform', event.transform.toString()));
    svg.call(zoom);
    svg.on('dblclick.zoom', null);

    // Edges — cycle edges in red, others dashed blue
    const link = g.append('g').attr('class', 'links')
      .selectAll('line')
      .data(edges)
      .join('line')
      .attr('stroke', (e: any) => isCycleEdge(e) ? '#f85149' : '#58a6ff')
      .attr('stroke-width', (e: any) => isCycleEdge(e) ? 2 : 1.5)
      .attr('stroke-opacity', (e: any) => isCycleEdge(e) ? 0.75 : 0.35)
      .attr('stroke-dasharray', (e: any) => isCycleEdge(e) ? 'none' : '5 4')
      .attr('marker-end', (e: any) => isCycleEdge(e) ? 'url(#arrow-cycle)' : 'url(#arrow)');

    let selectedId: string | null = null;

    const isConnected = (d: GraphNode) => connectedIds.has(d.id);

    const resetSelection = () => {
      selectedId = null;
      link
        .attr('stroke', (e: any) => isCycleEdge(e) ? '#f85149' : '#58a6ff')
        .attr('stroke-width', (e: any) => isCycleEdge(e) ? 2 : 1.5)
        .attr('stroke-opacity', (e: any) => isCycleEdge(e) ? 0.75 : 0.35)
        .attr('stroke-dasharray', (e: any) => isCycleEdge(e) ? 'none' : '5 4')
        .attr('marker-end', (e: any) => isCycleEdge(e) ? 'url(#arrow-cycle)' : 'url(#arrow)');
      nodeG.select('rect.card-bg')
        .attr('stroke-opacity', (d: any) => isConnected(d) ? 0.55 : 0.2)
        .attr('fill-opacity', 1);
      nodeG.select('rect.card-accent')
        .attr('fill-opacity', (d: any) => isConnected(d) ? 0.85 : 0.35);
      nodeG.select('text.card-name')
        .attr('fill', (d: any) => isConnected(d) ? '#e6edf3' : '#8b949e');
      nodeG.select('text.card-sub')
        .attr('fill', '#6e7681');
    };

    svg.on('click', event => {
      if (event.target === svg.node() || (event.target as Element).tagName === 'svg') {
        resetSelection();
      }
    });

    // Node groups — saved for dead code highlight updates
    const nodeG = g.append('g').attr('class', 'nodes')
      .selectAll<SVGGElement, typeof nodes[0]>('g')
      .data(nodes)
      .join('g')
      .attr('cursor', 'pointer')
      .call(
        d3.drag<SVGGElement, typeof nodes[0]>()
          .on('start', (event, d) => {
            if (!event.active) this.simulation?.alphaTarget(0.3).restart();
            d.fx = d.x; d.fy = d.y;
          })
          .on('drag', (event, d) => { d.fx = event.x; d.fy = event.y; })
          .on('end', (event, d) => {
            if (!event.active) this.simulation?.alphaTarget(0);
            d.fx = null; d.fy = null;
          })
      );

    // Card background
    nodeG.append('rect')
      .attr('class', 'card-bg')
      .attr('x', -CARD_W / 2)
      .attr('y', (d: any) => -cardH(d) / 2)
      .attr('width', CARD_W)
      .attr('height', (d: any) => cardH(d))
      .attr('rx', CARD_RX).attr('ry', CARD_RX)
      .attr('fill', '#161b22')
      .attr('stroke', (d: any) => langColor(d.language, d.type))
      .attr('stroke-width', 1)
      .attr('stroke-opacity', (d: any) => isConnected(d) ? 0.55 : 0.2)
      .attr('filter', 'url(#card-shadow)');

    // Left color accent bar
    nodeG.append('rect')
      .attr('class', 'card-accent')
      .attr('x', -CARD_W / 2)
      .attr('y', (d: any) => -cardH(d) / 2 + CARD_RX)
      .attr('width', ACCENT_W)
      .attr('height', (d: any) => cardH(d) - CARD_RX * 2)
      .attr('rx', 1.5)
      .attr('fill', (d: any) => langColor(d.language, d.type))
      .attr('fill-opacity', (d: any) => isConnected(d) ? 0.85 : 0.35);

    // Directory icon
    nodeG.filter((d: any) => d.type === 'directory')
      .append('text')
      .attr('x', -CARD_W / 2 + PAD_LEFT - 18)
      .attr('y', -4).attr('dy', '0.35em')
      .attr('font-size', 14).attr('pointer-events', 'none')
      .text('📁');

    // File language dot
    nodeG.filter((d: any) => d.type === 'file')
      .append('circle')
      .attr('cx', -CARD_W / 2 + PAD_LEFT - 16)
      .attr('cy', -9).attr('r', 4.5)
      .attr('fill', (d: any) => langColor(d.language, d.type))
      .attr('fill-opacity', 0.9).attr('pointer-events', 'none');

    // Node name
    nodeG.append('text')
      .attr('class', 'card-name')
      .attr('x', -CARD_W / 2 + PAD_LEFT)
      .attr('y', (d: any) => d.type === 'file' ? -7 : -1)
      .attr('dy', '0.35em')
      .attr('font-size', 12).attr('font-weight', '600')
      .attr('font-family', "'JetBrains Mono', 'Fira Code', monospace")
      .attr('fill', (d: any) => isConnected(d) ? '#e6edf3' : '#8b949e')
      .attr('pointer-events', 'none')
      .text((d: any) => d.name.length > 17 ? d.name.slice(0, 15) + '…' : d.name);

    // Subtitle
    nodeG.filter((d: any) => d.type === 'file')
      .append('text')
      .attr('class', 'card-sub')
      .attr('x', -CARD_W / 2 + PAD_LEFT)
      .attr('y', 10).attr('dy', '0.35em')
      .attr('font-size', 10)
      .attr('font-family', "'JetBrains Mono', 'Fira Code', monospace")
      .attr('fill', '#6e7681').attr('pointer-events', 'none')
      .text((d: any) => `${d.language || 'file'} · ${d.lines ?? 0} lines`);

    nodeG.filter((d: any) => d.type === 'directory')
      .append('text')
      .attr('class', 'card-sub')
      .attr('x', -CARD_W / 2 + PAD_LEFT)
      .attr('y', 16).attr('dy', '0.35em')
      .attr('font-size', 10)
      .attr('font-family', "'JetBrains Mono', 'Fira Code', monospace")
      .attr('fill', '#6e7681').attr('pointer-events', 'none')
      .text((d: any) => `${d.children ?? 0} items · double-click`);

    // Interactions
    nodeG
      .on('mouseover', (event, d) => {
        this.zone.run(() => {
          const rect = container.getBoundingClientRect();
          this.tooltip = { visible: true, x: event.clientX - rect.left + 14, y: event.clientY - rect.top - 10, node: d };
        });
        d3.select(event.currentTarget as SVGGElement).select('rect.card-bg').attr('fill', '#1c2128');
      })
      .on('mousemove', event => {
        const rect = container.getBoundingClientRect();
        this.zone.run(() => { this.tooltip.x = event.clientX - rect.left + 14; this.tooltip.y = event.clientY - rect.top - 10; });
      })
      .on('mouseout', event => {
        this.zone.run(() => { this.tooltip.visible = false; });
        d3.select(event.currentTarget as SVGGElement).select('rect.card-bg').attr('fill', '#161b22');
      })
      .on('click', (event, d) => {
        event.stopPropagation();
        if (selectedId === d.id) { resetSelection(); return; }
        selectedId = d.id;

        const neighbors = new Set<string>([d.id]);
        edges.forEach((e: any) => {
          const srcId = typeof e.source === 'object' ? e.source.id : e.source;
          const tgtId = typeof e.target === 'object' ? e.target.id : e.target;
          if (srcId === d.id) neighbors.add(tgtId);
          if (tgtId === d.id) neighbors.add(srcId);
        });

        link
          .attr('stroke', (e: any) => {
            const srcId = typeof e.source === 'object' ? e.source.id : e.source;
            const tgtId = typeof e.target === 'object' ? e.target.id : e.target;
            if (srcId === d.id) return '#f0883e';
            if (tgtId === d.id) return '#3fb950';
            if (isCycleEdge(e)) return '#f85149';
            return '#21262d';
          })
          .attr('stroke-width', (e: any) => {
            const srcId = typeof e.source === 'object' ? e.source.id : e.source;
            const tgtId = typeof e.target === 'object' ? e.target.id : e.target;
            return (srcId === d.id || tgtId === d.id) ? 2 : 1.5;
          })
          .attr('stroke-opacity', (e: any) => {
            const srcId = typeof e.source === 'object' ? e.source.id : e.source;
            const tgtId = typeof e.target === 'object' ? e.target.id : e.target;
            if (srcId === d.id || tgtId === d.id) return 0.9;
            if (isCycleEdge(e)) return 0.4;
            return 0.07;
          })
          .attr('stroke-dasharray', (e: any) => {
            const srcId = typeof e.source === 'object' ? e.source.id : e.source;
            const tgtId = typeof e.target === 'object' ? e.target.id : e.target;
            return (srcId === d.id || tgtId === d.id || isCycleEdge(e)) ? 'none' : '5 4';
          })
          .attr('marker-end', (e: any) => {
            const srcId = typeof e.source === 'object' ? e.source.id : e.source;
            const tgtId = typeof e.target === 'object' ? e.target.id : e.target;
            if (srcId === d.id) return 'url(#arrow-out)';
            if (tgtId === d.id) return 'url(#arrow-in)';
            if (isCycleEdge(e)) return 'url(#arrow-cycle)';
            return 'url(#arrow)';
          });

        nodeG.select('rect.card-bg')
          .attr('stroke-opacity', (n: any) => neighbors.has(n.id) ? 0.9 : 0.08)
          .attr('fill-opacity', (n: any) => neighbors.has(n.id) ? 1 : 0.35);
        nodeG.select('rect.card-accent')
          .attr('fill-opacity', (n: any) => neighbors.has(n.id) ? 1 : 0.1);
        nodeG.select('text.card-name')
          .attr('fill', (n: any) => neighbors.has(n.id) ? '#e6edf3' : '#3d444d');
        nodeG.select('text.card-sub')
          .attr('fill', (n: any) => neighbors.has(n.id) ? '#6e7681' : '#21262d');
      })
      .on('dblclick', (event, d) => {
        event.stopPropagation();
        if (d.type === 'directory') {
          this.zone.run(() => this.drillDown.emit(d));
        } else {
          this.zone.run(() => this.fileSelect.emit(d));
        }
      });

    // Force simulation — wider spacing for card layout
    const linkForce = d3.forceLink<any, any>(edges)
      .id((d: any) => d.id)
      .distance(240)
      .strength(0.5);

    this.simulation = d3.forceSimulation(nodes)
      .force('link', linkForce)
      .force('charge', d3.forceManyBody().strength(-900).distanceMax(600))
      .force('center', d3.forceCenter(width / 2, height / 2).strength(0.05))
      .force('x', d3.forceX(width / 2).strength(0.06))
      .force('y', d3.forceY(height / 2).strength(0.06))
      .force('collision', d3.forceCollide().radius(CARD_W / 2 + 24))
      .alphaDecay(0.02)
      .on('tick', () => {
        link
          .attr('x1', (e: any) => rectEdgePoint(e.target.x, e.target.y, e.source.x, e.source.y, CARD_W, cardH(e.source))[0])
          .attr('y1', (e: any) => rectEdgePoint(e.target.x, e.target.y, e.source.x, e.source.y, CARD_W, cardH(e.source))[1])
          .attr('x2', (e: any) => rectEdgePoint(e.source.x, e.source.y, e.target.x, e.target.y, CARD_W, cardH(e.target))[0])
          .attr('y2', (e: any) => rectEdgePoint(e.source.x, e.source.y, e.target.x, e.target.y, CARD_W, cardH(e.target))[1]);
        nodeG.attr('transform', (d: any) => `translate(${d.x ?? 0},${d.y ?? 0})`);
      });

    this.nodeGSel = nodeG;
    if (this.showDeadCode) this.applyDeadCodeStyles();
  }

  private renderEmpty(container: HTMLElement, width: number, height: number) {
    const svg = d3.select(container).append('svg').attr('width', width).attr('height', height);
    svg.append('text')
      .attr('x', width / 2).attr('y', height / 2)
      .attr('text-anchor', 'middle')
      .attr('fill', '#6e7681').attr('font-size', 16)
      .text('No nodes to display');
  }
}
