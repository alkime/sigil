# Payment Processing System Design

## Overview

This document describes the architecture for the new payment processing system. It handles credit card transactions, refunds, and subscription billing for all customer tiers.

<!-- @review-ref 0002 -->
## Requirements

- Process up to 10,000 transactions per minute at peak load
- Support Visa, Mastercard, and AMEX
- PCI DSS Level 1 compliance
- 99.99% uptime SLA
- Sub-200ms p99 latency for authorization requests

## Architecture

### Component Diagram

The system consists of three main services:

1. **Gateway Service** — accepts incoming payment requests, validates merchant credentials, and routes to the appropriate processor
2. **Processing Engine** — handles the actual transaction lifecycle: authorization, capture, void, and refund
3. **Ledger Service** — maintains the double-entry accounting ledger and reconciliation

### Data Flow

```
Client → API Gateway → Gateway Service → Processing Engine → Card Network
                                              ↓
                                        Ledger Service → PostgreSQL
```

### Database Schema

We use PostgreSQL with the following core tables:

| Table | Purpose | Estimated Rows |
|-------|---------|---------------|
| `transactions` | All payment events | ~500M |
| `merchants` | Merchant accounts | ~50K |
| `settlements` | Daily settlement batches | ~2M |
| `refunds` | Refund records linked to transactions | ~10M |

Key indexes:

```sql
CREATE INDEX idx_txn_merchant_date ON transactions (merchant_id, created_at);
CREATE INDEX idx_txn_status ON transactions (status) WHERE status = 'pending';
CREATE INDEX idx_settlements_date ON settlements (settlement_date);
```

### Authentication & Authorization

Merchants authenticate using API keys with HMAC-SHA256 request signing:

```python
import hmac
import hashlib

def sign_request(api_secret, method, path, body, timestamp):
    message = f"{method}\n{path}\n{timestamp}\n{body}"
    return hmac.new(
        api_secret.encode(),
        message.encode(),
        hashlib.sha256
    ).hexdigest()
```

Each API key has scoped permissions:

- `payments:write` — create charges and captures
- `payments:read` — view transaction history
- `refunds:write` — issue refunds
- `settlements:read` — view settlement reports

<!-- @review-ref 0001 -->
### Error Handling

All errors follow a structured format:

```json
{
  "error": {
    "code": "card_declined",
    "message": "The card was declined by the issuing bank.",
    "decline_code": "insufficient_funds",
    "param": "card_number"
  }
}
```

Common decline codes:

- `insufficient_funds` — card has insufficient balance
- `expired_card` — card expiration date has passed
- `fraud_suspected` — issuer flagged as potentially fraudulent
- `do_not_honor` — generic decline from issuer

### Rate Limiting

We implement token bucket rate limiting per merchant:

> **Default limits:**
> - 100 requests/second sustained
> - 500 requests/second burst
> - Configurable per merchant tier

Rate limit headers are included in every response:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 73
X-RateLimit-Reset: 1679616000
```

## Deployment

### Infrastructure

- **Compute:** 3x `m6i.2xlarge` instances in each of 3 AZs
- **Database:** RDS PostgreSQL Multi-AZ with read replicas
- **Cache:** ElastiCache Redis cluster for session and rate limit state
- **Queue:** SQS FIFO queues for async settlement processing

### Rollout Plan

Phase 1 (Week 1-2):
- Deploy to staging with synthetic traffic
- Run parallel processing against production (shadow mode)
- Validate ledger accuracy to 6 decimal places

Phase 2 (Week 3-4):
- Gradual rollout: 1% → 10% → 50% → 100% of live traffic
- Monitor error rates, latency percentiles, and settlement accuracy
- Automatic rollback trigger: error rate > 0.1% or p99 > 500ms

## Open Questions

- Should we support ACH/bank transfer in the initial release or defer to v2?
- What is the retention policy for raw transaction logs? Legal wants 7 years, ops wants 90 days hot / cold archive after that.
- Do we need real-time webhook delivery guarantees or is at-least-once with retry acceptable?

---

*Last updated: 2026-03-24*

<!--
@review-backmatter

"0001":
  offset: 1
  span: 1
  comment: "Errors!"
  status: open

"0002":
  offset: 1
  span: 26
  comment: "bad ass"
  status: open

-->
