// Command captoken-keygen prints a fresh ed25519 capability-token keypair in the
// exact base64 encoding the scheduler and KMS expect (spec/secrets.md §6):
//
//	SCHEDULER_TOKEN_PRIVKEY  — base64(std) of the 64-byte ed25519 private key
//	KMS_TOKEN_PUBKEYS        — base64(std) of the 32-byte ed25519 public key
//
// It uses captoken.GenerateKey/EncodeKey, the same functions the services parse
// with, so the output is guaranteed compatible. Provision production keys with
// this — do NOT reuse the dev keys in docker-compose.
//
// Usage:
//
//	go -C captoken run ./cmd/captoken-keygen
package main

import (
	"fmt"
	"os"

	"captoken"
)

func main() {
	pub, priv, err := captoken.GenerateKey()
	if err != nil {
		fmt.Fprintln(os.Stderr, "generating keypair:", err)
		os.Exit(1)
	}
	fmt.Println("SCHEDULER_TOKEN_PRIVKEY=" + captoken.EncodeKey(priv))
	fmt.Println("KMS_TOKEN_PUBKEYS=" + captoken.EncodeKey(pub))
}
