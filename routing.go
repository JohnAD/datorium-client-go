package datorium

import (
	"fmt"

	"github.com/JohnAD/datorium-client-go/v2/shard"
)

// RouteKind selects write vs read member targeting.
type RouteKind int

const (
	RouteWrite RouteKind = iota
	RouteRead
)

// Route is a resolved server target for a command.
type Route struct {
	ServerName string
	BaseURL    string
	Slot       byte
	SlotHex    string
}

func (c *Client) routeDocument(est *Establishment, documentID string, kind RouteKind) (Route, error) {
	if documentID == "" || documentID == "null" {
		// Legacy / misconfigured: creates must supply a client id. Fall back to
		// the establishment server; Prefer PreferServer if set.
		name := est.General.EstablishmentServer
		if c.cfg.PreferServer != "" {
			name = c.cfg.PreferServer
		}
		u := est.ServerBaseURL(name)
		if u == "" {
			u = c.cfg.EstablishmentURL
		}
		return Route{ServerName: name, BaseURL: c.rewriteURL(name, u)}, nil
	}
	slot := shard.Slot(documentID)
	a, ok := est.AssignmentForSlot(slot)
	if !ok {
		return Route{}, fmt.Errorf("datorium: no shard assignment for slot %02X", slot)
	}
	var name string
	switch kind {
	case RouteWrite:
		name = a.ShardSOTMember
	case RouteRead:
		name = pickReadMember(a, c.cfg.PreferServer)
	default:
		return Route{}, fmt.Errorf("datorium: unknown route kind")
	}
	if name == "" {
		return Route{}, fmt.Errorf("datorium: no server for slot %02X kind %v", slot, kind)
	}
	u := est.ServerBaseURL(name)
	if u == "" {
		return Route{}, fmt.Errorf("datorium: missing baseURL for server %q", name)
	}
	return Route{
		ServerName: name,
		BaseURL:    c.rewriteURL(name, u),
		Slot:       slot,
		SlotHex:    shard.SlotHex(documentID),
	}, nil
}

func pickReadMember(a ShardAssignment, prefer string) string {
	if prefer != "" {
		for _, n := range a.ShardReadMember {
			if n == prefer {
				return n
			}
		}
		// Prefer local dual-role when PreferServer is also the SOT.
		if a.ShardSOTMember == prefer {
			for _, n := range a.ShardReadMember {
				if n == prefer {
					return n
				}
			}
		}
	}
	if len(a.ShardReadMember) > 0 {
		return a.ShardReadMember[0]
	}
	// Fall back to SOT only if it is also listed as read (dual-role topologies
	// always list it). Never use PROXY_READ_MEMBER for smart-client reads.
	return ""
}

func (c *Client) routeSearch(est *Establishment, slot byte) (Route, error) {
	a, ok := est.AssignmentForSlot(slot)
	if !ok {
		return Route{}, fmt.Errorf("datorium: no shard assignment for search slot %02X", slot)
	}
	name := a.ShardSOTMember
	if c.cfg.PreferServer != "" {
		if c.cfg.PreferServer == a.ShardSOTMember {
			name = c.cfg.PreferServer
		} else {
			for _, n := range a.ShardReadMember {
				if n == c.cfg.PreferServer {
					name = n
					break
				}
			}
		}
	}
	if name == "" && len(a.ShardReadMember) > 0 {
		name = a.ShardReadMember[0]
	}
	if name == "" {
		return Route{}, fmt.Errorf("datorium: no server for search slot %02X", slot)
	}
	u := est.ServerBaseURL(name)
	if u == "" {
		return Route{}, fmt.Errorf("datorium: missing baseURL for server %q", name)
	}
	return Route{
		ServerName: name,
		BaseURL:    c.rewriteURL(name, u),
		Slot:       slot,
		SlotHex:    fmt.Sprintf("%02X", slot),
	}, nil
}
