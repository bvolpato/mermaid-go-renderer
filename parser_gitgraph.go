package mermaid

import "strings"

func parseGitGraph(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}
	graph := newGraph(DiagramGitGraph)
	graph.Source = input
	graph.GitMainBranch = "main"

	currentBranch := graph.GitMainBranch
	branchHead := map[string]string{}
	branches := map[string]struct{}{currentBranch: {}}
	commitSeq := 0

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		low := lower(line)
		if i == 0 && strings.HasPrefix(low, "gitgraph") {
			continue
		}
		switch {
		case strings.HasPrefix(low, "branch "):
			branch := sanitizeID(strings.TrimSpace(line[len("branch "):]), "")
			if branch == "" {
				continue
			}
			branches[branch] = struct{}{}
			if _, ok := branchHead[branch]; !ok {
				branchHead[branch] = branchHead[currentBranch]
			}
		case strings.HasPrefix(low, "checkout "):
			branch := sanitizeID(strings.TrimSpace(line[len("checkout "):]), "")
			if branch == "" {
				continue
			}
			currentBranch = branch
			branches[branch] = struct{}{}
		case strings.HasPrefix(low, "merge "):
			source := sanitizeID(strings.TrimSpace(line[len("merge "):]), "")
			if source == "" {
				continue
			}
			commitSeq++
			id := "commit_" + intString(commitSeq)
			label := "merge " + source
			graph.GitCommits = append(graph.GitCommits, GitCommit{
				ID:     id,
				Branch: currentBranch,
				Label:  label,
			})
			graph.ensureNode(id, label, ShapeCircle)
			if head := branchHead[currentBranch]; head != "" {
				graph.addEdge(Edge{From: head, To: id, Directed: true, ArrowEnd: true, Style: EdgeSolid})
			}
			if sourceHead := branchHead[source]; sourceHead != "" {
				graph.addEdge(Edge{From: sourceHead, To: id, Directed: true, ArrowEnd: true, Style: EdgeDotted})
			}
			branchHead[currentBranch] = id
		case strings.HasPrefix(low, "commit"):
			commitSeq++
			id, label := parseGitCommitMeta(line, commitSeq, currentBranch)
			graph.GitCommits = append(graph.GitCommits, GitCommit{
				ID:     id,
				Branch: currentBranch,
				Label:  label,
			})
			graph.ensureNode(id, label, ShapeCircle)
			if head := branchHead[currentBranch]; head != "" {
				graph.addEdge(Edge{
					From:     head,
					To:       id,
					Directed: true,
					ArrowEnd: true,
					Style:    EdgeSolid,
				})
			}
			branchHead[currentBranch] = id
		}
	}

	for branch := range branches {
		graph.GitBranches = append(graph.GitBranches, branch)
	}
	return ParseOutput{Graph: graph}, nil
}

func parseGitCommitMeta(line string, seq int, branch string) (string, string) {
	id := "commit_" + intString(seq)
	label := branch + " #" + intString(seq)

	parts := strings.Split(line, " ")
	for i := 0; i < len(parts)-1; i++ {
		if lower(parts[i]) == "id:" {
			candidate := sanitizeID(parts[i+1], "")
			if candidate != "" {
				id = candidate
			}
		}
	}
	if idx := strings.Index(line, "\""); idx >= 0 {
		rest := line[idx+1:]
		if end := strings.Index(rest, "\""); end >= 0 {
			msg := strings.TrimSpace(rest[:end])
			if msg != "" {
				label = msg
			}
		}
	}
	return id, label
}
