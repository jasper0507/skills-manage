package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jasper0507/skills-manage/internal/infra/index"
	"github.com/jasper0507/skills-manage/internal/server"
	"github.com/jasper0507/skills-manage/internal/ui"
	"github.com/jasper0507/skills-manage/internal/workbench"
)

func writeSkill(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: test\n---\n\n# Body\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func openServer(t *testing.T, names ...string) (*server.Server, *workbench.Workbench) {
	t.Helper()
	root := t.TempDir()
	for _, n := range names {
		writeSkill(t, filepath.Join(root, n), n)
	}
	wb := workbench.New(workbench.Config{
		ScanRoots: []string{root},
		Index:     index.NewMemoryStore(),
	})
	if err := wb.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	return server.New(wb).WithStatic(ui.FS), wb
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	var out map[string]any
	if rr.Body.Len() > 0 {
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode %s %s body %q: %v", method, path, rr.Body.String(), err)
		}
	}
	return rr.Code, out
}

func deskPlaceholders(state map[string]any) []map[string]any {
	desk, _ := state["desk"].(map[string]any)
	raw, _ := desk["placeholders"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		if m, ok := r.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func findPhByName(state map[string]any, name string) map[string]any {
	for _, p := range deskPlaceholders(state) {
		if p["name"] == name {
			return p
		}
	}
	return nil
}

func TestGETState_ReturnsDeskFromWorkbench(t *testing.T) {
	srv, _ := openServer(t, "alpha", "beta")
	code, state := doJSON(t, srv.Handler(), http.MethodGet, "/api/state", nil)
	if code != http.StatusOK {
		t.Fatalf("status = %d, body=%v", code, state)
	}
	phs := deskPlaceholders(state)
	if len(phs) != 2 {
		t.Fatalf("placeholders = %d, want 2: %v", len(phs), phs)
	}
	names := map[string]bool{}
	for _, p := range phs {
		names[p["name"].(string)] = true
		if p["id"] == nil || p["id"] == "" {
			t.Error("placeholder missing id")
		}
		if p["identity"] == nil || p["identity"] == "" {
			t.Error("placeholder missing identity")
		}
	}
	if !names["alpha"] || !names["beta"] {
		t.Errorf("names = %v, want alpha and beta", names)
	}
	desk := state["desk"].(map[string]any)
	if desk["recycleIcon"] == nil {
		t.Error("missing recycleIcon")
	}
	// Recycle bin present (may be empty).
	if _, ok := state["recycleBin"]; !ok {
		t.Error("missing recycleBin")
	}
}

func TestPOSTMovePlaceholderToDesktop_CreatesBoxViaWorkbench(t *testing.T) {
	// Smoke: HTTP must not reimplement domain rules — icon→icon collision
	// creates a simple box only because Workbench does.
	srv, wb := openServer(t, "alpha", "beta")
	code, state := doJSON(t, srv.Handler(), http.MethodGet, "/api/state", nil)
	if code != http.StatusOK {
		t.Fatalf("GET state: %d %v", code, state)
	}
	alpha := findPhByName(state, "alpha")
	beta := findPhByName(state, "beta")
	if alpha == nil || beta == nil {
		t.Fatalf("missing phs: alpha=%v beta=%v", alpha, beta)
	}
	aLoc := alpha["location"].(map[string]any)
	row := int(aLoc["row"].(float64))
	col := int(aLoc["col"].(float64))

	code, state = doJSON(t, srv.Handler(), http.MethodPost, "/api/placeholders/move-desktop", map[string]any{
		"placeholderId": beta["id"],
		"row":           row,
		"col":           col,
	})
	if code != http.StatusOK {
		t.Fatalf("move-desktop status = %d, body=%v", code, state)
	}
	desk := state["desk"].(map[string]any)
	boxes, _ := desk["boxes"].([]any)
	if len(boxes) != 1 {
		t.Fatalf("via HTTP: got %d boxes, want 1 (Workbench collision rule): %v", len(boxes), boxes)
	}
	// Same result visible on Workbench directly (single source of truth).
	if n := len(wb.Desk().Boxes); n != 1 {
		t.Fatalf("Workbench.Desk().Boxes = %d, want 1", n)
	}
	box := boxes[0].(map[string]any)
	if box["kind"] != "simple" {
		t.Errorf("box kind = %v, want simple", box["kind"])
	}
	items, _ := box["items"].([]any)
	if len(items) != 2 {
		t.Errorf("box items = %d, want 2 (icons visible after drop)", len(items))
	}
}

func TestPOSTTrashPlan_MapsToWorkbenchR2(t *testing.T) {
	srv, _ := openServer(t, "alpha")
	code, state := doJSON(t, srv.Handler(), http.MethodGet, "/api/state", nil)
	if code != http.StatusOK {
		t.Fatal(code, state)
	}
	alpha := findPhByName(state, "alpha")
	// Last live placeholder → skipped (R2), not body-delete.
	code, plan := doJSON(t, srv.Handler(), http.MethodPost, "/api/trash/plan", map[string]any{
		"placeholderIds": []string{alpha["id"].(string)},
	})
	if code != http.StatusOK {
		t.Fatalf("plan status = %d %v", code, plan)
	}
	// Empty branches encode as JSON arrays, not null (stable frontend contract).
	enter, ok := plan["enterBinIds"].([]any)
	if !ok {
		t.Fatalf("enterBinIds type = %T, want array (not null)", plan["enterBinIds"])
	}
	if len(enter) != 0 {
		t.Fatalf("enterBinIds = %v, want empty for last live", enter)
	}
	skipped, ok := plan["skippedIds"].([]any)
	if !ok {
		t.Fatalf("skippedIds type = %T, want array (not null)", plan["skippedIds"])
	}
	if len(skipped) != 1 {
		t.Fatalf("skippedIds = %v, want 1 (last live)", skipped)
	}
	// Body-lifecycle fields must not be required for R2.
	if _, has := plan["bodyItems"]; has {
		t.Error("plan must not expose bodyItems (body-delete stripped)")
	}
}

// HTTP smoke: non-last enter-bin → restore → empty match R2 Workbench post-conditions.
func TestHTTP_RecycleTrashRestoreEmpty_R2(t *testing.T) {
	srv, wb := openServer(t, "alpha")
	code, state := doJSON(t, srv.Handler(), http.MethodGet, "/api/state", nil)
	if code != http.StatusOK {
		t.Fatal(code, state)
	}
	alpha := findPhByName(state, "alpha")
	alphaID := alpha["id"].(string)
	identity := alpha["identity"].(string)

	// Copy so we have a non-last placeholder to enter the bin.
	code, state = doJSON(t, srv.Handler(), http.MethodPost, "/api/clipboard/set", map[string]any{
		"mode":           "copy",
		"placeholderIds": []string{alphaID},
	})
	if code != http.StatusOK {
		t.Fatalf("clipboard set: %d %v", code, state)
	}
	code, state = doJSON(t, srv.Handler(), http.MethodPost, "/api/clipboard/paste-desktop", map[string]any{
		"row": 5,
		"col": 2,
	})
	if code != http.StatusOK {
		t.Fatalf("paste: %d %v", code, state)
	}
	var copyID string
	for _, p := range deskPlaceholders(state) {
		if p["id"] != alphaID {
			copyID = p["id"].(string)
			break
		}
	}
	if copyID == "" {
		t.Fatal("missing copy after paste")
	}

	code, state = doJSON(t, srv.Handler(), http.MethodPost, "/api/trash/confirm", map[string]any{
		"placeholderIds": []string{copyID},
	})
	if code != http.StatusOK {
		t.Fatalf("confirm trash: %d %v", code, state)
	}
	bin, _ := state["recycleBin"].([]any)
	if len(bin) != 1 {
		t.Fatalf("recycleBin after confirm = %v, want 1", bin)
	}
	entry := bin[0].(map[string]any)
	if entry["id"] != copyID {
		t.Errorf("bin id = %v, want %s", entry["id"], copyID)
	}
	// No body lifecycle fields required.
	if entry["quarantinePath"] != nil && entry["quarantinePath"] != "" {
		t.Errorf("R2 bin must not require quarantinePath: %v", entry)
	}
	// Package still on disk; Workbench agrees.
	if _, err := os.Stat(filepath.Join(identity, "SKILL.md")); err != nil {
		t.Fatalf("package missing after icon trash: %v", err)
	}
	if n := len(wb.RecycleBin()); n != 1 {
		t.Fatalf("Workbench bin = %d, want 1", n)
	}

	code, state = doJSON(t, srv.Handler(), http.MethodPost, "/api/recycle/restore", map[string]any{
		"entryId": copyID,
	})
	if code != http.StatusOK {
		t.Fatalf("restore: %d %v", code, state)
	}
	bin, _ = state["recycleBin"].([]any)
	if len(bin) != 0 {
		t.Fatalf("bin after restore = %v, want empty", bin)
	}
	foundDesktop := false
	for _, p := range deskPlaceholders(state) {
		if p["id"] == copyID {
			loc := p["location"].(map[string]any)
			if loc["kind"] != "desktop" {
				t.Errorf("restored kind = %v, want desktop", loc["kind"])
			}
			foundDesktop = true
		}
	}
	if !foundDesktop {
		t.Error("restored placeholder not on desk")
	}

	// Re-trash and empty.
	code, state = doJSON(t, srv.Handler(), http.MethodPost, "/api/trash/confirm", map[string]any{
		"placeholderIds": []string{copyID},
	})
	if code != http.StatusOK {
		t.Fatalf("re-trash: %d %v", code, state)
	}
	code, state = doJSON(t, srv.Handler(), http.MethodPost, "/api/recycle/empty", nil)
	if code != http.StatusOK {
		t.Fatalf("empty: %d %v", code, state)
	}
	bin, _ = state["recycleBin"].([]any)
	if len(bin) != 0 {
		t.Fatalf("bin after empty = %v", bin)
	}
	if _, err := os.Stat(filepath.Join(identity, "SKILL.md")); err != nil {
		t.Fatalf("package deleted by empty: %v", err)
	}
	if n := len(wb.RecycleBin()); n != 0 {
		t.Fatalf("Workbench bin after empty = %d", n)
	}
}

func TestStaticIndex_EmbeddedWithoutResetButton(t *testing.T) {
	srv, _ := openServer(t, "alpha")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET / status = %d", rr.Code)
	}
	body := rr.Body.String()
	if body == "" {
		t.Fatal("empty index.html")
	}
	// Finished product: no end-user reset-layout control.
	if bytes.Contains(rr.Body.Bytes(), []byte("btn-reset")) ||
		bytes.Contains(rr.Body.Bytes(), []byte("重置（仅测试）")) {
		t.Error("embedded UI must not include reset-layout test control")
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("desktop")) {
		t.Error("embedded UI should include desktop surface")
	}
	// Assets must be embed-served (not only the shell HTML).
	for _, path := range []string{"/app.js", "/styles.css"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("GET %s status = %d", path, rr.Code)
		}
		if rr.Body.Len() == 0 {
			t.Errorf("GET %s empty body", path)
		}
	}
}

