from collections.abc import Callable
from datetime import date
from typing import Annotated, Any

from fastapi import FastAPI, Query, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from pydantic import Field, ValidationInfo, field_validator
from pydantic_core import PydanticUseDefault

from app.search import CamelModel, Listing, PageOutOfRange, Params, Result, normalize, search

MAX_PAGE_SIZE = 50


class SearchQuery(CamelModel):
    min_price: float | None = Field(None, ge=0, allow_inf_nan=False)
    max_price: float | None = Field(None, ge=0, allow_inf_nan=False)
    min_bedrooms: int | None = Field(None, ge=0)
    city: str = ""
    keyword: str = ""
    target_budget: float | None = Field(None, gt=0, allow_inf_nan=False)
    page: int = Field(1, ge=1)
    page_size: int = Field(10, ge=1, le=MAX_PAGE_SIZE)

    @field_validator("*", mode="before")
    @classmethod
    def blank_means_unset(cls, v: Any) -> Any:
        if isinstance(v, str) and not v.strip():
            raise PydanticUseDefault()
        return v

    @field_validator("max_price")
    @classmethod
    def max_not_below_min(cls, v: float | None, info: ValidationInfo) -> float | None:
        lo = info.data.get("min_price")
        if v is not None and lo is not None and v < lo:
            raise ValueError(f"maxPrice ({_num(v)}) must be >= minPrice ({_num(lo)})")
        return v


def _num(v: float) -> str:
    return str(int(v)) if v.is_integer() else str(v)


def _errors(status: int, errors: list[dict[str, str]]) -> JSONResponse:
    return JSONResponse({"errors": errors}, status_code=status)


def _field_error(e: dict[str, Any]) -> dict[str, str]:
    loc = e["loc"]
    field = loc[-1] if len(loc) > 1 and isinstance(loc[-1], str) else ""
    # Our own ValueErrors carry a full sentence; Pydantic prefixes them with "Value error, ".
    message = str(e["ctx"]["error"]) if e["type"] == "value_error" else e["msg"]
    return {"field": field, "message": message}


def create_app(listings: list[Listing], today: Callable[[], date]) -> FastAPI:
    app = FastAPI(title="Listing search")

    known_cities: dict[str, str] = {}
    for l in listings:
        known_cities.setdefault(normalize(l.city), l.city.strip())
    city_names = ", ".join(sorted(known_cities.values()))

    @app.exception_handler(RequestValidationError)
    async def validation_error(_: Request, exc: RequestValidationError) -> JSONResponse:
        return _errors(422, [_field_error(e) for e in exc.errors()])

    @app.exception_handler(Exception)
    async def unexpected_error(_: Request, exc: Exception) -> JSONResponse:
        return _errors(500, [{"field": "", "message": "internal error"}])

    @app.get("/api/listings/search", response_model=Result)
    def search_listings(q: Annotated[SearchQuery, Query()]):
        if normalize(q.city) and normalize(q.city) not in known_cities:
            message = f'unknown city "{q.city.strip()}"; known cities: {city_names}'
            return _errors(422, [{"field": "city", "message": message}])
        try:
            return search(listings, Params(**q.model_dump()), today())
        except PageOutOfRange as e:
            return _errors(422, [{"field": "page", "message": str(e)}])

    return app
