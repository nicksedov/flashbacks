package httpapi_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/flashbacks/registry/internal/store"
	"github.com/flashbacks/registry/internal/testutil"
)

func doRequest(t *testing.T, method, url, body string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func readJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func TestConfigCRUD(t *testing.T) {
	stack := testutil.Start(t)
	base := stack.Server.URL

	if resp := doRequest(t, http.MethodPut, base+"/registry/v1/config/exif/db_host", `{"value":"db.internal"}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT db_host status = %d", resp.StatusCode)
	}
	if resp := doRequest(t, http.MethodPut, base+"/registry/v1/config/exif/db_port", `{"value":"5432"}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT db_port status = %d", resp.StatusCode)
	}

	resp := doRequest(t, http.MethodGet, base+"/registry/v1/config/exif/db_host", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET db_host status = %d", resp.StatusCode)
	}
	got := readJSON[struct {
		Value string `json:"value"`
	}](t, resp)
	if got.Value != "db.internal" {
		t.Fatalf("GET db_host = %q, want db.internal", got.Value)
	}

	resp = doRequest(t, http.MethodGet, base+"/registry/v1/config/exif", "")
	list := readJSON[struct {
		Values map[string]string `json:"values"`
	}](t, resp)
	if list.Values["db_host"] != "db.internal" || list.Values["db_port"] != "5432" {
		t.Fatalf("list values = %v", list.Values)
	}

	if resp := doRequest(t, http.MethodDelete, base+"/registry/v1/config/exif/db_host", ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE db_host status = %d", resp.StatusCode)
	}
	if resp := doRequest(t, http.MethodGet, base+"/registry/v1/config/exif/db_host", ""); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET deleted db_host status = %d, want 404", resp.StatusCode)
	}
}

