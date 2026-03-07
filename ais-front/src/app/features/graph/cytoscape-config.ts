import { GraphView, GraphNode, GraphEdge } from '../../core/models';

// ---------------------------------------------------------------------------
// Element builders
// ---------------------------------------------------------------------------

export function buildCytoscapeElements(view: GraphView): cytoscape.ElementDefinition[] {
    const elements: cytoscape.ElementDefinition[] = [];

    for (const node of view.nodes) {
        elements.push({
            group: 'nodes',
            data: {
                ...node,
                // Normalize size: min 20, max 80 based on connectivity
                nodeSize: computeNodeSize(node),
                nodeColor: getNodeColor(node),
            },
        });
    }

    for (const edge of view.edges) {
        elements.push({
            group: 'edges',
            data: {
                source: edge.sourceId,
                target: edge.targetId,
                ...edge,
            },
        });
    }

    return elements;
}

function computeNodeSize(node: GraphNode): number {
    const connectivity = (node.fanIn || 0) + (node.fanOut || 0);
    if (node.type === 'Dir') return 50 + Math.min(connectivity * 3, 40);
    if (node.type === 'File') return 30 + Math.min(connectivity * 2, 35);
    if (node.type === 'Function' || node.type === 'Class') return 24;
    return 40;
}

function getNodeColor(node: GraphNode): string {
    if (node.hasCycle) return '#ef4444';

    switch (node.type) {
        case 'Repo': return '#6366f1';
        case 'Dir': return '#f59e0b';
        case 'File': return '#6366f1';
        case 'Function': return '#22c55e';
        case 'Class': return '#ec4899';
        default: return '#64748b';
    }
}

// ---------------------------------------------------------------------------
// Cytoscape style
// ---------------------------------------------------------------------------

export function getCytoscapeStyle(): any[] {
    return [
        {
            selector: 'node',
            style: {
                'background-color': 'data(nodeColor)',
                'width': 'data(nodeSize)',
                'height': 'data(nodeSize)',
                'label': 'data(name)',
                'color': '#e2e8f0',
                'font-size': 10,
                'font-family': '"JetBrains Mono", "Fira Mono", monospace',
                'text-valign': 'bottom',
                'text-halign': 'center',
                'text-margin-y': 6,
                'text-max-width': 120,
                'text-overflow-wrap': 'ellipsis',
                'border-width': 1.5,
                'border-color': 'data(nodeColor)',
                'border-opacity': 0.3,
                'transition-property': 'opacity background-color border-color',
                'transition-duration': '0.15s',
                'overlay-padding': 6,
            } as any,
        },
        {
            selector: 'node[type = "Dir"]',
            style: {
                'shape': 'roundrectangle',
                'background-color': '#1e293b',
                'border-color': '#f59e0b',
                'border-width': 2,
                'color': '#fbbf24',
                'font-weight': 600,
            } as any,
        },
        {
            selector: 'node[type = "File"]',
            style: {
                'shape': 'ellipse',
                'background-color': '#1e1e3e',
                'border-color': '#6366f1',
                'color': '#c4b5fd',
            } as any,
        },
        {
            selector: 'node[hasCycle = true]',
            style: {
                'border-color': '#ef4444',
                'border-width': 3,
                'border-style': 'dashed',
            } as any,
        },
        {
            selector: 'node[hasChildren = true]',
            style: {
                'text-decoration': 'none',
                'background-image': 'none',
                'content': 'data(name)',
            } as any,
        },
        {
            selector: 'node.selected',
            style: {
                'border-width': 3,
                'border-color': '#a78bfa',
                'background-color': '#2d2d4e',
                'z-index': 10,
            } as any,
        },
        {
            selector: 'node.dimmed',
            style: {
                'opacity': 0.25,
            },
        },
        {
            selector: 'edge',
            style: {
                'width': 1.5,
                'line-color': '#334155',
                'target-arrow-color': '#334155',
                'target-arrow-shape': 'triangle',
                'curve-style': 'bezier',
                'arrow-scale': 0.8,
                'opacity': 0.7,
                'transition-property': 'opacity line-color',
                'transition-duration': '0.15s',
            } as any,
        },
        {
            selector: 'edge[type = "IMPORTS"]',
            style: {
                'line-color': '#4f46e5',
                'target-arrow-color': '#4f46e5',
                'width': 1.5,
            } as any,
        },
        {
            selector: 'edge[type = "CALLS"]',
            style: {
                'line-color': '#22c55e',
                'target-arrow-color': '#22c55e',
                'line-style': 'dashed',
                'width': 1,
            } as any,
        },
        {
            selector: 'edge[type = "EXTENDS"]',
            style: {
                'line-color': '#ec4899',
                'target-arrow-color': '#ec4899',
                'target-arrow-shape': 'triangle-hollow',
            } as any,
        },
        {
            selector: 'edge.dimmed',
            style: { 'opacity': 0.05 },
        },
    ];
}

// ---------------------------------------------------------------------------
// Layout configuration
// ---------------------------------------------------------------------------

export const layoutConfig: cytoscape.LayoutOptions = {
    name: 'cose',
    // @ts-ignore
    idealEdgeLength: 100,
    nodeOverlap: 20,
    refresh: 20,
    fit: true,
    padding: 40,
    randomize: false,
    componentSpacing: 100,
    nodeRepulsion: 400000,
    edgeElasticity: 100,
    nestingFactor: 5,
    gravity: 80,
    numIter: 1000,
    initialTemp: 200,
    coolingFactor: 0.95,
    minTemp: 1.0,
    animate: true,
    animationDuration: 500,
};

export const dagreLayoutConfig: cytoscape.LayoutOptions = {
    name: 'dagre',
    // @ts-ignore
    rankDir: 'TB',
    ranker: 'network-simplex',
    rankSep: 60,
    nodeSep: 30,
    padding: 40,
    animate: true,
    animationDuration: 400,
};