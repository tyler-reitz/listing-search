import pytest
from fastapi.testclient import TestClient

from app.main import create_app
from factories import NOW, fixture


@pytest.fixture
def client() -> TestClient:
    return TestClient(create_app(fixture(), lambda: NOW))


def search(client: TestClient, query: str):
    return client.get(f"/api/listings/search?{query}")


@pytest.mark.parametrize(
    "query, fields",
    [
        ("minPrice=abc", ["minPrice"]),
        ("minPrice=NaN", ["minPrice"]),
        ("minPrice=inf", ["minPrice"]),
        ("minPrice=-1", ["minPrice"]),
        ("maxPrice=-1", ["maxPrice"]),
        ("minBedrooms=2.5", ["minBedrooms"]),
        ("minBedrooms=-1", ["minBedrooms"]),
        ("targetBudget=0", ["targetBudget"]),
        ("minPrice=500000&maxPrice=300000", ["maxPrice"]),
        ("page=0", ["page"]),
        ("pageSize=0", ["pageSize"]),
        ("pageSize=-1", ["pageSize"]),
        ("pageSize=51", ["pageSize"]),
        ("city=Atlantis", ["city"]),
        ("page=99", ["page"]),
        # Field errors are reported together. City needs the data, so it is checked
        # only after the query model validates.
        ("minPrice=abc&pageSize=0&city=Atlantis", ["minPrice", "pageSize"]),
        # The min/max check only runs once minPrice has parsed.
        ("minPrice=abc&maxPrice=1", ["minPrice"]),
    ],
)
def test_validation_errors(client, query, fields):
    res = search(client, query)
    assert res.status_code == 422, res.text
    assert [e["field"] for e in res.json()["errors"]] == fields


def test_unknown_city_lists_known_cities(client):
    assert "Fairfax, Reston, Springfield" in search(client, "city=Atlantis").text


def test_page_past_end_reports_last_page(client):
    assert "(1)" in search(client, "page=99").text


def test_min_max_message_is_readable(client):
    res = search(client, "minPrice=500000&maxPrice=300000")
    assert res.json()["errors"][0]["message"] == "maxPrice (300000) must be >= minPrice (500000)"


def test_valid_query_with_no_matches_is_200(client):
    res = search(client, "city=Fairfax&keyword=pool")
    assert res.status_code == 200
    assert (res.json()["results"], res.json()["total"]) == ([], 0)


def test_city_match_ignores_case_and_whitespace(client):
    res = search(client, "city=%20FAIRFAX%20")
    assert (res.status_code, res.json()["total"]) == (200, 1)


def test_blank_params_are_unset(client):
    res = search(client, "minPrice=&page=&city=")
    assert (res.status_code, res.json()["total"], res.json()["page"]) == (200, 3, 1)


def test_happy_path(client):
    res = search(client, "targetBudget=500000&pageSize=2")
    assert res.status_code == 200
    body = res.json()
    assert {k: body[k] for k in ("page", "pageSize", "total", "totalPages")} == {
        "page": 1, "pageSize": 2, "total": 3, "totalPages": 2,
    }
    assert [(r["id"], r["score"]) for r in body["results"]] == [("b", 97.0), ("a", 83.0)]
    assert body["results"][0]["listedDate"] == "2026-09-01"
