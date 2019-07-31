package checkpoint

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	downloadUrl = "https://www.solo.io"
)

var (
	baseExpectedResponse = &CheckResponse{
		Product:             "test",
		CurrentVersion:      "1.0",
		CurrentReleaseDate:  0,
		CurrentDownloadURL:  downloadUrl,
		CurrentChangelogURL: downloadUrl,
		ProjectWebsite:      downloadUrl,
	}
)

func TestMain(m *testing.M) {
	defer setup()()
	os.Exit(m.Run())
}

func setup() func() {
	verifyServerIsActive()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // for timeout case
		response := baseExpectedResponse
		json.NewEncoder(w).Encode(response)
	}))
	fmt.Println("using checkpoint server at ", srv.URL)
	os.Setenv("CHECKPOINT_URL", srv.URL)
	return func() {
		srv.Close()
		os.Setenv("CHECKPOINT_URL", "")
	}
}

func verifyServerIsActive() {
	_, err := check(&CheckParams{
		Product: "test",
		Version: "1.0",
	})
	if err != nil && strings.Contains(err.Error(), "connection refused") {
		fmt.Println("Unable to connect to checkpoint server, please confirm it is running.")
		os.Exit(1)
	}
}

func TestCheck(t *testing.T) {
	expected := baseExpectedResponse
	expected.Outdated = false
	expected.Alerts = nil

	actual, err := check(&CheckParams{
		Product: "test",
		Version: "1.0",
	})

	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("\nbad: %#v\nexp: %#v", actual, expected)
	}
}

func TestCheckTimeout(t *testing.T) {
	// if this test fails, try reducing the timeout
	os.Setenv("CHECKPOINT_TIMEOUT", "5")
	defer os.Setenv("CHECKPOINT_TIMEOUT", "")

	expected := "Client.Timeout exceeded while awaiting headers"

	actual, err := check(&CheckParams{
		Product: "test",
		Version: "1.0",
	})

	if err == nil {
		t.Fatalf("expected timeout error, none given")
	}
	if !strings.Contains(err.Error(), expected) {
		t.Fatalf("bad: %#v", actual)
	}
}

func TestCheck_disabled(t *testing.T) {
	os.Setenv("CHECKPOINT_DISABLE", "1")
	defer os.Setenv("CHECKPOINT_DISABLE", "")

	expected := &CheckResponse{}

	actual, err := check(&CheckParams{
		Product: "test",
		Version: "1.0",
	})

	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("expected %+v to equal %+v", actual, expected)
	}
}

func TestCheck_cache(t *testing.T) {
	dir, err := ioutil.TempDir("", "checkpoint")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	expected := baseExpectedResponse
	expected.Outdated = false
	expected.Alerts = nil

	var actual *CheckResponse
	for i := 0; i < 5; i++ {
		var err error
		actual, err = check(&CheckParams{
			Product:   "test",
			Version:   "1.0",
			CacheFile: filepath.Join(dir, "cache"),
		})
		if err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %#v", actual)
	}
}

func TestCheck_cacheNested(t *testing.T) {
	dir, err := ioutil.TempDir("", "checkpoint")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	expected := baseExpectedResponse
	expected.Outdated = false
	expected.Alerts = nil

	var actual *CheckResponse
	for i := 0; i < 5; i++ {
		var err error
		actual, err = check(&CheckParams{
			Product:   "test",
			Version:   "1.0",
			CacheFile: filepath.Join(dir, "nested", "cache"),
		})
		if err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %#v", actual)
	}
}

func TestCheckInterval(t *testing.T) {
	expected := baseExpectedResponse
	expected.Outdated = false
	expected.Alerts = nil

	params := &CheckParams{
		Product: "test",
		Version: "1.0",
	}

	calledCh := make(chan struct{})
	checkFn := func(actual *CheckResponse, err error) {
		defer close(calledCh)
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("bad: %#v", actual)
		}
	}

	doneCh := checkInterval(params, 500*time.Millisecond, checkFn)
	defer close(doneCh)

	select {
	case <-calledCh:
	case <-time.After(time.Second):
		t.Fatalf("timeout")
	}
}

func TestCheckInterval_disabled(t *testing.T) {
	os.Setenv("CHECKPOINT_DISABLE", "1")
	defer os.Setenv("CHECKPOINT_DISABLE", "")

	params := &CheckParams{
		Product: "test",
		Version: "1.0",
	}

	calledCh := make(chan struct{})
	checkFn := func(actual *CheckResponse, err error) {
		defer close(calledCh)
	}

	doneCh := checkInterval(params, 500*time.Millisecond, checkFn)
	defer close(doneCh)

	select {
	case <-calledCh:
		t.Fatal("expected callback to not invoke")
	case <-time.After(time.Second):
	}
}

func TestRandomStagger(t *testing.T) {
	intv := 24 * time.Hour
	min := 18 * time.Hour
	max := 30 * time.Hour
	for i := 0; i < 1000; i++ {
		out := randomStagger(intv, 10)
		if out < min || out > max {
			t.Fatalf("bad: %v", out)
		}
	}
}

func TestReport_sendsRequest(t *testing.T) {
	r := &ReportParams{
		Signature: "sig",
		Product:   "prod",
	}

	req, err := reportRequest(r)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !strings.HasSuffix(req.URL.Path, "/telemetry/prod") {
		t.Fatalf("Expected url to have the product. Got %s", req.URL.String())
	}

	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	var p ReportParams
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("err: %s", err)
	}

	if p.Signature != "sig" {
		t.Fatalf("Expected request body to have data from request. got %#v", p)
	}
}
