// Command mockgov runs the fake government e-filing portal for local demos of
// the browser submission agent. Point the API/worker at it with
// GOV_MOCK_PORTAL_URL=http://localhost:8091 and store credentials
// employer01 / s3cret! for agency MOCK, portal "mock". CAPTCHA: K7Q2P, OTP: 246810.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/halaostory/halaos/internal/govfiling/mockportal"
)

func main() {
	addr := os.Getenv("MOCKGOV_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8091"
	}
	p := mockportal.New()
	log.Printf("mock government portal on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, p.Handler()))
}