func TestConfigWatchNotifies(t *testing.T) {
	stack := testutil.Start(t)
	base := stack.Server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/registry/v1/config/exif/watch", nil)
	if err != nil {
		t.Fatalf("create watch request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open watch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("watch status = %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	waitForEventType(t, scanner, "ready")

	if putResp := doRequest(t, http.MethodPut, base+"/registry/v1/config/exif/db_host", `{"value":"watched"}`); putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d", putResp.StatusCode)
	}

	event := waitForEventType(t, scanner, "put")
	if !strings.HasSuffix(event.Key, "/db_host") || event.Value != "watched" {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestSecretsEncryptedAtRest(t *testing.T) {
	stack := testutil.Start(t)
	base := stack.Server.URL
	const plaintext = "s3cret-password"

	if resp := doRequest(t, http.MethodPut, base+"/registry/v1/secrets/exif/db_password", `{"value":"`+plaintext+`"}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT secret status = %d", resp.StatusCode)
	}

	resp := doRequest(t, http.MethodGet, base+"/registry/v1/secrets/exif/db_password", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET secret status = %d", resp.StatusCode)
	}
	got := readJSON[struct {
		Value string `json:"value"`
	}](t, resp)
	if got.Value != plaintext {
		t.Fatalf("GET secret = %q, want plaintext", got.Value)
	}

	raw, found, err := stack.Store.Get(context.Background(), store.SecretKey("exif", "db_password"))
	if err != nil {
		t.Fatalf("read raw secret: %v", err)
	}
	if !found {
		t.Fatal("secret was not persisted")
	}
	if raw == plaintext || strings.Contains(raw, plaintext) {
		t.Fatalf("persisted secret is not encrypted: %q", raw)
	}
}

func TestServiceRegistrationAndDiscovery(t *testing.T) {
	stack := testutil.Start(t)
	base := stack.Server.URL

	resp := doRequest(t, http.MethodPut, base+"/registry/v1/services/exif/exif-1?ttl=15", `{"host":"127.0.0.1","port":5172,"basePath":"/exif","healthPath":"/exif/health"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register status = %d", resp.StatusCode)
	}
	readJSON[map[string]any](t, resp)

	instances := listInstances(t, base, "exif")
	if len(instances) != 1 || instances[0].Host != "127.0.0.1" || instances[0].Port != 5172 {
		t.Fatalf("instances = %+v", instances)
	}

	if resp := doRequest(t, http.MethodDelete, base+"/registry/v1/services/exif/exif-1", ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("deregister status = %d", resp.StatusCode)
	}
	if got := listInstances(t, base, "exif"); len(got) != 0 {
		t.Fatalf("instances after deregister = %+v", got)
	}
}

func TestServiceLeaseExpiry(t *testing.T) {
	stack := testutil.Start(t)
	base := stack.Server.URL

	resp := doRequest(t, http.MethodPut, base+"/registry/v1/services/ocr/ocr-1?ttl=1", `{"host":"127.0.0.1","port":5174}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register status = %d", resp.StatusCode)
	}
	readJSON[map[string]any](t, resp)

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if len(listInstances(t, base, "ocr")) == 0 {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("registration did not expire after its TTL")
}

func TestHealthReportsReady(t *testing.T) {
	stack := testutil.Start(t)
	resp := doRequest(t, http.MethodGet, stack.Server.URL+"/registry/v1/health", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", resp.StatusCode)
	}
	got := readJSON[struct {
		Status string `json:"status"`
	}](t, resp)
	if got.Status != "ok" {
		t.Fatalf("health status = %q", got.Status)
	}
}

// TestDeregisterRequiresMatchingLease ensures a shutting-down instance cannot
// delete the registration of a replacement that reused the same instance ID.
func TestDeregisterRequiresMatchingLease(t *testing.T) {
	stack := testutil.Start(t)
	base := stack.Server.URL

	resp := doRequest(t, http.MethodPut, base+"/registry/v1/services/ocr/ocr?ttl=60", `{"host":"flashbacks-ocr","port":5174}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register status = %d", resp.StatusCode)
	}
	registered := readJSON[struct {
		LeaseID int64 `json:"leaseId"`
	}](t, resp)
	if registered.LeaseID == 0 {
		t.Fatal("register response carried no leaseId")
	}

	stale := doRequest(t, http.MethodDelete, fmt.Sprintf("%s/registry/v1/services/ocr/ocr?leaseId=%d", base, registered.LeaseID+1), "")
	if stale.StatusCode != http.StatusConflict {
		t.Fatalf("deregistration with a foreign lease status = %d, want 409", stale.StatusCode)
	}
	stale.Body.Close()
	if got := listInstances(t, base, "ocr"); len(got) != 1 {
		t.Fatalf("instance removed by a foreign lease: %+v", got)
	}

	owner := doRequest(t, http.MethodDelete, fmt.Sprintf("%s/registry/v1/services/ocr/ocr?leaseId=%d", base, registered.LeaseID), "")
	if owner.StatusCode != http.StatusOK {
		t.Fatalf("deregistration by the owning lease status = %d", owner.StatusCode)
	}
	owner.Body.Close()
	if got := listInstances(t, base, "ocr"); len(got) != 0 {
		t.Fatalf("instance still listed after owner deregistration: %+v", got)
	}
}

type instanceJSON struct {
	ID         string `json:"id"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	BasePath   string `json:"basePath"`
	HealthPath string `json:"healthPath"`
}

func listInstances(t *testing.T, base, service string) []instanceJSON {
	t.Helper()
	resp := doRequest(t, http.MethodGet, base+"/registry/v1/services/"+service, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list instances status = %d", resp.StatusCode)
	}
	body := readJSON[struct {
		Instances []instanceJSON `json:"instances"`
	}](t, resp)
	return body.Instances
}

func waitForEventType(t *testing.T, scanner *bufio.Scanner, wantType string) store.Event {
	t.Helper()
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event store.Event
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			continue
		}
		if string(event.Type) == wantType {
			return event
		}
	}
	t.Fatalf("did not observe event type %q: %v", wantType, scanner.Err())
	return store.Event{}
}

func Example_watchStream() {
	fmt.Println("watch streams are served as text/event-stream")
	// Output: watch streams are served as text/event-stream
}
