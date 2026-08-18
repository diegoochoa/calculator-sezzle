# Prompt log

One repository, two services:

- `frontend/` — React + TypeScript shell
- `backend/` — Go calculation service

They began as sibling directories and were consolidated on 17 August; see the
final entry.

## 1

Think as a full stack developer working for a fintech company and you have been given the task to implement a calculator with the next characteristics:

> 1. Front End made in React
> 2. There must be a clean UI/UX design
> 3. Organize folders according to the logic and architecture of the application
> 4. We need at first basic operations for a calculator

---

## 2. Choosing the scope

> Lets add tier 0, tier 1. This will improve UI/UX design and not give the user a full and complex view from the start

---

## 3. Splitting into front end and backend

> Separate the project into front end and backend, the back and must be made in golang. Lets plan ahead first of all requirements:
>
> 1. Endpoints for calculations and processes
> 2. Validate input and handle edge cases such as division by zero, invalid data
> 3. Data must bu returned in JSON format
> 4. lets add a rate limit
> 5. add a security layer, could be a token preferable
> 6. Lets consider that both projects should run with a docker configuration, but we can leave this at the end
> 7. Every functional code must have its unit testing and we also need coverage report

---

## 4. Containers

> Perfect, now lets move on with colima and docker compose, I need those

**Outcome:** colima, `docker-compose` and `docker-buildx` installed; both images
built and the stack verified end to end. Documenting `docker build --target
test` exposed two real bugs in it — a bash-only coverage script on a bash-free
Alpine image, and a coverage scope that contradicted the Makefile.

---
