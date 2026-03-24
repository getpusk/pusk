package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type PushRecord struct {
	Time    string            `json:"time"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	BodyLen int               `json:"body_len"`
}

var (
	mu      sync.Mutex
	records []PushRecord
)

func main() {
	port := "9876"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}

	http.HandleFunc("/push-receive", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		headers := make(map[string]string)
		for k, v := range r.Header {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}
		rec := PushRecord{
			Time:    time.Now().Format(time.RFC3339),
			Method:  r.Method,
			Headers: headers,
			BodyLen: len(body),
		}
		mu.Lock()
		records = append(records, rec)
		count := len(records)
		mu.Unlock()
		log.Printf("PUSH #%d: %d bytes, TTL=%s, Encoding=%s", count, len(body), r.Header.Get("TTL"), r.Header.Get("Content-Encoding"))
		w.WriteHeader(201)
	})

	http.HandleFunc("/push-results", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"total": len(records), "records": records})
	})

	http.HandleFunc("/push-reset", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		records = nil
		mu.Unlock()
		fmt.Fprintln(w, "reset")
	})

	log.Printf("Push receiver on :%s — POST /push-receive, GET /push-results", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
