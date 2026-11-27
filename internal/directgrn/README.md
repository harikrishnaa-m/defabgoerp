# DirectGRN PATCH API

This document describes the DirectGRN update endpoint implemented by the
backend. It is intended to be complete enough for frontend integration
without requiring additional clarification.

## Endpoint

```http
PATCH /api/direct-grn/:id
```

The `:id` value is the `grn_id` returned by the DirectGRN POST endpoint or by
the DirectGRN detail endpoint.

Example:

```http
PATCH /api/direct-grn/6c88b07a-2c3f-4b2f-ba28-2757a092ac54
Authorization: Bearer <JWT_TOKEN>
Content-Type: application/json
```

The endpoint is protected and is available to:

- `SuperAdmin`
- `StoreManager`
- `AccountsManager`

## Important update semantics

Although this endpoint uses the HTTP `PATCH` method, the request body is a
complete editable DirectGRN document. It uses the same payload structure as
the DirectGRN POST endpoint.

The frontend should send all required header fields and the complete `items`
and `charges` arrays. The submitted values replace the existing editable
DirectGRN aggregate:

- Purchase order header values are replaced.
- Existing purchase-order items are replaced by the submitted items.
- Existing charges are replaced by the submitted charges.
- Existing payment records are replaced by the submitted payment value.
- Invoice totals and item totals are recalculated.
- Existing inventory created by the original DirectGRN is reversed and the
  submitted inventory is applied.

The existing GRN, purchase order, invoice, GRN number, PO number, and invoice
record IDs remain associated with the document.

The DirectGRN POST implementation is separate and is not changed by this
endpoint.

## GET-to-PATCH round-trip metadata

The frontend must not infer variant or payment behavior from display names or
amounts. The following metadata is returned by
`GET /api/direct-grn/:id` specifically so the response can be used to
construct a faithful PATCH request.

### Item catalog IDs

Each item in the detail response includes:

```json
{
  "category": "Fabric",
  "category_id": "category-uuid",
  "item_name": "Cotton Fabric",
  "product_id": "product-uuid",
  "product_code": "991236"
}
```

- `category_id` identifies the category associated with the item.
- `product_id` identifies the catalog product associated with the item.
- `category` and `item_name` remain available as display fields.
- `product_code` identifies the catalog variant code when the item is a
  product variant.

The backend persists the IDs when the DirectGRN is created or patched. When a
new product or variant is created during the transaction, the backend stores
the actual generated product/category IDs, not only the IDs supplied in the
request.

The frontend should send the returned IDs back unchanged in a subsequent
PATCH request. It should not resolve a display name to a different category or
product unless the user intentionally changes the catalog selection.

### `create_variant`

Each item also includes:

```json
{
  "create_variant": true
}
```

This value records the original inventory mode:

- `true`: the item created a catalog variant and uses product stock.
- `false` with a non-empty `product_code`: the item used an existing active
  catalog variant and uses product stock.
- `false` with an empty `product_code`: the item is treated as raw material
  and uses raw-material stock.

The frontend must preserve this value when editing an item. It must not infer
it from `product_code`, category names, or product names.

When `create_variant` is `true`, `category_id` and `product_id` describe the
catalog context used to create the variant. The PATCH operation uses those
values to recreate the edited variant/stock state after reversing the
previous inventory movements.

PATCH does not create a duplicate variant when the submitted item identifies
an existing active variant:

- A non-empty `product_code` is resolved to the existing active variant.
- When `product_code` is empty but `product_id` is present, the existing
  variant for that product and item name is reused when available.
- A new variant is created only when no matching active variant can be
  resolved.

This allows a GET response to be sent back through PATCH without creating a
second catalog variant on every edit. The previous stock movement is still
reversed and the current movement is recreated for the edited quantity.

### `payment_method`

The top-level detail response includes the payment method:

```json
{
  "paid_amount": 40,
  "payment_method": "UPI",
  "status": "PARTIALLY_PAID"
}
```

The frontend must preserve `payment_method` when preserving a non-zero
`paid_amount`. It must not infer the method from the paid amount or from
`status`.

If `paid_amount` is changed to zero, the PATCH request removes the existing
supplier payment. When `paid_amount` is greater than zero,
`payment_method` must be one of `CASH`, `UPI`, `CARD`, or `BANK_TRANSFER`.

