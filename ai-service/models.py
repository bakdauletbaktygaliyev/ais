from pydantic import BaseModel


class ChatRequest(BaseModel):
    project_id: str
    question: str
    current_path: str = ""


class ChatResponse(BaseModel):
    answer: str
