// Test for the following package
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func handleHTTPRq(w http.ResponseWriter, r *http.Request) {
	//  log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte("Hello world"))
}

// func main() {
// 	http.HandleFunc("/", handleHTTPRq)
// 	log.Fatal(http.ListenAndServe("127.0.0.1:8000", nil))
// }

const host = "http://127.0.0.1:8081/"

func TestRequest(t *testing.T) {

	req, err := http.NewRequest("GET", host, nil)
	if err != nil {
		t.Fatal(err)
	}

	rw := httptest.NewRecorder()

	handleHTTPRq(rw, req)

	if rw.Code == 500 {
		t.Fatal("Internal server Error: " + rw.Body.String())
	}
	if rw.Body.String() != "Hello world" {
		t.Fatal("Expected " + rw.Body.String())
	}

}

func BenchmarkRequest(b *testing.B) {
	req, err := http.NewRequest("GET", host, nil)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		rw := httptest.NewRecorder()
		handleHTTPRq(rw, req)
	}

}