### Recommended frontend edit flow

1. Call `GET /api/direct-grn/:id`.
2. Populate the edit form from the response.
3. Keep each item’s `category_id`, `product_id`, and `create_variant` values in
   form state even if the UI displays category/product names.
4. Keep the top-level `payment_method` in form state separately from
   `paid_amount`.
5. When the user saves, send the complete PATCH payload with those values.
6. Call GET again after a successful PATCH and refresh the form from the
   persisted response.

The frontend should not construct a PATCH request by guessing any of these
three values.

### Metadata migration and older DirectGRNs

The metadata columns are added by migration
`000045_direct_grn_edit_metadata.up.sql`. This migration must be applied
before deploying the updated API:

```sql
ALTER TABLE purchase_order_items
    ADD COLUMN IF NOT EXISTS category_id UUID REFERENCES categories(id),
    ADD COLUMN IF NOT EXISTS product_id UUID REFERENCES products(id),
    ADD COLUMN IF NOT EXISTS create_variant BOOLEAN NOT NULL DEFAULT FALSE;
```

New POST and PATCH operations persist the metadata automatically after the
migration is applied.

DirectGRNs created before this migration may not have complete historical
metadata:

- `category_id` or `product_id` may be empty when the relationship cannot be
  reconstructed.
- `create_variant` defaults to `false` for legacy rows.
- `payment_method` is returned when a historical supplier payment record
  exists; it is empty when no payment record exists.

For a legacy GRN with missing metadata, the frontend should ask the user to
confirm the category/product, inventory mode, and payment method before
saving a PATCH. New GRNs created after the migration do not have this
ambiguity.

## Request headers

```http
Authorization: Bearer <JWT_TOKEN>
Content-Type: application/json
```

## Complete request payload

```json
{
  "supplier_id": "fe2ae11b-ae65-4fe0-b5a8-44e17fe1a310",
  "warehouse_id": "5f124893-8020-4337-a677-e6a636c008a5",
  "purchase_type": "LOCAL_PURCHASE",
  "order_date": "2026-11-25",
  "transport_supplier_id": "fe2ae11b-ae65-4fe0-b5a8-44e17fe1a310",
  "lr_number": "LR-10025",
  "invoice_number": "SUPPLIER-INV-10025",
  "invoice_date": "2026-11-25",
  "tax_inclusive": true,
  "discount_amount": 7.75,
  "round_off": -0.25,
  "notes": "Updated DirectGRN notes",
  "reference": "PURCHASE-REF-10025",
  "payment_method": "UPI",
  "payment_amount": 40.00,
  "items": [
    {
      "item_name": "Cotton Fabric",
      "description": "Updated item description",
      "product_code": "991236",
      "category": "Fabric",
      "category_id": "",
      "product_id": "",
      "create_variant": true,
      "hsn_code": "5007",
      "unit": "MTR",
      "quantity": 3.00,
      "free_qty": 1.00,
      "unit_price": 80.00,
      "selling_price": 125.00,
      "wholesale_price": 105.00,
      "gst_percent": 12.00,
      "additional_work": "Cutting and finishing",
      "additional_work_amount": 14.00,
      "paid_by_user_id": "32106f31-9351-4331-983f-9d577423a86d",
      "paid_to_supplier_id": "fe2ae11b-ae65-4fe0-b5a8-44e17fe1a310",
      "cash_amount": 40.00,
      "credit_amount": 192.00
    }
  ],
  "charges": [
    {
      "charge_type": "FREIGHT",
      "amount": 18.75
    }
  ]
}
```

## Header fields

