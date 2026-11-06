import math
from dataclasses import dataclass
from datetime import date
from pathlib import Path

from pydantic import BaseModel, ConfigDict, TypeAdapter
from pydantic.alias_generators import to_camel

BUDGET_WEIGHT = 0.7
RECENCY_WEIGHT = 0.3
RECENCY_WINDOW_DAYS = 90


class CamelModel(BaseModel):
    """snake_case in Python, camelCase in JSON."""

    model_config = ConfigDict(alias_generator=to_camel, validate_by_name=True)


class Listing(CamelModel):
    id: str
    source: str
    address: str
    city: str
    state: str
    zip: str
    price: float
    bedrooms: int
    bathrooms: float
    sqft: int
    latitude: float
    longitude: float
    listed_date: date
    status: str
    description: str


class ScoredListing(Listing):
    score: float


class Result(CamelModel):
    results: list[ScoredListing]
    page: int
    page_size: int
    total: int
    total_pages: int


@dataclass(frozen=True)
class Params:
    """Validated search input. None means "filter not set", which differs from 0."""

    min_price: float | None = None
    max_price: float | None = None
    min_bedrooms: int | None = None
    city: str = ""
    keyword: str = ""
    target_budget: float | None = None
    page: int = 1
    page_size: int = 10


class PageOutOfRange(Exception):
    def __init__(self, page: int, total_pages: int):
        super().__init__(f"page {page} is past the last page ({total_pages})")
        self.page = page
        self.total_pages = total_pages


def normalize(s: str) -> str:
    return s.strip().lower()


def load(path: Path | str) -> list[Listing]:
    return TypeAdapter(list[Listing]).validate_json(Path(path).read_bytes())


def score(listing: Listing, target_budget: float | None, today: date) -> float:
    """Relevance in 0..100. Both terms are linear and clamped to 0..1: budget fit
    falls off with relative distance from the target (symmetric over and under),
    and recency falls to 0 at RECENCY_WINDOW_DAYS."""
    age_days = max(0, (today - listing.listed_date).days)
    recency = max(0.0, 1 - age_days / RECENCY_WINDOW_DAYS)
    if target_budget is None:
        return _round1(100 * recency)
    budget_fit = max(0.0, 1 - abs(listing.price - target_budget) / target_budget)
    return _round1(100 * (BUDGET_WEIGHT * budget_fit + RECENCY_WEIGHT * recency))


def _round1(x: float) -> float:
    # round() rounds half to even; scores round half up, matching the Go version.
    return math.floor(x * 10 + 0.5) / 10


def search(listings: list[Listing], p: Params, today: date) -> Result:
    if p.page < 1 or p.page_size < 1:
        raise ValueError(f"invalid paging: page={p.page} page_size={p.page_size}")

    scored = [
        ScoredListing(**l.model_dump(), score=score(l, p.target_budget, today))
        for l in listings
        if _matches(l, p)
    ]
    scored.sort(key=lambda s: (-s.score, s.price, s.id))

    total = len(scored)
    total_pages = math.ceil(total / p.page_size)
    if total and p.page > total_pages:
        raise PageOutOfRange(p.page, total_pages)
    start = (p.page - 1) * p.page_size

    return Result(
        results=scored[start : start + p.page_size],
        page=p.page,
        page_size=p.page_size,
        total=total,
        total_pages=total_pages,
    )


def _matches(l: Listing, p: Params) -> bool:
    city, keyword = normalize(p.city), normalize(p.keyword)
    return (
        (p.min_price is None or l.price >= p.min_price)
        and (p.max_price is None or l.price <= p.max_price)
        and (p.min_bedrooms is None or l.bedrooms >= p.min_bedrooms)
        and (not city or normalize(l.city) == city)
        and (not keyword or keyword in l.description.lower())
    )
