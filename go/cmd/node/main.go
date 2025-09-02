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
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"p2p-llm-chat/node/proto"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	libp2p "github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
)

const ChatProtocolID = protocol.ID("/p2p-llm-chat/1.0.0")

type DirectoryClient struct {
	BaseURL string
	Client  *http.Client
}

func (dc *DirectoryClient) Register(username, peerID string, addrs []string) error {
	body := fmt.Sprintf(`{"username":"%s","peer_id":"%s","addrs":%s}`, username, peerID, toJSON(addrs))
	req, _ := http.NewRequest("POST", dc.BaseURL+"/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := dc.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("register failed: %s", string(data))
	}
	return nil
}

func (dc *DirectoryClient) Lookup(username string) (peerID string, addrs []string, err error) {
	req, _ := http.NewRequest("GET", dc.BaseURL+"/lookup?username="+username, nil)
	resp, err := dc.Client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("lookup failed: %s", string(data))
	}
	var out struct {
		PeerID string   `json:"peer_id"`
		Addrs  []string `json:"addrs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", nil, err
	}
	return out.PeerID, out.Addrs, nil
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

type Inbox struct {
	mu    sync.Mutex
	queue []proto.ChatMessage
}

func (i *Inbox) Push(m proto.ChatMessage) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.queue = append(i.queue, m)
}

func (i *Inbox) Drain(after string) []proto.ChatMessage {
	i.mu.Lock()
	defer i.mu.Unlock()
	if after == "" {
		cp := make([]proto.ChatMessage, len(i.queue))
		copy(cp, i.queue)
		return cp
	}
	out := []proto.ChatMessage{}
	found := false
	for _, m := range i.queue {
		if m.ID == after {
			found = true
			continue
		}
		if found {
			out = append(out, m)
		}
	}
	return out
}

func main() {
	username := envOr("MYNAMEIS", "userA")
	listenHTTP := envOr("HTTP_ADDR", "127.0.0.1:8081")
	dirURL := envOr("DIRECTORY_URL", "http://127.0.0.1:8080")
	bootstrap := envOr("BOOTSTRAP_ADDRS", "")

	ctx := context.Background()
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic-v1",
		),
		libp2p.Identity(generateKey()),
		libp2p.NATPortMap(),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer h.Close()

	// DHT
	_, err = dht.New(ctx, h, dht.Mode(dht.ModeAuto))
	if err != nil {
		log.Println("DHT init error:", err)
	}

	// Stream handler
	inbox := &Inbox{}
	h.SetStreamHandler(ChatProtocolID, func(s network.Stream) {
		defer s.Close()
		data, err := io.ReadAll(bufio.NewReader(s))
		if err != nil {
			log.Println("read stream:", err)
			return
		}
		var msg proto.ChatMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Println("unmarshal:", err)
			return
		}
		inbox.Push(msg)
		log.Printf("📩 Received from %s: %s\n", msg.FromUser, msg.Content)
	})

	// Register in directory
	dir := &DirectoryClient{BaseURL: dirURL, Client: &http.Client{Timeout: 5 * time.Second}}
	addrs := []string{}
	peerIDStr := h.ID().String()
	for _, a := range h.Addrs() {
		ma := a.Encapsulate(multiaddr.StringCast("/p2p/" + peerIDStr))
		addrs = append(addrs, ma.String())
	}

	if err := dir.Register(username, peerIDStr, addrs); err != nil {
		log.Fatal("directory register failed:", err)
	}
	log.Printf("👤 %s PeerID=%s", username, peerIDStr)

	// Bootstrap peers
	if bootstrap != "" {
		for _, addr := range strings.Split(bootstrap, ",") {
			addr = strings.TrimSpace(addr)
			if addr == "" {
				continue
			}
			ma, err := multiaddr.NewMultiaddr(addr)
			if err != nil {
				log.Println("bad bootstrap:", addr, err)
				continue
			}
			info, err := peer.AddrInfoFromP2pAddr(ma)
			if err != nil {
				log.Println("addrinfo:", err)
				continue
			}
			if err := h.Connect(ctx, *info); err != nil {
				log.Println("connect:", err)
			} else {
				log.Println("✅ connected to bootstrap", info.ID)
			}
		}
	}

	// HTTP API
	r := gin.Default()
	type SendBody struct {
		ToUsername string `json:"to_username"`
		Content    string `json:"content"`
	}
	r.POST("/send", func(c *gin.Context) {
		var body SendBody
		if err := c.BindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		peerIDStr, addrs, err := dir.Lookup(body.ToUsername)
		if err != nil {
			c.JSON(404, gin.H{"error": "user not found"})
			return
		}
		pi, err := peer.Decode(peerIDStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "bad peer id"})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		info := peer.AddrInfo{ID: pi}
		for _, a := range addrs {
			if ma, err := multiaddr.NewMultiaddr(a); err == nil {
				info.Addrs = append(info.Addrs, ma)
			}
		}
		_ = h.Connect(ctx, info)

		s, err := h.NewStream(ctx, pi, ChatProtocolID)
		if err != nil {
			c.JSON(500, gin.H{"error": "open stream failed: " + err.Error()})
			return
		}
		defer s.Close()

		msg := proto.ChatMessage{
			ID:        uuid.NewString(),
			FromUser:  username,
			ToUser:    body.ToUsername,
			Content:   body.Content,
			Timestamp: time.Now(),
		}
		b, _ := json.Marshal(msg)
		if _, err := s.Write(b); err != nil {
			c.JSON(500, gin.H{"error": "write failed: " + err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "sent", "id": msg.ID})
	})

	r.GET("/inbox", func(c *gin.Context) {
		after := c.Query("after")
		c.JSON(200, inbox.Drain(after))
	})

	r.GET("/me", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"username": username,
			"peer_id":  string(h.ID()),
			"addrs":    addrs,
		})
	})

	log.Println("📡 HTTP listening on", listenHTTP)
	if err := r.Run(listenHTTP); err != nil {
		log.Fatal(err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func generateKey() crypto.PrivKey {
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	return priv
}
