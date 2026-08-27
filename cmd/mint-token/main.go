// Command mint-token issues a development-only JWT for integration tests.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/JohnAD/datorium-client-go/v2/internal/testtoken"
)

func main() {
	auth := flag.String("auth", "", "path to __auth.json")
	key := flag.String("key", "", "path to Ed25519 PKCS8 PEM signing key")
	subject := flag.String("subject", "integration", "JWT subject")
	kind := flag.String("kind", "client", "datoriumdb.kind claim (client or admin)")
	lifetime := flag.Duration("lifetime", time.Hour, "token lifetime")
	flag.Parse()
	if *auth == "" || *key == "" {
		fmt.Fprintln(os.Stderr, "usage: mint-token -auth __auth.json -key key.pem [-subject name] [-kind client|admin]")
		os.Exit(2)
	}
	tok, err := testtoken.MintToken(*auth, *key, *subject, *kind, *lifetime)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(tok)
}
