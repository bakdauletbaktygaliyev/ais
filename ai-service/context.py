def build_context(project: dict, graph: dict, current_path: str) -> str:
    lines = []

    nodes = graph.get("nodes", [])
    edges = graph.get("edges", [])

    if current_path:
        lines.append(f"Current view: {current_path}")
    else:
        lines.append("Current view: project root")

    lines.append(f"\nVisible nodes ({len(nodes)} items):")
    for node in nodes[:50]:
        node_type = node.get("type", "file")
        lang = node.get("language", "")
        size_info = f" ({node.get('lines', 0)} lines)" if node_type == "file" else f" ({node.get('children', 0)} items)"
        lang_info = f" [{lang}]" if lang and lang not in ("text", "json", "yaml", "markdown") else ""
        lines.append(f"  {'📁' if node_type == 'directory' else '📄'} {node.get('path', '')}{lang_info}{size_info}")

    if edges:
        lines.append(f"\nDependencies ({len(edges)} import relationships):")
        for edge in edges[:30]:
            lines.append(f"  {edge.get('source', '')} → {edge.get('target', '')}")

    return "\n".join(lines)
