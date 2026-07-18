// Command todo-integration exercises the smart client against a live
// two-shard Todo establishment started by start_integration_test.sh.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	datorium "github.com/JohnAD/datorium-client-go"
	"github.com/JohnAD/datorium-client-go/refs"
	"github.com/JohnAD/datorium-client-go/searchpath"
	"github.com/JohnAD/datorium-client-go/shard"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "todo-integration FAILED: %v\n", err)
		os.Exit(1)
	}
	step("PASSED")
	fmt.Println("todo-integration PASSED")
}

func run() error {
	ctx := context.Background()
	base1 := envOr("DATORIUM_SERVER1_URL", "http://127.0.0.1:18081")
	base2 := envOr("DATORIUM_SERVER2_URL", "http://127.0.0.1:18082")
	token := os.Getenv("DATORIUM_TOKEN")
	if token == "" {
		return fmt.Errorf("DATORIUM_TOKEN is required")
	}

	client, err := datorium.New(datorium.Config{
		EstablishmentURL: base1,
		Token:            token,
		BaseURLRewrite: map[string]string{
			"server1":             base1,
			"server2":             base2,
			"http://server1:8080": base1,
			"http://server2:8080": base2,
		},
		WrongMachineRetries: 5,
	})
	if err != nil {
		return err
	}
	defer client.Close()

	step("WAIT_READY")
	if err := waitReady(ctx, client, 60*time.Second); err != nil {
		return err
	}

	step("ESTABLISH")
	if err := client.Establish(ctx); err != nil {
		return fmt.Errorf("establish: %w", err)
	}
	est := client.CachedEstablishment()
	if est == nil || est.General.Version < 1 {
		return fmt.Errorf("establishment cache missing")
	}
	detail("establishment %q version %d", est.General.Name, est.General.Version)

	step("PICK_IDS")
	userLow := findID(0x00, 0x7F)
	userHigh := findID(0x80, 0xFF)
	listLow := findIDExcluding(0x00, 0x7F, userLow)
	todoHigh := findIDExcluding(0x80, 0xFF, userHigh)
	detail("userLow=%s(%s) userHigh=%s(%s) listLow=%s(%s) todoHigh=%s(%s)",
		userLow, shard.SlotHex(userLow), userHigh, shard.SlotHex(userHigh),
		listLow, shard.SlotHex(listLow), todoHigh, shard.SlotHex(todoHigh))

	step("CREATING_USERS")
	if _, err := client.Create(ctx, "Users", userLow, map[string]any{
		"$": "Users:0", "displayName": "Ada", "email": "ada@example.com", "todoLists": []any{},
	}); err != nil {
		return fmt.Errorf("create userLow: %w", err)
	}
	if _, err := client.Create(ctx, "Users", userHigh, map[string]any{
		"$": "Users:0", "displayName": "Grace", "email": "grace@example.com", "todoLists": []any{},
	}); err != nil {
		return fmt.Errorf("create userHigh: %w", err)
	}

	step("CREATING_LIST")
	ownerDirect := refs.FormatDirect("Users", userHigh)
	ownerCached := refs.FormatCached("Users", userHigh)
	const listTitle = "Ship client"
	listWR, err := client.Create(ctx, "TodoLists", listLow, map[string]any{
		"$":            "TodoLists:0",
		"title":        listTitle,
		"owner":        ownerDirect,
		"ownerSummary": ownerCached,
	})
	if err != nil {
		return fmt.Errorf("create list: %w", err)
	}

	// O(1) front-page pattern: User.todoLists holds @@ refs to owned lists.
	step("LINK_LIST_TO_USER")
	ownerRR, err := client.Read(ctx, "Users", userHigh, nil)
	if err != nil {
		return fmt.Errorf("read owner before linking list: %w", err)
	}
	if _, err := client.Patch(ctx, "Users", userHigh, datorium.PatchDetailAppendingCachedRef(
		asString(ownerRR.SOT["$"]), asString(ownerRR.SOT["#"]),
		"todoLists", "TodoLists", listLow,
	)); err != nil {
		return fmt.Errorf("append todoLists cached ref: %w", err)
	}
	detail("appended @@__TodoLists__%s onto Users/%s.todoLists", listLow, userHigh)

	step("READ_USER_FRONT_PAGE")
	if err := waitUserFrontPageTitle(ctx, client, userHigh, listLow, listTitle, 45*time.Second, 2*time.Second); err != nil {
		return fmt.Errorf("user front page initial: %w", err)
	}
	detail("read Users/%s with cacheSummaries; todoLists front page shows title %q", userHigh, listTitle)

	step("PATCH_LIST_TITLE")
	listRR, err := client.Read(ctx, "TodoLists", listLow, nil)
	if err != nil {
		return fmt.Errorf("read list before title patch: %w", err)
	}
	const updatedListTitle = "Ship client v2"
	if _, err := client.Patch(ctx, "TodoLists", listLow, map[string]any{
		"$": listRR.SOT["$"],
		"#": listRR.SOT["#"],
		"RFC6902": []any{
			map[string]any{"op": "replace", "path": "/title", "value": updatedListTitle},
		},
	}); err != nil {
		return fmt.Errorf("patch list title: %w", err)
	}
	detail("patched TodoLists/%s title → %q", listLow, updatedListTitle)

	step("WAIT_FRONT_PAGE_UPDATE")
	if err := waitUserFrontPageTitle(ctx, client, userHigh, listLow, updatedListTitle, 15*time.Second, 2*time.Second); err != nil {
		return fmt.Errorf("user front page after list patch: %w", err)
	}
	detail("re-read Users/%s; cached TodoLists/%s.title now %q", userHigh, listLow, updatedListTitle)

	step("RESOLVE_LIVE_REF")
	listRR, err = client.Read(ctx, "TodoLists", listLow, &datorium.ReadOptions{CacheSummaries: true})
	if err != nil {
		return fmt.Errorf("read list: %w", err)
	}
	if listRR.SOT["owner"] != ownerDirect {
		return fmt.Errorf("expected owner %q, got %#v", ownerDirect, listRR.SOT["owner"])
	}
	ownerRR, err = client.ResolveDirectRef(ctx, asString(listRR.SOT["owner"]), nil)
	if err != nil {
		return fmt.Errorf("resolve owner: %w", err)
	}
	if ownerRR.SOT["displayName"] != "Grace" {
		return fmt.Errorf("resolved owner displayName=%#v", ownerRR.SOT["displayName"])
	}
	detail("live owner resolved to Grace")

	// Owner summary on the list. Important: pending cache-updates are
	// completed as no-ops if the read member has no stub yet, so we must
	// (1) read the referring list with cacheSummaries to create the stub,
	// then (2) patch the User so a fresh work item can fill it.
	step("READ_CACHED_REF")
	seedRR, err := client.Read(ctx, "TodoLists", listLow, &datorium.ReadOptions{CacheSummaries: true})
	if err != nil {
		return fmt.Errorf("seed list cacheSummaries read: %w", err)
	}
	if coll, _ := seedRR.CacheSummaries["Users"].(map[string]any); coll != nil {
		detail("seed read TodoLists/%s; Users/%s summary=%v", listLow, userHigh, coll[userHigh])
	} else {
		detail("seed read TodoLists/%s; no Users cacheSummaries yet", listLow)
	}

	step("PATCH_CACHED_TARGET")
	ownerRR, err = client.Read(ctx, "Users", userHigh, nil)
	if err != nil {
		return fmt.Errorf("read owner before patch: %w", err)
	}
	const updatedName = "Grace Hopper"
	if _, err := client.Patch(ctx, "Users", userHigh, map[string]any{
		"$": ownerRR.SOT["$"],
		"#": ownerRR.SOT["#"],
		"RFC6902": []any{
			map[string]any{"op": "replace", "path": "/displayName", "value": updatedName},
		},
	}); err != nil {
		return fmt.Errorf("patch owner displayName: %w", err)
	}
	detail("patched referenced Users/%s displayName → %q (re-fans cache updates)", userHigh, updatedName)

	step("WAIT_CACHE_UPDATE")
	if err := waitCacheSummary(ctx, client, "TodoLists", listLow, "Users", userHigh, "displayName", updatedName, 15*time.Second, 2*time.Second); err != nil {
		return fmt.Errorf("cached summary did not observe owner patch: %w", err)
	}
	detail("re-read TodoLists/%s; cached Users/%s.displayName now %q", listLow, userHigh, updatedName)

	step("CREATING_TODO")
	listDirect := refs.FormatDirect("TodoLists", listLow)
	listCached := refs.FormatCached("TodoLists", listLow)
	todoWR, err := client.Create(ctx, "Todos", todoHigh, map[string]any{
		"$":           "Todos:0",
		"title":       "Write integration test",
		"status":      "open",
		"list":        listDirect,
		"listSummary": listCached,
	})
	if err != nil {
		return fmt.Errorf("create todo: %w", err)
	}
	_ = listWR
	_ = todoWR

	step("PATCHING_TODO")
	todoRR, err := client.Read(ctx, "Todos", todoHigh, nil)
	if err != nil {
		return fmt.Errorf("read todo before patch: %w", err)
	}
	patched, err := client.Patch(ctx, "Todos", todoHigh, map[string]any{
		"$": todoRR.SOT["$"],
		"#": todoRR.SOT["#"],
		"RFC6902": []any{
			map[string]any{"op": "replace", "path": "/status", "value": "done"},
		},
	})
	if err != nil {
		return fmt.Errorf("patch todo: %w", err)
	}
	beforeVer := asString(todoRR.SOT["#"])
	if patched.Version == "" {
		return fmt.Errorf("expected versions.after after patch, got empty Version (raw=%v)", patched.Result.Raw["versions"])
	}
	if patched.Version == beforeVer {
		return fmt.Errorf("expected new version after patch, still %q", patched.Version)
	}
	if patched.VersionBefore != "" && patched.VersionBefore != beforeVer {
		return fmt.Errorf("versions.before=%q want %q", patched.VersionBefore, beforeVer)
	}
	todoRR, err = client.Read(ctx, "Todos", todoHigh, &datorium.ReadOptions{CacheSummaries: true})
	if err != nil {
		return fmt.Errorf("read todo after patch: %w", err)
	}
	if todoRR.SOT["status"] != "done" {
		return fmt.Errorf("status=%#v want done", todoRR.SOT["status"])
	}
	detail("status open → done; version %s → %s", beforeVer, patched.Version)

	step("WAIT_SEARCH")
	segs := searchpath.EqualsStringSegments("done")
	if err := waitSearch(ctx, client, "Todos", "byStatus", map[string]any{"status": "done"}, segs, todoHigh, 45*time.Second); err != nil {
		return err
	}
	detail("search Todos.byStatus matched %s", todoHigh)

	step("DELETING_TODO")
	if _, err := client.Delete(ctx, "Todos", todoHigh, map[string]any{
		"$": todoRR.SOT["$"],
		"#": todoRR.SOT["#"],
	}); err != nil {
		// Version may have advanced if something else touched it; re-read once.
		todoRR, rerr := client.Read(ctx, "Todos", todoHigh, nil)
		if rerr != nil {
			return fmt.Errorf("delete todo: %w (re-read: %v)", err, rerr)
		}
		if _, err := client.Delete(ctx, "Todos", todoHigh, map[string]any{
			"$": todoRR.SOT["$"],
			"#": todoRR.SOT["#"],
		}); err != nil {
			return fmt.Errorf("delete todo retry: %w", err)
		}
	}
	_, err = client.Read(ctx, "Todos", todoHigh, nil)
	if !datorium.IsAppCode(err, datorium.CodeDocumentNotFound) {
		return fmt.Errorf("expected documentNotFound after delete, got %v", err)
	}
	detail("delete confirmed (documentNotFound)")
	return nil
}

