# //
# //  Project: P2P LLM Chat
# //  Description: A local-first, peer-to-peer chat application with LLM co-pilot suggestions, using Go (libp2p), Streamlit, and Ollama.
# //  Author: Najy Fannoun
# //  Developed By: Najy Fannoun
# //  Version: 1.0.0
# //  Date: September 2025
# //  Copyright: © 2025 Najy Fannoun. All rights reserved.
# //
# //  License: This project is licensed under the MIT License.
# //  You are free to use, modify, and distribute this software under the terms of the MIT License.
# //  For more details, please refer to the LICENSE file in the project root directory.
# //
# //  Disclaimer: This project is intended for educational and research purposes only.
# //  The author is not responsible for any misuse or illegal activities that may arise from the use of this software.
# //  Please use this software responsibly and in compliance with applicable laws and regulations.
# //

import os
import time
import requests
import streamlit as st
from datetime import datetime, timezone

# Config
NODE_HTTP = os.environ.get("NODE_HTTP", "http://127.0.0.1:8081")
OLLAMA_URL = os.environ.get("OLLAMA_URL", "http://127.0.0.1:11434")
LLM_MODEL = os.environ.get("LLM_MODEL", "llama3.1")

st.set_page_config(page_title="P2P LLM Chat", layout="wide")
st.title("LLM-Powered P2P Chat 💬 using Go.")

# --- Session state init ---
if "sent_msgs" not in st.session_state:
    st.session_state.sent_msgs = []
if "all_msgs" not in st.session_state:
    st.session_state.all_msgs = []

# Fetch my info
me = requests.get(f"{NODE_HTTP}/me").json()
my_username = me.get("username", "me")

st.subheader(my_username)

col1, col2 = st.columns([1, 3])  

with col1:
    to_username = st.text_input("To Username", value="userB")

with col2:
    content = st.text_input("Message", value="Hey! How's it going?")

send_clicked = st.button("📤 Send", key="send_button")

if send_clicked:
    r = requests.post(f"{NODE_HTTP}/send", json={"to_username": to_username, "content": content})
    if r.status_code == 200:
        st.success("Sent!")
        if "sent_msgs" not in st.session_state:
            st.session_state.sent_msgs = []
        st.session_state.sent_msgs.append({
            "id": f"sent_{len(st.session_state.sent_msgs)+1}",
            "from_user": my_username,
            "to_user": to_username,
            "content": content,
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "sent": True,
        })
    else:
        st.error(f"Send failed: {r.text}")

st.markdown("""
<style>
div.stButton > button[key="send_button"] {
    background-color: #28a745;
    color: white;
    width: 100%;
    height: 50px;
    font-weight: bold;
    font-size: 16px;
    border-radius: 8px;
}
</style>
""", unsafe_allow_html=True)

st.divider()
st.subheader("Chat history + AI co-pilot")

def ai_suggest(prompt: str) -> str:
    try:
        resp = requests.post(f"{OLLAMA_URL}/api/generate", json={
            "model": LLM_MODEL,
            "prompt": f"You are a helpful assistant. Draft a concise, friendly reply to the following message:\n\n{prompt}\n\nReply:",
            "stream": False
        }, timeout=60)
        if resp.status_code == 200:
            data = resp.json()
            return data.get("response", "").strip()
        return "(LLM error)"
    except Exception as e:
        return f"(LLM unavailable: {e})"

def poll_inbox():
    try:
        r = requests.get(f"{NODE_HTTP}/inbox", params={"after": ""})
        if r.status_code == 200:
            msgs = r.json()
            for m in msgs:
                m["sent"] = False
            return msgs
        return []
    except Exception:
        return []

# --- Merge inbox + sent ---
inbox_msgs = poll_inbox()
all_msgs = inbox_msgs + st.session_state.sent_msgs

# Sort by timestamp (convert ISO / string → datetime)
def parse_ts(ts: str):
    try:
        dt = datetime.fromisoformat(ts.replace("Z", "+00:00"))  # handle Zulu time
        if dt.tzinfo is None:  # make naive → UTC aware
            dt = dt.replace(tzinfo=timezone.utc)
        return dt
    except Exception:
        return datetime.now(timezone.utc)


all_msgs.sort(key=lambda x: parse_ts(x["timestamp"]))

# Save so they persist
st.session_state.all_msgs = all_msgs

# --- Render chat like real messenger ---
for m in st.session_state.all_msgs:
    is_sent = m.get("sent", False)
    ts = m["timestamp"]
    if is_sent:
        st.markdown(
            f"""
            <div style="text-align:right; margin:8px; padding:8px; 
                        background:#DCF8C6; border-radius:12px; 
                        display:inline-block; max-width:70%;">
                <b>You → {m['to_user']}</b><br>{m['content']}<br>
                <small>{ts}</small>
            </div>
            """, unsafe_allow_html=True)
    else:
        # Incoming message
        st.markdown(
            f"""
            <div style="text-align:left; margin:8px; padding:8px; 
                        background:#EEE; border-radius:12px; 
                        display:inline-block; max-width:70%;">
                <b>{m['from_user']} → You</b><br>{m['content']}<br>
                <small>{ts}</small>
            </div>
            """, unsafe_allow_html=True)

        # AI suggestion button
        suggest_key = f"suggest_{m['id']}"
        if st.button("🤖 Suggest a reply", key=suggest_key):
            suggestion = ai_suggest(m["content"])
            st.session_state[f"suggestion_{m['id']}"] = suggestion

        # Show suggestion if exists
        if f"suggestion_{m['id']}" in st.session_state:
            sug = st.session_state[f"suggestion_{m['id']}"]
            st.markdown(
                f"<div style='text-align:left; margin:8px; padding:8px; background:#F0F8FF; border-radius:12px; display:inline-block; max-width:70%;'>💡 AI Suggestion:<br>{sug}</div>",
                unsafe_allow_html=True
            )
            # Button to send AI suggestion
            send_key = f"send_ai_{m['id']}"
            if st.button(f"Send AI reply to {m['from_user']}", key=send_key):
                rr = requests.post(f"{NODE_HTTP}/send", json={
                    "to_username": m["from_user"],
                    "content": sug
                })
                if rr.status_code == 200:
                    st.success("AI reply sent!")
                    st.session_state.sent_msgs.append({
                        "id": f"sent_{len(st.session_state.sent_msgs)+1}",
                        "from_user": my_username,
                        "to_user": m["from_user"],
                        "content": sug,
                        "timestamp": datetime.now(timezone.utc).isoformat(),
                        "sent": True,
                    })

# --- Auto-refresh every 2s ---
time.sleep(2)
st.rerun()
