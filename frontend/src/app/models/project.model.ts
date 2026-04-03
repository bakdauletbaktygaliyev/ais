export interface Project {
  id: string;
  url: string;
  name: string;
  status: 'pending' | 'analyzing' | 'done' | 'error';
  error?: string;
  created_at: string;
}

export interface GraphNode {
  id: string;
  name: string;
  type: 'file' | 'directory';
  language: string;
  size: number;
  lines: number;
  children: number;
  path: string;
  depth: number;
}

export interface GraphEdge {
  source: string;
  target: string;
  type: string;
}

export interface GraphData {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export interface ChatResponse {
  answer: string;
}
