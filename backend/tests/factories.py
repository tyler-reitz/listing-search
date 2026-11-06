from datetime import date
from pathlib import Path

from app.search import Listing

NOW = date(2026, 9, 10)
DATA = Path(__file__).parent.parent / "data" / "sample_listings.json"


def listing(id="x", price=500000.0, listed="2026-09-01", **kw) -> Listing:
    fields = dict(source="T", address="", city="", state="", zip="", bedrooms=0, bathrooms=0,
                  sqft=0, latitude=0, longitude=0, status="active", description="")
    return Listing(id=id, price=price, listed_date=date.fromisoformat(listed), **(fields | kw))


def fixture() -> list[Listing]:
    return [
        listing("a", 400000, city="Springfield", bedrooms=2, description="Pet friendly condo"),
        listing("b", 500000, city="Fairfax", bedrooms=3, description="Fenced yard"),
        listing("c", 600000, city="Reston", bedrooms=4, description="Near the Town Center, pets welcome"),
    ]
