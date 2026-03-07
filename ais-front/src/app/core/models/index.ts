// ---------------------------------------------------------------------------
// Repository models
// ---------------------------------------------------------------------------

export type RepoStatus = 'pending' | 'analyzing' | 'ready' | 'error';
export type Language = 'typescript' | 'javascript' | 'go' | 'mixed' | 'unknown';
export type MonorepoType = 'none' | 'turborepo' | 'nx' | 'pnpm_workspaces' | 'yarn_workspaces' | 'go_modules';

export interface Repo {
    id: string;
    url: string;
    owner: string;
    name: string;
    description: string;
    status: RepoStatus;
    language: Language;
    monorepoType: MonorepoType;
    commitHash: string;
    fileCount: number;
    dirCount: number;
    functionCount: number;
    classCount: number;
    cycleCount: number;
    starCount: number;
    forkCount: number;
    sizeKB: number;
    errorMessage?: string;
    createdAt: string;
    updatedAt: string;
    readyAt?: string;
}

// ---------------------------------------------------------------------------
// Graph models
// ---------------------------------------------------------------------------

export type NodeType = 'Repo' | 'Dir' | 'File' | 'Function' | 'Class';
export type EdgeType =
    | 'HAS_ROOT' | 'HAS_CHILD' | 'HAS_FILE' | 'HAS_FUNCTION' | 'HAS_CLASS'
    | 'IMPORTS' | 'CALLS' | 'EXTENDS' | 'IMPLEMENTS';

export interface GraphNode {
    id: string;
    repoId: string;
    type: NodeType;
    name: string;
    path: string;
    fanIn: number;
    fanOut: number;
    hasCycle: boolean;
    hasChildren: boolean;
    startLine?: number;
    endLine?: number;
    language?: string;
    size?: number;
}

export interface GraphEdge {
    id: string;
    sourceId: string;
    targetId: string;
    type: EdgeType;
    line?: number;
}

export interface GraphView {
    nodes: GraphNode[];
    edges: GraphEdge[];
    parent?: GraphNode;
}

export interface NodeMetrics {
    fanIn: number;
    fanOut: number;
    coupling: number;
    isInCycle: boolean;
    cycleMembers?: string[];
    depth: number;
}

export interface NodeDetail {
    node: GraphNode;
    imports: GraphNode[];
    importedBy: GraphNode[];
    functions: GraphNode[];
    classes: GraphNode[];
    callers: GraphNode[];
    callees: GraphNode[];
    metrics: NodeMetrics;
}

export interface GraphMetrics {
    repoId: string;
    nodeCount: number;
    edgeCount: number;
    fileCount: number;
    dirCount: number;
    functionCount: number;
    classCount: number;
    cycleCount: number;
    maxFanIn: number;
    maxFanOut: number;
    avgFanIn: number;
    avgFanOut: number;
}

export interface CycleResult {
    nodes: string[];
    edges: string[];
    length: number;
}

export interface ShortestPathResult {
    nodes: GraphNode[];
    edges: GraphEdge[];
    length: number;
}

// ---------------------------------------------------------------------------
// Analysis pipeline models
// ---------------------------------------------------------------------------

export type PipelineStep =
    | 'validate' | 'clone' | 'detect' | 'walk_fs'
    | 'parse_ast' | 'build_graph' | 'index_ai' | 'done' | 'error';

export interface ProgressEvent {
    repoId: string;
    step: PipelineStep;
    progress: number;
    message: string;
    error?: string;
    at: string;
}

// ---------------------------------------------------------------------------
// Chat models
// ---------------------------------------------------------------------------

export interface ChatMessage {
    id: string;
    role: 'user' | 'assistant';
    content: string;
    timestamp: Date;
    references?: FileRef[];
    isStreaming?: boolean;
}

export interface FileRef {
    filePath: string;
    startLine: number;
    endLine: number;
}

// ---------------------------------------------------------------------------
// WebSocket message models
// ---------------------------------------------------------------------------

export type WsMessageType =
    | 'subscribe' | 'unsubscribe' | 'chat'
    | 'progress' | 'chat_token' | 'error' | 'ping' | 'pong';

export interface WsIncoming {
    type: WsMessageType;
    repoId?: string;
    nodeId?: string;
    message?: string;
    history?: { role: string; content: string }[];
}

export interface WsOutgoing {
    type: WsMessageType;
    repoId?: string;
    step?: PipelineStep;
    progress?: number;
    message?: string;
    error?: string;
    token?: string;
    done?: boolean;
    references?: FileRef[];
    at?: string;
}

// ---------------------------------------------------------------------------
// Search models
// ---------------------------------------------------------------------------

export interface SearchResult {
    chunkId: string;
    filePath: string;
    content: string;
    score: number;
    startLine: number;
    endLine: number;
    chunkType: string;
    name: string;
}

// ---------------------------------------------------------------------------
// Breadcrumb models
// ---------------------------------------------------------------------------

export interface BreadcrumbItem {
    nodeId: string;
    name: string;
    type: NodeType;
    path: string;
}