| Field | Type | Required | Description |
|---|---|---:|---|
| `supplier_id` | string UUID | Yes | Supplier associated with the purchase and invoice. |
| `warehouse_id` | string UUID | Yes | Warehouse receiving the goods. Inventory is applied to this warehouse. |
| `purchase_type` | string | No | Purchase classification, such as `LOCAL_PURCHASE` or `OUTSTATION`. |
| `order_date` | string | No | Purchase order date. Use `YYYY-MM-DD`. If empty, the current server date is used. |
| `transport_supplier_id` | string UUID | No | Supplier used for transport. Send an empty string when not applicable. |
| `lr_number` | string | No | Lorry receipt or transport receipt number. |
| `invoice_number` | string | No | Supplier invoice number. It must be unique. If empty during PATCH, the existing invoice number is preserved. |
| `invoice_date` | string | Yes | Supplier invoice date. Use `YYYY-MM-DD`. |
| `tax_inclusive` | boolean | No | Applies to all submitted items. When `true`, `unit_price` includes GST. |
| `discount_amount` | number | No | Discount deducted from the taxable/subtotal amount. Defaults to zero when omitted. |
| `round_off` | number | No | Round-off adjustment applied to the invoice net amount. |
| `notes` | string | No | Invoice notes. |
| `reference` | string | No | Internal reference used by the GRN and stock movement records. |
| `payment_method` | string | Conditional | Required when `payment_amount` is greater than zero. |
| `payment_amount` | number | No | Payment amount for the supplier invoice. Zero removes the previous payment. |
| `items` | array | Yes | Must contain at least one item. |
| `charges` | array | No | Additional purchase charges. An empty array removes all charges. |

### Supported payment methods

When `payment_amount > 0`, `payment_method` must be one of:

```text
CASH
UPI
CARD
BANK_TRANSFER
```

The value is normalized to uppercase by the API.

## Item fields

| Field | Type | Required | Description |
|---|---|---:|---|
| `item_name` | string | Yes in practice | Item or product name. |
| `description` | string | No | Item description. |
| `product_code` | string | No | Existing variant code, or numeric variant code when creating a variant. Empty means raw material unless `create_variant` is true. |
| `category` | string | No | Category name. Used when a category ID is not supplied or when creating a product/category context. |
| `category_id` | string UUID | No | Existing category to use when creating a variant/product. |
| `product_id` | string UUID | No | Existing product under which a new variant is created. |
| `create_variant` | boolean | No | When `true`, creates a catalog variant and records product stock. |
| `hsn_code` | string | No | HSN/tax classification code. |
| `unit` | string | Yes in practice | Unit such as `PCS`, `MTR`, `KG`, or `BOX`. |
| `quantity` | number | Yes | Must be greater than zero. |
| `free_qty` | number | No | Free/promotional quantity. |
| `unit_price` | number | Yes in practice | Purchase price per unit. |
| `selling_price` | number | No | Selling price stored for a created variant. |
| `wholesale_price` | number | No | Wholesale price stored for a created variant. |
| `gst_percent` | number | No | GST percentage. |
| `additional_work` | string | No | Additional work description. |
| `additional_work_amount` | number | No | Amount associated with additional work. |
| `paid_by_user_id` | string UUID | No | User responsible for the item payment allocation. |
| `paid_to_supplier_id` | string UUID | No | Supplier receiving the item payment allocation. |
| `cash_amount` | number | No | Cash allocation for the item. |
| `credit_amount` | number | No | Credit allocation for the item. |

## Item and variant scenarios

### 1. Existing category and existing product

Use an existing `category_id` and `product_id`:

```json
{
  "item_name": "Existing Product Variant",
  "product_code": "991250",
  "category": "Existing Category",
  "category_id": "existing-category-uuid",
  "product_id": "existing-product-uuid",
  "create_variant": true,
  "hsn_code": "5007",
  "unit": "MTR",
  "quantity": 2,
  "unit_price": 100,
  "selling_price": 140,
  "wholesale_price": 120,
  "gst_percent": 5
}
```

The new variant is created under the supplied existing product.

### 2. Existing category and new product

Use an existing `category_id` and leave `product_id` empty:

```json
{
  "item_name": "New Product Under Existing Category",
  "product_code": "991251",
  "category": "Existing Category",
  "category_id": "existing-category-uuid",
  "product_id": "",
  "create_variant": true,
  "hsn_code": "5007",
  "unit": "PCS",
  "quantity": 5,
  "unit_price": 80,
  "selling_price": 120,
  "wholesale_price": 100,
  "gst_percent": 12
}
```

The backend finds or creates the product under the supplied category, then
creates the variant under that product.

### 3. Category name and new product/category context

When `category_id` and `product_id` are both empty, the backend uses the
category name. If the category does not exist, it is upserted. The product is
then found or created under that category, and the variant is created.

