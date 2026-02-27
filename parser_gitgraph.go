package mermaid

import (
	"encoding/binary"
	"fmt"
	"math/bits"
	"strings"
)

func parseGitGraph(input string) (ParseOutput, error) {
	lines, err := preprocessInput(input)
	if err != nil {
		return ParseOutput{}, err
	}
	graph := newGraph(DiagramGitGraph)
	graph.Source = input
	graph.GitMainBranch = "main"
	graph.Direction = DirectionLeftRight

	currentBranch := graph.GitMainBranch
	branchHead := map[string]string{currentBranch: ""}
	branchInsertion := map[string]int{currentBranch: 0}
	graph.GitBranchDefs = append(graph.GitBranchDefs, GitBranch{
		Name:           currentBranch,
		Order:          floatPtr(0.0),
		InsertionIndex: 0,
	})
	commitSeq := 0
	rng := newGitGraphIDRNG(hashSeed(input))

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		low := lower(line)
		if strings.HasPrefix(low, "gitgraph") {
			continue
		}
		if direction, ok := parseGitGraphDirection(line); ok {
			graph.Direction = direction
			continue
		}
		switch {
		case strings.HasPrefix(low, "branch "):
			branch := strings.TrimSpace(line[len("branch "):])
			if branch == "" {
				continue
			}
			order := extractGitGraphOrder(line)
			if _, ok := branchHead[branch]; !ok {
				branchHead[branch] = branchHead[currentBranch]
			}
			if _, ok := branchInsertion[branch]; !ok {
				idx := len(graph.GitBranchDefs)
				branchInsertion[branch] = idx
				graph.GitBranchDefs = append(graph.GitBranchDefs, GitBranch{
					Name:           branch,
					Order:          order,
					InsertionIndex: idx,
				})
			}
			currentBranch = branch
		case strings.HasPrefix(low, "checkout "), strings.HasPrefix(low, "switch "):
			branch := ""
			if strings.HasPrefix(low, "checkout ") {
				branch = strings.TrimSpace(line[len("checkout "):])
			} else {
				branch = strings.TrimSpace(line[len("switch "):])
			}
			if branch == "" {
				continue
			}
			currentBranch = branch
			if _, ok := branchHead[currentBranch]; !ok {
				branchHead[currentBranch] = ""
			}
			if _, ok := branchInsertion[branch]; !ok {
				idx := len(graph.GitBranchDefs)
				branchInsertion[branch] = idx
				graph.GitBranchDefs = append(graph.GitBranchDefs, GitBranch{
					Name:           branch,
					Order:          nil,
					InsertionIndex: idx,
				})
			}
		case strings.HasPrefix(low, "merge "):
			fromBranch := strings.TrimSpace(line[len("merge "):])
			if fromBranch == "" {
				continue
			}
			fromHead := branchHead[fromBranch]
			currentHead := branchHead[currentBranch]
			if fromHead == "" && currentHead == "" {
				continue
			}
			parents := make([]string, 0, 2)
			if currentHead != "" {
				parents = append(parents, currentHead)
			}
			if fromHead != "" {
				parents = append(parents, fromHead)
			}
			id := extractGitGraphID(line)
			customID := id != ""
			if id == "" {
				id = fmt.Sprintf("%d-%s", commitSeq, rng.nextHex(7))
			}
			tags := extractGitGraphTags(line)
			customType, hasCustomType := extractGitGraphCommitType(line)
			label := fmt.Sprintf("merged branch %s into %s", fromBranch, currentBranch)
			graph.GitCommits = append(graph.GitCommits, GitCommit{
				ID:            id,
				Branch:        currentBranch,
				Label:         label,
				Message:       label,
				Seq:           commitSeq,
				CommitType:    GitGraphCommitTypeMerge,
				CustomType:    customType,
				HasCustomType: hasCustomType,
				Tags:          tags,
				Parents:       parents,
				CustomID:      customID,
			})
			commitSeq++
			branchHead[currentBranch] = id
		case strings.HasPrefix(low, "commit"):
			id := extractGitGraphID(line)
			customID := id != ""
			if id == "" {
				id = fmt.Sprintf("%d-%s", commitSeq, rng.nextHex(7))
			}
			tags := extractGitGraphTags(line)
			commitType, hasCommitType := extractGitGraphCommitType(line)
			if !hasCommitType {
				commitType = GitGraphCommitTypeNormal
			}
			parents := []string{}
			if head := branchHead[currentBranch]; head != "" {
				parents = append(parents, head)
			}
			message := extractGitGraphMessage(line)
			label := message
			if label == "" {
				label = id
			}
			graph.GitCommits = append(graph.GitCommits, GitCommit{
				ID:            id,
				Branch:        currentBranch,
				Label:         label,
				Message:       message,
				Seq:           commitSeq,
				CommitType:    commitType,
				Tags:          tags,
				Parents:       parents,
				CustomID:      customID,
				HasCustomType: false,
			})
			commitSeq++
			branchHead[currentBranch] = id
		}
	}

	graph.GitBranches = graph.GitBranches[:0]
	for _, branch := range graph.GitBranchDefs {
		graph.GitBranches = append(graph.GitBranches, branch.Name)
	}
	return ParseOutput{Graph: graph}, nil
}

func parseGitGraphDirection(line string) (Direction, bool) {
	trimmed := strings.TrimSpace(line)
	switch upper(trimmed) {
	case "LR":
		return DirectionLeftRight, true
	case "TB", "TD":
		return DirectionTopDown, true
	case "BT":
		return DirectionBottomTop, true
	}
	low := lower(trimmed)
	if strings.HasPrefix(low, "direction") {
		token := strings.TrimSpace(trimmed[len("direction"):])
		switch upper(token) {
		case "LR":
			return DirectionLeftRight, true
		case "TB", "TD":
			return DirectionTopDown, true
		case "BT":
			return DirectionBottomTop, true
		}
	}
	return DirectionTopDown, false
}