func TestPOSTMoveManyDesktop_PlacesWithoutAutoBox(t *testing.T) {
	// Multi-select drop on empty desktop must free-cell place, not serial auto-box.
	srv, _ := openServer(t, "alpha", "beta")
	code, state := doJSON(t, srv.Handler(), http.MethodGet, "/api/state", nil)
	if code != http.StatusOK {
		t.Fatal(code, state)
	}
	alpha := findPhByName(state, "alpha")
	beta := findPhByName(state, "beta")
	// Drop both onto a free cell far from the default stack (col 5).
	code, state = doJSON(t, srv.Handler(), http.MethodPost, "/api/placeholders/move-many-desktop", map[string]any{
		"placeholderIds": []string{alpha["id"].(string), beta["id"].(string)},
		"row":            2,
		"col":            5,
	})
	if code != http.StatusOK {
		t.Fatalf("move-many-desktop status = %d %v", code, state)
	}
	desk := state["desk"].(map[string]any)
	boxes, _ := desk["boxes"].([]any)
	if len(boxes) != 0 {
		t.Fatalf("got %d boxes, want 0 (free multi place): %v", len(boxes), boxes)
	}
	phs := deskPlaceholders(state)
	if len(phs) != 2 {
		t.Fatalf("placeholders = %d, want 2", len(phs))
	}
	for _, p := range phs {
		loc := p["location"].(map[string]any)
		if loc["kind"] != "desktop" {
			t.Errorf("%s kind = %v, want desktop", p["name"], loc["kind"])
		}
	}
}

func TestPOSTMoveRecycle_MapsToWorkbench(t *testing.T) {
	srv, wb := openServer(t, "alpha")
	code, state := doJSON(t, srv.Handler(), http.MethodPost, "/api/recycle/move-desktop", map[string]any{
		"row": 3,
		"col": 4,
	})
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%v", code, state)
	}
	desk := state["desk"].(map[string]any)
	ri := desk["recycleIcon"].(map[string]any)
	loc := ri["location"].(map[string]any)
	if int(loc["row"].(float64)) != 3 || int(loc["col"].(float64)) != 4 {
		t.Fatalf("recycle location = %v, want (3,4)", loc)
	}
	if r := wb.Desk().RecycleIcon.Location; r.Row != 3 || r.Col != 4 {
		t.Fatalf("Workbench recycle = %+v", r)
	}
}

func TestPOSTUnknownAction_DoesNotPanic(t *testing.T) {
	srv, _ := openServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/no-such", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 404/405, body=%q", rr.Code, rr.Body.String())
	}
}
