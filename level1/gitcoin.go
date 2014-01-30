package main

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"
)

func main() {
	args := os.Args

	diff := args[1]
	if len(diff)%2 == 1 {
		diff += "f"
	}
	difficulty, _ := hex.DecodeString(diff)
	//io.WriteString(os.Stderr, fmt.Sprintf("I: %s", err))
	tree := args[2]
	parent := args[3]
	timestamp := args[4]

	body := fmt.Sprintf(`tree %s
parent %s
author CTF user <me@example.com> %s +0000
committer CTF user <me@example.com> %s +0000

Give me a Gitcoin
`, tree, parent, timestamp, timestamp)

	done := make(chan string)
	hashes := 0
	startTime := time.Now()
	for i := 0; i < 8; i++ {
		go func() {
			counter := make([]byte, 5)
			for {
				msg := fmt.Sprintf("commit %d\x00%s%x\n", len(body)+11, body, counter)
				h := sha1.New()
				io.WriteString(h, msg)
				hash := h.Sum(nil)
				hashes++
				solved := true
				for i, _ := range difficulty {
					if hash[i] > difficulty[i] {
						rand.Read(counter)
						solved = false
						break
					}
				}
				if solved {
					io.WriteString(os.Stderr, fmt.Sprintf("\nGo: %x\n", hash))
					done <- fmt.Sprintf("%s%x\n", body, counter)
					break
				}
			}
		}()
	}
	go func() {
		for {
			time.Sleep(1 * time.Second)
			io.WriteString(os.Stderr, fmt.Sprintf("Hashes/s: %v\n", float64(hashes)/time.Now().Sub(startTime).Seconds()))
		}
	}()
	success := <-done
	fmt.Printf(success)
}
