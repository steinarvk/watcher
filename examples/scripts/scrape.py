#!/usr/bin/env python3
import requests
import json
import sys
import os

url = sys.argv[1]

headers = {}
try:
    headers["User-Agent"] = os.environ["USER_AGENT"]
except KeyError:
    pass

resp = requests.get(url, headers=headers)

json.dump({
    "url": url,
    "content": resp.text,
    "headers": dict(resp.headers),
    "code": resp.status_code,
}, sys.stdout)