func waitReady(ctx context.Context, client *datorium.Client, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		res, err := client.Ready(ctx)
		if err == nil && res.OK {
			if ready, _ := res.Raw["ready"].(bool); ready {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("server not ready within %s", timeout)
}

// waitUserFrontPageTitle reads Users/{userID} with cacheSummaries until the
// ordered todoLists front-page summaries include listID with the wanted title.
func waitUserFrontPageTitle(ctx context.Context, client *datorium.Client, userID, listID, wantTitle string, timeout, interval time.Duration) error {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	deadline := time.Now().Add(timeout)
	var last string
	for {
		rr, err := client.Read(ctx, "Users", userID, &datorium.ReadOptions{CacheSummaries: true})
		if err != nil {
			last = err.Error()
		} else {
			sums, err := rr.SummariesForArrayField("todoLists")
			if err != nil {
				last = err.Error()
			} else {
				last = fmt.Sprintf("todoLists=%v cacheSummaries=%v summaries=%v",
					rr.SOT["todoLists"], rr.CacheSummaries, sums)
				for _, sum := range sums {
					if asString(sum["!"]) == listID && asString(sum["title"]) == wantTitle && sum["#"] != nil {
						return nil
					}
				}
			}
		}
		if !time.Now().Before(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	return fmt.Errorf("Users/%s front page missing TodoLists/%s title %q within %s (last=%s). If cacheSummaries is empty/nil while todoLists has @@ refs, rebuild the Compose image from a datoriumdb checkout that includes recursive FindRefFields (array cached refs)", userID, listID, wantTitle, timeout, last)
}

// waitCacheSummary re-reads the referring document with cacheSummaries until
// the referenced summary field matches want (client API only).
func waitCacheSummary(ctx context.Context, client *datorium.Client, collection, id, refColl, refID, field, want string, timeout, interval time.Duration) error {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	deadline := time.Now().Add(timeout)
	var last string
	for {
		rr, err := client.Read(ctx, collection, id, &datorium.ReadOptions{CacheSummaries: true})
		if err != nil {
			last = err.Error()
		} else if coll, ok := rr.CacheSummaries[refColl].(map[string]any); ok {
			if sum, ok := coll[refID].(map[string]any); ok {
				last = fmt.Sprintf("%v", sum)
				if asString(sum[field]) == want && sum["#"] != nil {
					return nil
				}
			} else {
				last = fmt.Sprintf("no summary for %s/%s in %#v", refColl, refID, rr.CacheSummaries)
			}
		} else {
			last = fmt.Sprintf("no cacheSummaries.%s (raw=%v)", refColl, rr.CacheSummaries)
		}
		if !time.Now().Before(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	return fmt.Errorf("cache summary %s/%s.%s != %q within %s (last=%s)", refColl, refID, field, want, timeout, last)
}

func waitSearch(ctx context.Context, client *datorium.Client, collection, name string, vars map[string]any, segs []string, wantID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last string
	for time.Now().Before(deadline) {
		sr, err := client.Search(ctx, collection, name, vars, segs)
		if err != nil {
			last = err.Error()
		} else {
			last = strings.Join(sr.Matches, ",")
			for _, m := range sr.Matches {
				if m == wantID {
					return nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("search %s.%s missing %s within %s (last=%s)", collection, name, wantID, timeout, last)
}

func findID(start, end byte) string {
	return findIDExcluding(start, end)
}

func findIDExcluding(start, end byte, exclude ...string) string {
	excl := map[string]bool{}
	for _, e := range exclude {
		excl[e] = true
	}
	for i := 0; i < 200000; i++ {
		id := fmt.Sprintf("todo%08d", i)
		if excl[id] {
			continue
		}
		s := shard.Slot(id)
		if s >= start && s <= end {
			return id
		}
	}
	panic("no id in range")
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func step(name string) {
	fmt.Printf("[%s]\n", name)
}

func detail(format string, args ...any) {
	fmt.Printf("  "+format+"\n", args...)
}
