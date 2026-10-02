package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// GHRunner és injectable perquè la integració es pugui provar sense una xarxa
// ni un binari gh real. En producció executa la CLI oficial sense shell.
var GHRunner = runGH

// GitHub exposa consultes de només lectura per issues i pull requests.
// No accepta ordres arbitràries: cada acció es tradueix a arguments tancats de
// gh, evitant que l'agent converteixi aquesta eina en un segon bash.
func GitHub(tool, argsJSON string) (string, error) {
	var a struct {
		Action string `json:"action"`
		Number int    `json:"number"`
		Query  string `json:"query"`
		Repo   string `json:"repo"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", fmt.Errorf("arguments GitHub il·legibles: %w", err)
	}
	a.Action = strings.ToLower(strings.TrimSpace(a.Action))
	if a.Repo != "" && !validRepo(a.Repo) {
		return "", fmt.Errorf("repo invàlid: usa owner/name")
	}
	if a.Query != "" && len(a.Query) > 200 {
		return "", fmt.Errorf("query massa llarga (màx 200 caràcters)")
	}
	args := []string{}
	switch tool {
	case "gh_issue":
		switch a.Action {
		case "list":
			args = []string{"issue", "list", "--limit", "30", "--json", "number,title,state,author,url,labels,updatedAt"}
			if a.Query != "" {
				args = append(args, "--search", a.Query)
			}
		case "view":
			if a.Number < 1 {
				return "", fmt.Errorf("gh_issue view necessita number")
			}
			args = []string{"issue", "view", strconv.Itoa(a.Number), "--json", "number,title,state,body,author,url,comments,labels,assignees"}
		default:
			return "", fmt.Errorf("gh_issue action ha de ser list o view")
		}
	case "gh_pr":
		switch a.Action {
		case "list":
			args = []string{"pr", "list", "--limit", "30", "--json", "number,title,state,author,url,headRefName,baseRefName,updatedAt"}
			if a.Query != "" {
				args = append(args, "--search", a.Query)
			}
		case "view":
			if a.Number < 1 {
				return "", fmt.Errorf("gh_pr view necessita number")
			}
			args = []string{"pr", "view", strconv.Itoa(a.Number), "--json", "number,title,state,body,author,url,comments,files,commits,reviewDecision,statusCheckRollup"}
		case "diff":
			if a.Number < 1 {
				return "", fmt.Errorf("gh_pr diff necessita number")
			}
			args = []string{"pr", "diff", strconv.Itoa(a.Number)}
		case "checks":
			if a.Number < 1 {
				return "", fmt.Errorf("gh_pr checks necessita number")
			}
			args = []string{"pr", "checks", strconv.Itoa(a.Number)}
		default:
			return "", fmt.Errorf("gh_pr action ha de ser list, view, diff o checks")
		}
	default:
		return "", fmt.Errorf("eina GitHub desconeguda: %s", tool)
	}
	if a.Repo != "" {
		args = append(args, "--repo", a.Repo)
	}
	out, err := GHRunner(context.Background(), args...)
	if err != nil {
		return "", fmt.Errorf("gh: %w", err)
	}
	if strings.TrimSpace(out) == "" {
		return "(sense resultats)", nil
	}
	return truncate(out, 12000), nil
}

func validRepo(repo string) bool {
	parts := strings.Split(repo, "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] != "" &&
		!strings.ContainsAny(repo, " \t\r\n")
}

func runGH(ctx context.Context, args ...string) (string, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultTimeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "gh", args...)
	return commandOutput(cmd)
}

func commandOutput(cmd *exec.Cmd) (string, error) {
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return "", err
		}
		return "", fmt.Errorf("%s: %w", msg, err)
	}
	return string(out), nil
}
