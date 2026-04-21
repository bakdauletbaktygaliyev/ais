import {
  Component, Input, Output, EventEmitter, OnChanges, SimpleChanges,
  ElementRef, ViewChild, AfterViewInit, NgZone, OnDestroy
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { GraphData, GraphNode, GraphEdge } from '../../models/project.model';
import { langColor, nodeRadius } from './graph.utils';
import * as d3 from 'd3';

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
  @Output() drillDown = new EventEmitter<GraphNode>();
  @Output() fileSelect = new EventEmitter<GraphNode>();

  @ViewChild('svgContainer') svgContainer!: ElementRef<HTMLDivElement>;

  tooltip = { visible: false, x: 0, y: 0, node: null as GraphNode | null };
  private simulation: d3.Simulation<any, any> | null = null;
  private initialized = false;
  private resizeObserver: ResizeObserver | null = null;

  constructor(private zone: NgZone) {}

  ngAfterViewInit() {
    this.initialized = true;
    this.resizeObserver = new ResizeObserver(() => {
      this.zone.run(() => this.renderGraph());
    });
    this.resizeObserver.observe(this.svgContainer.nativeElement);
    this.renderGraph();
  }

  ngOnChanges(changes: SimpleChanges) {
    if (this.initialized && (changes['graph'] || changes['currentPath'])) {
      this.renderGraph();
    }
  }

  ngOnDestroy() {
    this.simulation?.stop();
    this.resizeObserver?.disconnect();
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
    const edges: (GraphEdge & d3.SimulationLinkDatum<any>)[] = this.graph.edges
      .filter(e => nodeById.has(e.source as string) && nodeById.has(e.target as string))
      .map(e => ({ ...e }));

    // Track which nodes have connections
    const connectedIds = new Set<string>();
    edges.forEach(e => {
      connectedIds.add(e.source as string);
      connectedIds.add(e.target as string);
    });

    const svg = d3.select(container).append('svg')
      .attr('width', width)
      .attr('height', height)
      .style('background', 'var(--bg)');

    // Defs: arrow marker
    const defs = svg.append('defs');
    defs.append('marker')
      .attr('id', 'arrow')
      .attr('viewBox', '0 -4 10 8')
      .attr('refX', 20)
      .attr('refY', 0)
      .attr('markerWidth', 8)
      .attr('markerHeight', 8)
      .attr('orient', 'auto')
      .append('path')
      .attr('d', 'M0,-4L10,0L0,4')
      .attr('fill', '#58a6ff')
      .attr('opacity', 0.7);

    const g = svg.append('g');

    // Zoom + pan
    const zoom = d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.05, 6])
      .on('zoom', (event) => g.attr('transform', event.transform.toString()));
    svg.call(zoom);
    svg.on('dblclick.zoom', null);

    // Add highlighted arrow marker
    defs.append('marker')
      .attr('id', 'arrow-active')
      .attr('viewBox', '0 -4 10 8')
      .attr('refX', 20)
      .attr('refY', 0)
      .attr('markerWidth', 8)
      .attr('markerHeight', 8)
      .attr('orient', 'auto')
      .append('path')
      .attr('d', 'M0,-4L10,0L0,4')
      .attr('fill', '#f0883e');

    // Edges
    const link = g.append('g').attr('class', 'links')
      .selectAll('line')
      .data(edges)
      .join('line')
      .attr('stroke', '#58a6ff')
      .attr('stroke-width', 1.5)
      .attr('stroke-opacity', 0.45)
      .attr('marker-end', 'url(#arrow)');

    // Selection state
    let selectedId: string | null = null;

    const resetSelection = () => {
      selectedId = null;
      link
        .attr('stroke', '#58a6ff')
        .attr('stroke-width', 1.5)
        .attr('stroke-opacity', 0.45)
        .attr('marker-end', 'url(#arrow)');
      nodeG.select('circle.main-circle')
        .attr('stroke-opacity', (d: any) => isConnected(d) ? 1 : 0.4)
        .attr('fill-opacity', (d: any) => d.type === 'directory' ? 0.2 : isConnected(d) ? 0.9 : 0.5);
      nodeG.select('text.node-label')
        .attr('fill', (d: any) => isConnected(d) ? '#c9d1d9' : '#6e7681');
    };

    svg.on('click', (event) => {
      if (event.target === svg.node() || (event.target as Element).tagName === 'svg') {
        resetSelection();
      }
    });

    // Node groups
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

    const radius = nodeRadius;

    const isConnected = (d: GraphNode) => connectedIds.has(d.id);

    // Shadow / glow for connected nodes
    nodeG.filter(d => isConnected(d))
      .append('circle')
      .attr('r', d => radius(d) + 6)
      .attr('fill', 'none')
      .attr('stroke', d => langColor(d.language, d.type))
      .attr('stroke-width', 1)
      .attr('stroke-opacity', 0.25);

    // Main circle
    nodeG.append('circle')
      .attr('class', 'main-circle')
      .attr('r', radius)
      .attr('fill', d => langColor(d.language, d.type))
      .attr('fill-opacity', d => {
        if (d.type === 'directory') return 0.2;
        return isConnected(d) ? 0.9 : 0.5;
      })
      .attr('stroke', d => langColor(d.language, d.type))
      .attr('stroke-width', d => d.type === 'directory' ? 2 : 1.5)
      .attr('stroke-opacity', d => isConnected(d) ? 1 : 0.4);

    // Directory icon
    nodeG.filter(d => d.type === 'directory')
      .append('text')
      .attr('text-anchor', 'middle')
      .attr('dy', '0.35em')
      .attr('font-size', 13)
      .attr('pointer-events', 'none')
      .text('📁');

    // Labels
    nodeG.append('text')
      .attr('class', 'node-label')
      .attr('text-anchor', 'middle')
      .attr('dy', d => radius(d) + 14)
      .attr('font-size', d => d.type === 'directory' ? 12 : 11)
      .attr('font-weight', d => d.type === 'directory' ? '600' : '400')
      .attr('fill', d => isConnected(d) ? '#c9d1d9' : '#6e7681')
      .attr('pointer-events', 'none')
      .text(d => d.name.length > 22 ? d.name.slice(0, 20) + '…' : d.name);

    // Interaction
    nodeG
      .on('mouseover', (event, d) => {
        this.zone.run(() => {
          const rect = container.getBoundingClientRect();
          this.tooltip = {
            visible: true,
            x: event.clientX - rect.left + 14,
            y: event.clientY - rect.top - 10,
            node: d
          };
        });
      })
      .on('mousemove', (event) => {
        const rect = container.getBoundingClientRect();
        this.zone.run(() => {
          this.tooltip.x = event.clientX - rect.left + 14;
          this.tooltip.y = event.clientY - rect.top - 10;
        });
      })
      .on('mouseout', () => {
        this.zone.run(() => { this.tooltip.visible = false; });
      })
      .on('click', (event, d) => {
        event.stopPropagation();

        if (selectedId === d.id) {
          resetSelection();
          return;
        }
        selectedId = d.id;

        // Find directly connected node IDs
        const neighbors = new Set<string>([d.id]);
        edges.forEach((e: any) => {
          const srcId = typeof e.source === 'object' ? e.source.id : e.source;
          const tgtId = typeof e.target === 'object' ? e.target.id : e.target;
          if (srcId === d.id) neighbors.add(tgtId);
          if (tgtId === d.id) neighbors.add(srcId);
        });

        // Highlight connected edges, dim others
        link
          .attr('stroke', (e: any) => {
            const srcId = typeof e.source === 'object' ? e.source.id : e.source;
            const tgtId = typeof e.target === 'object' ? e.target.id : e.target;
            if (srcId === d.id) return '#f0883e';   // outgoing: orange
            if (tgtId === d.id) return '#3fb950';   // incoming: green
            return '#30363d';
          })
          .attr('stroke-width', (e: any) => {
            const srcId = typeof e.source === 'object' ? e.source.id : e.source;
            const tgtId = typeof e.target === 'object' ? e.target.id : e.target;
            return (srcId === d.id || tgtId === d.id) ? 2.5 : 1;
          })
          .attr('stroke-opacity', (e: any) => {
            const srcId = typeof e.source === 'object' ? e.source.id : e.source;
            const tgtId = typeof e.target === 'object' ? e.target.id : e.target;
            return (srcId === d.id || tgtId === d.id) ? 0.9 : 0.08;
          })
          .attr('marker-end', (e: any) => {
            const srcId = typeof e.source === 'object' ? e.source.id : e.source;
            return srcId === d.id ? 'url(#arrow-active)' : 'url(#arrow)';
          });

        // Highlight neighbor nodes, dim others
        nodeG.select('circle.main-circle')
          .attr('fill-opacity', (n: any) => {
            if (neighbors.has(n.id)) return n.type === 'directory' ? 0.35 : 1;
            return 0.1;
          })
          .attr('stroke-opacity', (n: any) => neighbors.has(n.id) ? 1 : 0.15);

        nodeG.select('text.node-label')
          .attr('fill', (n: any) => neighbors.has(n.id) ? '#e6edf3' : '#3d444d');
      })
      .on('dblclick', (event, d) => {
        event.stopPropagation();
        if (d.type === 'directory') {
          this.zone.run(() => this.drillDown.emit(d));
        } else {
          this.zone.run(() => this.fileSelect.emit(d));
        }
      });

    // Force simulation
    const linkForce = d3.forceLink<any, any>(edges)
      .id((d: any) => d.id)
      .distance(160)
      .strength(0.6);

    this.simulation = d3.forceSimulation(nodes)
      .force('link', linkForce)
      .force('charge', d3.forceManyBody().strength(-600).distanceMax(500))
      .force('center', d3.forceCenter(width / 2, height / 2).strength(0.05))
      .force('x', d3.forceX(width / 2).strength(0.06))
      .force('y', d3.forceY(height / 2).strength(0.06))
      .force('collision', d3.forceCollide().radius((d: any) => radius(d) + 22))
      .alphaDecay(0.02)
      .on('tick', () => {
        link
          .attr('x1', (d: any) => d.source.x)
          .attr('y1', (d: any) => d.source.y)
          .attr('x2', (d: any) => {
            const dx = d.target.x - d.source.x;
            const dy = d.target.y - d.source.y;
            const dist = Math.sqrt(dx * dx + dy * dy) || 1;
            return d.target.x - (dx / dist) * radius(d.target);
          })
          .attr('y2', (d: any) => {
            const dx = d.target.x - d.source.x;
            const dy = d.target.y - d.source.y;
            const dist = Math.sqrt(dx * dx + dy * dy) || 1;
            return d.target.y - (dy / dist) * radius(d.target);
          });
        nodeG.attr('transform', (d: any) => `translate(${d.x ?? 0},${d.y ?? 0})`);
      });
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
