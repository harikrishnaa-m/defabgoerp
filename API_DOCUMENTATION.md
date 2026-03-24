# Defab ERP Backend — API Documentation

**Base URL:** `http://localhost:3000/api`

## Authentication

All endpoints except `/api/auth/*` require a JWT token in the `Authorization` header:

```
Authorization: Bearer <token>
```

## Roles

| Role             | Description                                                             |
| ---------------- | ----------------------------------------------------------------------- |
| SuperAdmin       | Full access to all endpoints                                            |
| InventoryManager | Access to inventory, products, purchases, suppliers                     |
| StoreManager     | Access to stocks, stock requests, raw materials, warehouses (list only) |

---

## Table of Contents

1. [Auth](#1-auth)
2. [Roles](#2-roles)
3. [Branches](#3-branches)
4. [Warehouses](#4-warehouses)
5. [Users](#5-users)
6. [Categories](#6-categories)
7. [Attributes](#7-attributes)
8. [Products](#8-products)
9. [Variants](#9-variants)
10. [Product Descriptions](#10-product-descriptions)
11. [Suppliers](#11-suppliers)
12. [Purchase Orders](#12-purchase-orders)
13. [Goods Receipts](#13-goods-receipts)
14. [Stock Transfers](#14-stock-transfers)
15. [Stocks](#15-stocks)
16. [Stock Requests](#16-stock-requests)
17. [Coupons](#17-coupons)
18. [Raw Material Stocks](#18-raw-material-stocks)
19. [Purchase Invoices & Payments](#19-purchase-invoices--payments)
20. [Utility Endpoints](#20-utility-endpoints)

---

## 1. Auth

**Base Path:** `/api/auth`
**Access:** Public (no authentication required)

### POST `/auth/register`

Create a new user account.

**Request Body:**

```json
{
  "name": "string",
  "email": "string",
  "password": "string",
  "role_id": 1,
  "branch_id": "uuid | null"
}
```

**Response:** `201`

```json
{
  "id": "uuid",
  "name": "string",
  "email": "string",
  "role": { "id": 1, "name": "string" },
  "branch_id": "uuid | null"
}
```

---

### POST `/auth/login`

Authenticate and receive a JWT token.

**Request Body:**

```json
{
  "email": "string",
  "password": "string"
}
```

**Response:** `200`

```json
{
  "token": "jwt_token",
  "user": { "id": "uuid", "name": "string", "email": "string", "role": { ... } }
}
```

**Cookies Set:** `refresh_token` (httpOnly, Secure, SameSite=Strict, 7 days)

---

### POST `/auth/refresh`

Refresh the access token using the refresh_token cookie.

**Response:** `200`

```json
{ "token": "new_jwt_token" }
```

**Cookies:** Rotates `refresh_token`

---

### POST `/auth/logout`

Clear the refresh token cookie.

**Response:** `200` `"logged out"`

---

### POST `/auth/forgot-password`

Send password reset link via email.

**Request Body:**

```json
{ "email": "string" }
```

**Response:** `200` `"Password reset link sent to your email."`

---

### POST `/auth/reset-password`

Reset password using the token from email.

**Request Body:**

```json
{
  "token": "reset_token",
  "new_password": "string"
}
```

**Response:** `200` `"Password reset successful."`

---

## 2. Roles

**Base Path:** `/api/roles`
**Access:** SuperAdmin

### POST `/roles`

Create a new role.

**Request Body:**

```json
{
  "name": "string",
  "permissions": "string"
}
```

**Response:** `201`

---

### GET `/roles`

List all roles.

**Response:** `200`

```json
[{ "id": 1, "name": "string", "permissions": "string" }]
```

---

## 3. Branches

**Base Path:** `/api/branches`
**Access:** SuperAdmin

### POST `/branches`

Create a new branch.

**Request Body:**

```json
{
  "name": "string (required)",
  "address": "string",
  "manager_id": "string",
  "city": "string",
  "state": "string",
  "phone_number": "string"
}
```

**Response:** `201`
**Auto-generated:** `branch_code` (BRxxx)

---

### GET `/branches`

List all branches with manager details.

**Response:** `200` — Array of branches

---

### PATCH `/branches/:id`

Update a branch.

**Request Body:** (all optional)

```json
{
  "name": "string",
  "address": "string",
  "manager_id": "string",
  "phone_number": "string",
  "city": "string",
  "state": "string"
}
```

**Response:** `200` `{ "message": "branch updated" }`

---

### GET `/branches/:id`

Get branch details.

**Response:** `200` — Branch object

---

## 4. Warehouses

**Base Path:** `/api/warehouses`
**Access:** CRUD = SuperAdmin | List = SuperAdmin, InventoryManager, StoreManager

### POST `/warehouses`

Create a new warehouse. _SuperAdmin only._

**Request Body:**

```json
{
  "branch_id": "uuid (required)",
  "name": "string (required)",
  "type": "STORE | CENTRAL | FACTORY (optional, default: STORE)"
}
```

**Response:** `201`
**Auto-generated:** `warehouse_code` (WHxxx)

---

### GET `/warehouses`

List all warehouses with branch details. _All roles._

**Response:** `200` — Array of warehouses

---

### GET `/warehouses/:id`

Get warehouse details. _SuperAdmin only._

**Response:** `200` — Warehouse with branch details

---

### PATCH `/warehouses/:id`

Update a warehouse. _SuperAdmin only._

**Request Body:** (all optional)

```json
{
  "branch_id": "string",
  "name": "string",
  "type": "string"
}
```

---

### DELETE `/warehouses/:id`

Delete a warehouse. _SuperAdmin only._

---

## 5. Users

**Base Path:** `/api/users`
**Access:** SuperAdmin

### POST `/users`

Create a new user.

**Request Body:**

```json
{
  "name": "string",
  "email": "string",
  "password": "string",
  "role_id": 1,
  "branch_id": "uuid | null"
}
```

**Response:** `201` — User object (without password hash)

---

### GET `/users`

List users (paginated).

**Query Params:** `page` (default 1), `limit` (default 10)

**Response:** `200`

```json
{
  "data": [{ "id": "uuid", "name": "string", "email": "string", "role": {...}, "branch_id": "uuid", "is_active": true, "created_at": "timestamp" }],
  "page": 1,
  "limit": 10,
  "total": 25
}
```

---

### GET `/users/:id`

Get user details with role.

---

### PATCH `/users/:id`

Update a user.

**Request Body:** (all optional)

```json
{
  "name": "string",
  "role_id": 1,
  "branch_id": "string",
  "is_active": true
}
```

---

### PATCH `/users/:id/deactivate`

Deactivate a user.

### PATCH `/users/:id/activate`

Activate a user.

---

## 6. Categories

**Base Path:** `/api/categories`
**Access:** SuperAdmin, InventoryManager

### POST `/categories`

**Request Body:** `{ "name": "string (required)" }`

**Response:** `201`

---

### GET `/categories`

**Query Params:** `page` (default 1), `limit` (default 20)

**Response:** `200`

```json
{
  "data": [
    { "id": "uuid", "name": "string", "is_active": true, "products_count": 5 }
  ],
  "page": 1,
  "limit": 20,
  "total": 10
}
```

---

### GET `/categories/:id`

### PATCH `/categories/:id`

**Request Body:** `{ "name": "string" }`

### PATCH `/categories/:id/deactivate`

### PATCH `/categories/:id/activate`

---

## 7. Attributes

**Base Path:** `/api/attributes`
**Access:** SuperAdmin, InventoryManager

### POST `/attributes`

**Request Body:** `{ "name": "string (required)" }`

**Response:** `201` `{ "message": "attribute created" }`

---

### GET `/attributes`

**Response:** `200` `[{ "id": "uuid", "name": "string", "is_active": true }]`

---

### PATCH `/attributes/:id`

**Request Body:** `{ "name": "string" }`

### PATCH `/attributes/:id/deactivate`

### PATCH `/attributes/:id/activate`

---

### POST `/attributes/values`

Create an attribute value.

**Request Body:**

```json
{
  "attribute_id": "uuid (required)",
  "value": "string (required)"
}
```

**Response:** `201` `{ "message": "attribute value created" }`

---

### GET `/attributes/:id/values`

Get values for an attribute.

**Response:** `200` `[{ "id": "uuid", "value": "string", "is_active": true }]`

---

### PATCH `/attributes/values/:id`

**Request Body:** `{ "value": "string" }`

### PATCH `/attributes/values/:id/deactivate`

### PATCH `/attributes/values/:id/activate`

---

## 8. Products

**Base Path:** `/api/products`
**Access:** SuperAdmin, InventoryManager

### POST `/products`

**Content-Type:** `multipart/form-data`

**Form Fields:**
| Field | Type | Required |
|-------|------|----------|
| name | string | Yes |
| category_id | uuid | Yes |
| brand | string | No |
| description | string | No |
| fabric_composition | string | No |
| pattern | string | No |
| occasion | string | No |
| care_instructions | string | No |
| main_image | file | Yes |
| gallery_images | file[] | No |

**Response:** `201`

```json
{ "id": "uuid", "message": "product created" }
```

---

### GET `/products`

**Query Params:** `page` (default 1), `limit` (default 20)

**Response:** `200` — Paginated products with category and gallery

---

### GET `/products/:id`

**Response:** `200` — Product with gallery array `[{ "id": "uuid", "url": "string" }]`

---

### PATCH `/products/:id`

**Content-Type:** `application/json`

**Request Body:** (all optional)

```json
{
  "name": "string",
  "category_id": "uuid",
  "brand": "string",
  "description": "string",
  "fabric_composition": "string",
  "pattern": "string",
  "occasion": "string",
  "care_instructions": "string",
  "is_web_visible": true,
  "is_stitched": false,
  "uom": "string"
}
```

---

### PATCH `/products/:id/deactivate`

### PATCH `/products/:id/activate`

---

### PUT `/products/:id/main-image`

**Content-Type:** `multipart/form-data`
**Form Fields:** `main_image` (file, required)

---

### POST `/products/:id/images`

Upload gallery images.

**Content-Type:** `multipart/form-data`
**Form Fields:** `gallery_images` (file[], multiple)

---

### DELETE `/products/images/:imageId`

Delete a gallery image.

---

## 9. Variants

**Base Path:** `/api/variants`
**Access:** SuperAdmin, InventoryManager

### POST `/variants`

**Content-Type:** `multipart/form-data`

**Form Fields:**
| Field | Type | Required |
|-------|------|----------|
| product_id | uuid | Yes |
| name | string | Yes |
| price | number | Yes |
| cost_price | number | Yes |
| attribute_value_ids[] | uuid[] | No |
| images | file[] | No |

**Response:** `201`

```json
{ "message": "variant created", "id": "uuid", "sku": "VRxxx" }
```

**Auto-generated:** SKU (VRxxx)

---

### POST `/variants/generate`

Bulk generate variants from attribute combinations (cartesian product).

**Request Body:**

```json
{
  "product_id": "uuid",
  "base_price": 1500,
  "attribute_values": {
    "attr_id_1": ["value_id_1", "value_id_2"],
    "attr_id_2": ["value_id_3", "value_id_4"]
  }
}
```

**Response:** `200` `{ "message": "variants generated", "count": 4 }`

---

### GET `/variants/product/:productId`

List variants for a product.

**Response:** `200` `[{ "id": "uuid", "name": "string", "sku": "string", "price": 1500, "cost_price": 800, "is_active": true }]`

---

### GET `/variants/:id`

Get variant details with images and attributes.

**Response:** `200`

```json
{
  "id": "uuid",
  "product_id": "uuid",
  "name": "string",
  "sku": "string",
  "price": 1500,
  "cost_price": 800,
  "is_active": true,
  "images": [{ "id": "uuid", "url": "string" }],
  "attributes": [...]
}
```

---

### PATCH `/variants/:id`

**Request Body:** (all optional)

```json
{
  "name": "string",
  "price": 1500,
  "cost_price": 800,
  "attribute_value_ids[]": ["uuid"]
}
```

---

### PATCH `/variants/:id/deactivate`

### PATCH `/variants/:id/activate`

---

### POST `/variants/:id/images`

**Content-Type:** `multipart/form-data`
**Form Fields:** `images` (file[], multiple)

---

### GET `/variants/:id/images`

**Response:** `200` `[{ "id": "uuid", "url": "string", "created": "timestamp" }]`

---

### DELETE `/variants/images/:imageId`

Delete a variant image.

---

## 10. Product Descriptions

**Base Path:** `/api/product-descriptions`
**Access:** SuperAdmin

### POST `/product-descriptions`

**Content-Type:** `multipart/form-data`

**Form Fields:**
| Field | Type | Required |
|-------|------|----------|
| product_id | uuid | Yes |
| description | string | No |
| fabric_composition | string | No |
| pattern | string | No |
| occasion | string | No |
| care_instructions | string | No |
| length | number | No |
| width | number | No |
| blouse_piece | number | No |
| size_chart_image | file | No |

**Response:** `201`

---

### GET `/product-descriptions/:productId`

**Response:** `200`

```json
{
  "id": "uuid",
  "product_id": "uuid",
  "description": "string",
  "fabric_composition": "string",
  "pattern": "string",
  "occasion": "string",
  "care_instructions": "string",
  "length": 5.5,
  "width": 1.2,
  "blouse_piece": 0.8,
  "size_chart_image": "url",
  "created_at": "timestamp",
  "updated_at": "timestamp"
}
```

---

### PATCH `/product-descriptions/:productId`

**Content-Type:** `multipart/form-data`
**Form Fields:** Same as POST (all optional)

---

## 11. Suppliers

**Base Path:** `/api/suppliers`
**Access:** SuperAdmin, InventoryManager

### POST `/suppliers`

**Request Body:**

```json
{
  "name": "string (required)",
  "phone": "string",
  "email": "string",
  "address": "string",
  "gst_number": "string"
}
```

**Response:** `201`

```json
{ "id": "uuid", "supplier_code": "SUPxxx", "message": "..." }
```

**Auto-generated:** `supplier_code` (SUPxxx)
**Errors:** `409` if GST number already exists

---

### GET `/suppliers`

**Query Params:** `page` (default 1), `limit` (default 20)

**Response:** `200` — Paginated suppliers

---

### GET `/suppliers/:id`

### PATCH `/suppliers/:id`

**Request Body:** (all optional)

```json
{
  "name": "string",
  "phone": "string",
  "email": "string",
  "address": "string",
  "gst_number": "string"
}
```

### PATCH `/suppliers/:id/deactivate`

### PATCH `/suppliers/:id/activate`

---

## 12. Purchase Orders

**Base Path:** `/api/purchase-orders`
**Access:** SuperAdmin, InventoryManager

### POST `/purchase-orders`

**Request Body:**

```json
{
  "supplier_id": "uuid (required)",
  "warehouse_id": "uuid (required)",
  "order_date": "YYYY-MM-DD",
  "expected_date": "YYYY-MM-DD",
  "items": [
    {
      "item_name": "string (required)",
      "description": "string",
      "hsn_code": "string",
      "unit": "METER | KG | PCS | ROLL (required)",
      "quantity": 100,
      "unit_price": 50.0,
      "gst_percent": 18
    }
  ]
}
```

**Response:** `201`
**Auto-generated:** `po_number` (POxxx)

---

### GET `/purchase-orders`

**Query Params:** `page` (default 1), `limit` (default 20), `status`, `supplier_id`, `from_date`, `to_date`

**Response:** `200` — Paginated purchase orders with supplier details

---

### GET `/purchase-orders/:id`

**Response:** `200` — PO detail with items

---

### PATCH `/purchase-orders/:id`

**Request Body:** (all optional)

```json
{
  "supplier_id": "uuid",
  "warehouse_id": "uuid",
  "order_date": "YYYY-MM-DD",
  "expected_date": "YYYY-MM-DD",
  "status": "string"
}
```

---

### DELETE `/purchase-orders/:id`

Cancel a purchase order.

---

### POST `/purchase-orders/:id/items`

Add item to existing PO.

**Request Body:**

```json
{
  "item_name": "string (required)",
  "description": "string",
  "hsn_code": "string",
  "unit": "string (required)",
  "quantity": 100,
  "unit_price": 50.0,
  "gst_percent": 18
}
```

**Response:** `201` `{ "id": "uuid", "message": "Item added to purchase order" }`

---

### PATCH `/purchase-orders/:id/items/:itemId`

Update a PO item. (Same fields as POST, all optional)

---

### DELETE `/purchase-orders/:id/items/:itemId`

Delete a PO item.

---

## 13. Goods Receipts

**Base Path:** `/api/goods-receipts`
**Access:** SuperAdmin, InventoryManager

### POST `/goods-receipts`

Create a Goods Receipt Note (GRN).

**Request Body:**

```json
{
  "purchase_order_id": "uuid (required)",
  "reference": "string (optional, e.g. invoice/DC number)",
  "items": [
    {
      "purchase_order_item_id": "uuid (required)",
      "received_qty": 50
    }
  ]
}
```

**Response:** `201` — GRN object
**Auto-generated:** `grn_number`
**Side Effects:** Auto-creates stock in destination warehouse + stock movements

---

### GET `/goods-receipts`

List all GRNs.

---

### GET `/goods-receipts/:id`

GRN detail with items.

---

### GET `/goods-receipts/po/:poId`

All GRNs for a specific purchase order.

---

### DELETE `/goods-receipts/:id`

Cancel a GRN. Reverses stock movements.

**Response:** `200` `{ "message": "GRN cancelled and stock reversed" }`
**Errors:** `400` if already cancelled

---

## 14. Stock Transfers

**Base Path:** `/api/stock-transfers`
**Access:** SuperAdmin, InventoryManager

### POST `/stock-transfers`

Transfer stock between warehouses.

**Request Body:**

```json
{
  "from_warehouse_id": "uuid (required)",
  "to_warehouse_id": "uuid (required)",
  "reference": "string (optional)",
  "items": [
    {
      "variant_id": "uuid (required)",
      "quantity": 10
    }
  ]
}
```

**Response:** `201` `{ "message": "Stock transferred successfully" }`
**Side Effects:** Creates IN_TRANSIT movement, deducts from source warehouse
**Errors:** `409` insufficient stock, `404` stock not found

---

### POST `/stock-transfers/:id/receive`

Confirm receipt of transferred stock.

**Request Body:**

```json
{
  "received_qty": "decimal",
  "remarks": "string (optional)"
}
```

**Response:** `200` `{ "message": "Stock received successfully" }`
**Side Effects:** Adds stock to destination warehouse, completes movement

---

## 15. Stocks

**Base Path:** `/api/stocks`
**Access:** SuperAdmin, InventoryManager, StoreManager

> **Note:** StoreManagers are automatically filtered to see only their branch's data.

### POST `/stocks`

Create or upsert stock.

**Request Body:**

```json
{
  "variant_id": "uuid (required)",
  "warehouse_id": "uuid (required)",
  "quantity": 100,
  "stock_type": "PRODUCT | RAW_MATERIAL (optional, default: PRODUCT)"
}
```

**Response:** `201` `{ "message": "stock created", "id": "uuid" }`

---

### GET `/stocks`

List all stocks (paginated).

**Query Params:** `page` (default 1), `limit` (default 20)

**Response:** `200`

```json
{
  "data": [
    {
      "id": "uuid",
      "variant_id": "uuid",
      "variant_name": "string",
      "product_name": "string",
      "warehouse_id": "uuid",
      "warehouse_name": "string",
      "quantity": 100,
      "stock_type": "string"
    }
  ],
  "page": 1,
  "limit": 20,
  "total": 50
}
```

**Role Behavior:** StoreManagers see only their branch stocks.

---

### GET `/stocks/low`

Low stock alert.

**Query Params:** `threshold` (default 10)

**Response:** `200` — Items below threshold
**Role Behavior:** StoreManagers see only their branch.

---

### GET `/stocks/movements`

Movement audit log (paginated, filterable).

**Query Params:** `page` (default 1), `limit` (default 20), `variant_id`, `warehouse_id`, `type` (e.g. TRANSFER, ADJUSTMENT), `from_date`, `to_date`

**Response:** `200`

```json
{
  "data": [
    {
      "id": "uuid",
      "variant_id": "uuid",
      "variant_name": "string",
      "product_id": "uuid",
      "product_name": "string",
      "type": "TRANSFER",
      "quantity": 10,
      "from_warehouse_id": "uuid",
      "from_warehouse_name": "string",
      "to_warehouse_id": "uuid",
      "to_warehouse_name": "string",
      "reference": "string",
      "status": "IN_TRANSIT | RECEIVED | COMPLETED",
      "created_at": "timestamp"
    }
  ],
  "page": 1,
  "limit": 20,
  "total": 100
}
```

**Role Behavior:** StoreManagers automatically see only their branch movements (branch_id from token).

---

### GET `/stocks/movements/branch`

Movements for a specific branch. _Admin use._

**Query Params:** `branch_id` (required), `type`, `from_date`, `to_date`, `page`, `limit`

---

### GET `/stocks/movements/:id`

Single movement detail.

---

### GET `/stocks/available`

Stocks available in central warehouse / other branches.

**Query Params:** `page` (default 1), `limit` (default 20)

**Role Behavior:** StoreManagers see central + other branches' stocks.

---

### GET `/stocks/available/new`

Central warehouse stocks NOT present in user's branch.

**Query Params:** `page` (default 1), `limit` (default 20)

---

### GET `/stocks/warehouse/:id`

Stocks in a specific warehouse (paginated).

**Query Params:** `page` (default 1), `limit` (default 20)

---

### GET `/stocks/warehouse/:id/products`

Product summary for a warehouse.

---

### GET `/stocks/branch/:id`

Stocks in a branch (all its warehouses, paginated).

**Query Params:** `page` (default 1), `limit` (default 20)

---

### GET `/stocks/variant/:id`

Stock for a variant across all warehouses.

---

### GET `/stocks/product/:id`

Total stock per variant for a product.

---

### GET `/stocks/:id`

Single stock record detail.

---

### PATCH `/stocks/:id`

Raw stock update.

**Request Body:**

```json
{
  "variant_id": "uuid (required)",
  "warehouse_id": "uuid (required)",
  "quantity": 100,
  "stock_type": "string"
}
```

---

### POST `/stocks/:id/adjust`

Audited stock adjustment with movement record.

**Request Body:**

```json
{
  "new_quantity": 95,
  "reason": "string (required)"
}
```

**Response:** `200` `{ "message": "stock adjusted", "id": "uuid" }`
**Side Effects:** Creates a movement record with user_id for audit trail.

---

### DELETE `/stocks/:id`

Delete a stock record.

---

## 16. Stock Requests

**Base Path:** `/api/stock-requests`
**Access:** SuperAdmin, InventoryManager, StoreManager

### Status Flow

```
PENDING → APPROVED → PARTIAL_DISPATCH → FULL_DISPATCH → COMPLETED
              ↓              ↓
          REJECTED       CANCELLED
              ↓
          CANCELLED
```

| Status           | Description                                       |
| ---------------- | ------------------------------------------------- |
| PENDING          | Newly created, awaiting decision                  |
| APPROVED         | Approved by admin, ready for dispatch             |
| REJECTED         | Rejected by admin (blocked if already dispatched) |
| CANCELLED        | Cancelled                                         |
| PARTIAL_DISPATCH | Some items dispatched, more remain                |
| FULL_DISPATCH    | All items dispatched, awaiting receipt            |
| COMPLETED        | StoreManager has received all dispatched stock    |

---

### POST `/stock-requests`

Create a stock request. Warehouses are auto-fetched from the system.

**Request Body:**

```json
{
  "priority": "LOW | MEDIUM | HIGH | URGENT (optional, default: MEDIUM)",
  "expected_date": "YYYY-MM-DD (optional)",
  "items": [
    {
      "variant_id": "uuid (required)",
      "qty": 10
    }
  ]
}
```

**Response:** `201`

```json
{ "id": "uuid", "message": "Stock request created" }
```

**Auto-fetched:** `from_warehouse_id` (CENTRAL warehouse), `to_warehouse_id` (user's branch warehouse)
**Requirement:** User must have a branch assigned.

---

### GET `/stock-requests`

List stock requests (paginated, filterable).

**Query Params:** `page` (default 1), `limit` (default 20), `status`, `from_date`, `to_date`

**Response:** `200`

```json
{
  "page": 1,
  "limit": 20,
  "total": 45,
  "total_pages": 3,
  "data": [
    {
      "id": "uuid",
      "status": "PENDING",
      "priority": "MEDIUM",
      "from_warehouse_id": "uuid",
      "from_warehouse_name": "string",
      "to_warehouse_id": "uuid",
      "to_warehouse_name": "string",
      "requested_by": "uuid",
      "requested_by_name": "string",
      "expected_date": "YYYY-MM-DD",
      "created_at": "timestamp"
    }
  ]
}
```

**Role Behavior:** StoreManagers see only their branch's requests.

**Filter Examples:**

- `GET /stock-requests?page=1&limit=20` — all
- `GET /stock-requests?page=1&limit=20&status=PENDING`
- `GET /stock-requests?page=1&limit=20&status=PARTIAL_DISPATCH`
- `GET /stock-requests?status=APPROVED&from_date=2026-01-01&to_date=2026-03-21`

---

### GET `/stock-requests/branch`

List requests for a specific branch. _Admin use._

**Query Params:** `branch_id` (required), `page`, `limit`, `status`, `from_date`, `to_date`

---

### GET `/stock-requests/:id`

Get request detail with items.

**Response:** `200`

```json
{
  "id": "uuid",
  "status": "APPROVED",
  "priority": "HIGH",
  "from_warehouse_id": "uuid",
  "from_warehouse_name": "string",
  "to_warehouse_id": "uuid",
  "to_warehouse_name": "string",
  "requested_by": "uuid",
  "requested_by_name": "string",
  "expected_date": "2026-04-01",
  "created_at": "timestamp",
  "items": [
    {
      "id": "uuid",
      "variant_id": "uuid",
      "variant_name": "string",
      "requested_qty": 10,
      "approved_qty": 5
    }
  ]
}
```

---

### PATCH `/stock-requests/:id/decision`

Approve or reject a stock request.

**Request Body:**

```json
{
  "status": "APPROVED | PARTIAL | REJECTED | CANCELLED (required)",
  "remarks": "string (optional)"
}
```

**Response:** `200` `{ "message": "status updated" }`

**Errors:**

- `400` — Invalid status transition
- `400` — Cannot reject after stock has been dispatched (approved_qty > 0)

---

### DELETE `/stock-requests/:id`

Cancel a stock request (sets status to CANCELLED).

---

### POST `/stock-requests/:id/dispatch`

Dispatch stock from the central warehouse.

**Request Body:**

```json
{
  "from_warehouse_id": "uuid (required)",
  "items": [
    {
      "variant_id": "uuid (required)",
      "dispatch_qty": 5
    }
  ],
  "remarks": "string (optional)"
}
```

**Response:** `200` `{ "message": "Stock dispatched successfully" }`

**Side Effects:**

- Deducts stock from source warehouse
- Creates IN_TRANSIT movement(s)
- Sets status to `PARTIAL_DISPATCH` or `FULL_DISPATCH`

**Errors:**

- `400` `"dispatch qty exceeds remaining for variant <id> (remaining <N>)"`
- `400` `"insufficient stock for variant <id>"`
- `400` `"stock request already closed"`

---

### POST `/stock-requests/:id/receive`

StoreManager confirms receipt of dispatched stock.

**Request Body:** None

**Response:** `200` `{ "message": "Stock received successfully" }`

**Side Effects:**

- Upserts stock into branch warehouse (FINISHED_GOOD)
- Marks IN_TRANSIT movements as RECEIVED
- If all movements received and status was `FULL_DISPATCH` → sets status to `COMPLETED`

**Errors:**

- `400` `"no dispatched stock to receive"`
- `400` `"no in-transit stock to receive"`

---

## 17. Coupons

**Base Path:** `/api/coupons`
**Access:** SuperAdmin, InventoryManager

### POST `/coupons`

**Request Body:**

```json
{
  "code": "string (required)",
  "description": "string",
  "discount_type": "FLAT | PERCENT (required)",
  "discount_value": 100,
  "min_order_value": 500,
  "max_discount_amount": 200,
  "start_date": "ISO timestamp",
  "end_date": "ISO timestamp",
  "usage_limit": 100,
  "usage_per_customer": 1
}
```

**Response:** `201` `{ "id": "uuid", "message": "Coupon created" }`

---

### GET `/coupons`

**Query Params:** `page` (default 1), `limit` (default 20)

**Response:** `200` — Paginated coupons

---

### GET `/coupons/:id`

### PATCH `/coupons/:id`

**Request Body:** (all optional, same fields as POST)

### PATCH `/coupons/:id/activate`

### PATCH `/coupons/:id/deactivate`

---

### POST `/coupons/:id/variants`

Map variants to a coupon.

**Request Body:** `{ "variant_ids": ["uuid", ...] }`

---

### POST `/coupons/:id/categories`

Map categories to a coupon.

**Request Body:** `{ "category_ids": ["uuid", ...] }`

---

### DELETE `/coupons/variants/:mappingId`

Remove variant mapping.

### DELETE `/coupons/categories/:mappingId`

Remove category mapping.

---

## 18. Raw Material Stocks

**Base Path:** `/api/raw-material-stocks`
**Access:** SuperAdmin, InventoryManager, StoreManager

> **Note:** StoreManagers are automatically filtered to their branch.

### GET `/raw-material-stocks`

List all raw material stocks.

**Query Params:** `limit` (default 50), `offset` (default 0)

**Role Behavior:** StoreManagers see only their branch.

---

### GET `/raw-material-stocks/warehouse/:warehouseId`

Raw materials in a specific warehouse.

**Query Params:** `limit` (default 50), `offset` (default 0)

---

### GET `/raw-material-stocks/movements`

Raw material movement history.

**Query Params:** `limit` (default 50), `offset` (default 0), `stock_id`, `item_name`, `warehouse_id`

**Role Behavior:** StoreManagers see only their branch.

---

### GET `/raw-material-stocks/movements/:id`

Single movement detail.

---

### GET `/raw-material-stocks/movements/branch`

Movements for a branch. _Admin use._

**Query Params:** `branch_id` (required), `limit` (default 50), `offset` (default 0)

---

### GET `/raw-material-stocks/branch`

Raw materials in a branch.

**Query Params:** `branch_id` (required), `limit` (default 50), `offset` (default 0)

---

### POST `/raw-material-stocks/adjust`

Adjust raw material stock.

**Request Body:**

```json
{
  "stock_id": "uuid (required)",
  "quantity": 10,
  "type": "OUT | ADJUSTMENT (required)",
  "reference": "string (optional, e.g. 'Used for stitching batch #12')"
}
```

**Response:** `200` `{ "message": "Stock adjusted successfully" }`

---

## 19. Purchase Invoices & Payments

### Purchase Invoices

**Base Path:** `/api/purchase-invoices`
**Access:** SuperAdmin, InventoryManager

#### POST `/purchase-invoices`

**Request Body:**

```json
{
  "purchase_order_id": "uuid (required)",
  "invoice_number": "string",
  "invoice_date": "YYYY-MM-DD (required)",
  "discount_amount": 0,
  "round_off": 0,
  "notes": "string",
  "payment_method": "CASH | UPI | CARD | BANK_TRANSFER (optional)",
  "payment_amount": 5000,
  "reference": "string (optional)"
}
```

**Response:** `201` — Invoice object
**Auto-generated:** `invoice_number` prefix (INVxxx)
**Default Status:** `PENDING`

---

#### GET `/purchase-invoices`

List all invoices.

---

#### GET `/purchase-invoices/:id`

Invoice detail.

---

#### POST `/purchase-invoices/:id/payments`

Record a payment against an invoice.

**Request Body:**

```json
{
  "amount": 5000,
  "payment_method": "CASH | UPI | CARD | BANK_TRANSFER (required)",
  "reference": "string (optional)",
  "paid_at": "YYYY-MM-DD HH:MM:SS (optional, default: NOW)"
}
```

**Response:** `200` — Updated invoice object
**Errors:** `400` if amount exceeds balance or invoice is cancelled

---

#### DELETE `/purchase-invoices/:id`

Cancel an invoice.

**Response:** `200` `{ "message": "Purchase invoice cancelled" }`
**Errors:** `400` if not in PENDING state

---

### Supplier Payments

**Base Path:** `/api/supplier-payments`
**Access:** SuperAdmin, InventoryManager

#### GET `/supplier-payments`

List all payments across all invoices.

---

#### GET `/supplier-payments/outstanding`

Summary of outstanding/unpaid invoices.

---

#### GET `/supplier-payments/supplier/:supplierId`

Payments for a specific supplier.

---

## 20. Utility Endpoints

### GET `/api/me`

Get current authenticated user's profile.

**Access:** All authenticated users
**Response:** `200` — User object from JWT

---

### GET `/api/admin-only`

Test endpoint.

**Access:** SuperAdmin
**Response:** `200` `"Hello SuperAdmin"`

---

## Role Access Summary

| Module               | SuperAdmin | InventoryManager |  StoreManager   |
| -------------------- | :--------: | :--------------: | :-------------: |
| Auth                 |   Public   |      Public      |     Public      |
| Roles                |     ✓      |        ✗         |        ✗        |
| Branches             |     ✓      |        ✗         |        ✗        |
| Warehouses (CRUD)    |     ✓      |        ✗         |        ✗        |
| Warehouses (List)    |     ✓      |        ✓         |        ✓        |
| Users                |     ✓      |        ✗         |        ✗        |
| Categories           |     ✓      |        ✓         |        ✗        |
| Attributes           |     ✓      |        ✓         |        ✗        |
| Products             |     ✓      |        ✓         |        ✗        |
| Variants             |     ✓      |        ✓         |        ✗        |
| Product Descriptions |     ✓      |        ✗         |        ✗        |
| Suppliers            |     ✓      |        ✓         |        ✗        |
| Purchase Orders      |     ✓      |        ✓         |        ✗        |
| Goods Receipts       |     ✓      |        ✓         |        ✗        |
| Stock Transfers      |     ✓      |        ✓         |        ✗        |
| Stocks               |     ✓      |        ✓         | ✓ (branch only) |
| Stock Requests       |     ✓      |        ✓         | ✓ (branch only) |
| Coupons              |     ✓      |        ✓         |        ✗        |
| Raw Material Stocks  |     ✓      |        ✓         | ✓ (branch only) |
| Purchase Invoices    |     ✓      |        ✓         |        ✗        |
| Supplier Payments    |     ✓      |        ✓         |        ✗        |

---

## Error Response Format

All errors follow this format:

```json
{ "error": "error message" }
```

Common HTTP Status Codes:

- `200` — Success
- `201` — Created
- `400` — Bad Request (validation errors)
- `401` — Unauthorized (missing/invalid token)
- `403` — Forbidden (insufficient role)
- `404` — Not Found
- `409` — Conflict (duplicate)
- `500` — Internal Server Error
