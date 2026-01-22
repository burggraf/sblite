# ShopLite Admin Product Management Design

## Overview

Add admin product management to ShopLite, allowing admin users to add, edit, and delete products directly from the Home page.

## UI Approach

**Inline on Home page** - Admin controls integrated into existing product display:

1. **"Add Product" button** in page header next to "Our Products" heading
2. **Edit/Delete buttons** on each ProductCard below the Add to Cart button
3. All admin controls hidden for non-admin users

## Product Form Modal

Shared modal for Add and Edit operations.

**Fields (in order):**
- Name (text, required)
- Description (textarea, optional)
- Price (number, required, min 0, step 0.01)
- Image URL (text, optional)
- Stock (number, required, min 0, integer)

**Behavior:**
- Title: "Add Product" or "Edit Product"
- Pre-populated when editing
- Client-side validation before submit
- Loading state during API call
- Error message displayed inline on failure
- On success: close modal, refresh product list

## Delete Confirmation Modal

- Displays: "Delete [Product Name]?"
- Message: "This action cannot be undone."
- Buttons: Cancel (secondary), Delete (danger/red)
- Loading state during API call
- On success: close modal, remove from list

## Database Changes

Add RLS policies for admin write access to products table:

```sql
-- Admin can INSERT products
CREATE POLICY products_admin_insert ON products FOR INSERT
  USING (EXISTS (SELECT 1 FROM user_roles WHERE user_id = auth.uid() AND role = 'admin'));

-- Admin can UPDATE products
CREATE POLICY products_admin_update ON products FOR UPDATE
  USING (EXISTS (SELECT 1 FROM user_roles WHERE user_id = auth.uid() AND role = 'admin'));

-- Admin can DELETE products
CREATE POLICY products_admin_delete ON products FOR DELETE
  USING (EXISTS (SELECT 1 FROM user_roles WHERE user_id = auth.uid() AND role = 'admin'));
```

## Component Structure

**New components:**

1. **ProductFormModal.jsx**
   - Props: `isOpen`, `onClose`, `product`, `onSave`
   - Handles form state, validation, submit

2. **DeleteConfirmModal.jsx**
   - Props: `isOpen`, `onClose`, `product`, `onConfirm`

**Modified components:**

1. **Home.jsx**
   - Add "Add Product" button (admin only)
   - Manage modal state
   - Handle add/edit/delete operations

2. **ProductCard.jsx**
   - Accept `isAdmin`, `onEdit`, `onDelete` props
   - Render Edit/Delete buttons when admin

## Data Flow

- Home.jsx owns product state and modal state
- ProductCard triggers callbacks
- Modals receive data as props, return results via callbacks
- Re-fetch products after any change
