# P2P LLM Chat — Go (libp2p) + Streamlit + Ollama

A local-first, **peer-to-peer chat** app where each user has an **LLM co‑pilot** that proposes replies you can accept or ignore.  
Messages are sent **directly between peers** (no server relay). A tiny **Directory** service only maps username → PeerID (+addrs).

## System Diagram (ASCII)

```
+-------------------+           +-------------------+
|             UI    |  HTTP     |   UI              |
|  (userA machine)  +--------+  |  (userB machine)  |
+---------+---------+        |  +---------+---------+
          | localhost        |            | localhost
          v                  |            v
+---------+---------+        |  +---------+---------+
| Go P2P Node (A)   |<-------+--+ Go P2P Node (B)   |
| libp2p host       |  P2P      | libp2p host       |
| /p2p-llm-chat     |  stream   | /p2p-llm-chat     |
+---------+---------+           +---------+---------+
          ^                                 ^
          |                                 |
          | HTTP                            | HTTP
          |                                 |
     +----+-----+                      +----+-----+
     |  Ollama  |                      |  Ollama  |
     | (LLM)    |                      | (LLM)    |
     +----------+                      +----------+

         +-------------------------------+
         |        Directory (Go)         |
         |  username -> {peerID, addrs}  |
         +-------------------------------+

                    (Optional)
         +-------------------------------+
         |        Relay (Go, optional)   |
         |   libp2p circuit relay v2     |
         +-------------------------------+
```

## Components
- **Go P2P Node**: runs a libp2p host, exposes `POST /send` and `GET /inbox` to a local UI, handles direct streams.
- **Directory**: tiny Go service to register/lookup usernames → peer info (not a message relay).
- **UI**: local chatbox that calls the node API and **queries an LLM** (via Local [Ollama](https://ollama.com/)) for suggestions.
- **Relay (optional)**: helps with NAT traversal in real networks; for local/LAN dev you may not need it.

## Quickstart

### 0) Prereqs
- Go 1.22+
- Python 3.10+ with `streamlit` and `requests`
- Ollama installed with at least one model (`ollama run llama3.1`), as docker container.


#### Little hint: you can run start_all.sh to run all the following, but you still need to have your LLM locally:
```bash

#Run all in one click
chmod +x start_all.sh
./start_all.sh

# Run Ollama container
docker run -d \
  --name ollama \
  -v ollama:/root/.ollama \
  -p 11434:11434 \
  ollama/ollama

# Pull the llama3.1 model inside the container
docker exec -it ollama ollama pull llama3.1
```

### 1) Start the Directory
```bash
cd go/cmd/directory
go run .
```
This listens on `127.0.0.1:8080`.

<!-- *(If you need OS env support, replace the small getenv helper with os.Getenv as noted in the file.)* -->

### 2) Start two P2P nodes (User A and User B)

Terminal 1 (userA):
```bash
cd go/cmd/node
export MYNAMEIS=userA
export HTTP_ADDR=127.0.0.1:8081
export DIRECTORY_URL=http://127.0.0.1:8080
go run .
```

Terminal 2 (userB):
```bash
cd go/cmd/node
export MYNAMEIS=userB
export HTTP_ADDR=127.0.0.1:8082
export DIRECTORY_URL=http://127.0.0.1:8080
go run .
```

### 3) Run Streamlit (for each user)
Terminal 3 (userA UI):
```bash
cd web
export NODE_HTTP=http://127.0.0.1:8081
export OLLAMA_URL=http://127.0.0.1:11434
export LLM_MODEL=llama3.1
streamlit run streamlit_app.py --server.port 8501
```

Terminal 4 (userB UI):
```bash
cd web
export NODE_HTTP=http://127.0.0.1:8082
export OLLAMA_URL=http://127.0.0.1:11434
export LLM_MODEL=llama3.1
streamlit run streamlit_app.py --server.port 8502
```

Open `http://localhost:8501` and `http://localhost:8502` in your browser.
- From userA UI, send a message to `userB`.
- userB sees it and gets an **AI suggestion** after clicking on **Suggest a Reply** or type their own reply and use the Send btn.

## Training / Personalizing Your LLM

For a quick local setup, rely on **Ollama** default models (e.g., `llama3`).  
If you want your agent to sound like **you** or specialize for your domain, you can train your own.

## Security Notes
- libp2p encrypts streams (noise/TLS). Still consider:
  - Do not log secrets.
  - Add message signing/verification if needed.
  - Persist identities (save keypair) for stable PeerIDs.
- The Directory is a **trust anchor**; harden or self-host it. Messages remain P2P.

## Roadmap Ideas
- WebRTC transport for browser clients.
- WebSockets from node → Streamlit for live updates.
- Message persistence (SQLite) and receipt acks.
- Multi-device sync (CRDTs), offline queue, group chats.

---

![P2P LLM Chat Screenshot](../p2p-llm-chat/screenshots/Screenshot.png)

---
