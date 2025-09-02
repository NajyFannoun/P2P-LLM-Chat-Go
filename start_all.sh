#!/bin/bash

# --- Run the Directory server ---
echo "Starting Directory server..."
(cd go/cmd/directory && go run .) &

# --- Run Node 1 (Najy) ---
echo "Starting Node 1 (Najy)..."
(cd go/cmd/node && \
  export MYNAMEIS=Najy && \
  export HTTP_ADDR=127.0.0.1:8081 && \
  export DIRECTORY_URL=http://127.0.0.1:8080 && \
  go run .) &

# --- Run Node 2 (Cannan) ---
echo "Starting Node 2 (Cannan)..."
(cd go/cmd/node && \
  export MYNAMEIS=Cannan && \
  export HTTP_ADDR=127.0.0.1:8082 && \
  export DIRECTORY_URL=http://127.0.0.1:8080 && \
  go run .) &

# --- Give Go nodes some time to start ---
sleep 5

# --- Run Streamlit for Node 1 ---
echo "Starting Streamlit app for Node 1..."
(cd web && \
  export NODE_HTTP=http://127.0.0.1:8081 && \
  export OLLAMA_URL=http://127.0.0.1:11434 && \
  export LLM_MODEL=llama3.1 && \
  streamlit run streamlit_app.py --server.port 8501) &

# --- Run Streamlit for Node 2 ---
echo "Starting Streamlit app for Node 2..."
(cd web && \
  export NODE_HTTP=http://127.0.0.1:8082 && \
  export OLLAMA_URL=http://127.0.0.1:11434 && \
  export LLM_MODEL=llama3.1 && \
  streamlit run streamlit_app.py --server.port 8502) &

echo "All services started!"
wait
