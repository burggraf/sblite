# ShopLite Product Image Upload Design

## Overview

Add file upload capability for product images, storing them in sblite storage with fallback to external URLs.

## Storage Architecture

- **Bucket**: `product-images` (public, 2MB limit, images only)
- **File naming**: `{product-id}.{extension}` - one image per product
- **URL storage**: `image_url` field stores either:
  - Uploaded: `/storage/v1/object/public/product-images/{product-id}.jpg`
  - External: Any valid http(s) URL
  - Placeholder: Default placehold.co URL if neither

## ProductFormModal UI

**Image section (new):**
- File input button: "Choose Image"
- Preview thumbnail of selected/current image
- "Remove" button to clear selection
- File info: filename and size

**Image URL field (kept, demoted):**
- Label: "Or enter Image URL"
- Disabled while file is selected
- Used only if no file uploaded

**Validation:**
- Max 2MB file size
- Allowed: image/jpeg, image/png, image/gif, image/webp

## Delete Flow

When deleting a product:
1. Check if `image_url` contains `/storage/v1/object/public/product-images/`
2. If yes, delete file from storage first
3. Delete product from database
4. Storage delete failure logs warning but doesn't block product deletion

## Migration

New migration creates the bucket:
```sql
INSERT INTO storage_buckets (id, name, public, file_size_limit, allowed_mime_types)
VALUES (
  'product-images',
  'product-images',
  1,
  2097152,
  '["image/jpeg","image/png","image/gif","image/webp"]'
);
```

## Files to Modify

1. **ProductFormModal.jsx** - Add file input, preview, upload state
2. **Home.jsx** - Handle upload in save, delete image on product delete
3. **index.css** - Styles for image upload section
4. **New migration** - Create product-images bucket
