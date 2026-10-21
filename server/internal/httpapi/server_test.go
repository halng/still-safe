package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"nhooyr.io/websocket"
	"safecircle/internal/live"
	"safecircle/internal/store"
)

type manualClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *manualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *manualClock) Set(t time.Time) {
	c.mu.Lock()
	c.t = t
	c.mu.Unlock()
}

func (c *manualClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://circle:circle@127.0.0.1:5432/circle_test?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := store.Connect(ctx, url)
	if err != nil {
		panic(err)
	}
	if err := pool.Ping(ctx); err != nil {
		panic(err)
	}
	if err := store.New(pool).Migrate(ctx); err != nil {
		panic(err)
	}
	testPool = pool
	os.Exit(m.Run())
}

type fixture struct {
	srv   *Server
	clock *manualClock
	ts    *httptest.Server
	live  *live.Live
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	if _, err := testPool.Exec(context.Background(), `TRUNCATE users CASCADE`); err != nil {
		t.Fatal(err)
	}
	clk := &manualClock{t: time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)}
	var liveClient *live.Live
	if client, err := live.Open("redis://127.0.0.1:6379/0"); err == nil {
		liveClient = client
		t.Cleanup(func() { _ = client.Close() })
	}
	srv := New(Options{
		Store:      store.New(testPool),
		Live:       liveClient,
		UploadDir:  t.TempDir(),
		BcryptCost: bcrypt.MinCost,
		Now:        clk.Now,
		Quiet:      true,
	})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Listen(ctx)
	return &fixture{srv: srv, clock: clk, ts: ts, live: liveClient}
}

func (f *fixture) do(t *testing.T, method, path, tok string, body any) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, f.ts.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func decode(t *testing.T, res *http.Response) map[string]any {
	t.Helper()
	defer res.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func (f *fixture) register(t *testing.T, name string) string {
	t.Helper()
	res := f.do(t, http.MethodPost, "/v1/auth/register", "", map[string]string{
		"displayName": name,
		"passphrase":  "correct horse",
	})
	body := decode(t, res)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("register %s: %d %v", name, res.StatusCode, body)
	}
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatal("missing token")
	}
	return token
}

func (f *fixture) submitID(t *testing.T, tok, dob, number string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("legalName", "Ada Lovelace")
	_ = w.WriteField("dateOfBirth", dob)
	_ = w.WriteField("documentType", "passport")
	_ = w.WriteField("documentNumber", number)
	part, err := w.CreateFormFile("photo", "id.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(tinyPNG(t)); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	req, err := http.NewRequest(http.MethodPost, f.ts.URL+"/v1/me/id", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tok)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func (f *fixture) authOK(t *testing.T, name, dob, number string) (token, id string) {
	t.Helper()
	token = f.register(t, name)
	res := f.submitID(t, token, dob, number)
	body := decode(t, res)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("id %s: %d %v", name, res.StatusCode, body)
	}
	me := decode(t, f.do(t, http.MethodGet, "/v1/me", token, nil))
	id, _ = me["id"].(string)
	return token, id
}

