package datorium

import (
	"fmt"
	"sync"

	"github.com/JohnAD/datorium-client-go/v2/shard"
	"github.com/JohnAD/ojson"
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
	// MaxFileBytes is the streamed binary attachment upload limit (bytes).
	// Zero means the server default (1 GiB).
	MaxFileBytes int64 `json:"maxFileBytes,omitempty"`
}

// SchemaEntry is one collection schema from establish.
type SchemaEntry struct {
	Version int
	// Doc is the ordered schema object (ojson). Never round-trip via map[string]any.
	Doc ojson.JSONValue
}

// Establishment is the cached establish document (without the ok envelope).
type Establishment struct {
	General  GeneralConfig
	Servers  map[string]ServerEntry
	ShardMap map[string]ShardAssignment
	Schemas  map[string]SchemaEntry
	Searches ojson.JSONValue
	Auth     ojson.JSONValue
	// Env is the full establish envelope object (ordered).
	Env ojson.JSONValue
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
	env := res.Env
	if env.IsMissing() && len(res.Body) > 0 {
		var err error
		env, err = ojson.ReadBytesNoSchema(res.Body)
		if err != nil {
			return nil, fmt.Errorf("parse establishment: %w", err)
		}
	}
	if !env.IsObject() {
		return nil, fmt.Errorf("parse establishment: expected object envelope")
	}

	est := &Establishment{
		Servers:  map[string]ServerEntry{},
		ShardMap: map[string]ShardAssignment{},
		Schemas:  map[string]SchemaEntry{},
		Searches: env.Get("searches"),
		Auth:     env.Get("auth"),
		Env:      env,
	}

	if general := env.Get("general"); general.IsObject() {
		if err := general.ToStructTry(&est.General); err != nil {
			return nil, fmt.Errorf("parse establishment general: %w", err)
		}
	}

	if servers := env.Get("servers"); servers.IsObject() {
		for name := range servers.ToMap() {
			entry := servers.Get(name)
			var se ServerEntry
			if err := entry.ToStructTry(&se); err != nil {
				return nil, fmt.Errorf("parse establishment server %q: %w", name, err)
			}
			est.Servers[name] = se
		}
	}

	if shardMap := env.Get("shardMap"); shardMap.IsObject() {
		defaultMap := shardMap.Get("default")
		if defaultMap.IsObject() {
			for rawRange := range defaultMap.ToMap() {
				var a ShardAssignment
				if err := defaultMap.Get(rawRange).ToStructTry(&a); err != nil {
					return nil, fmt.Errorf("parse establishment shard %q: %w", rawRange, err)
				}
				est.ShardMap[rawRange] = a
			}
		}
	}

	if schemas := env.Get("schemas"); schemas.IsObject() {
		for name := range schemas.ToMap() {
			entry := schemas.Get(name)
			est.Schemas[name] = SchemaEntry{
				Version: entry.Get("version").ToIntOrDefault(0),
				Doc:     entry.Get("schema"),
			}
		}
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