```json
{
  "item_name": "New Product",
  "product_code": "991252",
  "category": "New Category",
  "category_id": "",
  "product_id": "",
  "create_variant": true
}
```

### 4. Existing variant/product-code stock

Set `create_variant` to `false` and provide an existing active
`product_code`:

```json
{
  "item_name": "Existing Variant",
  "product_code": "991253",
  "category": "",
  "category_id": "",
  "product_id": "",
  "create_variant": false,
  "quantity": 4,
  "unit_price": 90,
  "gst_percent": 5,
  "unit": "PCS"
}
```

The product code must match an active catalog variant. The quantity is added
to product stock in the submitted warehouse.

### 5. Raw-material stock

Leave `product_code` empty and set `create_variant` to `false`:

```json
{
  "item_name": "Uncatalogued Raw Material",
  "product_code": "",
  "create_variant": false,
  "hsn_code": "5201",
  "unit": "KG",
  "quantity": 10,
  "unit_price": 50,
  "gst_percent": 5
}
```

The quantity is applied to raw-material stock instead of product variant
stock.

## Charge fields

| Field | Type | Required | Description |
|---|---|---:|---|
| `charge_type` | string | Yes for a stored charge | Charge name, such as `FREIGHT`, `COOLIE`, or `HANDLING`. |
| `amount` | number | Yes for a stored charge | Charge amount. Charges with an amount less than or equal to zero are ignored. |

Charges are stored against the purchase order. Submitting an empty `charges`
array removes the existing charges.

## Amount calculation

For tax-exclusive items:

```text
line_base = quantity × unit_price
gst_amount = line_base × gst_percent / 100
line_total = line_base + gst_amount
```

For tax-inclusive items:

```text
line_total = quantity × unit_price
gst_amount = line_total × gst_percent / (100 + gst_percent)
line_base = line_total - gst_amount
```

Invoice totals are calculated as:

```text
sub_amount = sum(line_base) - discount_amount
net_amount = sub_amount + total_gst_amount + round_off
```

Additional charges are stored separately and do not change the invoice
`net_amount` calculation in the DirectGRN implementation.

## Success response

The endpoint returns HTTP `200 OK` with the following response:

```json
{
  "po_id": "1c0c3fcb-31c9-4cec-aef5-6d3d5c7f7a10",
  "po_number": "PO-20261127101530",
  "grn_id": "6c88b07a-2c3f-4b2f-ba28-2757a092ac54",
  "grn_number": "GRN-125",
  "invoice_id": "3d5b9e8d-4c80-4c0f-b7e7-5e8e4a0b7ad2",
  "invoice_number": "SUPPLIER-INV-10025",
  "net_amount": 232.00
}
```

| Field | Type | Description |
|---|---|---|
| `po_id` | string UUID | Existing purchase order ID. |
| `po_number` | string | Existing purchase order number. |
| `grn_id` | string UUID | DirectGRN ID used in future PATCH and GET requests. |
| `grn_number` | string | Existing GRN number. |
| `invoice_id` | string UUID | Existing purchase invoice ID. |
| `invoice_number` | string | Submitted invoice number, or the previous invoice number when the PATCH request sends an empty value. |
| `net_amount` | number | Recalculated invoice net amount. |

## Detail response for verification

After PATCH, the frontend can verify the persisted document with:

```http
GET /api/direct-grn/:id
```

Example:

