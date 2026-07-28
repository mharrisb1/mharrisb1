#!/usr/bin/bash

SCHEME=http
HOST=127.0.0.1
PORT=8000
ENTRYPOINT=public

zellij run --close-on-exit --name tailscale-serve -- sudo tailscale serve $SCHEME://$HOST:$PORT
python3 -m http.server $PORT --bind $HOST -d $ENTRYPOINT
