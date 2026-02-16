# Donation API Documentation

## Overview

The Donation API provides a "helping hand" feature that allows users to donate to communities. This backend architecture supports:

- Creating donations to communities
- Tracking donation statistics and supporters
- Managing anonymous and public donations
- Payment processing integration (ready for webhooks)

## Architecture

### Database Schema

#### `donations` Table
Tracks individual donations from users to communities.

| Column | Type | Description |
|--------|------|-------------|
| id | BINARY(12) | Unique donation identifier |
| user_id | BINARY(12) | User making the donation |
| community_id | BINARY(12) | Community receiving the donation |
| amount | INT | Donation amount in cents |
| currency | VARCHAR(3) | Currency code (e.g., "USD") |
| status | VARCHAR(20) | pending, completed, failed, refunded |
| payment_method | VARCHAR(50) | Payment method used (e.g., "stripe") |
| payment_reference | VARCHAR(255) | External payment ID/reference |
| message | TEXT | Optional message from donor |
| anonymous | BOOLEAN | Whether to hide donor identity |
| created_at | DATETIME | When donation was created |
| updated_at | DATETIME | When donation was last updated |

#### `community_donation_stats` Table
Aggregated statistics per community.

| Column | Type | Description |
|--------|------|-------------|
| community_id | BINARY(12) | Community identifier |
| total_amount | INT | Total donations received (in cents) |
| donor_count | INT | Number of unique donors |
| donation_count | INT | Total number of donations |
| last_donation_at | DATETIME | Most recent donation timestamp |
| updated_at | DATETIME | Last statistics update |

#### `donation_supporters` Table
Tracks top supporters per community.

| Column | Type | Description |
|--------|------|-------------|
| id | INT | Auto-increment identifier |
| community_id | BINARY(12) | Community identifier |
| user_id | BINARY(12) | Supporter user ID |
| total_donated | INT | Total amount donated (in cents) |
| donation_count | INT | Number of donations made |
| first_donation_at | DATETIME | First donation timestamp |
| last_donation_at | DATETIME | Most recent donation timestamp |

### Core Models

The `core/donation.go` file provides:

- **Donation**: Main donation entity
- **CommunityDonationStats**: Aggregated statistics
- **DonationSupporter**: Supporter information
- **DonationStatus**: Enum for donation states

### API Endpoints

All endpoints are prefixed with `/api/`.

## Endpoints

### 1. Create Donation

**POST** `/api/communities/{communityId}/donate`

Creates a new donation to a community.

**Authentication**: Required

**Path Parameters**:
- `communityId` (string): The community's unique identifier

**Request Body**:
```json
{
  "amount": 1000,           // Amount in cents (e.g., $10.00)
  "currency": "USD",        // Currency code (optional, defaults to "USD")
  "message": "Great work!", // Optional message from donor
  "anonymous": false        // Whether to donate anonymously
}
```

**Response** (200 OK):
```json
{
  "id": "donation_id",
  "userId": "user_id",
  "communityId": "community_id",
  "amount": 1000,
  "currency": "USD",
  "status": "pending",
  "message": "Great work!",
  "anonymous": false,
  "createdAt": "2026-02-16T21:50:00Z",
  "updatedAt": "2026-02-16T21:50:00Z"
}
```

**Errors**:
- `401 Unauthorized`: User not logged in
- `400 Bad Request`: Invalid amount or community ID
- `404 Not Found`: Community not found

---

### 2. Complete Donation

**POST** `/api/donations/{donationId}/complete`

Marks a donation as completed. Typically called by payment processor webhooks.

**Authentication**: Required (Admin or webhook)

**Path Parameters**:
- `donationId` (string): The donation's unique identifier

**Request Body**:
```json
{
  "paymentMethod": "stripe",
  "paymentReference": "ch_1234567890"
}
```

**Response** (200 OK):
```json
{
  "id": "donation_id",
  "userId": "user_id",
  "communityId": "community_id",
  "amount": 1000,
  "currency": "USD",
  "status": "completed",
  "paymentMethod": "stripe",
  "paymentReference": "ch_1234567890",
  "message": "Great work!",
  "anonymous": false,
  "createdAt": "2026-02-16T21:50:00Z",
  "updatedAt": "2026-02-16T21:50:10Z"
}
```

**Note**: This endpoint updates the donation statistics automatically.

---

### 3. Get User Donations

**GET** `/api/users/{username}/donations`

Retrieves donations made by a specific user.

**Authentication**: Required (Own donations only, or admin)

**Path Parameters**:
- `username` (string): The user's username

**Query Parameters**:
- `limit` (integer, optional): Number of results to return (default: 20)

**Response** (200 OK):
```json
[
  {
    "id": "donation_id",
    "userId": "user_id",
    "communityId": "community_id",
    "amount": 1000,
    "currency": "USD",
    "status": "completed",
    "message": "Great work!",
    "anonymous": false,
    "createdAt": "2026-02-16T21:50:00Z",
    "updatedAt": "2026-02-16T21:50:10Z"
  }
]
```

**Errors**:
- `401 Unauthorized`: User not logged in
- `403 Forbidden`: Cannot view other users' donations
- `404 Not Found`: User not found

---