func TestAuthAndAgeGate(t *testing.T) {
	f := newFixture(t)
	token := f.register(t, "Ada")
	bad := f.do(t, http.MethodPost, "/v1/auth/login", "", map[string]string{
		"displayName": "Ada",
		"passphrase":  "wrong-passphrase",
	})
	if bad.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad login %d", bad.StatusCode)
	}
	bad.Body.Close()

	dup := f.do(t, http.MethodPost, "/v1/auth/register", "", map[string]string{
		"displayName": "ada",
		"passphrase":  "correct horse",
	})
	if dup.StatusCode != http.StatusConflict {
		t.Fatalf("dup name %d", dup.StatusCode)
	}
	dup.Body.Close()

	young := f.submitID(t, token, "2010-10-21", "P0000001")
	youngBody := decode(t, young)
	if young.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("exact 16: %d %v", young.StatusCode, youngBody)
	}
	adult := f.submitID(t, token, "2010-10-20", "P0000001")
	adultBody := decode(t, adult)
	if adult.StatusCode != http.StatusOK {
		t.Fatalf("over 16: %d %v", adult.StatusCode, adultBody)
	}
	me := decode(t, f.do(t, http.MethodGet, "/v1/me", token, nil))
	idn := me["identity"].(map[string]any)
	if idn["dateOfBirth"] != "2010-10-20" || idn["status"] != "id_on_file" || idn["documentLast4"] != "0001" {
		t.Fatalf("identity %#v", idn)
	}
	if _, ok := idn["photoPath"]; ok {
		t.Fatal("photo path leaked")
	}

	selfInvite := decode(t, f.do(t, http.MethodPost, "/v1/circle/invites", token, nil))
	self := f.do(t, http.MethodPost, "/v1/invites/accept", token, map[string]string{"code": selfInvite["code"].(string)})
	if self.StatusCode != http.StatusConflict {
		t.Fatalf("self supervise %d", self.StatusCode)
	}
	self.Body.Close()

	blocked := f.do(t, http.MethodPost, "/v1/location", f.register(t, "Bea"), map[string]any{
		"latitude": 1.0, "longitude": 2.0, "accuracyM": 5.0,
	})
	if blocked.StatusCode != http.StatusForbidden {
		t.Fatalf("location before id %d", blocked.StatusCode)
	}
	blocked.Body.Close()
}

