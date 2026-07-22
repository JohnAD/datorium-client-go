// Command todo-integration exercises the smart client against a live
// two-shard Todo establishment started by start_integration_test.sh.
//
// Each document operation is covered in both forms:
//   - raw Client.Create/Read/Patch/Delete (order-unsafe escape hatch)
//   - typed CollectionClient methods after Collection.Bind
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
	"github.com/JohnAD/ojson"
)

type User struct {
	DisplayName string   `json:"displayName"`
	Email       string   `json:"email"`
	TodoLists   []string `json:"todoLists"`
}

type TodoList struct {
	Title        string `json:"title"`
	Owner        string `json:"owner"`
	OwnerSummary string `json:"ownerSummary"`
}

type Todo struct {
	Title       string `json:"title"`
	Status      string `json:"status"`
	List        string `json:"list"`
	ListSummary string `json:"listSummary"`
}

var (
	Users     = datorium.MustCollection[User]("Users", 0)
	TodoLists = datorium.MustCollection[TodoList]("TodoLists", 0)
	Todos     = datorium.MustCollection[Todo]("Todos", 0)
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
	if err := client.Establish(ctx, Users, TodoLists, Todos); err != nil {
		return fmt.Errorf("establish: %w", err)
	}
	est := client.CachedEstablishment()
	if est == nil || est.General.Version < 1 {
		return fmt.Errorf("establishment cache missing")
	}
	detail("establishment %q version %d", est.General.Name, est.General.Version)

	step("BIND_TYPED")
	users, err := Users.Bind(client)
	if err != nil {
		return fmt.Errorf("bind Users: %w", err)
	}
	todoLists, err := TodoLists.Bind(client)
	if err != nil {
		return fmt.Errorf("bind TodoLists: %w", err)
	}
	todos, err := Todos.Bind(client)
	if err != nil {
		return fmt.Errorf("bind Todos: %w", err)
	}
	detail("bound Users, TodoLists, Todos collection clients")

	step("PICK_IDS")
	userLow := findID(0x00, 0x7F)
	userHigh := findID(0x80, 0xFF)
	listLow := findIDExcluding(0x00, 0x7F, userLow)
	todoHigh := findIDExcluding(0x80, 0xFF, userHigh)
	todoTyped := findIDExcluding(0x80, 0xFF, userHigh, todoHigh)
	detail("userLow=%s(%s) userHigh=%s(%s) listLow=%s(%s) todoHigh=%s(%s) todoTyped=%s(%s)",
		userLow, shard.SlotHex(userLow), userHigh, shard.SlotHex(userHigh),
		listLow, shard.SlotHex(listLow), todoHigh, shard.SlotHex(todoHigh),
		todoTyped, shard.SlotHex(todoTyped))

	// --- Create: raw + typed ---
	step("CREATING_USERS_RAW")
	if _, err := client.Create(ctx, "Users", userLow, map[string]any{
		"$": "Users:0", "displayName": "Ada", "email": "ada@example.com", "todoLists": []any{},
	}); err != nil {
		return fmt.Errorf("raw create userLow: %w", err)
	}
	detail("raw Create Users/%s (Ada)", userLow)

	step("CREATING_USERS_TYPED")
	graceID := userHigh
	if _, err := users.CreateDoc(ctx, &graceID, User{
		DisplayName: "Grace",
		Email:       "grace@example.com",
		TodoLists:   []string{},
	}); err != nil {
		return fmt.Errorf("typed create userHigh: %w", err)
	}
	detail("typed CreateDoc Users/%s (Grace)", userHigh)

	step("CREATING_LIST_TYPED")
	ownerDirect := refs.FormatDirect("Users", userHigh)
	ownerCached := refs.FormatCached("Users", userHigh)
	const listTitle = "Ship client"
	listID := listLow
	listWR, err := todoLists.CreateDoc(ctx, &listID, TodoList{
		Title:        listTitle,
		Owner:        ownerDirect,
		OwnerSummary: ownerCached,
	})
	if err != nil {
		return fmt.Errorf("typed create list: %w", err)
	}
	detail("typed CreateDoc TodoLists/%s title=%q", listLow, listTitle)

	// --- Patch: raw helper for front-page append ---
	step("LINK_LIST_TO_USER_RAW")
	ownerRR, err := client.Read(ctx, "Users", userHigh, nil)
	if err != nil {
		return fmt.Errorf("raw read owner before linking list: %w", err)
	}
	if _, err := client.Patch(ctx, "Users", userHigh, datorium.PatchDetailAppendingCachedRef(
		ownerRR.SOT.Get("$").ToStringOrEmpty(), ownerRR.SOT.Get("#").ToStringOrEmpty(),
		"todoLists", "TodoLists", listLow,
	)); err != nil {
		return fmt.Errorf("raw append todoLists cached ref: %w", err)
	}
	detail("raw Patch appended @@__TodoLists__%s onto Users/%s.todoLists", listLow, userHigh)

	step("READ_USER_FRONT_PAGE")
	if err := waitUserFrontPageTitle(ctx, client, userHigh, listLow, listTitle, 45*time.Second, 2*time.Second); err != nil {
		return fmt.Errorf("user front page initial: %w", err)
	}
	detail("raw Read Users/%s with cacheSummaries; todoLists front page shows title %q", userHigh, listTitle)

	// --- Patch: typed CreatePatchFromChanges ---
	step("PATCH_LIST_TITLE_TYPED")
	listItem, err := todoLists.GetDoc(ctx, listLow)
	if err != nil {
		return fmt.Errorf("typed get list before title patch: %w", err)
	}
	const updatedListTitle = "Ship client v2"
	listItem.Doc.Title = updatedListTitle
	listPatch, err := todoLists.CreatePatchFromChanges(listItem)
	if err != nil {
		return fmt.Errorf("typed create patch for list title: %w", err)
	}
	if _, err := todoLists.PatchDoc(ctx, listPatch); err != nil {
		return fmt.Errorf("typed patch list title: %w", err)
	}
	detail("typed CreatePatchFromChanges + PatchDoc TodoLists/%s title → %q", listLow, updatedListTitle)

	step("WAIT_FRONT_PAGE_UPDATE")
	if err := waitUserFrontPageTitle(ctx, client, userHigh, listLow, updatedListTitle, 15*time.Second, 2*time.Second); err != nil {
		return fmt.Errorf("user front page after list patch: %w", err)
	}
	detail("re-read Users/%s; cached TodoLists/%s.title now %q", userHigh, listLow, updatedListTitle)

	// --- Read: typed GetDoc + raw ResolveDirectRef ---
	step("RESOLVE_LIVE_REF")
	listItem, err = todoLists.GetDocOpts(ctx, listLow, &datorium.ReadOptions{CacheSummaries: true})
	if err != nil {
		return fmt.Errorf("typed get list: %w", err)
	}
	if listItem.Doc.Owner != ownerDirect {
		return fmt.Errorf("expected owner %q, got %#v", ownerDirect, listItem.Doc.Owner)
	}
	ownerRR, err = client.ResolveDirectRef(ctx, listItem.Doc.Owner, nil)
	if err != nil {
		return fmt.Errorf("resolve owner: %w", err)
	}
	if ownerRR.SOT.Get("displayName").ToStringOrEmpty() != "Grace" {
		return fmt.Errorf("resolved owner displayName=%#v", ownerRR.SOT.Get("displayName"))
	}
	detail("typed GetDocOpts + raw ResolveDirectRef → Grace")

	// Owner summary on the list. Important: pending cache-updates are
	// completed as no-ops if the read member has no stub yet, so we must
	// (1) read the referring list with cacheSummaries to create the stub,
	// then (2) patch the User so a fresh work item can fill it.
	step("READ_CACHED_REF")
	seedRR, err := client.Read(ctx, "TodoLists", listLow, &datorium.ReadOptions{CacheSummaries: true})
	if err != nil {
		return fmt.Errorf("seed list cacheSummaries read: %w", err)
	}
	if sum := seedRR.CacheSummaries.Get("Users").Get(userHigh); sum.IsObject() {
		detail("raw seed read TodoLists/%s; Users/%s summary=%s", listLow, userHigh, sum.ToJSON())
	} else {
		detail("raw seed read TodoLists/%s; no Users cacheSummaries yet", listLow)
	}

	// --- Patch: typed hand-built ojson ops ---
	step("PATCH_CACHED_TARGET_TYPED")
	ownerItem, err := users.GetDoc(ctx, userHigh)
	if err != nil {
		return fmt.Errorf("typed get owner before patch: %w", err)
	}
	const updatedName = "Grace Hopper"
	op, err := ojson.NewPatch(ojson.PatchReplace("/displayName", ojson.NewString(updatedName)))
	if err != nil {
		return fmt.Errorf("build owner patch: %w", err)
	}
	ownerPatch, err := users.CreatePatch(ownerItem, op)
	if err != nil {
		return fmt.Errorf("typed CreatePatch owner displayName: %w", err)
	}
	if _, err := users.PatchDoc(ctx, ownerPatch); err != nil {
		return fmt.Errorf("typed PatchDoc owner displayName: %w", err)
	}
	detail("typed CreatePatch + PatchDoc Users/%s displayName → %q", userHigh, updatedName)

	step("WAIT_CACHE_UPDATE")
	if err := waitCacheSummary(ctx, client, "TodoLists", listLow, "Users", userHigh, "displayName", updatedName, 15*time.Second, 2*time.Second); err != nil {
		return fmt.Errorf("cached summary did not observe owner patch: %w", err)
	}
	detail("re-read TodoLists/%s; cached Users/%s.displayName now %q", listLow, userHigh, updatedName)

	// --- Create/read/patch/delete Todos: raw path for search story ---
	step("CREATING_TODO_RAW")
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
		return fmt.Errorf("raw create todo: %w", err)
	}
	_ = listWR
	_ = todoWR
	detail("raw Create Todos/%s status=open", todoHigh)

	step("PATCHING_TODO_RAW")
	todoRR, err := client.Read(ctx, "Todos", todoHigh, nil)
	if err != nil {
		return fmt.Errorf("raw read todo before patch: %w", err)
	}
	patched, err := client.Patch(ctx, "Todos", todoHigh, map[string]any{
		"$": todoRR.SOT.Get("$").ToStringOrEmpty(),
		"#": todoRR.SOT.Get("#").ToStringOrEmpty(),
		"RFC6902": []any{
			map[string]any{"op": "replace", "path": "/status", "value": "done"},
		},
	})
	if err != nil {
		return fmt.Errorf("raw patch todo: %w", err)
	}
	beforeVer := todoRR.SOT.Get("#").ToStringOrEmpty()
	if patched.Version == "" {
		return fmt.Errorf("expected versions.after after raw patch, got empty Version (env=%s)", patched.Result.ValueField("versions").ToJSON())
	}
	if patched.Version == beforeVer {
		return fmt.Errorf("expected new version after raw patch, still %q", patched.Version)
	}
	if patched.VersionBefore != "" && patched.VersionBefore != beforeVer {
		return fmt.Errorf("versions.before=%q want %q", patched.VersionBefore, beforeVer)
	}
	todoRR, err = client.Read(ctx, "Todos", todoHigh, &datorium.ReadOptions{CacheSummaries: true})
	if err != nil {
		return fmt.Errorf("raw read todo after patch: %w", err)
	}
	if todoRR.SOT.Get("status").ToStringOrEmpty() != "done" {
		return fmt.Errorf("status=%#v want done", todoRR.SOT.Get("status"))
	}
	detail("raw Patch status open → done; version %s → %s", beforeVer, patched.Version)

	step("WAIT_SEARCH")
	segs := searchpath.EqualsStringSegments("done")
	if err := waitSearch(ctx, client, "Todos", "byStatus", map[string]any{"status": "done"}, segs, todoHigh, 45*time.Second); err != nil {
		return err
	}
	detail("search Todos.byStatus matched %s", todoHigh)

	step("DELETING_TODO_RAW")
	if _, err := client.Delete(ctx, "Todos", todoHigh, map[string]any{
		"$": todoRR.SOT.Get("$").ToStringOrEmpty(),
		"#": todoRR.SOT.Get("#").ToStringOrEmpty(),
	}); err != nil {
		todoRR, rerr := client.Read(ctx, "Todos", todoHigh, nil)
		if rerr != nil {
			return fmt.Errorf("raw delete todo: %w (re-read: %v)", err, rerr)
		}
		if _, err := client.Delete(ctx, "Todos", todoHigh, map[string]any{
			"$": todoRR.SOT.Get("$").ToStringOrEmpty(),
			"#": todoRR.SOT.Get("#").ToStringOrEmpty(),
		}); err != nil {
			return fmt.Errorf("raw delete todo retry: %w", err)
		}
	}
	_, err = client.Read(ctx, "Todos", todoHigh, nil)
	if !datorium.IsAppCode(err, datorium.CodeDocumentNotFound) {
		return fmt.Errorf("expected documentNotFound after raw delete, got %v", err)
	}
	detail("raw Delete confirmed (documentNotFound)")

	// --- Typed Todo create / get / patch-from-changes / delete ---
	step("TYPED_TODO_CRUD")
	typedID := todoTyped
	if _, err := todos.CreateDoc(ctx, &typedID, Todo{
		Title:       "Typed path coverage",
		Status:      "open",
		List:        listDirect,
		ListSummary: listCached,
	}); err != nil {
		return fmt.Errorf("typed create todo: %w", err)
	}
	todoItem, err := todos.GetDoc(ctx, todoTyped)
	if err != nil {
		return fmt.Errorf("typed get todo: %w", err)
	}
	if todoItem.Doc.Status != "open" || todoItem.OriginalDoc.Status != "open" {
		return fmt.Errorf("typed get todo status=%q original=%q", todoItem.Doc.Status, todoItem.OriginalDoc.Status)
	}
	todoItem.Doc.Status = "done"
	todoItem.OriginalDoc.Status = "corrupted" // must not affect private baseline
	typedPatch, err := todos.CreatePatchFromChanges(todoItem)
	if err != nil {
		return fmt.Errorf("typed CreatePatchFromChanges todo: %w", err)
	}
	typedPatched, err := todos.PatchDoc(ctx, typedPatch)
	if err != nil {
		return fmt.Errorf("typed PatchDoc todo: %w", err)
	}
	if typedPatched.Version == "" || typedPatched.Version == todoItem.Meta.Version {
		return fmt.Errorf("typed patch did not advance version: before=%q after=%q", todoItem.Meta.Version, typedPatched.Version)
	}
	todoItem, err = todos.GetDoc(ctx, todoTyped)
	if err != nil {
		return fmt.Errorf("typed get todo after patch: %w", err)
	}
	if todoItem.Doc.Status != "done" {
		return fmt.Errorf("typed todo status=%q want done", todoItem.Doc.Status)
	}
	if _, err := todos.DeleteDoc(ctx, todoItem); err != nil {
		todoItem, rerr := todos.GetDoc(ctx, todoTyped)
		if rerr != nil {
			return fmt.Errorf("typed delete todo: %w (re-get: %v)", err, rerr)
		}
		if _, err := todos.DeleteDoc(ctx, todoItem); err != nil {
			return fmt.Errorf("typed delete todo retry: %w", err)
		}
	}
	_, err = todos.GetDoc(ctx, todoTyped)
	if !datorium.IsAppCode(err, datorium.CodeDocumentNotFound) {
		return fmt.Errorf("expected documentNotFound after typed delete, got %v", err)
	}
	detail("typed CreateDoc/GetDoc/CreatePatchFromChanges/PatchDoc/DeleteDoc on Todos/%s", todoTyped)

	return nil
}

