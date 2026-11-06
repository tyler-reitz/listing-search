import argparse
from datetime import date

import uvicorn

from app.main import create_app
from app.search import load

parser = argparse.ArgumentParser(prog="python -m app")
parser.add_argument("--data", default="data/sample_listings.json", help="path to listings JSON")
parser.add_argument("--port", type=int, default=8080)
parser.add_argument("--now", type=date.fromisoformat, help="pin the clock to YYYY-MM-DD (the sample data is from Aug-Sep 2026)")
args = parser.parse_args()

listings = load(args.data)
today = (lambda: args.now) if args.now else date.today
uvicorn.run(create_app(listings, today), host="127.0.0.1", port=args.port)
