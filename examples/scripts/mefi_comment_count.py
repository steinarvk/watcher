#!/usr/bin/env python3
import sys
import json
from bs4 import BeautifulSoup

soup = BeautifulSoup(json.load(sys.stdin)["content"])

id_to_counts = {x.attrs["href"].split("/")[1]: int(x.text.split()[0])
                for x in soup.find_all(class_="more")}

json.dump(id_to_counts, sys.stdout)