func waitReady(ctx context.Context, client *datorium.Client, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		res, err := client.Ready(ctx)
		if err == nil && res.OK {
			if ready, err := res.ValueField("ready").ToBoolTry(); err == nil && ready {
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
				last = fmt.Sprintf("todoLists=%s cacheSummaries=%s summaries=%v",
					rr.SOT.Get("todoLists").ToJSON(), rr.CacheSummaries.ToJSON(), sums)
				for _, sum := range sums {
					if sum.Get("!").ToStringOrEmpty() == listID &&
						sum.Get("title").ToStringOrEmpty() == wantTitle &&
						!sum.Get("#").IsMissing() && !sum.Get("#").IsNull() {
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
		} else if sum := rr.CacheSummaries.Get(refColl).Get(refID); sum.IsObject() {
			last = sum.ToJSON()
			if sum.Get(field).ToStringOrEmpty() == want && !sum.Get("#").IsMissing() && !sum.Get("#").IsNull() {
				return nil
			}
		} else if rr.CacheSummaries.Get(refColl).IsObject() {
			last = fmt.Sprintf("no summary for %s/%s in %s", refColl, refID, rr.CacheSummaries.ToJSON())
		} else {
			last = fmt.Sprintf("no cacheSummaries.%s (env=%s)", refColl, rr.CacheSummaries.ToJSON())
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

func step(name string) {
	fmt.Printf("[%s]\n", name)
}

func detail(format string, args ...any) {
	fmt.Printf("  "+format+"\n", args...)
}
