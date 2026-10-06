package agent

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

func previewRequest(args string) (string, []string, error) {
	var request struct {
		Kind    string `json:"kind"`
		Content string `json:"content"`
		Title   string `json:"title,omitempty"`
	}
	if err := json.Unmarshal([]byte(args), &request); err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(request.Content) == "" || len(request.Content) > 200000 {
		return "", nil, fmt.Errorf("preview content must contain 1 to 200000 bytes")
	}
	switch request.Kind {
	case "html", "markdown":
	case "url":
		u, err := url.Parse(request.Content)
		if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") {
			return "", nil, fmt.Errorf("preview URL must use a localhost HTTP endpoint without credentials")
		}
	default:
		return "", nil, fmt.Errorf("unsupported preview kind")
	}
	if len(request.Title) > 200 {
		return "", nil, fmt.Errorf("preview title is too long")
	}
	data, err := json.Marshal(request)
	return string(data), nil, err
}