func extractGitGraphID(line string) string {
	return extractGitGraphAttr(line, "id")
}

func extractGitGraphMessage(line string) string {
	return extractGitGraphAttr(line, "msg")
}

func extractGitGraphCommitType(line string) (GitGraphCommitType, bool) {
	raw := upper(extractGitGraphAttr(line, "type"))
	switch raw {
	case "NORMAL":
		return GitGraphCommitTypeNormal, true
	case "REVERSE":
		return GitGraphCommitTypeReverse, true
	case "HIGHLIGHT":
		return GitGraphCommitTypeHighlight, true
	case "CHERRY-PICK", "CHERRYPICK":
		return GitGraphCommitTypeCherryPick, true
	default:
		return "", false
	}
}

func extractGitGraphOrder(line string) *float64 {
	raw := extractGitGraphAttr(line, "order")
	if raw == "" {
		return nil
	}
	if value, ok := parseFloat(raw); ok {
		return &value
	}
	return nil
}

func extractGitGraphTags(line string) []string {
	return extractGitGraphAttrs(line, "tag")
}

func extractGitGraphAttrs(line, key string) []string {
	values := []string{}
	lowerLine := strings.ToLower(line)
	needle := strings.ToLower(key) + ":"
	start := 0
	for {
		idx := strings.Index(lowerLine[start:], needle)
		if idx < 0 {
			break
		}
		offset := start + idx + len(needle)
		value, next, ok := extractGitGraphAttrAt(line, offset)
		if !ok {
			break
		}
		values = append(values, value)
		start = next
	}
	return values
}

func extractGitGraphAttr(line, key string) string {
	values := extractGitGraphAttrs(line, key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func extractGitGraphAttrAt(line string, start int) (value string, next int, ok bool) {
	if start >= len(line) {
		return "", start, false
	}
	i := start
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i >= len(line) {
		return "", i, false
	}
	if line[i] == '"' || line[i] == '\'' {
		quote := line[i]
		i++
		begin := i
		for i < len(line) && line[i] != quote {
			i++
		}
		return line[begin:i], minInt(i+1, len(line)), true
	}
	begin := i
	for i < len(line) && line[i] != ' ' && line[i] != '\t' && line[i] != ',' {
		i++
	}
	return line[begin:i], i, true
}

func hashSeed(input string) uint64 {
	// Rust hashes &str by feeding bytes plus a 0xFF sentinel.
	// parser.rs uses DefaultHasher (SipHash-1-3), so we mirror it here.
	data := append([]byte(input), 0xFF)
	return sipHash13(data, 0, 0)
}

func sipHash13(data []byte, k0 uint64, k1 uint64) uint64 {
	v0 := k0 ^ 0x736f6d6570736575
	v1 := k1 ^ 0x646f72616e646f6d
	v2 := k0 ^ 0x6c7967656e657261
	v3 := k1 ^ 0x7465646279746573

	fullLen := len(data) &^ 7
	for i := 0; i < fullLen; i += 8 {
		m := binary.LittleEndian.Uint64(data[i : i+8])
		v3 ^= m
		sipRound(&v0, &v1, &v2, &v3)
		v0 ^= m
	}

	var b uint64 = uint64(len(data)) << 56
	rem := data[fullLen:]
	switch len(rem) {
	case 7:
		b |= uint64(rem[6]) << 48
		fallthrough
	case 6:
		b |= uint64(rem[5]) << 40
		fallthrough
	case 5:
		b |= uint64(rem[4]) << 32
		fallthrough
	case 4:
		b |= uint64(rem[3]) << 24
		fallthrough
	case 3:
		b |= uint64(rem[2]) << 16
		fallthrough
	case 2:
		b |= uint64(rem[1]) << 8
		fallthrough
	case 1:
		b |= uint64(rem[0])
	}

	v3 ^= b
	sipRound(&v0, &v1, &v2, &v3)
	v0 ^= b
	v2 ^= 0xFF
	sipRound(&v0, &v1, &v2, &v3)
	sipRound(&v0, &v1, &v2, &v3)
	sipRound(&v0, &v1, &v2, &v3)
	return v0 ^ v1 ^ v2 ^ v3
}

func sipRound(v0, v1, v2, v3 *uint64) {
	*v0 += *v1
	*v1 = bits.RotateLeft64(*v1, 13)
	*v1 ^= *v0
	*v0 = bits.RotateLeft64(*v0, 32)

	*v2 += *v3
	*v3 = bits.RotateLeft64(*v3, 16)
	*v3 ^= *v2

	*v0 += *v3
	*v3 = bits.RotateLeft64(*v3, 21)
	*v3 ^= *v0

	*v2 += *v1
	*v1 = bits.RotateLeft64(*v1, 17)
	*v1 ^= *v2
	*v2 = bits.RotateLeft64(*v2, 32)
}

type gitGraphIDRNG struct {
	state uint64
}

func newGitGraphIDRNG(seed uint64) *gitGraphIDRNG {
	if seed == 0 {
		seed = 0xA5A5A5A55A5A5A5A
	}
	return &gitGraphIDRNG{state: seed}
}

func (r *gitGraphIDRNG) nextU32() uint32 {
	x := r.state
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	r.state = x
	return uint32(x >> 32)
}

func (r *gitGraphIDRNG) nextHex(length int) string {
	const hexDigits = "0123456789abcdef"
	out := make([]byte, 0, length)
	for i := 0; i < length; i++ {
		out = append(out, hexDigits[r.nextU32()&0xF])
	}
	return string(out)
}

func floatPtr(v float64) *float64 {
	return &v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
