---
title: "Advanced FastAPI"
date: 2026-02-05
tags: [python, fastapi, api, middleware, mcp]
summary: "Some learnings from building and maintaining a 300k+ lines FastAPI app"
draft: false
---

# Some learnings from building and maintaining a 300k+ lines FastAPI app

The main backend service for [Definite](https://definite.app) is written in FastAPI and is over 300k lines of pure Python. Before we pivoted, during the Luabase days, we used Flask but found it lacking given all of the hype around FastAPI. When we got the chance to write a backend from scratch, we knew we wanted to try FastAPI. Many on the team were _familiar_ with FastAPI but had never built anything of scale with it so there were a lot of trial and error.

There are tons of talks about how to squeeze out some performance or what the "best practices" are, but I wanted to add some of the specific things we had to figure out on our own.

## Unified Auth

Originally, we had different routes for things like core API (for Supabase session-based clients), the public API (used with API key), and MCP.

That quickly got out of hand because if we needed to change in one (like "create integration") then we would have to make the same change in multiple places. That is an obvious code smell that is mostly addressed with a [service layer](#service-layer-vs-router-layer), but I wanted to go a step further and remove the need for multiple routes entirely. Everything should use the same route regardless of business logic.

So before, we had something like:

```py
@router.post(
    path="/integrations",
    dependencies=[Depends(UserAuth)],
    ...
)
async def create_integration(...) -> ...: ...

@router.post(
    path="/api/integrations",
    dependencies=[Depends(ApiKeyAuth)],
    ...
)
async def create_integration_api(...) -> ...: ...
```

This is of course a broad simplification for demonstrative purposes but still close-ish enough.

A naive solution to unification would be to just allow all authentication types.

For us we supported:

- Session tokens
- API keys
- Service accounts
- Public/unauthed

So something like:

```py
class UnifiedAuth:
    async def __call__(...) -> ...: ...
    async def _handle_session_auth(...) -> ...: ...
    async def _handle_api_key_auth(...) -> ...: ...
    async def _handle_service_account_auth(...) -> ...: ...
```

The main issue with that is that not all routes supported all auth types. The developer needed a way to specify/whitelist authentication methods per route. So you can do something like:

```py
ALL_METHODS: None = None

class UnifiedAuth:
    def __init__(self, allowed_auth: list[AuthMethod] | None = ALL_METHODS) -> None: ...
    async def __call__(...) -> ...: ...
    async def _handle_session_auth(...) -> ...: ...
    async def _handle_api_key_auth(...) -> ...: ...
    async def _handle_service_account_auth(...) -> ...: ...


@router.post(
    path="...",
    dependencies=[Depends(UnifiedAuth(allowed_auth=[AuthMethod.SESSION_TOKEN, AuthMethod.API_KEY]))],
    ...
)
...
```

This is a decent start, but a major part that is missing from this is a common authenticated principal data model that all handlers must resolve to. That way, regardless of the method used, we end up with the same data to use downstream. So now we have something like:

```py
ALL_METHODS: None = None

class Principal:
    ...
    method: AuthMethod
    ...

class UnifiedAuth:
    def __init__(self, allowed_auth: list[AuthMethod] | None = ALL_METHODS) -> None: ...
    async def __call__(...) -> Principal: ...
    async def _handle_session_auth(...) -> Principal: ...
    async def _handle_api_key_auth(...) -> Principal: ...
    async def _handle_service_account_auth(...) -> Principal: ...
```

This is nice because you can now use the result of the unified auth without needing to abunch of type checking or whatever.

```py
@router.post(
    path="/integrations",
    ...
)
async def create_integration(..., *, principal: Annotated[Principal, Depends(UnifiedAuth)]) -> ...: ...
```

Now, as long as the principal model provides enough context you can do whatever you need to with it like modifying a query template to filter on team, assert that a team is on a sufficient billing plan or has not gone over there usage, etc. This leads us to the next point, [team contexts](#team-contexts).

## Team Contexts

With our universal `Principal` model, we can start to do a lot of interesting things in the routes.

Our product is multi-tenant so each user gets a team. Each team has some context that we care about and that is required for the route handler.

Here's a little bit more of the final `Principal` model with some of the team contexts added:

```py
class Principal
    user_id: UUID
    team_id: UUID
    team_plan: TeamPlan
    method: AuthMethod
    ...
```

The first thing that this allows is we do not need to include a team in the route URL like `/teams/{team_id}/integrations`, we can just to `/integrations` because the team ID is automatically provided as context to all routes (again, regardless of authentication method). This might not seem like a big deal because how hard is is to support that vs. not? Well, for client-side applications that likely also have the team ID cached somewhere, building those URLs is trivial, but if you provide an API or SDK that wraps that API, or if you provide routes for an agent to use via MCP then requiring that the user either provides the ID at instantiation _or_ even worse, that information is fetched before subsequent calls are made is a poor implementation and leaks details to the user that they do not care about.

Second, and more important for us, is each of our team is on some sort of billing tier which entitles them to different things. The lowest tiers have resource caps and usage constraints and the higher tier does not (or is at least relaxed). So for something like create integration call, we can enforce some constraints on users depending on what billing tier their team is on.

```py
@router.post(
    path="/integrations",
    ...
)
async def create_integration(..., *, principal: Annotated[Principal, Depends(UnifiedAuth)]) -> Integration:
    if principal.team_plan == "free":
        # check of resource limits and maybe raise some exception
    ...
```

This is nice, but with just this approach you start to get away from DRY code, which leads us to the next item, [leaning on dependencies](#lean-on-dependencies)

## Lean on Dependencies

Since a lot of our routes will now have some similar code in them that checks for resource limits or some other business constraint, we want to factor that out to something we can reuse.

An immediately obvious choice may be to wrap it in a function like:

```py
def check_not_free_plan(principal: Principal):
    if principal.team_plan == "free":
        # check of resource limits and maybe raise some exception


@router.post(
    path="/integrations",
    ...
)
async def create_integration(..., *, principal: Annotated[Principal, Depends(UnifiedAuth)]) -> Integration:
    check_not_free_plan(principal)
    ...
```

This is okay, but a better approach is to not include it in the route handler, but instead to put it in a dependency like:

```py
async def check_not_free_plan(principal: Principal):
    if principal.team_plan == "free":
        # check of resource limits and maybe raise some exception

RequirePaidPlanDep = Depends(check_not_free_plan)

@router.post(
    ...
    dependencies=[
        Depends(UnifiedAuth(allowed_auth=[AuthMethod.SESSION_TOKEN, AuthMethod.API_KEY])),
        RequirePaidPlanDep,
    ]
    ...
)
...
```

I prefer this approach because it is more declarative and somewhat self-documenting.

But, now because sometimes we need to check if a user's team is paying or we need to check that a user's team is on a specific plan like `TeamPlan.Enterprise`. So, just using the approach above, we would possible have many dependencies to choose from. A better approach to that is:

```py
class RequirePlans:
    def __init__(self, plans: list[TeamPlan]) -> None: ...
    async def __call__(self) -> None: ...

@router.post(
    ...
    dependencies=[
        Depends(UnifiedAuth(allowed_auth=[AuthMethod.SESSION_TOKEN, AuthMethod.API_KEY])),
        Depends(RequirePlans(plans=[TeamPlan.Enterprise])),
    ]
    ...
)
...
```

You can kind of see where this is going. The more common business requirements we add around our routes, we want to generalize and factor out to the dependency layer if possible to take advantage of DRY programming and the self-documenting nature of the route decorators. The thing that still bugged me about this approach is that we still needed a way to have developer opt-in to using this. Obviously, the more someone has worked in the codebase and has gotten up to speed with the standards the less we need to worry about this, but for new team members or for team members that are maybe more frontend focused that still need to be able to read and modify backend code, we ran into the innevitable issues of enforcements.

This brings us to the next sections, [building a higher level of abstraction](#building-a-higher-layer-of-abstraction).

## Building a Higher Layer of Abstraction

To help enforce a lot of these approaches and to just streamline the developer experience, we've created a thin abstraction on top of FastAPI `APIRouter`.

```py
class CustomRouter(APIRouter):  # demonstration name only
    ...
    def get(  # type: ignore[override]
        *,
        path: str | None = None,
        auth: Iterable[AuthMethod] | None = ALL_METHODS,
        status_code: int = status.HTTP_200_OK,
        error_codes: Iterable[int] | None = None,
        summary: str | None = None,
        dependencies: Sequence[params.Depends] | None = None,
        deprecated: bool | None = None,
    ): ...
    # same thing for post, patch, put, delete, etc.
```

Some parameters to call out here:

1. `auth`: the developer provides allowed authentication methods (`None` is equivalent to saying support all)
2. `status_code`: The success status code returned from this route
3. `error_codes`: Error codes that can be returned by this route (more on this below in [automatic response model generation](#automatic-response-model-generation))

We intentionally take away a lot of parameters from the original methods because we want to limit choice here to make it more obvious to the developer what they need to do and to enforce conformity amongst routes.

### API Documentation Standards Enforcement

A benefit of this approach is that when you look at our OpenAPI spec (or navigate to `/docs` or `/redoc`) things are neat and organized, there is a familiarity across routes, and the documentation is more complete which is nice for downstream development like SDKs or letting agents interact with the APIs.

With the user of `auth` parameter, there will also be automatica metadata added to routes to let the developer know how they can access this route. This makes API documentation much easier to generate and prevents us on relying on overworked developers needing to manually add all of this information.

### Automatic Response Model Generation

An important piece of the standards and enforcement is the automatic generation of documentation for error codes. The way we achieve this is by only making the developer add the integer representations of error codes. This way we avoid confusion on what should the description and summary of this error code be? We curated a collection of generic, but still semantically correct descriptions and summaries for error codes that automatically resolve based on which error codes are expected from a route.

Since a lot of error codes will show for virtually all routes (e.g. authentication errors, 5XX errors, etc.) we also automatically inject those without the developer needing to do that. The developer just needs to add error codes that are not automatically included like 404s, etc.

We also [standardize the response models for successes and errors](#response-wrappers) across all routes so this makes it easier to automatically generate all of this since all routes will return a `ResponseModel[M]` with the generic model `M` being the only differentiator.

### Auth Explicitly Separate from Other Dependencies

You'll see from our earlier code samples that auth is being done via dependencies, but we have separates these from the `dependecies` parameter for the two reasons:

1. Auth is not optional and additional dependencies are
2. Cleaner separation of authentication and downstream business requirements like resource caps, service account scopes, etc.

What this also does is improve the self documentation (and automatic documentation generation) experience turning this:

```py
class RequirePlans:
    def __init__(self, plans: list[TeamPlan]) -> None: ...
    async def __call__(self) -> None: ...

class RequireRoles:
    def __init__(self, roles: list[TeamRole]) -> None: ...
    async def __call__(self) -> None: ...

@router.post(
    ...
    dependencies=[
        Depends(UnifiedAuth(allowed_auth=[AuthMethod.SESSION_TOKEN, AuthMethod.API_KEY])),
        Depends(RequirePlans(plans=[TeamPlan.Enterprise])),
        Depends(RequireRoles(plans=[TeamRole.Admin])),
    ]
    ...
)
```

Into this:

```py
class RequirePlans:
    def __init__(self, plans: list[TeamPlan]) -> None: ...
    async def __call__(self) -> None: ...

class RequireRoles:
    def __init__(self, roles: list[TeamRole]) -> None: ...
    async def __call__(self) -> None: ...

@router.post(
    ...
    auth=[AuthMethod.SESSION_TOKEN, AuthMethod.API_KEY],
    dependencies=[
        Depends(RequirePlans(plans=[TeamPlan.Enterprise])),
        Depends(RequireRoles(plans=[TeamRole.Admin])),
    ]
    ...
)
```

And, if we start seeing this exact pattern over and over and over again we can continue to abstract into something like:

```py
@router.post(
    ...
    auth=[AuthMethod.SESSION_TOKEN, AuthMethod.API_KEY],
    plans=[TeamPlan.Enterprise],
    roles=[TeamRole.Admin],
    ...
)
```

So we will progressively remove all scalfolding/undifferentiated code overtime.

## Service Layer vs. Router Layer

Jumping back out a level, we also strive to have as slim of route handlers as possible. We want the routes to serve as a declaritive and documentation layer that basically is just the decorator and a handler that is only one line (minus any docstrings). This helps us by making sure that we've encapsulated things correctly in the service layer and making sure that any and all code paths are handled locally instead of bolted on at the router. Something we might have had before would have been:

```py
@router.post(
    path="/integrations",
    summary="Create integration",
    status_code=201,
    responses: {
        201: {...},
        401: {...},
        ...,
        500: {...},
    },
    ...,
async def create_integration(
    req: CreateIntegrationReq,
    *,
    principal: PrincipalDep,
    integrations: IntegrationControllerDep,
) -> Integration:
    if principal.role == TeamRole.Viewer:
        # Not allowed error response
    if principal.team_plan == TeamPlan.Free:
        # Not allowed error response
    try:
        match req.type:
            # handle different integration types
            case IntegrationType.Postgres:
                ...
            case IntegrationType.MySQL:
                ...
            ...
    except ... as e:
        # some exception
    except ... as e:
        # some exception
    except ... as e:
        # some exception
```

Then the problem we would face is when internal services needed to control other internal services. All of this logic would either need to be factored out to the service layer (preferred) or duplicated. Instead, we start with that end in mind and make sure that everything is encapsulated at the start and that everything related to what the controller needs to do lives within that controller and does not leak out to other layers. So with this principal and the other things that we've talkes about above, the sample turns into this instead:

```py
@router.post(
    path="/integrations",
    summary="Create integration",
    status_code=201,
    auth=[AuthMethod.SESSION_TOKEN, AuthMethod.API_KEY],
    plans=[Not(TeamPlan.Free)],
    roles=[Not(TeamRole.Viewer)],
    ...
)
async def create_integration(
    req: CreateIntegrationReq,
    *,
    integrations: IntegrationControllerDep,
) -> Integration:
    return await integrations.create(req)
```

This is preferred because it is more declaritve and obvious what is happening here with what the handler will do and what the documentation will show.

A big unlock for this is [resolvable errors](#resolvable-errors).

## Resolvable Errors

If we encapsulate everything in the service layer then we somehow need an exception thrown locally to bubble up to the client response. To do this we've create an extensive exceptions layer starting with the base exception class:

```py
class BaseApiError(Exception):
    ...
```

The role of this base class is to allow us to convert an exception to an [error response](#response-wrappers) including a status code, a summary, a machine-readable error, and any context and details necessary for the client to debug.

We've also added automatic FastAPI exception handlers to be mounted for all subclasses of that base class. That way, no matter where in the codebase an exception is thrown, if it is not caught it will automatically result in a correct error response to the client without a developer needing to worry about anything other than creating the exceptions.

This layer is still in the early phases but at a high-level consists of these components:

```py
class ErrorCode(StrEnum):
    AUTH_INVALID_TOKEN = "AUTH_INVALID_TOKEN"
    ...
    @classmethod
    def from_status_code(cls, status_code: int) -> ErrorCode: ...
    def to_status_code(self) -> int: ...

class ErrorDetail(BaseModel):
    field: str = Field(description="Field that caused the error")
    message: str = Field(description="Human-readable error message")
    code: str = Field(description="Field-specific error code")

class ErrorBody(BaseModel):
    code: ErrorCode = Field(description="Machine-readble error code")
    message: str = Field(description="Client message")
    details: list[ErrorDetail] = Field(default_factory=list)
    context: dict[str, Any] | None = Field(default=None)

class ErrorResponse(BaseResponse[Literal[False], ErrorBody]): ...

class BaseApiError(Exception):
    def __init__(self, error: ErrorBody) -> None: ...
    @classmethod
    async def exc_handler[T: BaseApiError](cls: type[T], request: Request, exc: T) -> JSONResponse: ...
    @classmethod
    def register[T: BaseApiError](cls: type[T], app: FastAPI): ...
    def to_response(self) -> JSONResponse: ...
```

Then we have automatic recursive registration of all exception subclasses with:

```py
def register_exception_handlers(app: FastAPI):
    def register_recursive(cls: type[BaseApiError]):
        cls.register(app)
        for subclass in cls.__subclasses__():
            register_recursive(subclass)

    register_recursive(BaseApiError)
```

And we can create new subclasses like this:

```py
class NotFound(BaseApiError):
    def __init__(self, resource_name: str, resource_id: UUID) -> None:
        error = ErrorBody(code=ErrorCode.RESOURCE_NOT_FOUND, message=f"{resource_name.upper()} {resource_id} not found")
        super().__init__(error)
```

And continue building up abstractions too:

```py
class IntegrationNotFound(NotFound):
    def __init__(self, integration_id: UUID) -> None:
        super().__init__("Integration", integration_id)
```

## Response Wrappers

Something to highlight from above is this line:

```py
class ErrorResponse(BaseResponse[Literal[False], ErrorBody]): ...
```

Something we came learned we wanted to hard way was a universal response model that had the same shape regardless of if it was an error or a success. This is mainly because the `try {} catch {}` approach used througout our frontend client codebase just becomes a mess and we ended up doing all of these ridiculous type gymnastics to allow the caller to know the returned type depending on if it was a success or failure. If we just _always_ return a generic `ResponseModel[M]` then everything downstream becomes a lot easier.

So now we have:

```py
class ResponseMeta(BaseModel):
    request_id: str = Field(default="", alias="requestId", description="Unique request identifier for tracing")
    timestamp: dt.datetime = Field(
        default_factory=lambda: dt.datetime.now(dt.UTC), description="Response timestamp (UTC)"
    )
    duration_ms: int | None = Field(
        default=None, alias="durationMs", description="Request processing time in milliseconds"
    )

class BaseResponse[K: bool, T](BaseModel):
    success: K = Field(description="Flag for it request was successful")
    data: T = Field(description="Response data")
    meta: ResponseMeta = Field(default_factory=ResponseMeta, description="Response metadata")

    def as_content(self) -> dict[str, Any]:
        return self.model_dump(by_alias=True, mode="json")

class SuccessResponse[T](BaseResponse[Literal[True], T]): ...

class ErrorResponse(BaseResponse[Literal[False], ErrorBody]): ...
```

Resulting in a universal response shape to the client like:

```json
{
    "success": ...,
    "data": {...},
    "meta": {
        "requestId": "...",
        "timestamp": "...",
        "durationMs": "...",
    }
}
```

Not only do we get the benefit of easier client typing, but we also get to have a standard for debugging responses with the `requestId` and timing metadata. The `requestId` is the corrlation ID used in Sentry and Google Cloud Logging, so as soon as we see an error client-side we can grab that ID and immediately find the cause. Same thing if we see a large duration number and want to track down the related traces and logs.

## Caching Within a Request

We've talked a lot about depedencies here, but another feature we make use of to remove common code is [middleware](https://fastapi.tiangolo.com/tutorial/middleware). Since a lot of our requirements center around the team contexts, we need a way to provide the principal object to the middleware. To do that we've made use of [Starlette Request State](https://starlette.dev/requests/#other-state):

```py
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import JSONResponse, Response


class CustomMiddleware(BaseHTTPMiddleware):
    async def dispatch(self, request: Request, call_next: RequestResponseEndpoint) -> Response:
        if not hasattr(request.state, "principal"):
            raise AuthMissingCredentials
        principal = request.state.principal
        ...
```