func TestSupervisorCapAndPrivacy(t *testing.T) {
	f := newFixture(t)
	ada, adaID := f.authOK(t, "Ada", "1990-01-02", "P1000001")
	bea, _ := f.authOK(t, "Bea", "1988-05-05", "P1000002")
	cam, _ := f.authOK(t, "Cam", "1985-03-03", "P1000003")

	res := f.do(t, http.MethodPut, "/v1/circle/limit", ada, map[string]any{"supervisorLimit": 1})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("limit %d", res.StatusCode)
	}
	res.Body.Close()

	invite := decode(t, f.do(t, http.MethodPost, "/v1/circle/invites", ada, map[string]any{}))
	code := invite["code"].(string)
	if len(code) != 8 {
		t.Fatalf("code %q", code)
	}
	accepted := decode(t, f.do(t, http.MethodPost, "/v1/invites/accept", bea, map[string]string{"code": code}))
	if accepted["personId"] != adaID {
		t.Fatalf("accept %#v", accepted)
	}

	self := f.do(t, http.MethodPost, "/v1/invites/accept", ada, map[string]string{"code": code})
	if self.StatusCode != http.StatusNotFound {
		t.Fatalf("reuse code %d", self.StatusCode)
	}
	self.Body.Close()

	own := decode(t, f.do(t, http.MethodPost, "/v1/circle/invites", ada, nil))
	// circle is full, invite should be refused
	if own["error"] == nil {
		t.Fatalf("expected full circle, got %#v", own)
	}

	// Raise the cap, then prove a stranger and a revoked supervisor cannot read.
	res = f.do(t, http.MethodPut, "/v1/circle/limit", ada, map[string]any{"supervisorLimit": 2})
	res.Body.Close()
	invite = decode(t, f.do(t, http.MethodPost, "/v1/circle/invites", ada, nil))
	code = invite["code"].(string)
	f.do(t, http.MethodPost, "/v1/invites/accept", cam, map[string]string{"code": code}).Body.Close()

	res = f.do(t, http.MethodPut, "/v1/circle/limit", ada, map[string]any{"supervisorLimit": 1})
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("lower limit %d", res.StatusCode)
	}
	res.Body.Close()

	off := f.do(t, http.MethodPost, "/v1/location", ada, map[string]any{
		"latitude": 37.7, "longitude": -122.4, "accuracyM": 8,
	})
	if off.StatusCode != http.StatusConflict {
		t.Fatalf("sharing off %d", off.StatusCode)
	}
	off.Body.Close()
	f.do(t, http.MethodPut, "/v1/circle/sharing", ada, map[string]any{"enabled": true}).Body.Close()
	posted := decode(t, f.do(t, http.MethodPost, "/v1/location", ada, map[string]any{
		"latitude": 37.7, "longitude": -122.4, "accuracyM": 8,
	}))
	if posted["stored"] != true {
		t.Fatalf("store location %#v", posted)
	}
	again := decode(t, f.do(t, http.MethodPost, "/v1/location", ada, map[string]any{
		"latitude": 37.8, "longitude": -122.5, "accuracyM": 8,
	}))
	if again["stored"] != false {
		t.Fatalf("throttle %#v", again)
	}

	place := decode(t, f.do(t, http.MethodPost, "/v1/places", ada, map[string]any{
		"label": "Home", "latitude": 37.7, "longitude": -122.4,
	}))
	if place["label"] != "Home" {
		t.Fatalf("place %#v", place)
	}

	stranger := f.register(t, "Dee")
	f.submitID(t, stranger, "1991-01-01", "P1000004").Body.Close()
	if res := f.do(t, http.MethodGet, "/v1/people/"+adaID, stranger, nil); res.StatusCode != http.StatusNotFound {
		t.Fatalf("stranger person %d", res.StatusCode)
	} else {
		res.Body.Close()
	}
	if res := f.do(t, http.MethodGet, "/v1/people/"+adaID+"/trail", stranger, nil); res.StatusCode != http.StatusNotFound {
		t.Fatalf("stranger trail %d", res.StatusCode)
	} else {
		res.Body.Close()
	}
	if res := f.do(t, http.MethodGet, "/v1/people/"+adaID+"/places", stranger, nil); res.StatusCode != http.StatusNotFound {
		t.Fatalf("stranger places %d", res.StatusCode)
	} else {
		res.Body.Close()
	}

	detail := decode(t, f.do(t, http.MethodGet, "/v1/people/"+adaID, bea, nil))
	if detail["displayName"] != "Ada" {
		t.Fatalf("detail %#v", detail)
	}
	trail := detail["trail"].([]any)
	if len(trail) != 1 {
		t.Fatalf("trail %#v", detail["trail"])
	}

	sups := decode(t, f.do(t, http.MethodGet, "/v1/circle/supervisors", ada, nil))
	list := sups["supervisors"].([]any)
	var camLink string
	for _, item := range list {
		row := item.(map[string]any)
		if row["displayName"] == "Cam" {
			camLink = row["linkId"].(string)
		}
	}
	if camLink == "" {
		t.Fatal("cam missing")
	}
	rev := f.do(t, http.MethodDelete, "/v1/circle/supervisors/"+camLink, ada, nil)
	if rev.StatusCode != http.StatusNoContent {
		t.Fatalf("revoke %d", rev.StatusCode)
	}
	rev.Body.Close()
	if res := f.do(t, http.MethodGet, "/v1/people/"+adaID, cam, nil); res.StatusCode != http.StatusNotFound {
		t.Fatalf("revoked read %d", res.StatusCode)
	} else {
		res.Body.Close()
	}

	if f.live != nil {
		lng, lat, ok, err := f.live.Position(context.Background(), adaID)
		if err != nil || !ok {
			t.Fatalf("geo %v %v", ok, err)
		}
		if lng == 0 || lat == 0 {
			t.Fatalf("geo position %v %v", lng, lat)
		}
	}
}

