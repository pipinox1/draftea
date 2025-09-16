## Event Catalog

### 1. Payment Domain Events

#### payment.created
```json
{
  "type": "payment.created",
  "id": "payment-123",
  "version": "1.0",
  "timestamp": "2024-01-15T10:30:00Z",
  "payload": {
    "payment_id": "payment-123",
    "user_id": "user-456",
    "amount": {
      "value": 100.00,
      "currency": "USD"
    },
    "payment_methods": [
      {
        "type": "wallet",
        "walletId": "wallet-789",
        "value": 100.00
      }
    ]
  }
}
```

#### payment.payment_operation.completed (success operation)
```json
{
  "type": "payment.payment_operation.completed",
  "id": "payment-123",
  "version": "1.0",
  "timestamp": "2024-01-15T10:30:00Z",
  "payload": {
    "payment_id": "payment-123",
    "type": "wallet_movement",
    "id": "id-wallet-movement-operations",
    "status": "success",
    "amount": {
      "value": 100.00,
      "currency": "USD"
    }
  }
}
```

#### payment.payment_operation.completed (fail operation creation)
```json
{
  "type": "payment.payment_operation.completed",
  "id": "payment-123",
  "version": "1.0",
  "timestamp": "2024-01-15T10:30:00Z",
  "payload": {
    "payment_id": "payment-123",
    "type": "wallet_movement",
    "status": "fail",
    "amount": {
      "value": 100.00,
      "currency": "USD",
      "reason": "wallet timeout"
    }
  }
}
```

#### payment.payment_operation.inconsistent (fail operation creation)
```json
{
  "type": "payment.payment_operation.completed",
  "id": "payment-123",
  "version": "1.0",
  "timestamp": "2024-01-15T10:30:00Z",
  "payload": {
    "payment_id": "payment-123",
    "type": "wallet_movement",
    "id": "id-wallet-movement-operation",
    "amount": {
      "value": 100.00,
      "currency": "USD",
      "reason": "wallet timeout"
    }
  }
}
```

#### payment.success
```json
{
  "type": "payment.payment_operation.completed",
  "id": "payment-123",
  "version": "1.0",
  "timestamp": "2024-01-15T10:30:00Z",
  "payload": {
    "payment_id": "payment-123",
    "user_id": "user-456",
    "status": "success",
    "amount": {
      "value": 100.00,
      "currency": "USD"
    },
    "payment_methods": [
      {
        "type": "wallet",
        "walletId": "wallet-789",
        "value": 100.00
      }
    ],
    "operations": [
      {
        "type": "wallet",
        "id": "asdasd",
        "status": "success",
        "value": 100.00
      }
    ]
  }
}
```

#### payment.failed
```json
{
  "type": "payment.payment_operation.completed",
  "id": "payment-123",
  "version": "1.0",
  "timestamp": "2024-01-15T10:30:00Z",
  "payload": {
    "payment_id": "payment-123",
    "user_id": "user-456",
    "status": "fail",
    "amount": {
      "value": 100.00,
      "currency": "USD"
    },
    "payment_methods": [
      {
        "type": "wallet",
        "walletId": "wallet-789",
        "value": 100.00
      }
    ],
    "operations": [
      {
        "type": "wallet",
        "id": "asdasd",
        "status": "fail",
        "reason": "Invalid CVV",
        "value": 100.00
      }
    ]
  }
}
```

### 2. Wallet Domain Events

#### wallet.movement_required
```json
{
  "type": "wallet.movement_required",
  "id": "asdasd",
  "version": "1.0",
  "timestamp": "2024-01-15T10:30:00Z",
  "payload": {
    "type": "expense",
    "wallet_id": "wall-123",
    "amount": {
      "value": 100.00,
      "currency": "USD"
    },
    "reference": {
      "type": "payment",
      "id": "123-payment-id"
    }
  }
}
```

#### wallet.movement_updates
```json
{
  "type": "wallet.movement_updates",
  "id": "asdasd",
  "version": "1.0",
  "timestamp": "2024-01-15T10:30:00Z",
  "payload": {
    "id": "id-wallet-movement-operations",
    "type": "expense",
    "wallet_id": "wall-123",
    "amount": {
      "value": 100.00,
      "currency": "USD"
    },
    "reference": {
      "type": "payment",
      "id": "123-payment-id"
    }
  }
}
```


#### wallet.balance_update
```json
{
  "type": "wallet.balance_update",
  "id": "asdasd",
  "version": "1.0",
  "timestamp": "2024-01-15T10:30:00Z",
  "payload": {
    "id": "wallet_id",
    "user_id": "123-asd",
    "amount": {
      "value": 100.00,
      "currency": "USD"
    }
  }
}
```