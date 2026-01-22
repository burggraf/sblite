-- Create product-images bucket for storing product photos
-- Public bucket: anyone can view images
-- 2MB limit, images only
INSERT INTO storage_buckets (id, name, public, file_size_limit, allowed_mime_types)
VALUES (
  'product-images',
  'product-images',
  1,
  2097152,
  '["image/jpeg","image/png","image/gif","image/webp"]'
);
