//
//  Project: P2P LLM Chat
//  Description: A local-first, peer-to-peer chat application with LLM co-pilot suggestions, using Go (libp2p), Streamlit, and Ollama.
//  Author: Najy Fannoun
//  Developed By: Najy Fannoun
//  Version: 1.0.0
//  Date: September 2025
//  Copyright: © 2025 Najy Fannoun. All rights reserved.
//
//  License: This project is licensed under the MIT License.
//  You are free to use, modify, and distribute this software under the terms of the MIT License.
//  For more details, please refer to the LICENSE file in the project root directory.
//
//  Disclaimer: This project is intended for educational and research purposes only.
//  The author is not responsible for any misuse or illegal activities that may arise from the use of this software.
//  Please use this software responsibly and in compliance with applicable laws and regulations.
//

package main

import (
	"log"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type record struct {
	PeerID string    `json:"peer_id"`
	Addrs  []string  `json:"addrs"`
	Last   time.Time `json:"last"`
}

type memStore struct {
	mu   sync.RWMutex
	data map[string]record // username -> record
}

func (s *memStore) set(username string, rec record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = map[string]record{}
	}
	s.data[username] = rec
}

func (s *memStore) get(username string) (record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.data[username]
	return rec, ok
}

func main() {
	addr := getenv("ADDR", "127.0.0.1:8080")
	r := gin.Default()
	store := &memStore{}

	r.POST("/register", func(c *gin.Context) {
		var body struct {
			Username string   `json:"username"`
			PeerID   string   `json:"peer_id"`
			Addrs    []string `json:"addrs"`
		}
		if err := c.BindJSON(&body); err != nil {
			c.String(400, err.Error())
			return
		}
		if body.Username == "" || body.PeerID == "" {
			c.String(400, "missing fields")
			return
		}
		store.set(body.Username, record{PeerID: body.PeerID, Addrs: body.Addrs, Last: time.Now()})
		c.JSON(200, gin.H{"ok": true})
	})

	r.GET("/lookup", func(c *gin.Context) {
		u := c.Query("username")
		if u == "" {
			c.String(400, "username required")
			return
		}
		rec, ok := store.get(u)
		if !ok {
			c.String(404, "not found")
			return
		}
		c.JSON(200, gin.H{"peer_id": rec.PeerID, "addrs": rec.Addrs})
	})

	log.Println("📒 Directory on", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

func getenv(k, d string) string {
	if v := mapEnv(k); v != "" {
		return v
	}
	return d
}

func mapEnv(k string) string {
	return os.Getenv(k)
}
