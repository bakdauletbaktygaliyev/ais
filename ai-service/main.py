import os
import httpx
from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from anthropic import Anthropic

from models import ChatRequest, ChatResponse
from context import build_context

app = FastAPI(title="AIS AI Service")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)

client = Anthropic(api_key=os.environ.get("ANTHROPIC_API_KEY", ""))
BACKEND_URL = os.environ.get("BACKEND_URL", "http://backend:8080")


@app.get("/health")
def health():
    return {"ok": True}


@app.post("/chat", response_model=ChatResponse)
async def chat(req: ChatRequest):
    api_key = os.environ.get("ANTHROPIC_API_KEY", "")
    if not api_key or api_key == "your_anthropic_api_key_here":
        raise HTTPException(status_code=503, detail="ANTHROPIC_API_KEY not configured")

    async with httpx.AsyncClient(timeout=30) as http:
        try:
            proj_resp = await http.get(f"{BACKEND_URL}/api/projects/{req.project_id}")
            if proj_resp.status_code != 200:
                raise HTTPException(status_code=404, detail="Project not found")
            project = proj_resp.json()

            graph_path = req.current_path if req.current_path else ""
            graph_url = f"{BACKEND_URL}/api/projects/{req.project_id}/graph"
            if graph_path:
                graph_url += f"?path={graph_path}"
            graph_resp = await http.get(graph_url)
            graph = graph_resp.json() if graph_resp.status_code == 200 else {"nodes": [], "edges": []}
        except httpx.RequestError as e:
            raise HTTPException(status_code=502, detail=f"Backend unreachable: {e}")

    context = build_context(project, graph, req.current_path)

    try:
        message = client.messages.create(
            model="claude-opus-4-6",
            max_tokens=1024,
            system=f"""You are an expert software architect analyzing a code repository.
You have access to the project structure and dependency graph.
Answer questions concisely and accurately based on the provided context.
When referencing files or modules, use their paths.

Project: {project.get('name', 'Unknown')}
URL: {project.get('url', '')}

{context}""",
            messages=[{"role": "user", "content": req.question}],
        )
        answer = message.content[0].text
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Claude API error: {str(e)}")

    return ChatResponse(answer=answer)