func TestSafetyCheckFlow(t *testing.T) {
	f := newFixture(t)
	ada, adaID := f.authOK(t, "Ada", "1990-01-02", "P2000001")
	bea, _ := f.authOK(t, "Bea", "1988-05-05", "P2000002")
	invite := decode(t, f.do(t, http.MethodPost, "/v1/circle/invites", ada, nil))
	f.do(t, http.MethodPost, "/v1/invites/accept", bea, map[string]string{"code": invite["code"].(string)}).Body.Close()

	started := decode(t, f.do(t, http.MethodPost, "/v1/people/"+adaID+"/checks", bea, map[string]string{"reason": "self_harm"}))
	chk := started["check"].(map[string]any)
	if chk["status"] != "pending" || chk["reason"] != "self_harm" || chk["startedByName"] != "Bea" {
		t.Fatalf("check %#v", chk)
	}
	pending := decode(t, f.do(t, http.MethodGet, "/v1/checks/pending", ada, nil))
	if pending["check"].(map[string]any)["id"] != chk["id"] {
		t.Fatalf("pending %#v", pending)
	}
	again := f.do(t, http.MethodPost, "/v1/people/"+adaID+"/checks", bea, map[string]string{"reason": "missing"})
	if again.StatusCode != http.StatusConflict {
		t.Fatalf("second check %d", again.StatusCode)
	}
	again.Body.Close()

	confirmed := decode(t, f.do(t, http.MethodPost, "/v1/checks/"+chk["id"].(string)+"/confirm", ada, map[string]any{}))
	got := confirmed["check"].(map[string]any)
	if got["status"] != "confirmed_safe" || got["confirmedByName"] != "Ada" {
		t.Fatalf("confirm %#v", got)
	}

	f.clock.Advance(time.Second)
	started = decode(t, f.do(t, http.MethodPost, "/v1/people/"+adaID+"/checks", bea, map[string]string{"reason": "missing"}))
	id := started["check"].(map[string]any)["id"].(string)
	f.clock.Advance(10 * time.Minute)
	late := f.do(t, http.MethodPost, "/v1/checks/"+id+"/confirm", ada, map[string]any{})
	if late.StatusCode != http.StatusConflict {
		t.Fatalf("late confirm %d", late.StatusCode)
	}
	late.Body.Close()
	detail := decode(t, f.do(t, http.MethodGet, "/v1/people/"+adaID, bea, nil))
	if detail["check"].(map[string]any)["status"] != "unanswered" {
		t.Fatalf("unanswered %#v", detail["check"])
	}
	none := decode(t, f.do(t, http.MethodGet, "/v1/checks/pending", ada, nil))
	if none["check"] != nil {
		t.Fatalf("pending after deadline %#v", none)
	}

	stranger := f.register(t, "Eve")
	f.submitID(t, stranger, "1992-02-02", "P2000003").Body.Close()
	if res := f.do(t, http.MethodPost, "/v1/people/"+adaID+"/checks", stranger, map[string]string{"reason": "unwell"}); res.StatusCode != http.StatusNotFound {
		t.Fatalf("stranger check %d", res.StatusCode)
	} else {
		res.Body.Close()
	}
}

func TestLiveFanoutReachesOnlySupervisors(t *testing.T) {
	f := newFixture(t)
	if f.live == nil {
		t.Skip("redis is not running")
	}
	time.Sleep(200 * time.Millisecond)
	ada, adaID := f.authOK(t, "Ada", "1990-01-02", "P3000001")
	bea, _ := f.authOK(t, "Bea", "1988-05-05", "P3000002")
	cam, _ := f.authOK(t, "Cam", "1985-03-03", "P3000003")
	invite := decode(t, f.do(t, http.MethodPost, "/v1/circle/invites", ada, nil))
	f.do(t, http.MethodPost, "/v1/invites/accept", bea, map[string]string{"code": invite["code"].(string)}).Body.Close()
	f.do(t, http.MethodPut, "/v1/circle/sharing", ada, map[string]any{"enabled": true}).Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	supervisorConn := dial(t, ctx, f.ts.URL, bea)
	defer supervisorConn.Close(websocket.StatusNormalClosure, "")
	strangerConn := dial(t, ctx, f.ts.URL, cam)
	defer strangerConn.Close(websocket.StatusNormalClosure, "")

	posted := decode(t, f.do(t, http.MethodPost, "/v1/location", ada, map[string]any{
		"latitude": 40.7, "longitude": -74.0, "accuracyM": 12,
	}))
	if posted["live"] != true {
		t.Fatalf("live %#v", posted)
	}
	readCtx, readCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer readCancel()
	_, msg, err := supervisorConn.Read(readCtx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(msg), adaID) || !strings.Contains(string(msg), `"type":"location"`) {
		t.Fatalf("message %s", msg)
	}
	missCtx, missCancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer missCancel()
	_, stray, err := strangerConn.Read(missCtx)
	if err == nil {
		t.Fatalf("stranger received %s", stray)
	}

	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer rdb.Close()
	pos, err := rdb.GeoPos(context.Background(), "geo:live", adaID).Result()
	if err != nil || len(pos) == 0 || pos[0] == nil {
		t.Fatalf("geopos %v %v", pos, err)
	}
}

func dial(t *testing.T, ctx context.Context, httpURL, token string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(httpURL, "http") + "/v1/ws?access_token=" + token
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}
