package guard

import "sync"

// CostTag represents a metadata tag for cost allocation and reporting.
type CostTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

var (
	costTagsMu sync.RWMutex
	costTags   = make(map[string][]CostTag) // keyed by client_key or project_id
)

// SetCostTags assigns allocation tags to a client or project.
func SetCostTags(entityID string, tags []CostTag) {
	costTagsMu.Lock()
	defer costTagsMu.Unlock()
	costTags[entityID] = tags
}

// GetCostTags returns the allocation tags for a client or project.
func GetCostTags(entityID string) []CostTag {
	costTagsMu.RLock()
	defer costTagsMu.RUnlock()
	tags := costTags[entityID]
	out := make([]CostTag, len(tags))
	copy(out, tags)
	return out
}

// MergeTags combines base tags with overrides; overrides win on duplicate keys.
func MergeTags(base, overrides []CostTag) []CostTag {
	merged := make(map[string]string, len(base)+len(overrides))
	for _, t := range base {
		merged[t.Key] = t.Value
	}
	for _, t := range overrides {
		merged[t.Key] = t.Value
	}
	out := make([]CostTag, 0, len(merged))
	for k, v := range merged {
		out = append(out, CostTag{Key: k, Value: v})
	}
	return out
}