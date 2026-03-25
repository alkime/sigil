# Architecture Design

<!-- @review-ref 0001 -->
The system uses a simple token-based auth flow
where users authenticate via a shared secret
that is passed in the Authorization header
on every request.

<!-- @review-ref 0002 -->
## Database Schema

We use a single `users` table with no indexes.

## Deployment

Standard Docker-based deployment to fly.io.
<!-- @review-ref 0003 -->

<!--
@review-backmatter

"0001":
  offset: 1
  span: 4
  comment: "This undersells the OAuth complexity. Expand with redirect flow details."
  status: open

"0002":
  offset: 1
  span: 1
  comment: "Missing the indexes discussion entirely."
  status: open

"0003":
  offset: 1
  span: 1
  comment: "New comment at the end."
  status: open

-->
