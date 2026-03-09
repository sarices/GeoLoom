package domain

// DedupResult 表示节点去重结果。
type DedupResult struct {
	Nodes          []NodeMetadata
	DuplicateCount int
}

// DedupNodes 按稳定指纹对节点去重，并合并来源名。
func DedupNodes(nodes []NodeMetadata) (DedupResult, error) {
	if len(nodes) == 0 {
		return DedupResult{Nodes: []NodeMetadata{}}, nil
	}

	result := DedupResult{Nodes: make([]NodeMetadata, 0, len(nodes))}
	indexByFingerprint := make(map[string]int, len(nodes))
	for _, node := range nodes {
		fingerprint, err := BuildNodeFingerprint(node)
		if err != nil {
			return DedupResult{}, err
		}
		node.Fingerprint = fingerprint

		if idx, exists := indexByFingerprint[fingerprint]; exists {
			result.Nodes[idx].SourceNames = mergeUniqueStrings(result.Nodes[idx].SourceNames, node.SourceNames)
			result.DuplicateCount++
			continue
		}

		node.SourceNames = mergeUniqueStrings(nil, node.SourceNames)
		indexByFingerprint[fingerprint] = len(result.Nodes)
		result.Nodes = append(result.Nodes, node)
	}
	return result, nil
}

func mergeUniqueStrings(base []string, values []string) []string {
	if len(base) == 0 && len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(base)+len(values))
	seen := make(map[string]struct{}, len(base)+len(values))
	for _, item := range append(append([]string{}, base...), values...) {
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
