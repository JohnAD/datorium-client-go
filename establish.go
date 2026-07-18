package datorium

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/JohnAD/datorium-client-go/shard"
)

// ServerEntry is one establishment servers map entry.
type ServerEntry struct {
	BaseURL string `json:"baseURL"`
}

// ShardAssignment is one shard-map range assignment.
type ShardAssignment struct {
	ShardSOTMember  string   `json:"SHARD_SOT_MEMBER"`
	ShardReadMember []string `json:"SHARD_READ_MEMBER"`
	ProxyReadMember []string `json:"PROXY_READ_MEMBER"`
}

// GeneralConfig is the establishment general block.
type GeneralConfig struct {
	Name                                string `json:"name"`
	EstablishmentServer                 string `json:"establishmentServer"`
	Version                             int    `json:"version"`
	ReadMemberCheckinSeconds            int    `json:"readMemberCheckinSeconds"`
	CacheUpdateCheckinSeconds           int    `json:"cacheUpdateCheckinSeconds"`
	ReadMemberFailedCheckinsBeforeStale int    `json:"readMemberFailedCheckinsBeforeStale"`
}

// SchemaEntry is one collection schema from establish.
type SchemaEntry struct {
	Version int            `json:"version"`
	Schema  map[string]any `json:"schema"`
}

// Establishment is the cached establish document (without the ok envelope).
type Establishment struct {
	General  GeneralConfig              `json:"general"`
	Servers  map[string]ServerEntry     `json:"servers"`
	ShardMap map[string]ShardAssignment `json:"-"`
	Schemas  map[string]SchemaEntry     `json:"schemas"`
	Searches map[string]map[string]any  `json:"searches"`
	Auth     map[string]any             `json:"auth"`
	Raw      map[string]any             `json:"-"`
}

type establishmentCache struct {
	mu  sync.RWMutex
	cfg *Establishment
}

func (c *establishmentCache) get() *Establishment {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cfg
}

func (c *establishmentCache) set(cfg *Establishment) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg = cfg
}

func parseEstablishment(res Result) (*Establishment, error) {
	if !res.OK {
		return nil, appErrorFromResult(res)
	}
	// Re-marshal the raw map (minus ok) into typed fields.
	raw := map[string]any{}
	for k, v := range res.Raw {
		if k == "ok" || k == "errors" {
			continue
		}
		raw[k] = v
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var partial struct {
		General  GeneralConfig          `json:"general"`
		Servers  map[string]ServerEntry `json:"servers"`
		ShardMap struct {
			Default map[string]ShardAssignment `json:"default"`
		} `json:"shardMap"`
		Schemas  map[string]SchemaEntry    `json:"schemas"`
		Searches map[string]map[string]any `json:"searches"`
		Auth     map[string]any            `json:"auth"`
	}
	if err := json.Unmarshal(body, &partial); err != nil {
		return nil, fmt.Errorf("parse establishment: %w", err)
	}
	est := &Establishment{
		General:  partial.General,
		Servers:  partial.Servers,
		ShardMap: partial.ShardMap.Default,
		Schemas:  partial.Schemas,
		Searches: partial.Searches,
		Auth:     partial.Auth,
		Raw:      raw,
	}
	if est.Servers == nil {
		est.Servers = map[string]ServerEntry{}
	}
	if est.ShardMap == nil {
		est.ShardMap = map[string]ShardAssignment{}
	}
	var ranges []shard.Range
	for rawRange := range est.ShardMap {
		r, err := shard.ParseRange(rawRange)
		if err != nil {
			return nil, fmt.Errorf("establishment shard range %q: %w", rawRange, err)
		}
		ranges = append(ranges, r)
	}
	if len(ranges) > 0 {
		if err := shard.ValidateFullCoverage(ranges); err != nil {
			return nil, fmt.Errorf("establishment shard map: %w", err)
		}
	}
	return est, nil
}

// AssignmentForSlot returns the shard assignment covering slot.
func (e *Establishment) AssignmentForSlot(slot byte) (ShardAssignment, bool) {
	if e == nil {
		return ShardAssignment{}, false
	}
	for raw, a := range e.ShardMap {
		r, err := shard.ParseRange(raw)
		if err != nil {
			continue
		}
		if r.Contains(slot) {
			return a, true
		}
	}
	return ShardAssignment{}, false
}

// ServerBaseURL returns the configured base URL for a server name.
func (e *Establishment) ServerBaseURL(name string) string {
	if e == nil {
		return ""
	}
	return e.Servers[name].BaseURL
}
