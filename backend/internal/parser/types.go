package parser

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, ".idea": true,
	"__pycache__": true, ".venv": true, "venv": true, "dist": true,
	"build": true, ".next": true, ".nuxt": true, "coverage": true,
	".cache": true, "target": true, ".gradle": true,
}

var skipExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true,
	".ico": true, ".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
	".mp4": true, ".mp3": true, ".zip": true, ".tar": true, ".gz": true,
	".pdf": true, ".DS_Store": true, ".lock": true,
}

type Node struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Language string `json:"language"`
	Size     int64  `json:"size"`
	Lines    int    `json:"lines"`
	Children int    `json:"children"`
	Path     string `json:"path"`
	Depth    int    `json:"depth"`
}

type Edge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type GraphData struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type FileNode struct {
	Path     string     `json:"path"`
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Language string     `json:"language,omitempty"`
	Size     int64      `json:"size,omitempty"`
	Lines    int        `json:"lines,omitempty"`
	Children []FileNode `json:"children,omitempty"`
}

type CloneError struct {
	Msg string
}

func (e *CloneError) Error() string { return "git clone failed: " + e.Msg }