```json
{
  "grn_id": "6c88b07a-2c3f-4b2f-ba28-2757a092ac54",
  "grn_number": "GRN-125",
  "po_id": "1c0c3fcb-31c9-4cec-aef5-6d3d5c7f7a10",
  "po_number": "PO-20261127101530",
  "purchase_type": "PATCHED_PURCHASE",
  "supplier_id": "fe2ae11b-ae65-4fe0-b5a8-44e17fe1a310",
  "warehouse_id": "5f124893-8020-4337-a677-e6a636c008a5",
  "transport_supplier_id": "fe2ae11b-ae65-4fe0-b5a8-44e17fe1a310",
  "lr_number": "LR-PATCH-001",
  "invoice_id": "3d5b9e8d-4c80-4c0f-b7e7-5e8e4a0b7ad2",
  "invoice_number": "SUPPLIER-INV-10025",
  "invoice_date": "2026-11-25",
  "sub_amount": 214.29,
  "discount_amount": 7.75,
  "gst_amount": 25.71,
  "round_off": -0.25,
  "net_amount": 232.00,
  "paid_amount": 40.00,
  "status": "PARTIALLY_PAID",
  "notes": "Updated DirectGRN notes",
  "reference": "PURCHASE-REF-10025",
  "items": [
    {
      "id": "8bb7e5cd-cf44-48dd-a36f-6e9d8bd8672a",
      "item_name": "Cotton Fabric",
      "description": "Updated item description",
      "product_code": "991236",
      "category": "Fabric",
      "hsn_code": "5007",
      "unit": "MTR",
      "quantity": 3,
      "free_qty": 1,
      "unit_price": 80,
      "selling_price": 125,
      "wholesale_price": 105,
      "gst_percent": 12,
      "gst_amount": 25.71,
      "total_price": 240.00,
      "tax_inclusive": true,
      "additional_work": "Cutting and finishing",
      "additional_work_amount": 14,
      "paid_by_user_id": "32106f31-9351-4331-983f-9d577423a86d",
      "paid_to_supplier_id": "fe2ae11b-ae65-4fe0-b5a8-44e17fe1a310",
      "cash_amount": 40,
      "credit_amount": 192
    }
  ],
  "charges": [
    {
      "id": "a0b1c2d3-e4f5-6789-abcd-ef0123456789",
      "charge_type": "FREIGHT",
      "amount": 18.75
    }
  ]
}
```

The detail response includes `category_id`, `product_id`, and
`create_variant` for items created after the edit-metadata migration is
applied. It also includes `payment_method` at the invoice level. Older
DirectGRNs created before this metadata was persisted may return empty IDs,
`false` for `create_variant`, or an empty payment method when the historical
value cannot be reconstructed.

## Validation errors

### Invalid JSON

```http
400 Bad Request
```

```json
{
  "error": "Invalid JSON body"
}
```

### Missing required fields

The endpoint returns `400 Bad Request` for:

- Missing `supplier_id`
- Missing `warehouse_id`
- Missing `invoice_date`
- An empty `items` array
- Missing `payment_method` when `payment_amount > 0`
- An unsupported payment method

Example:

```json
{
  "error": "at least one item is required"
}
```

### Invalid item quantity

Every item quantity must be greater than zero:

```json
{
  "error": "quantity for item \"Cotton Fabric\" must be greater than zero"
}
```

### Invalid existing product code

When `create_variant` is `false` and `product_code` is supplied, the active
variant must exist:

```json
{
  "error": "no active product variant found with code 991253: sql: no rows in result set"
}
```

### Payment greater than invoice amount

```json
{
  "error": "payment 500.00 exceeds invoice amount 232.00"
}
```

### Missing GRN

```http
404 Not Found
```

```json
{
  "error": "Direct GRN not found"
}
```

### Duplicate invoice number

```http
409 Conflict
```

```json
{
  "error": "Invoice number already exists"
}
```

## Inventory behavior

PATCH performs inventory reconciliation inside the same database transaction:

1. Product stock movements belonging to the existing purchase order are read.
2. Their quantities are subtracted from product stock.
3. Raw-material movements belonging to the existing purchase order are read.
4. Their quantities are subtracted from raw-material stock.
5. Existing movement records and linked item/charge/payment records are
   replaced.
6. Stock for the submitted items is applied to the submitted warehouse.
7. New product or raw-material movement records are inserted.

If any part of the transaction fails, the transaction is rolled back.

## Accounting behavior

After a successful PATCH, the purchase invoice is sent to the configured
accounting recorder asynchronously, using the same behavior as DirectGRN
POST. The PATCH response is returned without waiting for that asynchronous
accounting operation.

The item-level `cash_amount` and `credit_amount` values should be supplied
consistently with the invoice/payment data to avoid an unbalanced accounting
voucher.

## Postman workflow

1. Authenticate with `POST /api/auth/login`.
2. Store the returned JWT token.
3. Create a DirectGRN with `POST /api/direct-grn`, or use an existing
   `grn_id`.
4. Send the complete editable DirectGRN payload to
   `PATCH /api/direct-grn/:id`.
5. Use `GET /api/direct-grn/:id` to verify the updated values.

