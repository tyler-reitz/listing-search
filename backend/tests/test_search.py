import json
from datetime import date

import pytest
from pydantic import ValidationError

from app.search import PageOutOfRange, Params, _round1, load, score, search
from factories import DATA, NOW, fixture, listing


def ids(result) -> list[str]:
    return [r.id for r in result.results]


def test_load_sample_data():
    ls = load(DATA)
    assert len(ls) == 12
    assert (ls[0].id, ls[0].price, ls[0].listed_date) == ("A1", 450000, date(2026, 8, 29))


def test_load_rejects_bad_date(tmp_path):
    record = load(DATA)[0].model_dump(mode="json", by_alias=True) | {"listedDate": "09/01/2026"}
    bad = tmp_path / "bad.json"
    bad.write_text(json.dumps([record]))
    with pytest.raises(ValidationError) as exc:
        load(bad)
    assert [e["loc"][-1] for e in exc.value.errors()] == ["listedDate"]


@pytest.mark.parametrize(
    "price, listed, budget, want",
    [
        # 0.7*1.0 + 0.3*(1 - 9/90) = 0.97
        (500000, "2026-09-01", 500000, 97.0),
        # 0.7*0.9 + 0.3*(1 - 30/90) = 0.83
        (450000, "2026-08-11", 500000, 83.0),
        (550000, "2026-08-11", 500000, 83.0),
        (1100000, "2026-05-01", 500000, 0.0),
        (500000, "2026-09-01", None, 90.0),
        (500000, "2026-09-20", None, 100.0),
    ],
    ids=["exact budget", "10% under", "10% over same as under", "clamps to 0", "no budget", "future date"],
)
def test_score(price, listed, budget, want):
    assert score(listing(price=price, listed=listed), budget, NOW) == want


@pytest.mark.parametrize(
    "params, want",
    [
        (Params(), ["a", "b", "c"]),
        (Params(min_price=500000), ["b", "c"]),
        (Params(max_price=500000), ["a", "b"]),
        (Params(min_bedrooms=3), ["b", "c"]),
        (Params(city="  fairFAX "), ["b"]),
        (Params(keyword="PET"), ["a", "c"]),
        (Params(keyword="   "), ["a", "b", "c"]),
        (Params(min_price=450000, keyword="pet"), ["c"]),
        (Params(keyword="pool"), []),
    ],
    ids=["none", "minPrice inclusive", "maxPrice inclusive", "minBedrooms", "city normalized",
         "keyword case", "blank keyword", "combined", "no matches"],
)
def test_search_filters(params, want):
    # No budget and equal dates means equal scores, so order is price asc.
    result = search(fixture(), params, NOW)
    assert ids(result) == want
    assert result.total == len(want)


def test_search_ranks_by_score():
    # b is on budget; a and c are both 20% off, so they tie and a wins on price.
    assert ids(search(fixture(), Params(target_budget=500000), NOW)) == ["b", "a", "c"]


def test_search_tie_breaks_by_price_then_id():
    ls = [listing("X", 600000), listing("Y", 500000), listing("W", 500000)]
    assert ids(search(ls, Params(), NOW)) == ["W", "Y", "X"]


def make_listings(n: int) -> list[Listing]:
    return [listing(f"L{i:02d}", 100000 + i * 1000) for i in range(n)]


@pytest.mark.parametrize(
    "n, page, size, want, want_pages",
    [
        (5, 1, 2, ["L00", "L01"], 3),
        (5, 3, 2, ["L04"], 3),
        (4, 2, 2, ["L02", "L03"], 2),
        (0, 1, 2, [], 0),
    ],
    ids=["first page", "partial last page", "exact multiple", "no results"],
)
def test_search_pagination(n, page, size, want, want_pages):
    result = search(make_listings(n), Params(page=page, page_size=size), NOW)
    assert ids(result) == want
    assert (result.total, result.total_pages, result.page, result.page_size) == (n, want_pages, page, size)


def test_search_page_past_end():
    with pytest.raises(PageOutOfRange) as exc:
        search(make_listings(5), Params(page=4, page_size=2), NOW)
    assert exc.value.total_pages == 3


@pytest.mark.parametrize("params", [Params(page=0), Params(page_size=0)])
def test_search_rejects_invalid_paging(params):
    with pytest.raises(ValueError):
        search(fixture(), params, NOW)


def test_scores_round_half_up():
    # Built-in round() would give 0.2 and 83.2 (half to even).
    assert (_round1(0.25), _round1(83.25)) == (0.3, 83.3)