### 4. Get Community Donations

**GET** `/api/communities/{communityId}/donations`

Retrieves public (non-anonymous) donations for a community.

**Authentication**: Optional (Admins/Mods see all donations)

**Path Parameters**:
- `communityId` (string): The community's unique identifier

**Query Parameters**:
- `limit` (integer, optional): Number of results to return (default: 20)

**Response** (200 OK):
```json
[
  {
    "id": "donation_id",
    "userId": "user_id",
    "communityId": "community_id",
    "amount": 1000,
    "currency": "USD",
    "status": "completed",
    "message": "Great work!",
    "anonymous": false,
    "createdAt": "2026-02-16T21:50:00Z",
    "updatedAt": "2026-02-16T21:50:10Z",
    "user": {
      "id": "user_id",
      "username": "johndoe",
      // ... other user fields
    }
  }
]
```

**Note**: Anonymous donations are only visible to community moderators and admins.

---

### 5. Get Community Donation Statistics

**GET** `/api/communities/{communityId}/donations/stats`

Retrieves aggregated donation statistics for a community.

**Authentication**: Not required

**Path Parameters**:
- `communityId` (string): The community's unique identifier

**Response** (200 OK):
```json
{
  "communityId": "community_id",
  "totalAmount": 50000,       // Total raised in cents ($500.00)
  "donorCount": 25,            // Number of unique donors
  "donationCount": 47,         // Total number of donations
  "lastDonationAt": "2026-02-16T21:50:00Z",
  "updatedAt": "2026-02-16T21:50:00Z"
}
```

**Note**: Returns zero statistics if no donations exist yet.

---

### 6. Get Top Supporters

**GET** `/api/communities/{communityId}/supporters`

Retrieves the top supporters (donors) for a community.

**Authentication**: Not required

**Path Parameters**:
- `communityId` (string): The community's unique identifier

**Query Parameters**:
- `limit` (integer, optional): Number of results to return (default: 10, max: 50)

**Response** (200 OK):
```json
[
  {
    "id": 1,
    "communityId": "community_id",
    "userId": "user_id",
    "totalDonated": 5000,      // Total donated in cents ($50.00)
    "donationCount": 5,
    "firstDonationAt": "2026-01-01T10:00:00Z",
    "lastDonationAt": "2026-02-16T21:50:00Z",
    "user": {
      "id": "user_id",
      "username": "johndoe",
      // ... other user fields
    }
  }
]
```

---

## Integration Guide

### Payment Processing Integration

The donation system is designed to integrate with payment processors like Stripe, PayPal, or cryptocurrency wallets:

1. **Create Donation**: Frontend calls `POST /api/communities/{communityId}/donate`
2. **Payment Processing**: Frontend redirects to payment processor
3. **Payment Webhook**: Payment processor calls `POST /api/donations/{donationId}/complete`
4. **Statistics Update**: System automatically updates statistics when donation is completed

### Example Flow

```javascript
// 1. Create donation
const donation = await fetch('/api/communities/comm_id/donate', {
  method: 'POST',
  body: JSON.stringify({
    amount: 1000,  // $10.00
    currency: 'USD',
    message: 'Keep up the great work!',
    anonymous: false
  })
});

// 2. Process payment with Stripe (example)
const stripe = Stripe('pk_...');
const result = await stripe.confirmCardPayment(clientSecret, {
  payment_method: {
    card: cardElement,
  }
});

// 3. Webhook completes donation (backend)
// Payment processor webhook → POST /api/donations/{donationId}/complete
```

## Error Handling

All endpoints return standard HTTP status codes:

- `200 OK`: Successful request
- `400 Bad Request`: Invalid input or validation error
- `401 Unauthorized`: Authentication required
- `403 Forbidden`: Insufficient permissions
- `404 Not Found`: Resource not found
- `500 Internal Server Error`: Server error

Error responses follow this format:
```json
{
  "code": "error_code",
  "message": "Human-readable error message"
}
```

## Security Considerations

1. **Authentication**: All donation creation requires authentication
2. **Authorization**: Users can only view their own donations (unless admin/mod)
3. **Anonymous Donations**: User identity is protected for anonymous donations
4. **Payment Security**: Integration with trusted payment processors recommended
5. **Rate Limiting**: Consider implementing rate limits on donation endpoints
6. **Audit Trail**: All donations are tracked with timestamps and payment references

## Future Enhancements

Potential future features:

- Donation goals and progress tracking
- Recurring donations/subscriptions
- Donation tiers with perks
- Tax receipt generation
- Refund handling
- Multi-currency support
- Cryptocurrency payment integration
- Donation notifications
- Donor badges/recognition system

## Testing

To test the donation system:

1. Run migrations: `./discuit migrate run`
2. Start the server: `./discuit serve`
3. Create a test donation using curl or Postman
4. Verify database records
5. Check statistics updates

Example test:
```bash
# Create donation
curl -X POST http://localhost:8080/api/communities/{communityId}/donate \
  -H "Content-Type: application/json" \
  -H "Cookie: session=..." \
  -d '{"amount": 1000, "currency": "USD", "message": "Test donation"}'

# Get community stats
curl http://localhost:8080/api/communities/{communityId}/donations/stats
```
